package smtest

import (
	"strings"
	"fmt"
	"testing"
)


func renderEvents[A any](evs []A) string {
	var builder strings.Builder
	for i, ev := range evs {
		builder.WriteString(fmt.Sprintf("  %3d: %+v\n", i, ev))
	}
	return builder.String()
}

// verifyExactMatch is an utility function that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
func verifyExactMatch[A fmt.Stringer](preds []EventP[A], given []A) error {
	if len(preds) != len(given) {
		return fmt.Errorf(
			"expecting exact match, but length is not the same:\nwant: %d\ngiven: %d\nevents:\n%s",
			len(preds),
			len(given),
			renderEvents(given),
		)
	}
	for i, pred := range preds {
		if !pred.Call(given[i]) {
			return fmt.Errorf(
				"expecting exact match, but entry %d did not match:\ncriteria: %s\nevent: %s\nevents:\n%s",
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
func AssertExactMatch[A fmt.Stringer](t *testing.T, evs []A, preds []EventP[A]) {
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
func verifyPartialMatch[A fmt.Stringer](preds []EventP[A], given []A) []EventP[A] {
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
func AssertPartialMatch[A fmt.Stringer](t *testing.T, evs []A, preds []EventP[A]) {
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


