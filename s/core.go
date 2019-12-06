package s

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

// rootSupervisorName is the name the root supervisor has, this is used to
// compare the process current name to the rootSupervisorName
var rootSupervisorName = ""

////////////////////////////////////////////////////////////////////////////////
// Supervisor (dynamic tree) functionality

// Stop is a synchronous procedure that halts the execution of the whole
// supervision tree.
func (sup Supervisor) Stop() error {
	stopTime := time.Now()
	sup.cancel()
	err := sup.wait()
	sup.spec.getEventNotifier().ProcessStopped(sup.runtimeName, stopTime, err)
	return err
}

// Wait blocks the execution of the current goroutine until the Supervisor
// finishes it execution.
func (sup Supervisor) Wait() error {
	return sup.wait()
}

// Name returns the name of the Spec used to start this Supervisor
func (sup Supervisor) Name() string {
	return sup.spec.Name()
}

////////////////////////////////////////////////////////////////////////////////
// Spec (static tree) functionality

// emptyEventNotifier is an utility function that works as a default value
// whenever an EventNotifier is not specified on the Supervisor Spec
func emptyEventNotifier(_ Event) {}

// getEventNotifier returns the configured EventNotifier or emptyEventNotifier
// (if none is given via WithEventNotifier)
func (spec SupervisorSpec) getEventNotifier() EventNotifier {
	if spec.eventNotifier == nil {
		return emptyEventNotifier
	}
	return spec.eventNotifier
}

// subtreeMain contains the main logic of the Child spec that runs a supervision
// sub-tree. It returns an error if the child supervisor fails to start.
func subtreeMain(
	parentName string,
	spec SupervisorSpec,
) func(context.Context, c.NotifyStartFn) error {
	// we use the start version that receives the notifyChildStart callback, this
	// is essential, as we need this callback to signal the sub-tree children have
	// started before signaling we have started
	return func(parentCtx context.Context, notifyChildStart c.NotifyStartFn) error {
		// in this function we use the private versions of start and wait
		// given we don't want to signal the eventNotifier more than once
		// on sub-trees

		ctx, cancelFn := context.WithCancel(parentCtx)
		defer cancelFn()
		sup, err := spec.start(ctx, parentName)
		notifyChildStart(err)
		if err != nil {
			return err
		}
		return sup.wait()
	}
}

// Subtree allows to register a Supervisor Spec as a sub-tree of a bigger
// Supervisor Spec.
func (spec SupervisorSpec) Subtree(
	subtreeSpec SupervisorSpec,
	copts0 ...c.Opt,
) c.ChildSpec {
	subtreeSpec.eventNotifier = spec.eventNotifier

	// NOTE: Child goroutines that are running a sub-tree supervisor must always
	// have a timeout of Infinity, as specified in the documentation from OTP
	// http://erlang.org/doc/design_principles/sup_princ.html#child-specification
	copts := append(copts0, c.WithShutdown(c.Inf))

	return c.NewWithNotifyStart(
		subtreeSpec.Name(),
		subtreeMain(spec.name, subtreeSpec),
		copts...,
	)
}

