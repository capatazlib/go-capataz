package s_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

////////////////////////////////////////////////////////////////////////////////

// EventP represents a predicate function that allows us to assert properties of
// an Event signaled by the supervision system
type EventP interface {

	// Call will execute the logic of this event predicate
	Call(s.Event) bool

	// Returns an string representation of this event predicate (for debugging
	// purposes)
	String() string
}

// EventTagP is a predicate that asserts the `s.EventTag` of a given `s.Event`
// matches an expected `s.EventTag`
type EventTagP struct {
	tag s.EventTag
}

func (p EventTagP) Call(ev s.Event) bool {
	return ev.Tag() == p.tag
}

func (p EventTagP) String() string {
	return fmt.Sprintf("tag == %s", p.tag.String())
}

// ProcessNameP is a predicate that asserts the name of the `Child` that
// triggered the event matches the expected name
type ProcessNameP struct {
	name string
}

func (p ProcessNameP) Call(ev s.Event) bool {
	return ev.Name() == p.name
}

func (p ProcessNameP) String() string {
	return fmt.Sprintf("name == %s", p.name)
}

// AndP is a predicate that builds the conjunction of a group EventP predicates
// (e.g. join EventP predicates with &&)
type AndP struct {
	preds []EventP
}

// Call will try and verify that all it's grouped predicates return true, if any
// returns false, this predicate function will return false
func (p AndP) Call(ev s.Event) bool {
	acc := true
	for _, pred := range p.preds {
		acc = acc && pred.Call(ev)
		if !acc {
			return acc
		}
	}
	return acc
}

func (p AndP) String() string {
	acc := make([]string, 0, len(p.preds))
	for _, pred := range p.preds {
		acc = append(acc, pred.String())
	}
	return strings.Join(acc, " && ")
}

// ProcessStarted is a predicate to assert an event represents a process that
// got started
func ProcessStarted(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessStarted},
			ProcessNameP{name: name},
		},
	}
}

// ProcessStopped is a predicate to assert an event represents a process that
// got stopped by its parent supervisor
func ProcessStopped(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessStopped},
			ProcessNameP{name: name},
		},
	}
}

// NOTE: Leaving this for later
//
// func ProcessFailed(name string) EventP {
//	return AndP{
//		preds: []EventP{
//			EventTagP{tag:s.ProcessFailed},
//			ProcessNameP{name:name},
//		}
//	}
// }

////////////////////////////////////////////////////////////////////////////////

// noWait is a notification callback function that does nothing
func noWait() {}

// startEventCollector starts a routine that collects all the events sent to the
// given channel. It then returns a function that when called, waits for the
// routine to finish and returns a slice with all the events captured in the
// supervision system
func startEventCollector(evCh chan s.Event) func() []s.Event {
	buffer := make([]s.Event, 0, 10)

	var wg sync.WaitGroup
	wg.Add(1)

	// This goro will be listening to the supervision-system events, and it will
	// collect them until the event channel is closed, this will happen once the
	// returned function gets called
	go func() {
		defer wg.Done()
		for ev := range evCh {
			buffer = append(buffer, ev)
		}
	}()

	// This function will wait until the collector is done, and return the buffer
	// that accumulated all the events reported by the supervision system
	return func() []s.Event {
		close(evCh)
		wg.Wait()
		return buffer
	}
}

// newCollectorNotifier creates an EventNotifier that collects all the events
// registered by a supervision system execution. This notifier is used to assert
// the behavior of a supervision system. It finally returns a blocking function
// that returns a collection with the events reported by the system once it has
// finished.
func newCollectorNotifier() (s.EventNotifier, func() []s.Event) {
	evCh := make(chan s.Event)
	notifier := func(ev s.Event) {
		evCh <- ev
	}
	collectEvents := startEventCollector(evCh)
	return notifier, collectEvents
}

// observeSupervisor is an utility function that receives all the arguments
// required to build a SupervisorSpec, and a callback that when executed will
// block until some point in the future (after we performed the side-effects we
// are testing).
func observeSupervisor(
	supName string,
	supOpts0 []s.Opt,
	waitStopSignal func(),
) ([]s.Event, error) {
	evCollector, collectEvents := newCollectorNotifier()

	supOpts := append([]s.Opt{s.WithNotifier(evCollector)}, supOpts0...)
	supSpec := s.New(supName, supOpts...)

	sup, err := supSpec.Start(context.TODO())
	if err != nil {
		return nil, err
	}

	waitStopSignal()

	err = sup.Stop()
	if err != nil {
		return nil, err
	}

	return collectEvents(), nil
}

// verifyExactMatch is an utility function that checks the input slide of EventP
// predicate match 1 to 1 with an input list of supervision system events.
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

// assertExactMatch is an assertion that checks the input slide of EventP
// predicate match 1 to 1 with an input list of supervision system events.
func assertExactMatch(t *testing.T, evs []s.Event, preds []EventP) {
	t.Helper()
	err := verifyExactMatch(preds, evs)
	if err != nil {
		t.Error(err)
	}
}

