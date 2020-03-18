package s

// This file contains the start/stop children logic

import (
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

func stopChild(
	eventNotifier EventNotifier,
	supChildErrMap map[string]error,
	ch c.Child,
) map[string]error {
	chSpec := ch.Spec()
	stoppingTime := time.Now()
	terminationErr := ch.Stop()

	if terminationErr != nil {
		// if a child fails to stop (either because of a legit failure or a
		// timeout), we store the terminationError so that we can report all of them
		// later
		supChildErrMap[chSpec.Name()] = terminationErr

		// we also notify that the process failed
		eventNotifier.ProcessFailed(chSpec.Tag(), ch.RuntimeName(), terminationErr)
	} else {
		// we need to notify that the process stopped
		eventNotifier.ProcessStopped(chSpec.Tag(), ch.RuntimeName(), stoppingTime)
	}

	return supChildErrMap
}

// stopChildren is used on the shutdown of the supervisor tree, it stops
// supChildren in the desired order. The starting argument indicates if the
// supervision tree is starting, if that is the case, it is more permisive
// around spec supChildren not matching one to one with it's corresponding runtime
// supChildren, this may happen because we had a start error in the middle of
// supervision tree initialization, and we never got to initialize all supChildren
// at this supervision level.
func stopChildren(
	spec SupervisorSpec,
	supChildren map[string]c.Child,
	starting bool,
) map[string]error {
	eventNotifier := spec.eventNotifier
	childrenSpecs := spec.order.SortStop(spec.children)
	supChildErrMap := make(map[string]error)

	for _, cs := range childrenSpecs {
		ch, ok := supChildren[cs.Name()]
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
			supChildErrMap = stopChild(eventNotifier, supChildErrMap, ch)
		}
	}
	return supChildErrMap
}

// startChildren iterates over all the children (specified with `s.WithChildren`
// and `s.WithSubtree`) starting a goroutine for each. The children iteration
// will be sorted as specified with the `s.WithOrder` option. In case any child
// fails to start, the supervisor start operation will be aborted and all the
// started children so far will be stopped in the reverse order.
func startChildren(
	spec SupervisorSpec,
	runtimeName string,
	notifyCh chan c.ChildNotification,
) (map[string]c.Child, error) {
	eventNotifier := spec.getEventNotifier()
	children := make(map[string]c.Child)

	// Start children
	for _, cs := range spec.order.SortStart(spec.children) {
		startedTime := time.Now()
		ch, startErr := cs.Start(runtimeName, notifyCh)

		// NOTE: The error handling code bellow gets executed when the children
		// fails at start time
		if startErr != nil {
			cRuntimeName := strings.Join([]string{runtimeName, cs.Name()}, childSepToken)
			eventNotifier.ProcessStartFailed(cs.Tag(), cRuntimeName, startErr)
			childErrMap := stopChildren(spec, children, true /* starting? */)
			// Is important we stop the children before we finish the supervisor
			return nil, SupervisorError{
				err:         startErr,
				runtimeName: runtimeName,
				childErrMap: childErrMap,
			}
		}

		// NOTE: we only notify when child is a worker because sub-trees supervisors
		// are responsible of their own notification
		if cs.IsWorker() {
			eventNotifier.WorkerStarted(ch.RuntimeName(), startedTime)
		}
		children[cs.Name()] = ch
	}
	return children, nil
}
