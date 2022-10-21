package s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

type supRuntimeName = string

type strategyRestartFn = func(
	context.Context,
	SupervisorSpec, []c.ChildSpec, // supSpec arguments
	supRuntimeName, map[string]c.Child, chan c.ChildNotification, // runtime arguments
	c.Child, // source child that failed
) (map[string]c.Child, error)

// NodeSepToken is the token use to separate sub-trees and child node names in
// the supervision tree
const NodeSepToken = "/"

////////////////////////////////////////////////////////////////////////////////

func getRestartStrategy(supSpec SupervisorSpec) strategyRestartFn {
	switch supSpec.strategy {
	case OneForOne:
		return oneForOneRestart
	case OneForAll:
		return oneForAllRestart
	default:
		panic("unknown restart strategy, check getRestartStrategy implementation")
	}
}

func execRestartLoop(
	supCtx context.Context,
	supTolerance *restartToleranceManager,
	supSpec SupervisorSpec,
	supChildrenSpecs []c.ChildSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyChan chan c.ChildNotification,
	sourceCh c.Child,
	sourceErr error,
) (map[string]c.Child, *RestartToleranceReached) {
	execRestart := getRestartStrategy(supSpec)
	var prevErr, restartErr error

	// we initialize prevErr with the original child error that caused this logic to get
	// executed. It could happen that this error gets eclipsed by a restart error later
	// on
	prevErr = sourceErr

	for {
		if prevErr != nil {
			ok := supTolerance.checkToleranceExceeded(prevErr)
			if !ok {
				// Very important! even though we return an error value
				// here, we want to return a supChildren, this collection
				// gets replaced on every iteration, and if we return a nil
				// value, children will skip termination (e.g. leak).
				return supChildren, NewRestartToleranceReached(
					supTolerance.restartTolerance,
					sourceCh,
					supTolerance.sourceErr,
					prevErr,
				)
			}
		}

		supChildren, restartErr = execRestart(
			supCtx,
			supSpec, supChildrenSpecs,
			supRuntimeName, supChildren, supNotifyChan,
			sourceCh,
		)

		if restartErr == nil {
			return supChildren, nil
		}

		prevErr = restartErr
	}
}

func handleChildNodeError(
	supCtx context.Context,
	supTolerance *restartToleranceManager,
	supSpec SupervisorSpec, supChildrenSpecs []c.ChildSpec,

	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyChan chan c.ChildNotification,

	sourceCh c.Child, sourceErr error,
) (map[string]c.Child, *RestartToleranceReached) {
	eventNotifier := supSpec.getEventNotifier()
	chSpec := sourceCh.GetSpec()

	eventNotifier.processFailed(chSpec.GetTag(), sourceCh.GetRuntimeName(), sourceErr)

	switch chSpec.GetRestart() {
	case c.Permanent, c.Transient:
		// On error scenarios, Permanent and Transient try as much as possible
		// to restart the failing child
		return execRestartLoop(
			supCtx,
			supTolerance,
			supSpec, supChildrenSpecs,
			supRuntimeName, supChildren, supNotifyChan,
			sourceCh, sourceErr,
		)

	default: /* Temporary */
		// Temporary children can complete or fail, supervisor will not restart them
		delete(supChildren, chSpec.GetName())
		return supChildren, nil
	}
}

func handleChildNodeCompletion(
	supCtx context.Context,
	supTolerance *restartToleranceManager,
	supSpec SupervisorSpec, supChildSpecs []c.ChildSpec,

	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyChan chan c.ChildNotification,

	sourceCh c.Child,
) (map[string]c.Child, *RestartToleranceReached) {
	eventNotifier := supSpec.getEventNotifier()

	if sourceCh.IsWorker() {
		eventNotifier.workerCompleted(sourceCh.GetRuntimeName())
	}

	chSpec := sourceCh.GetSpec()

	switch chSpec.GetRestart() {

	case c.Transient, c.Temporary:
		delete(supChildren, chSpec.GetName())
		// Do nothing
		return supChildren, nil
	default: /* Permanent */
		// On child completion, the supervisor still restart the child when the
		// c.Restart is Permanent
		return execRestartLoop(
			supCtx,
			supTolerance,
			supSpec, supChildSpecs,
			supRuntimeName, supChildren, supNotifyChan,
			sourceCh,
			nil, /* error */
		)
	}
}

