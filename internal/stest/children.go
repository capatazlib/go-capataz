package stest

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

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
func FailOnSignalChild(
	totalErrCount int32,
	name string,
	opts ...c.Opt,
) (c.ChildSpec, func(bool)) {
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
func CompleteOnSignalChild(
	totalCompleteCount int32,
	name string,
	opts ...c.Opt,
) (c.ChildSpec, func()) {
	currentCompleteCount := int32(0)
	startCh := make(chan struct{})
	startSignal := func() {
		startCh <- struct{}{}
	}
	return c.New(
		name,
		func(ctx context.Context) error {
			<-startCh
			if currentCompleteCount < totalCompleteCount {
				atomic.AddInt32(&currentCompleteCount, 1)
				return nil
			}
			close(startCh)
			<-ctx.Done()
			return nil
		},
		opts...,
	), startSignal
}