// // verifyPartialMatch is an utility function that matches in order a list of
// // EventP predicates to a list of supervision system events. The input events
// // need to match in order, but the events do not need to be a 1 to 1 match (e.g.
// // the input events slice length may be bigger than the predicates slice
// // length). This function is useful when we want to test that _some_ events are
// // present in the expected order in a noisy system.
// func verifyPartialMatch(preds []EventP, given []s.Event) []EventP {
//	for len(preds) > 0 {
//		// if we went through all the given events, we did not partially match
//		if len(given) == 0 {
//			return preds
//		}

//		// if predicate matches given, we move forward on both
//		// predicates and given
//		if preds[0].Call(given[0]) {
//			preds = preds[1:]
//			given = given[1:]
//		} else {
//			// if predicate does not match, we move forward only
//			// on given
//			given = given[1:]
//		}
//	}

//	// once preds is empty, we know we did all the partial matches
//	return preds
// }

// // assertPartialMatch is an assertion that matches in order a list of EventP
// // predicates to a list of supervision system events. The input events need to
// // match in order, but the events do not need to be a 1 to 1 match (e.g. the
// // input events slice length may be bigger than the predicates slice length).
// // This function is useful when we want to test that _some_ events are present
// // in the expected order in a noisy system.
// func assertPartialMatch(t *testing.T, evs []s.Event, preds []EventP) {
//	t.Helper()
//	pendingPreds := verifyPartialMatch(preds, evs)

//	if len(pendingPreds) > 0 {
//		pendingPredStrs := make([]string, 0, len(preds))
//		for _, pred := range pendingPreds {
//			pendingPredStrs = append(pendingPredStrs, pred.String())
//		}

//		evStrs := make([]string, 0, len(evs))
//		for _, ev := range evs {
//			evStrs = append(evStrs, ev.String())
//		}

//		if len(pendingPreds) == 1 {
//			t.Errorf(
//				"Last match didn't work:\n%s\nInput events:\n%s",
//				strings.Join(pendingPredStrs, "\n"),
//				strings.Join(evStrs, "\n"),
//			)
//		} else {
//			t.Errorf(
//				"Last %d matches didn't work:\n%s\nInput events:\n%s",
//				len(pendingPreds),
//				strings.Join(pendingPredStrs, "\n"),
//				strings.Join(evStrs, "\n"),
//			)
//		}
//	}
// }

// assertPredMatchesN is an assertion that verifies a given Predicate matches
// exactly N times in a list of supervision system events slice.
func assertPredMatchesN(t *testing.T, n int, evs []s.Event, pred EventP) {
	t.Helper()

	acc := 0
	for _, ev := range evs {
		if pred.Call(ev) {
			acc += 1
		}
	}
	if n != acc {
		evStrs := make([]string, 0, len(evs))
		for _, ev := range evs {
			evStrs = append(evStrs, ev.String())
		}
		t.Errorf(
			"Expecting pred to match %d times, it matched %d instead\nPred:%s\nInput events:%s",
			n,
			acc,
			pred.String(),
			strings.Join(evStrs, "\n"),
		)
	}
}

////////////////////////////////////////////////////////////////////////////////

// waitDoneChild creates a `ChildSpec` that runs a goroutine that will block
// until the `Done` channel of given `context.Context` returns
func waitDoneChild(name string) c.Spec {
	cspec := c.New(name, func(ctx context.Context) error {
		// In real-world code, here we would have some business logic. For this
		// particular scenario, we want to block until we get a stop notification
		// from our parent supervisor and return `nil`
		<-ctx.Done()
		return nil
	})
	return cspec
}

/*
// Leaving these utilities here for later, not sure if I'm going to need them

func blockingChild(name string, waitCb func()) (c.Spec, func()) {
	terminateCh := make(chan struct{})
	cspec, _ := c.New(name, func(_ context.Context) error {
		defer close(terminateCh)
		if waitCb != nil {
			waitCb()
		}
		return nil
	})
	return cspec, func() { <-terminateCh }
}

func orderedChidren(n int) ([]c.Spec, func()) {
	acc := make([]c.Spec, 0, n)
	var cspec c.Spec
	var waitSignal func()

	for i := 0; i < n; i++ {
		name := fmt.Sprintf("child%d", i)
		cspec, waitSignal = blockingChild(name, waitSignal)
		acc = append(acc, cspec)
	}

	return acc, waitSignal
}

func signalChild(
	name string,
	start0 func(context.Context, func()) error,
	opts ...c.Opt,
) (c.Spec, func()) {
	terminatedCh := make(chan struct{})
	start := func(ctx context.Context) error {
		return start0(ctx, func() { close(terminatedCh) })
	}
	cs, _ := c.New(name, start, opts...)
	return cs, func() { <-terminatedCh }
}
*/

////////////////////////////////////////////////////////////////////////////////
