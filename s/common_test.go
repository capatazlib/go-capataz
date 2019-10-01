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

type EventP interface {
	Call(s.Event) bool
	String() string
}

type EventTagP struct {
	tag s.EventTag
}

func (p EventTagP) Call(ev s.Event) bool {
	return ev.Tag() == p.tag
}

func (p EventTagP) String() string {
	return fmt.Sprintf("tag == %s", p.tag.String())
}

type ProcessNameP struct {
	name string
}

func (p ProcessNameP) Call(ev s.Event) bool {
	return ev.Name() == p.name
}

func (p ProcessNameP) String() string {
	return fmt.Sprintf("name == %s", p.name)
}

type AndP struct {
	preds []EventP
}

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

func ProcessStarted(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessStarted},
			ProcessNameP{name: name},
		},
	}
}

func ProcessStopped(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessStopped},
			ProcessNameP{name: name},
		},
	}
}

// func ProcessFailed(name string) EventP {
//	return AndP{
//		preds: []EventP{
//			EventTagP{tag:s.ProcessFailed},
//			ProcessNameP{name:name},
//		}
//	}
// }

////////////////////////////////////////////////////////////////////////////////

func noWait() {}

func startEventCollector(evCh chan s.Event) func() []s.Event {
	buffer := make([]s.Event, 0, 10)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for ev := range evCh {
			buffer = append(buffer, ev)
		}
	}()

	return func() []s.Event {
		close(evCh)
		wg.Wait()
		return buffer
	}
}

func newCollectorNotifier() (s.EventNotifier, func() []s.Event) {
	evCh := make(chan s.Event)
	notifier := func(ev s.Event) {
		evCh <- ev
	}
	collectEvents := startEventCollector(evCh)
	return notifier, collectEvents
}

func observeSupervisor(
	supName string,
	supOpts0 []s.Opt,
	waitStopSignal func(),
) ([]s.Event, error) {
	evCollector, collectEvents := newCollectorNotifier()

	supOpts := append([]s.Opt{s.WithNotifier(evCollector)}, supOpts0...)
	supSpec, err := s.New(supName, supOpts...)
	if err != nil {
		return nil, err
	}

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

func assertPartialMatch(t *testing.T, evs []s.Event, preds []EventP) {
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

		if len(pendingPreds) == 1 {
			t.Errorf(
				"Last match didn't work:\n%s\nInput events:\n%s",
				strings.Join(pendingPredStrs, "\n"),
				strings.Join(evStrs, "\n"),
			)
		} else {
			t.Errorf(
				"Last %d matches didn't work:\n%s\nInput events:\n%s",
				len(pendingPreds),
				strings.Join(pendingPredStrs, "\n"),
				strings.Join(evStrs, "\n"),
			)
		}
	}
}

func assertPredMatchesN(t *testing.T, n int, pred EventP, evs []s.Event) {
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

func waitDoneChild(name string) c.Spec {
	cspec, _ := c.New(name, func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})
	return cspec
}

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

// func signalChild(
//	name string,
//	start0 func(context.Context, func()) error,
//	opts ...c.Opt,
// ) (c.Spec, func()) {
//	terminatedCh := make(chan struct{})
//	start := func(ctx context.Context) error {
//		return start0(ctx, func() { close(terminatedCh) })
//	}
//	cs, _ := c.New(name, start, opts...)
//	return cs, func() { <-terminatedCh }
// }

////////////////////////////////////////////////////////////////////////////////
