package s

// This file contains logic on supervision sub-trees

import (
	"context"

	"github.com/capatazlib/go-capataz/c"
)

// subtreeMain contains the main logic of the Child spec that runs a supervision
// sub-tree. It returns an error if the child supervisor fails to start.
func subtreeMain(
	parentName string,
	spec SupervisorSpec,
) func(context.Context, c.NotifyStartFn) error {
	// we use the start version that receives the notifyChildStart callback, this
	// is essential, as we need this callback to signal the sub-tree children have
	// started before signaling we have started
	return func(parentCtx context.Context, notifyChildStart c.NotifyStartFn) error {
		// in this function we use the private versions of run given we don't want
		// to spawn yet another goroutine
		ctx, cancelFn := context.WithCancel(parentCtx)
		defer cancelFn()
		return spec.run(ctx, parentName, notifyChildStart)
	}
}

// subtree allows to register a Supervisor Spec as a sub-tree of a bigger
// Supervisor Spec. The sub-tree is executed in a `c.Child` goroutine, ergo, the
// returned `c.ChildSpec` is going to contain the supervisor internally.
func (spec SupervisorSpec) subtree(
	subtreeSpec SupervisorSpec,
	copts0 ...c.Opt,
) c.ChildSpec {
	subtreeSpec.eventNotifier = spec.eventNotifier

	// NOTE: Child goroutines that are running a sub-tree supervisor must always
	// have a timeout of Infinity, as specified in the documentation from OTP
	// http://erlang.org/doc/design_principles/sup_princ.html#child-specification
	copts := append(copts0, c.WithShutdown(c.Inf))

	return c.NewWithNotifyStart(
		subtreeSpec.Name(),
		subtreeMain(spec.name, subtreeSpec),
		copts...,
	)
}
