package capataz

// This file contains logic on supervision sub-trees

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// run performs the main logic of a Supervisor. This function:
//
// 1) spawns each child goroutine in the correct order
//
// 2) stops all the spawned children in the correct order once it gets a stop
// signal
//
// 3) it monitors and reacts to errors reported by the supervised children
//
func (spec SupervisorSpec) run(
	ctx context.Context,
	parentName string,
	onStart c.NotifyStartFn,
) error {
	// Build childrenSpec and resource cleanup
	supChildrenSpecs, supRscCleanup, rscAllocError := spec.buildChildrenSpecs()

	// Do not even start the monitor loop if we find an error on the resource
	// allocation logic
	if rscAllocError != nil {
		onStart(rscAllocError)
		return rscAllocError
	}

	// notifyCh is used to keep track of errors from children
	notifyCh := make(chan c.ChildNotification)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	// ctrlCh := make(chan ControlMsg)

	supRuntimeName := buildRuntimeName(spec, parentName)

	onTerminate := func(err terminateError) {}

	startTime := time.Now()
	// spawn goroutine with supervisor monitorLoop
	return runMonitorLoop(
		ctx,
		spec,
		supChildrenSpecs,
		supRuntimeName,
		supRscCleanup,
		notifyCh,
		// ctrlCh,
		startTime,
		onStart,
		onTerminate,
	)
}

// subtreeMain contains the main logic of the Child spec that runs a supervision
// sub-tree. It returns an error if the child supervisor fails to start.
func subtreeMain(
	parentName string,
	supSpec SupervisorSpec,
) func(context.Context, c.NotifyStartFn) error {
	// we use the start version that receives the notifyChildStart callback, this
	// is essential, as we need this callback to signal the sub-tree children have
	// started before signaling we have started
	return func(parentCtx context.Context, notifyChildStart c.NotifyStartFn) error {
		// in this function we use the private versions of run given we don't want
		// to spawn yet another goroutine
		ctx, cancelFn := context.WithCancel(parentCtx)
		defer cancelFn()
		return supSpec.run(ctx, parentName, notifyChildStart)
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
	copts := append(
		copts0,
		c.WithShutdown(c.Indefinitely),
		c.WithTag(c.Supervisor),
		c.WithTolerance(1, 5*time.Second),
	)

	return c.NewWithNotifyStart(
		subtreeSpec.GetName(),
		subtreeMain(spec.name, subtreeSpec),
		copts...,
	)
}

// Subtree transforms SupervisorSpec into a Node.
//
// Note the subtree SupervisorSpec is going to inherit the event notifier from
// its parent supervisor.
func Subtree(subtreeSpec SupervisorSpec, opts ...WorkerOpt) Node {
	return func(supSpec SupervisorSpec) c.ChildSpec {
		return supSpec.subtree(subtreeSpec, opts...)
	}
}
