package stest

import (
	"context"

	"github.com/capatazlib/go-capataz/cap"
	"github.com/capatazlib/go-capataz/smtest"
)

// ObserveDynSupervisor is an utility function that receives all the arguments
// required to build a DynSupervisor, and a callback that when executed will
// block until some point in the future (after we performed the side-effects we
// are testing). This function returns the list of events that happened in the monitored
// supervised tree, as well as any crash errors.
func ObserveDynSupervisor(
	ctx0 context.Context,
	rootName string,
	childNodes []cap.Node,
	opts0 []cap.Opt,
	callback func(cap.DynSupervisor, EventManager),
) ([]cap.Event, []error) {
	ctx, done := context.WithCancel(ctx0)
	defer done()

	evManager := smtest.NewEventManager[cap.Event]()
	// Accumulate the events as they happen
	evManager.StartCollector(ctx)

	// Create a new Supervisor Opts that adds the EventManager's Notifier at the
	// very beginning of the system setup, the order here is important as it
	// propagates to sub-trees specified in this options
	opts := append([]cap.Opt{
		cap.WithNotifier(evManager.EventCollector(ctx)),
	}, opts0...)

	// We always want to start the supervisor for test purposes, so this is
	// embedded in the ObserveDynSupervisor call
	sup, startErr := cap.NewDynSupervisor(ctx, rootName, opts...)

	if startErr != nil {
		return evManager.Snapshot(), []error{startErr}
	}

	errors := []error{}

	// start procedurally the given children
	for _, node := range childNodes {
		_, spawnErr := sup.Spawn(node)
		if spawnErr != nil {
			errors = append(errors, spawnErr)
		}
	}

	evIt := evManager.Iterator()

	if len(errors) != 0 {
		// once tests are done, we stop the supervisor
		if terminateErr := sup.Terminate(); terminateErr != nil {
			errors = append(errors, terminateErr)
		}
		evIt.WaitTill(SupervisorTerminated(rootName))
		return evManager.Snapshot(), errors
	}

	// NOTE: We execute WaitTill to make sure all the supervision tree got
	// started (or failed) before doing assertions/returning an error. Also,
	// note we use ProcessName instead of ProcessStarted/ProcessFailed given
	// that ProcessName matches an event in both success and error cases. The
	// event from root must be the last event reported
	evIt.WaitTill(ProcessName(rootName))

	// callback to do assertions with the event manager
	callback(sup, evManager)

	// once tests are done, we stop the supervisor
	terminateErr := sup.Terminate()

	// We wait till all the events have been reported (event from root must be the
	// last event)
	evIt.WaitTill(ProcessName(rootName))

	if terminateErr != nil {
		return evManager.Snapshot(), []error{terminateErr}
	}

	// return all the events reported by the supervision system
	return evManager.Snapshot(), nil
}

// ObserveSupervisor is an utility function that receives all the arguments
// required to build a SupervisorSpec, and a callback that when executed will
// block until some point in the future (after we performed the side-effects we
// are testing). This function returns the list of events that happened in the
// monitored supervised tree, as well as any crash errors.
func ObserveSupervisor(
	ctx context.Context,
	rootName string,
	buildNodes cap.BuildNodesFn,
	opts0 []cap.Opt,
	callback func(EventManager),
) ([]cap.Event, error) {
	return ObserveSupervisorWithNotifiers(
		ctx,
		rootName,
		buildNodes,
		opts0,
		[]cap.EventNotifier{},
		callback,
	)
}

// mergeNotifiers is a naive implementation of merged notifiers. It simply
// concatenates the invocations. This should be sufficient for some simple
// validation tests
func mergeNotifiers(notifiers []cap.EventNotifier) cap.EventNotifier {
	if len(notifiers) == 0 {
		return func(cap.Event) {}
	}
	if len(notifiers) == 1 {
		return notifiers[0]
	}

	return func(ev cap.Event) {
		for _, n := range notifiers {
			n(ev)
		}
	}
}

// ObserveSupervisorWithNotifiers is an utility function that receives all the arguments
// required to build a SupervisorSpec, and a callback that when executed will
// block until some point in the future (after we performed the side-effects we
// are testing). This function returns the list of events that happened in the
// monitored supervised tree, as well as any crash errors.
func ObserveSupervisorWithNotifiers(
	ctx0 context.Context,
	rootName string,
	buildNodes cap.BuildNodesFn,
	opts0 []cap.Opt,
	notifiers []cap.EventNotifier,
	callback func(EventManager),
) ([]cap.Event, error) {
	ctx, stop := context.WithCancel(ctx0)
	defer stop()

	evManager := smtest.NewEventManager[cap.Event]()
	// Accumulate the events as they happen
	evManager.StartCollector(ctx)

	// Merge the supplied notifiers with the event collector
	mergedNotifiers := cap.WithNotifier(
		mergeNotifiers(
			append([]cap.EventNotifier{evManager.EventCollector(ctx)}, notifiers...),
		),
	)

	// Create a new Supervisor Opts that adds the EventManager's Notifier at the
	// very beginning of the system setup, the order here is important as it
	// propagates to sub-trees specified in this options
	opts := append([]cap.Opt{mergedNotifiers}, opts0...)
	supSpec := cap.NewSupervisorSpec(rootName, buildNodes, opts...)

	// We always want to start the supervisor for test purposes, so this is
	// embedded in the ObserveSupervisor call
	sup, startErr := supSpec.Start(ctx)

	evIt := evManager.Iterator()

	// NOTE: We execute WaitTill to make sure all the supervision tree got started
	// (or failed) before doing assertions/returning an error. Also, note we use
	// ProcessName instead of ProcessStarted/ProcessFailed given that ProcessName
	// matches an event in both success and error cases. The event from root must
	// be the last event reported
	evIt.WaitTill(ProcessName(rootName))

	if startErr != nil {
		callback(evManager)
		return evManager.Snapshot(), startErr
	}

	// callback to do assertions with the event manager
	callback(evManager)

	// once tests are done, we stop the supervisor
	terminateErr := sup.Terminate()

	// We wait till all the events have been reported (event from root must be the
	// last event)
	evIt.WaitTill(ProcessName(rootName))

	if terminateErr != nil {
		return evManager.Snapshot(), terminateErr
	}

	// return all the events reported by the supervision system
	return evManager.Snapshot(), nil
}
