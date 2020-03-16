package s

// This file contains the start/stop children logic

import (
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

// stopChildren is used on the shutdown of the supervisor tree, it stops
// children in the desired order. The starting argument indicates if the
// supervision tree is starting, if that is the case, it is more permisive
// around spec children not matching one to one with it's corresponding runtime
// children, this may happen because we had a start error in the middle of
// supervision tree initialization, and we never got to initialize all children
// at this supervision level.
func stopChildren(
	spec SupervisorSpec,
	children map[string]c.Child,
	starting bool,
) map[string]error {
	eventNotifier := spec.eventNotifier
	childrenSpecs := spec.order.SortStop(spec.children)
	childErrMap := make(map[string]error)

	for _, cs := range childrenSpecs {
		ch, ok := children[cs.Name()]
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
		stoppingTime := time.Now()
		terminationErr := ch.Stop()

		if terminationErr != nil {
			// if a child fails to stop (either because of a legit failure or a
			// timeout), we store the terminationError so that we can report all of them
			// later
			childErrMap[cs.Name()] = terminationErr

			// we also notify that the process failed
			eventNotifier.ProcessFailed(ch.RuntimeName(), terminationErr)
		} else {
			// we need to modify that the process stopped
			eventNotifier.ProcessStopped(ch.RuntimeName(), stoppingTime)
		}
	}
	return childErrMap
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
			eventNotifier.ProcessStartFailed(cRuntimeName, startErr)
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
			eventNotifier.ProcessStarted(ch.RuntimeName(), startedTime)
		}
		children[cs.Name()] = ch
	}
	return children, nil
}