// start is routine that contains the main logic of a Supervisor. This function:
//
// 1) spawns a new goroutine for the supervisor loop
//
// 2) spawns each child goroutine in the correct order
//
// 3) stops all the spawned children in the correct order once it gets a stop
// signal
//
// 4) it monitors and reacts to errors reported by the supervised children
//
func (spec SupervisorSpec) start(parentCtx context.Context, parentName string) (Supervisor, error) {
	// cancelFn is used when Stop is requested
	ctx, cancelFn := context.WithCancel(parentCtx)

	// notifyCh is used to keep track of errors from children
	notifyCh := make(chan c.ChildNotification)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	// ctrlCh := make(chan ControlMsg)

	// startCh is used to track when the supervisor loop thread has started
	startCh := make(chan startError)

	// terminateCh is used when waiting for cancelFn to complete
	terminateCh := make(chan terminateError)

	eventNotifier := spec.getEventNotifier()

	var runtimeName string
	if parentName == rootSupervisorName {
		// We are the root supervisor, no need to add prefix
		runtimeName = spec.Name()
	} else {
		runtimeName = strings.Join([]string{parentName, spec.Name()}, "/")
	}

	sup := Supervisor{
		runtimeName: runtimeName,
		spec:        spec,
		children:    make(map[string]c.Child, len(spec.children)),
		cancel:      cancelFn,
		wait: func() error {
			// Let's us wait for the Supervisor goroutine to terminate, if there are
			// errors in the termination (e.g. Timeout of child, error treshold
			// reached, etc.), the terminateCh is going to return an error, otherwise
			// it will nil
			err := <-terminateCh
			return err
		},
	}

	// stopChildrenFn is used on the shutdown of the supervisor tree, it stops
	// children in the desired order. The starting argument indicates if the
	// supervision tree is starting, if that is the case, it is more permisive
	// around spec children not matching one to one with it's corresponding
	// runtime children, this may happen because we had a start error in the
	// middle of supervision tree initialization, and we never got to initialize
	// all children at this supervision level.
	stopChildrenFn := func(starting bool) map[string]error {
		children := spec.order.SortStop(spec.children)
		childErrMap := make(map[string]error)
		for _, cs := range children {
			c, ok := sup.children[cs.Name()]
			if !ok && starting {
				// skip it as we may have not started this child before a previous one
				// failed
				continue
			} else if !ok {
				// There is no excuse for a runtime child to not have a corresponding
				// spec, this is a serious implementation error.
				panic(
					fmt.Sprintf(
						"Invariant violetated: Child %s is not on started list",
						cs.Name(),
					),
				)
			}
			stopTime := time.Now()
			err := c.Stop()
			// If a child fails to stop (either because of a legit failure or a
			// timeout), we store the error so that we can report all of them later
			if err != nil {
				childErrMap[cs.Name()] = err
			}
			eventNotifier.ProcessStopped(c.RuntimeName(), stopTime, err)
		}
		return childErrMap
	}

	go func() {
		defer close(terminateCh)

		// Start children
		for _, cs := range spec.order.SortStart(spec.children) {
			startTime := time.Now()
			c, err := cs.Start(sup.runtimeName, notifyCh)
			// NOTE: The error handling code bellow gets executed when the children
			// fails at start time
			if err != nil {
				cRuntimeName := strings.Join([]string{sup.runtimeName, cs.Name()}, "/")
				eventNotifier.ProcessStopped(cRuntimeName, startTime, err)
				childErrMap := stopChildrenFn(true /* starting? */)
				// Is important we stop the children before we finish the supervisor
				startCh <- SupervisorError{
					err:         err,
					runtimeName: runtimeName,
					childErrMap: childErrMap,
				}
				return
			}
			eventNotifier.ProcessStarted(c.RuntimeName(), startTime)
			sup.children[cs.Name()] = c
		}

		// Once children have been spawned we notify the supervisor thread has
		// started
		close(startCh)

		// Supervisor Loop
	supervisorLoop:
		for {
			select {
			// parent context is done
			case <-ctx.Done():
				childErrMap := stopChildrenFn(false /* starting? */)
				// If any of the children fails to stop, we should report that as an
				// error
				if len(childErrMap) > 0 {
					terminateCh <- SupervisorError{
						err:         errors.New("Supervisor stop error"),
						runtimeName: runtimeName,
						childErrMap: childErrMap,
					}
				}
				break supervisorLoop
			case /* childNotification = */ <-notifyCh:
				// TODO: Deal with errors on children
				// case msg := <-ctrlCh:
				// TODO: Deal with public facing API calls
			}
		}
	}()

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout
	err := <-startCh
	if err != nil {
		// Let's wait for the supervisor to stop all children before returning the
		// final error
		_ /* err */ = sup.wait()
		return Supervisor{}, err
	}

	return sup, nil
}

// Name returns the specified name for a Supervisor Spec
func (spec SupervisorSpec) Name() string {
	return spec.name
}

// Start creates a Supervisor from the SupervisorSpec. A Supervisor is a tree of
// Child records where each Child handles a goroutine. The Start algorithm
// begins with the spawning of the leaf children goroutines first. Depending on
// the SupervisorSpec's order, it will do an initialization in pre-order
// (LeftToRight) or post-order (RightToLeft).
//
// ### Initialization of the tree
//
// Once all the children are initialized and running, the supervisor will
// execute it's supervision logic (listening to failures on its children).
// Invoking this method will block the thread until all the children and its
// sub-tree's childrens have been started.
//
// ### Failure on child initialization
//
// In case one of the children fails to start, the Supervisor is going to retry
// a number of times before giving up and returning an error. In case this
// supervisor is a sub-tree, it's parent supervisor will retry the
// initialization until the failure treshold is reached; eventually, the errors
// will reach the root supervisor and the program will get a hard failure.
//
func (spec SupervisorSpec) Start(parentCtx context.Context) (Supervisor, error) {
	startTime := time.Now()
	sup, err := spec.start(parentCtx, rootSupervisorName)
	if err != nil {
		// NOTE we are using the spec.Name() as we know this is the top-level supervisor
		spec.getEventNotifier().ProcessStopped(spec.Name(), startTime, err)
		return Supervisor{}, err
	}
	spec.getEventNotifier().ProcessStarted(sup.runtimeName, startTime)
	return sup, nil
}

// New creates a SupervisorSpec. It requires the name of the supervisor (for
// tracing purposes), all the other settings can be specified via Opt calls
func New(name string, opts ...Opt) SupervisorSpec {
	spec := SupervisorSpec{
		children:      make([]c.ChildSpec, 0, 10),
		eventNotifier: emptyEventNotifier,
	}

	// Check name cannot be empty
	if name == "" {
		panic("Supervisor cannot have empty name")
	}
	spec.name = name

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}

	// return spec
	return spec
}
