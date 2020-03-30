package s

import (
	"errors"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

func oneForOneRestart(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan<- c.ChildNotification,
	prevChild c.Child,
) (c.Child, error) {
	chSpec := prevChild.GetSpec()
	chName := chSpec.GetName()

	startTime := time.Now()
	newCh, restartErr := prevChild.Restart(supRuntimeName, supNotifyCh)

	if restartErr != nil {
		return c.Child{}, restartErr
	}

	// We want to keep track of the updated restartCount which is in the newCh
	// record, we must override the child independently of the outcome.
	supChildren[chName] = newCh

	if newCh.GetTag() == c.Worker {
		eventNotifier.WorkerStarted(newCh.GetRuntimeName(), startTime)
	}
	return newCh, nil
}

func oneForOneRestartLoop(
	eventNotifier EventNotifier,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan<- c.ChildNotification,
	prevChild c.Child,
) *c.ErrorToleranceReached {
	for {
		newCh, restartErr := oneForOneRestart(
			eventNotifier,
			supRuntimeName,
			supChildren,
			supNotifyCh,
			prevChild,
		)
		// if we don't get start errors, break the loop
		if restartErr == nil {
			return nil
		}

		// The restartError could be that the error threshold was reached
		// or that there was a start error

		// in case of error tolerance reached, just fail
		var toleranceErr *c.ErrorToleranceReached
		if errors.As(restartErr, &toleranceErr) {
			// Remove children from runtime child map to skip terminate procedure
			delete(supChildren, prevChild.GetName())
			return toleranceErr
		}

		// otherwise, repeat until error threshold is met
		prevChild = newCh
	}
}
