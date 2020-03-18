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
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

// notifyTerminationFn is a callback that gets called when a supervisor is
// terminating (with or without an error).
type notifyTerminationFn = func(terminateError)

func oneForOneRestart(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan<- c.ChildNotification,
	prevChild c.Child,
) (c.Child, error) {
	chSpec := prevChild.Spec()
	chName := chSpec.Name()

	startTime := time.Now()
	newChild, err := chSpec.Restart(prevChild, supRuntimeName, supNotifyCh)

	// We want to keep track of the updated restartCount which is in the newChild
	// record, we must override the child independently of the outcome.
	supChildren[chName] = newChild

	if err != nil {
		// Given this error ocurred after supervisor bootstrap, it is treated as the
		// child failed on supervision time, _not_ start time; return the error so that
		// the supervisor does the restarting
		if newChild.Tag() == c.Worker {
			eventNotifier.WorkerFailed(newChild.RuntimeName(), err)
		}
		return c.Child{}, err
	}

	if newChild.Tag() == c.Worker {
		eventNotifier.WorkerStarted(newChild.RuntimeName(), startTime)
	}
	return newChild, nil
}

func oneForOneRestartLoop(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan<- c.ChildNotification,
	prevChild c.Child,
) error {
	for {
		newChild, startErr := oneForOneRestart(
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevChild,
		)
		// if we don't get start errors, break the loop
		if startErr == nil {
			return nil
		}

		// otherwise, repeat until error treshold is met
		// TODO: deal with restart error treshold here
		prevChild = newChild
	}
}

func handleChildError(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevChild c.Child,
	prevChErr error,
) error {
	chSpec := prevChild.Spec()

	eventNotifier.ProcessFailed(chSpec.Tag(), prevChild.RuntimeName(), prevChErr)

	switch chSpec.GetRestart() {
	case c.Permanent, c.Transient:
		// On error scenarios, Permanent and Transient try as much as possible
		// to restart the failing child
		for {
			return oneForOneRestartLoop(
				eventNotifier,
				supRuntimeName,
				supChildren,
				supNotifyCh,
				prevChild,
			)
		}

	default: /* Temporary */
		// Temporary children can complete or fail, supervisor will not restart them
		delete(supChildren, chSpec.Name())
		return nil
	}
}

func handleChildCompletion(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevChild c.Child,
) error {
	eventNotifier.WorkerCompleted(prevChild.RuntimeName())

	chSpec := prevChild.Spec()

	switch chSpec.GetRestart() {

	case c.Transient, c.Temporary:
		delete(supChildren, chSpec.Name())
		// Do nothing
		return nil
	default: /* Permanent */
		// On child completion, the supervisor still restart the child when the
		// c.Restart is Permanent
		for {
			return oneForOneRestartLoop(
				eventNotifier,
				supRuntimeName,
				supChildren,
				supNotifyCh,
				prevChild,
			)
		}
	}
}

func handleChildNotification(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevChild c.Child,
	chNotification c.ChildNotification,
) error {
	chErr := chNotification.Unwrap()

	if chErr != nil {
		// if the notification contains an error, we send a notification
		// saying that the process failed
		return handleChildError(
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevChild,
			chErr,
		)
	}

	return handleChildCompletion(
		eventNotifier,
		supRuntimeName,
		supChildren,
		supNotifyCh,
		prevChild,
	)
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
	supSpec SupervisorSpec,
	supRuntimeName string,
	supNotifyCh chan c.ChildNotification,
	supStartTime time.Time,
	onStart c.NotifyStartFn,
	onTerminate notifyTerminationFn,
) error {
	// Start children
	supChildren, startErr := startChildren(supSpec, supRuntimeName, supNotifyCh)
	if startErr != nil {
		// in case we run in the async strategy we notify the spawner that we
		// started with an error
		onStart(startErr)
		// We signal that we terminated, the error is not reported here because
		// it was reported in the onStart callback
		onTerminate(nil)
		return startErr
	}

	// Supervisors are responsible of notifying their start events, this is
	// important because only the supervisor goroutine nows the exact time it gets
	// started (we would get race-conditions if we notify from the parent
	// otherwise).
	eventNotifier := supSpec.getEventNotifier()
	eventNotifier.SupervisorStarted(supRuntimeName, supStartTime)

	/// Once children have been spawned, we notify to the caller thread that the
	// main loop has started without errors.
	onStart(nil)

	// Supervisor Loop
	for {
		select {
		// parent context is done
		case <-ctx.Done():
			supChildErrMap := stopChildren(supSpec, supChildren, false /* starting? */)
			// If any of the children fails to stop, we should report that as an
			// error
			if len(supChildErrMap) > 0 {
				// On async strategy, we notify that the spawner terminated with an
				// error
				stopErr := SupervisorError{
					err:         errors.New("Supervisor stop error"),
					runtimeName: supRuntimeName,
					childErrMap: supChildErrMap,
				}
				onTerminate(stopErr)
				// On sync strategy, we return the error
				return stopErr
			}
			onTerminate(nil)
			return nil

		case chNotification := <-supNotifyCh:
			prevChild, ok := supChildren[chNotification.Name()]

			if !ok {
				// TODO: Expand on this case, I think this is highly unlikely, but would
				// like to exercise this branch in test somehow (if possible)
				panic(
					fmt.Errorf(
						"something horribly wrong happened here (name: %s, tag: %s)",
						prevChild.RuntimeName(),
						prevChild.Tag(),
					),
				)
			}

			handleChildNotification(
				eventNotifier,
				supRuntimeName,
				supChildren,
				supNotifyCh,
				prevChild,
				chNotification,
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
				eventNotifier.SupervisorFailed(runtimeName, terminateErr)
				return terminateErr
			}

			// stopingTime is only relevant when we call the internal wait function
			// from the Stop() public API; if we just called from Wait(), we don't
			// need to keep track of the stop duration
			if stopingTime == (time.Time{}) {
				stopingTime = time.Now()
			}

			eventNotifier.SupervisorStopped(runtimeName, stopingTime)
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
		eventNotifier.SupervisorStartFailed(runtimeName, startErr)

		// Let's wait for the supervisor to stop all children before returning the
		// final error
		stopingTime := time.Now()
		_ /* err */ = sup.wait(stopingTime, startErr)

		return Supervisor{}, startErr
	}

	return sup, nil
}
