package s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// childSepToken is the token use to separate sub-trees and child names in the
// supervision tree
const childSepToken = "/"

////////////////////////////////////////////////////////////////////////////////

func handleChildError(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevCh c.Child,
	prevChErr error,
) *c.ErrorToleranceReached {
	chSpec := prevCh.GetSpec()

	eventNotifier.ProcessFailed(chSpec.GetTag(), prevCh.GetRuntimeName(), prevChErr)

	switch chSpec.GetRestart() {
	case c.Permanent, c.Transient:
		// On error scenarios, Permanent and Transient try as much as possible
		// to restart the failing child
		return oneForOneRestartLoop(
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevCh,
		)

	default: /* Temporary */
		// Temporary children can complete or fail, supervisor will not restart them
		delete(supChildren, chSpec.GetName())
		return nil
	}
}

func handleChildCompletion(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevCh c.Child,
) *c.ErrorToleranceReached {

	if prevCh.IsWorker() {
		eventNotifier.WorkerCompleted(prevCh.GetRuntimeName())
	}

	chSpec := prevCh.GetSpec()

	switch chSpec.GetRestart() {

	case c.Transient, c.Temporary:
		delete(supChildren, chSpec.GetName())
		// Do nothing
		return nil
	default: /* Permanent */
		// On child completion, the supervisor still restart the child when the
		// c.Restart is Permanent
		return oneForOneRestartLoop(
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevCh,
		)
	}
}

func handleChildNotification(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevCh c.Child,
	chNotification c.ChildNotification,
) *c.ErrorToleranceReached {
	chErr := chNotification.Unwrap()

	if chErr != nil {
		// if the notification contains an error, we send a notification
		// saying that the process failed
		return handleChildError(
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevCh,
			chErr,
		)
	}

	return handleChildCompletion(
		eventNotifier,
		supRuntimeName,
		supChildren,
		supNotifyCh,
		prevCh,
	)
}

////////////////////////////////////////////////////////////////////////////////

// startChildren iterates over all the children (specified with `s.WithChildren`
// and `s.WithSubtree`) starting a goroutine for each. The children iteration
// will be sorted as specified with the `s.WithOrder` option. In case any child
// fails to start, the supervisor start operation will be aborted and all the
// started children so far will be stopped in the reverse order.
func startChildren(
	spec SupervisorSpec,
	supRuntimeName string,
	notifyCh chan c.ChildNotification,
) (map[string]c.Child, error) {
	eventNotifier := spec.getEventNotifier()
	children := make(map[string]c.Child)

	// Start children
	for _, chSpec := range spec.order.SortStart(spec.children) {
		startedTime := time.Now()
		ch, chStartErr := chSpec.DoStart(supRuntimeName, notifyCh)

		// NOTE: The error handling code bellow gets executed when the children
		// fails at start time
		if chStartErr != nil {
			cRuntimeName := strings.Join(
				[]string{supRuntimeName, chSpec.GetName()},
				childSepToken,
			)
			eventNotifier.ProcessStartFailed(chSpec.GetTag(), cRuntimeName, chStartErr)
			childErrMap := terminateChildren(spec, children)
			// Is important we stop the children before we finish the supervisor
			return nil, &SupervisorTerminationError{
				supRuntimeName: supRuntimeName,
				childErr:       chStartErr,
				childErrMap:    childErrMap,
			}
		}

		// NOTE: we only notify when child is a worker because sub-trees supervisors
		// are responsible of their own notification
		if chSpec.IsWorker() {
			eventNotifier.WorkerStarted(ch.GetRuntimeName(), startedTime)
		}
		children[chSpec.GetName()] = ch
	}
	return children, nil
}

