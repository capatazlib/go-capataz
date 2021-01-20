package cap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// nodeSepToken is the token use to separate sub-trees and child node names in
// the supervision tree
const nodeSepToken = "/"

////////////////////////////////////////////////////////////////////////////////

func handleChildNodeError(
	supCtx context.Context,
	eventNotifier EventNotifier,
	supRuntimeName string,
	supTolerance *restartToleranceManager,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevCh c.Child,
	prevChErr error,
) *RestartToleranceReached {
	chSpec := prevCh.GetSpec()

	eventNotifier.processFailed(chSpec.GetTag(), prevCh.GetRuntimeName(), prevChErr)

	switch chSpec.GetRestart() {
	case c.Permanent, c.Transient:
		// On error scenarios, Permanent and Transient try as much as possible
		// to restart the failing child
		return oneForOneRestartLoop(
			supCtx,
			eventNotifier,
			supRuntimeName,
			supTolerance,
			supChildren,
			supNotifyCh,
			prevCh,
			prevChErr,
		)

	default: /* Temporary */
		// Temporary children can complete or fail, supervisor will not restart them
		delete(supChildren, chSpec.GetName())
		return nil
	}
}

func handleChildNodeCompletion(
	supCtx context.Context,
	eventNotifier EventNotifier,
	supRuntimeName string,
	supTolerance *restartToleranceManager,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevCh c.Child,
) *RestartToleranceReached {

	if prevCh.IsWorker() {
		eventNotifier.workerCompleted(prevCh.GetRuntimeName())
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
			supCtx,
			eventNotifier,
			supRuntimeName,
			supTolerance,
			supChildren,
			supNotifyCh,
			prevCh,
			nil,
		)
	}
}

func handleChildNodeNotification(
	supCtx context.Context,
	eventNotifier EventNotifier,
	supRuntimeName string,
	supTolerance *restartToleranceManager,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	prevCh c.Child,
	chNotification c.ChildNotification,
) *RestartToleranceReached {
	chErr := chNotification.Unwrap()

	if chErr != nil {
		// if the notification contains an error, we send a notification
		// saying that the process failed
		return handleChildNodeError(
			supCtx,
			eventNotifier,
			supRuntimeName,
			supTolerance,
			supChildren,
			supNotifyCh,
			prevCh,
			chErr,
		)
	}

	return handleChildNodeCompletion(
		supCtx,
		eventNotifier,
		supRuntimeName,
		supTolerance,
		supChildren,
		supNotifyCh,
		prevCh,
	)
}

////////////////////////////////////////////////////////////////////////////////

// startChildNode is responsible of starting a single child. This function will
// deal with the child lifecycle notification. It will return an error if
// something goes wrong with the initialization of this child.
func startChildNode(
	startCtx context.Context,
	spec SupervisorSpec,
	supRuntimeName string,
	notifyCh chan c.ChildNotification,
	chSpec c.ChildSpec,
) (c.Child, error) {
	eventNotifier := spec.getEventNotifier()
	startedTime := time.Now()
	ch, chStartErr := chSpec.DoStart(startCtx, supRuntimeName, notifyCh)

	// NOTE: The error handling code bellow gets executed when the children
	// fails at start time
	if chStartErr != nil {
		cRuntimeName := strings.Join(
			[]string{supRuntimeName, chSpec.GetName()},
			nodeSepToken,
		)
		eventNotifier.processStartFailed(chSpec.GetTag(), cRuntimeName, chStartErr)
		return c.Child{}, chStartErr
	}

	// NOTE: we only notify when child is a worker because sub-trees supervisors
	// are responsible of their own notification
	if chSpec.IsWorker() {
		eventNotifier.workerStarted(ch.GetRuntimeName(), startedTime)
	}
	return ch, nil
}

// startChildNodes iterates over all the children (specified with `cap.WithNodes`
// and `cap.WithSubtree`) starting a goroutine for each. The children iteration
// will be sorted as specified with the `cap.WithStartOrder` option. In case any child
// fails to start, the supervisor start operation will be aborted and all the
// started children so far will be stopped in the reverse order.
func startChildNodes(
	startCtx context.Context,
	spec SupervisorSpec,
	supChildrenSpecs []c.ChildSpec,
	supRuntimeName string,
	notifyCh chan c.ChildNotification,
) (map[string]c.Child, error) {
	children := make(map[string]c.Child)

	// Start children in the correct order
	for _, chSpec := range spec.order.sortStart(supChildrenSpecs) {
		// the function above will modify the children internally
		ch, chStartErr := startChildNode(
			startCtx,
			spec,
			supRuntimeName,
			notifyCh,
			chSpec,
		)
		if chStartErr != nil {
			// we must stop previously started children before we finish the supervisor
			nodeErrMap := terminateChildNodes(spec, supChildrenSpecs, children)
			var terminationErr *SupervisorTerminationError
			if len(nodeErrMap) > 0 {
				terminationErr = &SupervisorTerminationError{
					supRuntimeName: supRuntimeName,
					nodeErrMap:     nodeErrMap,
					rscCleanupErr:  nil,
				}
			}

			return nil, &SupervisorStartError{
				supRuntimeName: supRuntimeName,
				nodeName:       chSpec.GetName(),
				nodeErr:        chStartErr,
				terminationErr: terminationErr,
			}
		}
		children[chSpec.GetName()] = ch
	}

	return children, nil
}

