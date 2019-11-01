package stest

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

// verifyExactMatch is an utility function that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
func verifyExactMatch(preds []EventP, given []s.Event) error {
	if len(preds) != len(given) {
		return fmt.Errorf(
			"Expecting exact match, but length is not the same:\nwant %d\ngiven: %d",
			len(preds),
			len(given),
		)
	}
	for i, pred := range preds {
		if !pred.Call(given[i]) {
			return fmt.Errorf(
				"Expecting exact match, but entry %d did not match:\ncriteria:%s\nevent:%s",
				i,
				pred.String(),
				given[i].String(),
			)
		}
	}
	return nil
}

// AssertExactMatch is an assertion that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
func AssertExactMatch(t *testing.T, evs []s.Event, preds []EventP) {
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
// predicates, however, there has not to be a one to one match between the input
// events and the predicates; we may have more input events and it is ok to
// skip some events in between matches.
//
// This function is useful when we want to test that some events are present in
// the expected order. This is useful in test-cases where a supervision system
// emits an overwhelming number of events.
//
// This function returns all predicates that didn't match (in order) the given
// input events. If the returned slice is empty, it means there was a succesful
// match.
func verifyPartialMatch(preds []EventP, given []s.Event) []EventP {
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
// The input events need to match in the predicate corder, but the events do not
// need to be a one to one match (e.g. the input events slice length may be bigger
// than the predicates slice length).
//
// This function is useful when we want to test that some events are present in
// the expected order. This is useful in test-cases where a supervision system
// emits an overwhelming number of events.
func AssertPartialMatch(t *testing.T, evs []s.Event, preds []EventP) {
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

// WaitDoneChild creates a `ChildSpec` that runs a goroutine that will block
// until the `Done` channel of given `context.Context` returns
func WaitDoneChild(name string) c.ChildSpec {
	cspec := c.New(name, func(ctx context.Context) error {
		// In real-world code, here we would have some business logic. For this
		// particular scenario, we want to block until we get a stop notification
		// from our parent supervisor and return `nil`
		<-ctx.Done()
		return nil
	})
	return cspec
}

// ObserveSupervisor is an utility function that receives all the arguments
// required to build a SupervisorSpec, and a callback that when executed will
// block until some point in the future (after we performed the side-effects we
// are testing).
func ObserveSupervisor(
	ctx context.Context,
	rootName string,
	opts0 []s.Opt,
	callback func(EventManager),
) ([]s.Event, error) {
	evManager := NewEventManager()
	// Accumulate the events as they happen
	evManager.StartCollector(ctx)

	// Create a new Supervisor Opts that adds the EventManager's Notifier at the
	// very beginning of the system setup, the order here is important as it
	// propagates to sub-trees specified in this options
	opts := append([]s.Opt{
		s.WithNotifier(evManager.EventCollector(ctx)),
	}, opts0...)
	spec := s.New(rootName, opts...)

	// We always want to start the supervisor for test purposes, so this is
	// embedded in the ObserveSupervisor call
	sup, err := spec.Start(ctx)
	if err != nil {
		return []s.Event{}, err
	}

	evIt := evManager.Iterator()

	// Make sure all the tree started before doing assertions
	evIt.SkipTill(ProcessStarted(rootName))

	// callback to do assertions with the event manager
	callback(evManager)

	// once tests are done, we stop the supervisor
	sup.Stop()
	// we wait till all the events have been reported
	evIt.SkipTill(ProcessStopped(rootName))

	// return all the events reported by the supervision system
	return evManager.Snapshot(), nil
}
