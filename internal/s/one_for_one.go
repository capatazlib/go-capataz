package s

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

var oneForOneRestart strategyRestartFn = func(
	supCtx context.Context,
	spec SupervisorSpec, supChildrenSpecs []c.ChildSpec,

	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyChan chan c.ChildNotification,

	sourceCh c.Child,
) (map[string]c.Child, error) {
	eventNotifier := spec.getEventNotifier()

	chSpec := sourceCh.GetSpec()
	chName := chSpec.GetName()

	startTime := time.Now()
	newCh, chRestartErr := chSpec.DoStart(supCtx, supRuntimeName, supNotifyChan)

	if chRestartErr != nil {
		// Very important! even though we return an error value here, we want to
		// return a supChildren, this collection gets replaced on every iteration,
		// and if we return a nil value, all children won't get terminated
		// appropietly.
		return supChildren, chRestartErr
	}

	supChildren[chName] = newCh

	// notify event only for workers, supervisors are responsible of their
	// own notifications
	if newCh.GetTag() == c.Worker {
		eventNotifier.workerStarted(newCh.GetRuntimeName(), startTime)
	}
	return supChildren, nil
}