func handleChildNodeNotification(
	supCtx context.Context,
	supTolerance *restartToleranceManager,
	supSpec SupervisorSpec,
	supChildSpecs []c.ChildSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyChan chan c.ChildNotification,
	sourceCh c.Child,
	chNotification c.ChildNotification,
) (map[string]c.Child, *RestartToleranceReached) {
	sourceErr := chNotification.Unwrap()

	if sourceErr != nil {
		// if the notification contains an error, we send a notification
		// saying that the process failed
		return handleChildNodeError(
			supCtx,
			supTolerance,
			supSpec, supChildSpecs,

			supRuntimeName, supChildren, supNotifyChan,
			sourceCh,
			sourceErr,
		)
	}

	return handleChildNodeCompletion(
		supCtx,
		supTolerance,
		supSpec, supChildSpecs,
		supRuntimeName, supChildren, supNotifyChan,
		sourceCh,
	)
}

////////////////////////////////////////////////////////////////////////////////

// skipChildFn is a function used to skip a child during an iteration of
// children inside the supervision tree. This function is useful to modify the
// behavior of start/termination of children in a supervision tree and restart
// strategies.
type skipChildFn = func(int, c.ChildSpec) bool

// noChildSkip is a skipChildFn that doesn't skip any child when iterating a
// children list
var noChildSkip skipChildFn = func(_ int, _ c.ChildSpec) bool {
	return false
}

// skipChild is a skipChildFn that skips a child entry from an iteration that
// matches the given child.
func skipChild(ch c.Child) skipChildFn {
	chSpec := ch.GetSpec()
	return func(_ int, otherChSpec c.ChildSpec) bool {
		return chSpec.GetName() == otherChSpec.GetName()
	}
}

// TODO: use functions bellow for RestForOne strategy
//

// func skipChildBeforeIndex(i int) skipChildFn {
//	return func(j int, _ c.ChildSpec) bool {
//		return i < j
//	}
// }

// func getChildIndex(chs []c.ChildSpec, ch c.Child) (int, bool) {
//	chSpec := ch.GetSpec()
//	for i, other := range chs {
//		if chSpec.GetName() == other.GetName() {
//			return i, true
//		}
//	}
//	return 0, false
// }

////////////////////////////////////////////////////////////////////////////////

