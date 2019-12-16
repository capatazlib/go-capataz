package s

// This file contains the implementation of the supervision start
//
// * Creation of Supervisor from SupervisorSpec
// * Children bootstrap
// * Monitor Loop
//

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

// oneForOneRestart executes the restart logic for a OneForOne algorithm
func (sup Supervisor) oneForOneRestart(
	prevChild c.Child,
	notifyCh chan<- c.ChildNotification,
) error {
	spec := sup.spec
	eventNotifier := spec.getEventNotifier()

	cspec := prevChild.Spec()
	cname := cspec.Name()

	startTime := time.Now()
	newChild, err := cspec.Restart(prevChild, sup.runtimeName, notifyCh)

	// We want to keep track of the updated restartCount which is in newChild,
	// ergo we must override the child independently of the outcome
	sup.children[cname] = newChild

	if err != nil {
		// Given this error ocurred after supervisor bootstrap, it is treated as the
		// child failed on supervision time, _not_ start time; return the error so
		// that the supervisor does the restarting
		eventNotifier.ProcessFailed(newChild.RuntimeName(), err)
		return err
	}
	eventNotifier.ProcessStarted(newChild.RuntimeName(), startTime)
	return nil
}

// supervisionLoop does the initialization of supervisor's children and then runs an
// infinite loop that monitors each child error.
func (sup Supervisor) supervisionLoop(
	ctx context.Context,
	startTime time.Time,
	startCh chan startError,
	terminateCh chan terminateError,
	notifyCh chan c.ChildNotification,
) {
	defer close(terminateCh)
	spec := sup.spec
	eventNotifier := spec.getEventNotifier()

	// Start children
	err := sup.startChildren(notifyCh)
	if err != nil {
		startCh <- err
		return
	}

	eventNotifier.ProcessStarted(sup.runtimeName, startTime)
	// Once children have been spawned we notify the supervisor thread has
	// started
	close(startCh)

	// Supervisor Loop
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
					runtimeName: sup.runtimeName,
					childErrMap: childErrMap,
				}
			}
			return
		case childNotification := <-notifyCh:
			oldChild, ok := sup.children[childNotification.Name()]
			if !ok {
				// TODO: Expand on this case, I think this is highly unlikely, but would
				// like to exercise this branch in test somehow (if possible)
				panic("Something horribly wrong happened here")
			}

			if err := childNotification.Unwrap(); err != nil {
				// If the childNotification is from an error, we report it here
				eventNotifier.ProcessFailed(oldChild.RuntimeName(), err)
			}

			// TODO: Verify error treshold here
			// TODO: Deal with other restart strategies here
			// TODO: Deal with err returned for start failures
			_ /* err */ = sup.oneForOneRestart(oldChild, notifyCh)
			// case msg := <-ctrlCh:
			// TODO: Deal with public facing API calls
		}
	}
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
	eventNotifier := spec.getEventNotifier()

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

	// Calculate the runtime name of this supervisor
	var runtimeName string
	if parentName == rootSupervisorName {
		// We are the root supervisor, no need to add prefix
		runtimeName = spec.Name()
	} else {
		runtimeName = strings.Join([]string{parentName, spec.Name()}, childSepToken)
	}

	sup := Supervisor{
		runtimeName: runtimeName,
		spec:        spec,
		children:    make(map[string]c.Child, len(spec.children)),
		cancel:      cancelFn,
		wait: func(stoppingTime time.Time, stoppingErr error) error {
			// Let's us wait for the Supervisor goroutine to terminate, if there are
			// errors in the termination (e.g. Timeout of child, error treshold
			// reached, etc.), the terminateCh is going to return an error, otherwise
			// it will nil
			terminateErr := <-terminateCh
			// stoppingTime is only relevant when we call the internal wait function from
			// the Stop() public API; if we just get called from Wait(), we don't need
			// to keep track of the stop duration
			if stoppingTime == (time.Time{}) {
				stoppingTime = time.Now()
			}

			if terminateErr != nil {
				eventNotifier.ProcessFailed(runtimeName, terminateErr)
				return terminateErr
			}
			if stoppingErr != nil {
				eventNotifier.ProcessFailed(runtimeName, stoppingErr)
				return stoppingErr
			}
			eventNotifier.ProcessStopped(runtimeName, stoppingTime)
			return nil
		},
	}

	startTime := time.Now()
	// spawn supervision goroutine
	go sup.supervisionLoop(ctx, startTime, startCh, terminateCh, notifyCh)

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout

	// We check if there was an start error reported from the supervisionLoop, if this
	// is the case, we wait for the termination of started children and return the
	// reported error
	err := <-startCh
	if err != nil {
		// Let's wait for the supervisor to stop all children before returning the
		// final error
		stoppingTime := time.Now()
		_ /* err */ = sup.wait(stoppingTime, err)
		return Supervisor{}, err
	}

	return sup, nil
}
