package s

import (
	"context"
	"errors"
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

	go func() {
		defer close(terminateCh)

		// Start children
		err := sup.startChildren(notifyCh)
		if err != nil {
			startCh <- err
			return
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
				childErrMap := sup.stopChildren(false /* starting? */)
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
