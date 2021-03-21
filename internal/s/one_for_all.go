package s

import (
	"context"
	"github.com/capatazlib/go-capataz/internal/c"
)

func oneForAllRestart(
	supCtx context.Context,
	spec SupervisorSpec, supChildrenSpecs []c.ChildSpec,

	supRuntimeName string,
	supChildren0 map[string]c.Child,
	supNotifyChan chan c.ChildNotification,

	sourceCh c.Child,
) (map[string]c.Child, error) {
	// we do not want to stop the restart procedure if a termination fails,
	// nonetheless, this error is not going unnoticed given the event
	// notifier gets called on child termination.
	_ /* nodeErrMap */ = terminateChildNodes(
		spec, supChildrenSpecs, supChildren0, skipChild(sourceCh),
	)

	return startChildNodes(
		supCtx,
		spec,
		supChildrenSpecs,
		supRuntimeName,
		supNotifyChan,
	)
}
