package stest

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

func renderEvents(evs []s.Event) string {
	var builder strings.Builder
	for i, ev := range evs {
		builder.WriteString(fmt.Sprintf("  %3d: %+v\n", i, ev))
	}
	return builder.String()
}

// verifyExactMatch is an utility function that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
func verifyExactMatch(preds []EventP, given []s.Event) error {
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
// The input events need to match in the predicate order, but the events do not
// need to be a one to one match (e.g. the input events slice length may be
// bigger than the predicates slice length).
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

// WaitDoneChild creates a `c.ChildSpec` that runs a goroutine that blocks until
// the `context.Done` channel indicates a supervisor termination
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

// FailStartChild creates a `c.ChildSpec` that runs a goroutine that fails on
// start
func FailStartChild(name string) c.ChildSpec {
	cspec := c.NewWithNotifyStart(
		name,
		func(ctx context.Context, notifyStart c.NotifyStartFn) error {
			err := fmt.Errorf("FailStartChild %s", name)
			notifyStart(err)
			// NOTE: Even though we return the err value here, this err will never be
			// caught by our supervisor restart logic. If we invoke notifyStart with a
			// non-nil err, the supervisor will never get to the supervision loop, but
			// instead is going to terminate all started children and abort the
			// bootstrap of the supervision tree.
			return err
		})
	return cspec
}

// NeverTerminateChild creates a `c.ChildSpec` that runs a goroutine that never stops
// when asked to, causing the goroutine to leak in the runtime
func NeverTerminateChild(name string) c.ChildSpec {
	// For the sake of making the test go fast, lets reduce the amount of time we
	// wait for the child to terminate
	waitTime := 10 * time.Millisecond
	cspec := c.New(
		name,
		func(ctx context.Context) error {
			ctx.Done()
			// Wait a few milliseconds more than the specified time the supervisor
			// waits to finish
			time.Sleep(waitTime + (100 * time.Millisecond))
			return nil
		},
		// Here we explicitly say how much we are going to wait for this child
		// termination
		c.WithShutdown(c.Timeout(waitTime)),
	)
	return cspec
}

// FailOnSignalChild creates a `c.ChildSpec` that runs a goroutine that will fail at
// least the given number of times as soon as the returned start signal is
// called. Once this number of times has been reached, it waits until the given
// `context.Done` channel indicates a supervisor termination.
func FailOnSignalChild(totalErrCount int32, name string, opts ...c.Opt) (c.ChildSpec, func(bool)) {
	currentFailCount := int32(0)
	startCh := make(chan struct{})
	startSignal := func(done bool) {
		if done {
			close(startCh)
			return
		}
		startCh <- struct{}{}
	}
	return c.New(
		name,
		func(ctx context.Context) error {
			<-startCh
			if currentFailCount < totalErrCount {
				atomic.AddInt32(&currentFailCount, 1)
				return fmt.Errorf("Failing child (%d out of %d)", currentFailCount, totalErrCount)
			}
			<-ctx.Done()
			return nil
		},
		opts...,
	), startSignal
}

// CompleteOnSignalChild creates a `c.ChildSpec` that runs a goroutine that will complete at
// at as soon as the returned start signal is called.
func CompleteOnSignalChild(name string, opts ...c.Opt) (c.ChildSpec, func()) {
	startCh := make(chan struct{})
	startSignal := func() { close(startCh) }
	return c.New(
		name,
		func(ctx context.Context) error {
			<-startCh
			return nil
		},
		opts...,
	), startSignal
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
	supSpec := s.New(rootName, opts...)

	// We always want to start the supervisor for test purposes, so this is
	// embedded in the ObserveSupervisor call
	sup, err := supSpec.Start(ctx)

	evIt := evManager.Iterator()

	// NOTE: We execute SkipTill to make sure all the supervision tree got started
	// (or failed) before doing assertions/returning an error. Also, note we use
	// ProcessName instead of ProcessStarted/ProcessFailed given that ProcessName
	// matches an event in both success and error cases. The event from root must
	// be the last event reported
	evIt.SkipTill(ProcessName(rootName))

	if err != nil {
		callback(evManager)
		return evManager.Snapshot(), err
	}

	// callback to do assertions with the event manager
	callback(evManager)

	// once tests are done, we stop the supervisor
	err = sup.Terminate()

	// We wait till all the events have been reported (event from root must be the
	// last event)
	evIt.SkipTill(ProcessName(rootName))

	if err != nil {
		return evManager.Snapshot(), err
	}

	// return all the events reported by the supervision system
	return evManager.Snapshot(), nil
}
