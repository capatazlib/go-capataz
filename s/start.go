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

// notifyTerminationFn is a callback that gets called when a supervisor is
// terminating (with or without an error).
type notifyTerminationFn = func(terminateError)

func oneForOneRestart(
	supRuntimeName string,
	eventNotifier EventNotifier,
	children map[string]c.Child,
	prevChild c.Child,
	notifyCh chan<- c.ChildNotification,
) error {
	chSpec := prevChild.Spec()
	chName := chSpec.Name()

	startTime := time.Now()
	newChild, err := chSpec.Restart(prevChild, supRuntimeName, notifyCh)

	// We want to keep track of the updated restartCount which is in the newChild
	// record, we must override the child independently of the outcome.
	children[chName] = newChild

	if err != nil {
		// Given this error ocurred after supervisor bootstrap, it is treated as the
		// child failed on supervision time, _not_ start time; return the error so that
		// the supervisor does the restarting
		eventNotifier.ProcessFailed(newChild.RuntimeName(), err)
		return err
	}

	eventNotifier.ProcessStarted(newChild.RuntimeName(), startTime)
	return nil
}

// runMonitorLoop does the initialization of supervisor's children and then runs
// an infinite loop that monitors each child error.
//
// This function is used for both async and sync strategies, given this, we
// receive an onStart and onTerminate callbacks that behave differently
// depending on which strategy is used:
//
// 1) When called with the async strategy, these callbacks will interact with
// gochans that communicate with the spawner goroutine.
//
// 2) When called with the sync strategy, these callbacks will return the given
// error, note this implementation returns the result of the callback calls
//
func runMonitorLoop(
	ctx context.Context,
	spec SupervisorSpec,
	runtimeName string,
	notifyCh chan c.ChildNotification,
	startTime time.Time,
	onStart c.NotifyStartFn,
	onTerminate notifyTerminationFn,
) error {
	// Start children
	children, startErr := startChildren(spec, runtimeName, notifyCh)
	if startErr != nil {
		// in case we run in the async strategy we notify the spawner that we
		// started with an error
		onStart(startErr)
		// We signal that we terminated, the error is not reported here because
		// it was reported in the onStart callback
		onTerminate(nil)
		return startErr
	}

	// Supervisors are responsible of notifying their start events
	eventNotifier := spec.getEventNotifier()
	eventNotifier.ProcessStarted(runtimeName, startTime)

	/// Once children have been spawned, we notify to the caller thread that the
	// main loop has started without errors.
	onStart(nil)

	// Supervisor Loop
	for {
		select {
		// parent context is done
		case <-ctx.Done():
			childErrMap := stopChildren(spec, children, false /* starting? */)
			// If any of the children fails to stop, we should report that as an
			// error
			if len(childErrMap) > 0 {
				// On async strategy, we notify that the spawner terminated with an
				// error
				stopErr := SupervisorError{
					err:         errors.New("Supervisor stop error"),
					runtimeName: runtimeName,
					childErrMap: childErrMap,
				}
				onTerminate(stopErr)
				// On sync strategy, we return the error
				return stopErr
			}
			onTerminate(nil)
			return nil
		case childNotification := <-notifyCh:
			oldChild, ok := children[childNotification.Name()]

			if !ok {
				// TODO: Expand on this case, I think this is highly unlikely, but would
				// like to exercise this branch in test somehow (if possible)
				panic("something horribly wrong happened here")
			}

			if err := childNotification.Unwrap(); err != nil {
				eventNotifier.ProcessFailed(oldChild.RuntimeName(), err)
			}

			// TODO: Verify error treshold here
			// TODO: Deal with other restart strategies here
			// TODO: Deal with err returned start failures
			_ /* err */ = oneForOneRestart(
				runtimeName,
				eventNotifier,
				children,
				oldChild,
				notifyCh,
			)
			// case msg := <-ctrlCh:
			// TODO: Deal with public facing API calls
		}
	}
}

// buildRuntimeName creates the runtimeName of a Supervisor from the parent name
// and the spec name
func buildRuntimeName(spec SupervisorSpec, parentName string) string {
	var runtimeName string
	if parentName == rootSupervisorName {
		// We are the root supervisor, no need to add prefix
		runtimeName = spec.Name()
	} else {
		runtimeName = strings.Join([]string{parentName, spec.Name()}, "/")
	}
	return runtimeName
}

// run performs the main logic of a Supervisor. This function:
//
// 1) spawns each child goroutine in the correct order
//
// 2) stops all the spawned children in the correct order once it gets a stop
// signal
//
// 3) it monitors and reacts to errors reported by the supervised children
//
func (spec SupervisorSpec) run(
	ctx context.Context,
	parentName string,
	onStart c.NotifyStartFn,
) error {
	// notifyCh is used to keep track of errors from children
	notifyCh := make(chan c.ChildNotification)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	// ctrlCh := make(chan ControlMsg)

	runtimeName := buildRuntimeName(spec, parentName)

	onTerminate := func(err terminateError) {}

	startTime := time.Now()
	// spawn goroutine with supervisor monitorLoop
	return runMonitorLoop(
		ctx,
		spec,
		runtimeName,
		notifyCh,
		// ctrlCh,
		startTime,
		onStart,
		onTerminate,
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
func (spec SupervisorSpec) start(
	parentCtx context.Context,
	parentName string,
) (Supervisor, error) {
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

	runtimeName := buildRuntimeName(spec, parentName)

	sup := Supervisor{
		runtimeName: runtimeName,
		spec:        spec,
		children:    make(map[string]c.Child, len(spec.children)),
		cancel:      cancelFn,
		wait: func(stopingTime time.Time, stopingErr error) error {
			eventNotifier := spec.getEventNotifier()

			if stopingErr != nil {
				return stopingErr
			}

			// Let's us wait for the Supervisor goroutine to terminate, if there are
			// errors in the termination (e.g. Timeout of child, error treshold
			// reached, etc.), the terminateCh is going to return an error, otherwise
			// it will return nil
			terminateErr := <-terminateCh

			if terminateErr != nil {
				eventNotifier.ProcessFailed(runtimeName, terminateErr)
				return terminateErr
			}

			// stopingTime is only relevant when we call the internal wait function
			// from the Stop() public API; if we just called from Wait(), we don't
			// need to keep track of the stop duration
			if stopingTime == (time.Time{}) {
				stopingTime = time.Now()
			}

			eventNotifier.ProcessStopped(runtimeName, stopingTime)
			return nil
		},
	}

	onStart := func(err startError) {
		if err != nil {
			startCh <- err
		}
		close(startCh)
	}

	onTerminate := func(err terminateError) {
		if err != nil {
			terminateCh <- err
		}
		close(terminateCh)
	}

	// spawn goroutine with supervisor monitorLoop
	go func() {
		// NOTE: we ignore the returned error as that is being handled by the
		// onStart and onTerminate callbacks
		startTime := time.Now()
		_ = runMonitorLoop(
			ctx,
			spec,
			runtimeName,
			notifyCh,
			// ctrlCh,
			startTime,
			onStart,
			onTerminate,
		)
	}()

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout

	// We check if there was an start error reported from the monitorLoop, if this
	// is the case, we wait for the termination of started children and return the
	// reported error
	startErr := <-startCh
	if startErr != nil {
		eventNotifier := spec.getEventNotifier()
		eventNotifier.ProcessStartFailed(runtimeName, startErr)

		// Let's wait for the supervisor to stop all children before returning the
		// final error
		stopingTime := time.Now()
		_ /* err */ = sup.wait(stopingTime, startErr)

		return Supervisor{}, startErr
	}

	return sup, nil
}
