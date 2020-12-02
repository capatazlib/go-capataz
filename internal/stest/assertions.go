package stest

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/capatazlib/go-capataz/cap"
)

func renderEvents(evs []cap.Event) string {
	var builder strings.Builder
	for i, ev := range evs {
		builder.WriteString(fmt.Sprintf("  %3d: %+v\n", i, ev))
	}
	return builder.String()
}

// verifyExactMatch is an utility function that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
func verifyExactMatch(preds []EventP, given []cap.Event) error {
	if len(preds) != len(given) {
		return fmt.Errorf(
			"Expecting exact match, but length is not the same:\nwant: %d\ngiven: %d\nevents:\n%s",
			len(preds),
			len(given),
			renderEvents(given),
		)
	}
	for i, pred := range preds {
		if !pred.Call(given[i]) {
			return fmt.Errorf(
				"Expecting exact match, but entry %d did not match:\ncriteria: %s\nevent: %s\nevents:\n%s",
				i,
				pred.String(),
				given[i].String(),
				renderEvents(given),
			)
		}
	}
	return nil
}

// AssertExactMatch is an assertion that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
func AssertExactMatch(t *testing.T, evs []cap.Event, preds []EventP) {
	t.Helper()
	err := verifyExactMatch(preds, evs)
	if err != nil {
		t.Error(err)
	}
}

// verifyPartialMatch is a utility function that matches (in order) a list of
// EventP predicates to a list of supervision system events.
//
// The supervision system events need to match in order all the list of given
// predicates, however, there does not need to be a one to one match between the
// input events and the predicates; we may have more input events and it is ok
// to skip some events in between matches.
//
// This function is useful when we want to test that some events are present in
// the expected order. This helps in test-cases where a supervision system emits
// an overwhelming number of events.
//
// This function returns all predicates that didn't match (in order) the given
// input events. If the returned slice is empty, it means there was a succesful
// match.
func verifyPartialMatch(preds []EventP, given []cap.Event) []EventP {
	for len(preds) > 0 {
		// if we went through all the given events, we did not partially match
		if len(given) == 0 {
			return preds
		}

		// if predicate matches given, we move forward on both
		// predicates and given
		if preds[0].Call(given[0]) {
			preds = preds[1:]
			given = given[1:]
		} else {
			// if predicate does not match, we move forward only
			// on given
			given = given[1:]
		}
	}

	// once preds is empty, we know we did all the partial matches
	return preds
}

// AssertPartialMatch is an assertion that matches in order a list of EventP
// predicates to a list of supervision system events.
//
// The input events need to match in the predicate order, but the events do not
// need to be a one to one match (e.g. the input events slice length may be
// bigger than the predicates slice length).
//
// This function is useful when we want to test that some events are present in
// the expected order. This is useful in test-cases where a supervision system
// emits an overwhelming number of events.
func AssertPartialMatch(t *testing.T, evs []cap.Event, preds []EventP) {
	t.Helper()
	pendingPreds := verifyPartialMatch(preds, evs)

	if len(pendingPreds) > 0 {
		pendingPredStrs := make([]string, 0, len(preds))
		for _, pred := range pendingPreds {
			pendingPredStrs = append(pendingPredStrs, pred.String())
		}

		evStrs := make([]string, 0, len(evs))
		for _, ev := range evs {
			evStrs = append(evStrs, ev.String())
		}

		t.Errorf(
			"Last match(es) didn't work - pending count: %d:\n%s\nInput events:\n%s",
			len(pendingPreds),
			strings.Join(pendingPredStrs, "\n"),
			strings.Join(evStrs, "\n"),
		)
	}
}

// ObserveDynSupervisor is an utility function that receives all the arguments
// required to build a DynSupervisor, and a callback that when executed will
// block until some point in the future (after we performed the side-effects we
// are testing). This function returns the list of events that happened in the monitored
// supervised tree, as well as any crash errors.
func ObserveDynSupervisor(
	ctx context.Context,
	rootName string,
	childNodes []cap.Node,
	opts0 []cap.Opt,
	callback func(cap.DynSupervisor, EventManager),
) ([]cap.Event, []error) {
	evManager := NewEventManager()
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
		evIt.SkipTill(SupervisorTerminated(rootName))
		return evManager.Snapshot(), errors
	}

	// NOTE: We execute SkipTill to make sure all the supervision tree got started
	// (or failed) before doing assertions/returning an error. Also, note we use
	// ProcessName instead of ProcessStarted/ProcessFailed given that ProcessName
	// matches an event in both success and error cases. The event from root must
	// be the last event reported
	evIt.SkipTill(ProcessName(rootName))

	// callback to do assertions with the event manager
	callback(sup, evManager)

	// once tests are done, we stop the supervisor
	terminateErr := sup.Terminate()

	// We wait till all the events have been reported (event from root must be the
	// last event)
	evIt.SkipTill(ProcessName(rootName))

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
	ctx context.Context,
	rootName string,
	buildNodes cap.BuildNodesFn,
	opts0 []cap.Opt,
	notifiers []cap.EventNotifier,
	callback func(EventManager),
) ([]cap.Event, error) {
	evManager := NewEventManager()
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

	// NOTE: We execute SkipTill to make sure all the supervision tree got started
	// (or failed) before doing assertions/returning an error. Also, note we use
	// ProcessName instead of ProcessStarted/ProcessFailed given that ProcessName
	// matches an event in both success and error cases. The event from root must
	// be the last event reported
	evIt.SkipTill(ProcessName(rootName))

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
	evIt.SkipTill(ProcessName(rootName))

	if terminateErr != nil {
		return evManager.Snapshot(), terminateErr
	}

	// return all the events reported by the supervision system
	return evManager.Snapshot(), nil
}
