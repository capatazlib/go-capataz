package cap

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

func oneForOneRestart(
	supCtx context.Context,
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan<- c.ChildNotification,
	prevCh c.Child,
) (c.Child, error) {
	chSpec := prevCh.GetSpec()
	chName := chSpec.GetName()

	startTime := time.Now()
	newCh, chRestartErr := chSpec.DoStart(supCtx, supRuntimeName, supNotifyCh)

	if chRestartErr != nil {
		return c.Child{}, chRestartErr
	}

	supChildren[chName] = newCh

	// notify event only for workers, supervisors are responsible of their own
	if newCh.GetTag() == c.Worker {
		eventNotifier.workerStarted(newCh.GetRuntimeName(), startTime)
	}
	return newCh, nil
}

func oneForOneRestartLoop(
	supCtx context.Context,
	eventNotifier EventNotifier,
	supRuntimeName string,
	supTolerance *errToleranceManager,
	supChildren map[string]c.Child,
	supNotifyCh chan<- c.ChildNotification,
	prevCh c.Child,
	prevErr error,
) *c.ErrorToleranceReached {
	for {
		if prevErr != nil {
			ok := supTolerance.checkTolerance()
			if !ok {
				// Remove children from runtime child map to skip terminate procedure
				delete(supChildren, prevCh.GetName())
				return c.NewErrorToleranceReached(supTolerance.errTolerance, prevErr, prevCh)
			}
		}

		newCh, restartErr := oneForOneRestart(
			supCtx,
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevCh,
		)

		// if we don't get start errors, break the loop
		if restartErr == nil {
			return nil
		}

		// otherwise, repeat until error threshold is met
		prevCh = newCh
		prevErr = restartErr
	}
}
