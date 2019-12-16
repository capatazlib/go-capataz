package c

import (
	"context"
	"errors"
	"strings"
	"time"
)

// WithRestart specifies how the parent supervisor should restart this child
// after an error is encountered.
func WithRestart(r Restart) Opt {
	return func(spec *ChildSpec) {
		spec.restart = r
	}
}

// WithShutdown specifies how the shutdown of the child is going to be handled.
// Read `Inf` and `Timeout` Shutdown values documentation for details.
func WithShutdown(s Shutdown) Opt {
	return func(spec *ChildSpec) {
		spec.shutdown = s
	}
}

func WithTag(t ChildTag) Opt {
	return func(spec *ChildSpec) {
		spec.tag = t
	}
}

////////////////////////////////////////////////////////////////////////////////

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
// The `startFn` function attribute of a `ChildSpec` is going to be used to
// spawn a new supervised goroutine; in this function is where your business
// logic should be located.
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
		shutdown: Timeout(5 * time.Second),
	}

	if name == "" {
		panic("Child cannot have empty name")
	}
	spec.name = name

	if startFn == nil {
		panic("Child cannot have empty start function")
	}

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}
	spec.start = startFn

	// return spec
	return spec
}

// Name returns the specified name for a Child Spec
func (cs ChildSpec) Name() string {
	return cs.name
}

// waitTimeout is the internal function used by Child to wait for the execution
// of it's thread to stop.
func waitTimeout(
	terminateCh <-chan ChildNotification,
) func(Shutdown) error {
	return func(shutdown Shutdown) error {
		switch shutdown.tag {
		case infinityT:
			// We wait forever for the result
			childNotification, ok := <-terminateCh
			if !ok {
				return nil
			}
			// A child may have terminated with an error
			return childNotification.Unwrap()
		case timeoutT:
			// we wait until some duration
			select {
			case childNotification, ok := <-terminateCh:
				if !ok {
					return nil
				}
				// A child may have terminated with an error
				return childNotification.Unwrap()
			case <-time.After(shutdown.duration):
				return errors.New("Child shutdown timeout")
			}
		default:
			// This should never happen if we use the already defined Shutdown types
			panic("Invalid shutdown value received")
		}
	}
}

// Start spawns a new goroutine that will execute the start attribute of the
// ChildSpec, this function will block until the spawned goroutine notifies it
// has been initialized.
//
// ### The notifyResult callback
//
// This callback notifies this child's supervisor that the goroutine has
// finished (either with or without an error). The runtime name of the child is
// also given so that the supervisor can use the spec for that child when
// restarting.
//
// #### Why a callback?
//
// By using a callback we avoid coupling the Supervisor types to the Child
// logic.
//
func (cs ChildSpec) Start(
	parentName string,
	notifyCh chan<- ChildNotification,
) (Child, error) {

	runtimeName := strings.Join([]string{parentName, cs.name}, "/")
	childCtx, cancelFn := context.WithCancel(context.Background())

	startCh := make(chan startError)
	terminateCh := make(chan ChildNotification)

	// Child Goroutine is bootstraped
	go func() {
		// TODO: Recover from panics

		// we tell the spawner this child thread has stopped
		defer close(terminateCh)

		// we cancel the childCtx on regular termination
		defer cancelFn()

		// client logic starts here, despite the call here being a "start", we will
		// block and wait here until an error (or lack of) is reported from the
		// client code
		err := cs.start(childCtx, func(err error) {
			// we tell the spawner this child thread has started running
			if err != nil {
				startCh <- err
			}
			close(startCh)
		})

		childNotification := ChildNotification{
			runtimeName: runtimeName,
			err:         err,
		}

		// We send the childNotification that got created to our parent supervisor.
		//
		// There are two ways the supervisor could receive this notification:
		//
		// 1) If the supervisor is running it's supervision loop (e.g. normal
		// execution), the notification will be received over the `notifyCh`
		// channel; this will execute the restart mechanisms.
		//
		// 2) If the supervisor is shutting down, it won't be reading the
		// `notifyCh`, but instead is going to be executing the `stopChildren`
		// function, which calls the `child.Stop` method for each of the supervised
		// internally, this function reads the `terminateCh`.
		//
		select {
		// (1)
		case notifyCh <- childNotification:
		// (2)
		case terminateCh <- childNotification:
		}

	}()

	// Wait until child thread notifies it has started or failed with an error
	err := <-startCh
	if err != nil {
		return Child{}, err
	}

	return Child{
		runtimeName: runtimeName,
		spec:        cs,
		cancel:      cancelFn,
		wait:        waitTimeout(terminateCh),
	}, nil
}

// RuntimeName returns a name that contains a prefix with the name of this child
// parents.
func (c Child) RuntimeName() string {
	return c.runtimeName
}

// Name returns the specified name for a Child Spec
func (c Child) Name() string {
	return c.spec.Name()
}

// Stop is a synchronous procedure that halts the execution of the child
func (c Child) Stop() error {
	c.cancel()
	return c.wait(c.spec.shutdown)
}