// terminateChild executes the Terminate procedure on the given child, in case
// there is an error on termination it notifies the event system and appends a
// new entry to the given error map.
func terminateChild(
	eventNotifier EventNotifier,
	supChildErrMap map[string]error,
	ch c.Child,
) map[string]error {
	chSpec := ch.GetSpec()
	stoppingTime := time.Now()
	terminationErr := ch.Terminate()

	if terminationErr != nil {
		// if a child fails to stop (either because of a legit failure or a
		// timeout), we store the terminationError so that we can report all of them
		// later
		supChildErrMap[chSpec.GetName()] = terminationErr

		// we also notify that the process failed
		eventNotifier.ProcessFailed(chSpec.GetTag(), ch.GetRuntimeName(), terminationErr)
	} else {
		// we need to notify that the process stopped
		eventNotifier.ProcessTerminated(chSpec.GetTag(), ch.GetRuntimeName(), stoppingTime)
	}

	return supChildErrMap
}

// terminateChildren is used on the shutdown of the supervisor tree, it stops
// children in the desired order.
func terminateChildren(
	spec SupervisorSpec,
	supChildren map[string]c.Child,
) map[string]error {
	eventNotifier := spec.eventNotifier
	childrenSpecs := spec.order.SortTermination(spec.children)
	supChildErrMap := make(map[string]error)

	for _, chSpec := range childrenSpecs {
		ch, ok := supChildren[chSpec.GetName()]
		// There are scenarios where is ok to ignore supChildren not having the
		// entry:
		//
		// * On start, there may be a failure mid-way in the initialization and on
		// the rollback we iterate over children spec that are not present in the
		// runtime children map
		//
		// * On stop, there may be a Transient child that completed, or a Temporary child
		// that completed or failed.
		if ok {
			supChildErrMap = terminateChild(eventNotifier, supChildErrMap, ch)
		}
	}
	return supChildErrMap
}

// terminateSupervisor stops all children an signal any errors to the
// given onTerminate callback
func terminateSupervisor(
	supSpec SupervisorSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	onTerminate func(error),
	restartErr *c.ErrorToleranceReached,
) error {
	var terminateErr *SupervisorTerminationError
	supChildErrMap := terminateChildren(supSpec, supChildren)

	// If any of the children fails to stop, we should report that as an
	// error
	if len(supChildErrMap) > 0 {

		// On async strategy, we notify that the spawner terminated with an
		// error
		terminateErr = &SupervisorTerminationError{
			supRuntimeName: supRuntimeName,
			childErrMap:    supChildErrMap,
		}
	}

	// If we have a terminateErr or a restartErr, we should report that back to the
	// parent
	if restartErr != nil || terminateErr != nil {
		supErr := &SupervisorRestartError{
			supRuntimeName: supRuntimeName,
			terminateErr:   terminateErr,
			childErr:       restartErr,
		}
		onTerminate(supErr)
		return supErr
	}

	onTerminate(nil)
	return nil
}

////////////////////////////////////////////////////////////////////////////////

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
	supChildren, restartErr := startChildren(supSpec, supRuntimeName, supNotifyCh)
	if restartErr != nil {
		// in case we run in the async strategy we notify the spawner that we
		// started with an error
		onStart(restartErr)
		// We signal that we terminated, the error is not reported here because
		// it was reported in the onStart callback
		onTerminate(nil)
		return restartErr
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
			return terminateSupervisor(
				supSpec,
				supRuntimeName,
				supChildren,
				onTerminate,
				nil, /* restart error */
			)

		case chNotification := <-supNotifyCh:
			prevCh, ok := supChildren[chNotification.GetName()]

			if !ok {
				// TODO: Expand on this case, I think this is highly unlikely, but would
				// like to exercise this branch in test somehow (if possible)
				panic(
					fmt.Errorf(
						"something horribly wrong happened here (name: %s, tag: %s)",
						prevCh.GetRuntimeName(),
						prevCh.GetTag(),
					),
				)
			}

			restartErr := handleChildNotification(
				eventNotifier,
				supRuntimeName,
				supChildren,
				supNotifyCh,
				prevCh,
				chNotification,
			)

			if restartErr != nil {
				return terminateSupervisor(
					supSpec,
					supRuntimeName,
					supChildren,
					onTerminate,
					restartErr,
				)
			}

			// case msg := <-ctrlCh:
			// TODO: Deal with public facing API calls
		}
	}
}