// terminateChildNode executes the Terminate procedure on the given child, in case
// there is an error on termination it notifies the event system
func terminateChildNode(
	eventNotifier EventNotifier,
	ch c.Child,
) error {
	chSpec := ch.GetSpec()
	stoppingTime := time.Now()
	terminationErr := ch.Terminate()

	if terminationErr != nil {
		// we also notify that the process failed
		eventNotifier.processFailed(chSpec.GetTag(), ch.GetRuntimeName(), terminationErr)
		return terminationErr
	}
	// we need to notify that the process stopped
	eventNotifier.processTerminated(chSpec.GetTag(), ch.GetRuntimeName(), stoppingTime)
	return nil
}

// terminateChildNodes is used on the shutdown of the supervisor tree, it stops
// children in the desired order.
func terminateChildNodes(
	spec SupervisorSpec,
	supChildrenSpecs0 []c.ChildSpec,
	supChildren map[string]c.Child,
) map[string]error {
	eventNotifier := spec.eventNotifier
	supChildrenSpecs := spec.order.sortTermination(supChildrenSpecs0)
	supNodeErrMap := make(map[string]error)

	for _, chSpec := range supChildrenSpecs {
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
			terminationErr := terminateChildNode(eventNotifier, ch)
			if terminationErr != nil {
				// if a child fails to stop (either because of a legit failure or a
				// timeout), we store the terminationError so that we can report all of them
				// later
				supNodeErrMap[chSpec.GetName()] = terminationErr
			}
		}
	}
	return supNodeErrMap
}

// terminateSupervisor stops all children an signal any errors to the
// given onTerminate callback
func terminateSupervisor(
	supSpec SupervisorSpec,
	supChildrenSpecs []c.ChildSpec,
	supRuntimeName string,
	supRscCleanup CleanupResourcesFn,
	supChildren map[string]c.Child,
	onTerminate func(error),
	restartErr *RestartToleranceReached,
) error {
	var terminateErr *SupervisorTerminationError
	supNodeErrMap := terminateChildNodes(supSpec, supChildrenSpecs, supChildren)
	supRscCleanupErr := supRscCleanup()

	// If any of the children fails to stop, we should report that as an
	// error
	if len(supNodeErrMap) > 0 || supRscCleanupErr != nil {

		// On async strategy, we notify that the spawner terminated with an
		// error
		terminateErr = &SupervisorTerminationError{
			supRuntimeName: supRuntimeName,
			nodeErrMap:     supNodeErrMap,
			rscCleanupErr:  supRscCleanupErr,
		}
	}

	// If we have a terminateErr or a restartErr, we should report that back to the
	// parent
	if restartErr != nil && terminateErr != nil {
		supErr := &SupervisorRestartError{
			supRuntimeName: supRuntimeName,
			nodeErr:        restartErr,
			terminationErr: terminateErr,
		}
		onTerminate(supErr)
		return supErr
	}

	// If we have a restartErr only, report the restart error only
	if restartErr != nil {
		supErr := &SupervisorRestartError{
			supRuntimeName: supRuntimeName,
			nodeErr:        restartErr,
			terminationErr: nil,
		}
		onTerminate(supErr)
		return supErr
	}

	// If we have a terminateErr only, report the termination error only
	if terminateErr != nil {
		onTerminate(terminateErr)
		return terminateErr
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
	supCtx context.Context,
	supSpec SupervisorSpec,
	supChildrenSpecs []c.ChildSpec,
	supRuntimeName string,
	supTolerance *restartToleranceManager,
	supRscCleanup CleanupResourcesFn,
	supNotifyCh chan c.ChildNotification,
	ctrlCh chan ctrlMsg,
	supStartTime time.Time,
	onStart c.NotifyStartFn,
	onTerminate notifyTerminationFn,
) error {
	// Start children
	supChildren, restartErr := startChildNodes(
		supCtx,
		supSpec,
		supChildrenSpecs,
		supRuntimeName,
		supNotifyCh,
	)
	if restartErr != nil {
		// in case we run in the async strategy we notify the spawner that we
		// started with an error
		onStart(restartErr)
		return restartErr
	}

	// Supervisors are responsible of notifying their start events, this is
	// important because only the supervisor goroutine nows the exact time it gets
	// started (we would get race-conditions if we notify from the parent
	// otherwise).
	eventNotifier := supSpec.getEventNotifier()
	eventNotifier.supervisorStarted(supRuntimeName, supStartTime)

	/// Once children have been spawned, we notify to the caller thread that the
	// main loop has started without errors.
	onStart(nil)

	// Supervisor Loop
	for {
		select {
		// parent context is done
		case <-supCtx.Done():
			return terminateSupervisor(
				supSpec,
				supChildrenSpecs,
				supRuntimeName,
				supRscCleanup,
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

			restartErr := handleChildNodeNotification(
				supCtx,
				eventNotifier,
				supRuntimeName,
				supTolerance,
				supChildren,
				supNotifyCh,
				prevCh,
				chNotification,
			)

			if restartErr != nil {
				return terminateSupervisor(
					supSpec,
					supChildrenSpecs,
					supRuntimeName,
					supRscCleanup,
					supChildren,
					onTerminate,
					restartErr,
				)
			}

		case msg := <-ctrlCh:
			supChildrenSpecs, supChildren = handleCtrlMsg(
				supCtx,
				eventNotifier,
				supSpec,
				supChildrenSpecs,
				supRuntimeName,
				supChildren,
				supNotifyCh,
				msg,
			)
		}
	}
}
