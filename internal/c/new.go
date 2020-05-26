package c

import (
	"context"
	"time"
)

// New creates a `ChildSpec` that represents a worker goroutine. It requires two
// arguments: a `name` that is used for runtime tracing and a `startFn` function.
//
// ### The `name` argument
//
// The `name` argument must not be empty nor contain forward slash characters
// (e.g. `/`), otherwise, the system will panic. This method is preferred as
// opposed to return an error given it is considered a bad implementation
// (ideally a compilation error).
//
// ### The `startFn` argument
//
// The `startFn` function where your business logic should be located. This
// attribute of a `ChildSpec` is going to be used to spawn a new supervised
// goroutine.
//
// The `startFn` function will receive a `context.Context` record that _must_ be
// used inside your business logic to accept stop signals from it's parent
// supervisor.
//
// Depending on the `Shutdown` values used in the `ChildSpec` , if the `startFn`
// function does not respect the given context, the parent supervisor will
// either block forever or leak goroutines after a timeout has been reached.
func New(name string, startFn func(context.Context) error, opts ...Opt) ChildSpec {
	return NewWithNotifyStart(
		name,
		func(ctx context.Context, notifyChildStart NotifyStartFn) error {
			notifyChildStart(nil)
			return startFn(ctx)
		},
		opts...,
	)
}

// NewWithNotifyStart accomplishes the same goal as `New` with the addition of
// passing a `notifyStart` callback function to the `start` parameter.
//
// ### The `NotifyStartFn` argument
//
// The `NotifyStartFn` is a callback that allows the spawned worker goroutine to
// signal when it has officially started. Is essential to call this callback
// function in your business logic as soon as you consider the worker is
// initialized, otherwise the parent supervisor will block and eventually fail
// with a timeout.
//
// #### Report a start error on `NotifyStartFn`
//
// If for some reason, a child is not able to start correctly, the child should
// call the `NotifyStartFn` function with the start `error`.
//
func NewWithNotifyStart(
	name string,
	startFn func(context.Context, NotifyStartFn) error,
	opts ...Opt,
) ChildSpec {
	spec := ChildSpec{
		// Child workers by default will have 5 seconds to terminate before
		// reporting a timeout error as specified on the Erlang OTP documentation.
		// http://erlang.org/doc/design_principles/sup_princ.html#tuning-the-intensity-and-period
		//
		// A point worth bringing up is that golang *does not* provide a hard kill
		// mechanism for goroutines. There is no known way to kill a goroutine via a
		// signal other than using `context.Done` and the goroutine respecting this
		// mechanism; If the timeout is reached and the goroutine does not stop, the
		// supervisor will continue with the shutdown procedure, possibly leaving
		// the goroutine running in memory (e.g. memory leak).
		Shutdown: Timeout(5 * time.Second),

		// Children will have a tolerance of 1 error every 5 seconds before telling
		// the supervisor to give up, this is insipired by Erlang OTP documentation.
		// http://erlang.org/doc/design_principles/sup_princ.html#maximum-restart-intensity
		ErrTolerance: ErrTolerance{MaxErrCount: 1, ErrWindow: 5 * time.Second},

		// All panics are going to be supervised by default
		CapturePanic: true,
	}

	if name == "" {
		panic("Child cannot have empty name")
	}
	spec.Name = name

	if startFn == nil {
		panic("Child cannot have empty start function")
	}

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}
	spec.Start = startFn

	// return spec
	return spec
}