// startChildNode is responsible of starting a single child. This function will
// deal with the child lifecycle notification. It will return an error if
// something goes wrong with the initialization of this child.
func startChildNode(
	startCtx context.Context,
	supSpec SupervisorSpec,
	supRuntimeName string,
	notifyCh chan c.ChildNotification,
	chSpec c.ChildSpec,
) (c.Child, error) {
	eventNotifier := supSpec.getEventNotifier()
	startedTime := time.Now()
	ch, chStartErr := chSpec.DoStart(startCtx, supRuntimeName, notifyCh)

	// NOTE: The error handling code bellow gets executed when the children
	// fails at start time
	if chStartErr != nil {
		cRuntimeName := strings.Join(
			[]string{supRuntimeName, chSpec.GetName()},
			NodeSepToken,
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

// TODO: introduce shouldSkip when supporting RestForOne functionality

// startChildNodes iterates over all the children (specified with `cap.WithNodes`
// and `cap.WithSubtree`) starting a goroutine for each. The children iteration
// will be sorted as specified with the `cap.WithStartOrder` option. In case any child
// fails to start, the supervisor start operation will be aborted and all the
// started children so far will be stopped in the reverse order.
func startChildNodes(
	startCtx context.Context,
	supSpec SupervisorSpec,
	supChildrenSpecs []c.ChildSpec,
	supRuntimeName string,
	notifyCh chan c.ChildNotification,
) (map[string]c.Child, error) {
	children := make(map[string]c.Child)

	// Start children in the correct order
	for _, chSpec := range supSpec.order.sortStart(supChildrenSpecs) {
		// the function above will modify the children internally
		ch, chStartErr := startChildNode(
			startCtx,
			supSpec,
			supRuntimeName,
			notifyCh,
			chSpec,
		)
		if chStartErr != nil {
			// we must stop previously started children before we finish the supervisor
			nodeErrMap := terminateChildNodes(
				supSpec,
				supChildrenSpecs,
				children,
				noChildSkip,
			)
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

// terminateChildNode executes the Terminate procedure on the given child, in case there is
// an error on termination it notifies the event system
func terminateChildNode(
	eventNotifier EventNotifier,
	ch c.Child,
) error {
	chSpec := ch.GetSpec()
	stoppingTime := time.Now()
	isFirstTermination, terminationErr := ch.Terminate()

	// if it is not the first termination (it was terminated before, or finished because
	// of a failure), we have already made notice of this termination before, so we are
	// going to skip notifications.
	if !isFirstTermination {
		return nil
	}

	if terminationErr != nil {
		// we also notify that the process failed
		eventNotifier.processFailed(
			chSpec.GetTag(), ch.GetRuntimeName(), terminationErr,
		)
		return terminationErr
	}
	// we need to notify that the process stopped
	eventNotifier.processTerminated(chSpec.GetTag(), ch.GetRuntimeName(), stoppingTime)
	return nil
}

// terminateChildNodes is used on the shutdown of the supervisor tree, it stops
// children in the desired order.
func terminateChildNodes(
	supSpec SupervisorSpec,
	supChildrenSpecs0 []c.ChildSpec,
	supChildren map[string]c.Child,
	shouldSkip skipChildFn,
) map[string]error {
	eventNotifier := supSpec.eventNotifier
	supChildrenSpecs := supSpec.order.sortTermination(supChildrenSpecs0)
	supNodeErrMap := make(map[string]error)

	for i, chSpec := range supChildrenSpecs {
		if shouldSkip(i, chSpec) {
			continue
		}

		ch, ok := supChildren[chSpec.GetName()]
		// There are scenarios where is ok to ignore supChildren not having the
		// entry:
		//
		// * On start, there may be a failure mid-way in the initialization and on
		// the rollback we iterate over children supSpec that are not present in the
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
	supNodeErrMap := terminateChildNodes(
		supSpec,
		supChildrenSpecs,
		supChildren,
		noChildSkip,
	)
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
func runMonitorLoop(
	supCtx context.Context,
	supSpec SupervisorSpec,
	supChildrenSpecs []c.ChildSpec,
	supRuntimeName string,
	supTolerance *restartToleranceManager,
	supRscCleanup CleanupResourcesFn,
	supNotifyChan chan c.ChildNotification,
	ctrlChan chan ctrlMsg,
	supStartTime time.Time,
	onStart c.NotifyStartFn,
	onTerminate notifyTerminationFn,
) error {
	var startErr error
	var restartErr *RestartToleranceReached

	// Start children
	supChildren, startErr := startChildNodes(
		supCtx,
		supSpec,
		supChildrenSpecs,
		supRuntimeName,
		supNotifyChan,
	)
	if startErr != nil {
		// in case we run in the async strategy we notify the spawner that we
		// started with an error
		onStart(startErr)
		return startErr
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

		case chNotification := <-supNotifyChan:
			sourceCh, ok := supChildren[chNotification.GetName()]

			if !ok {
				// TODO: Expand on this case, I think this is highly unlikely, but would
				// like to exercise this branch in test somehow (if possible)
				panic(
					fmt.Errorf(
						"something horribly wrong happened here (name: %s, tag: %s)",
						sourceCh.GetRuntimeName(),
						sourceCh.GetTag(),
					),
				)
			}

			supChildren, restartErr = handleChildNodeNotification(
				supCtx,
				supTolerance,
				supSpec, supChildrenSpecs,
				supRuntimeName, supChildren, supNotifyChan,
				sourceCh, chNotification,
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

		case msg := <-ctrlChan:
			supChildrenSpecs, supChildren = handleCtrlMsg(
				supCtx,
				eventNotifier,
				supSpec,
				supChildrenSpecs,
				supRuntimeName,
				supChildren,
				supNotifyChan,
				msg,
			)
		}
	}
}
