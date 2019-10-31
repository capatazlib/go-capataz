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
func WithShutdown(s Shutdown) Opt {
	return func(spec *ChildSpec) {
		spec.shutdown = s
	}
}

////////////////////////////////////////////////////////////////////////////////

// New creates a `ChildSpec` that represents a worker goroutine. It requires two
// arguments: a `name` that is used for runtime tracing and a `start` function.
//
// ### The `name` argument
//
// The `name` argument must not be empty nor contain forward slash characters
// (e.g. `/`), otherwise, the system will panic. This method is preferred as
// opposed to return an error given it is considered a bad implementation
// (ideally a compilation error).
//
// ### The `start` argument
//
// The `start` function attribute of a `ChildSpec` is going to be used to spawn
// a new supervised goroutine; in this function is where your business logic
// should be located.
//
// The `start` function will receive a `context.Context` record that _must_ be
// used inside your business logic to accept stop signals from it's parent
// supervisor.
//
// Depending on the `Shutdown` values used in the `ChildSpec` , if the start
// function does not respect the given context, the parent supervisor will
// either block forever or leak goroutines after a timeout has been reached.
func New(name string, childMain func(context.Context) error, opts ...Opt) ChildSpec {
	return NewWithNotifyStart(
		name,
		func(ctx context.Context, notifyChildStart NotifyStartFn) error {
			notifyChildStart(nil)
			return childMain(ctx)
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
// #### Report an start error on `NotifyStartFn`
//
// If for some reason, a child is not able to start correctly, the child should
// call the `NotifyStartFn` function with the start `error`.
//
func NewWithNotifyStart(
	name string,
	childMain func(context.Context, NotifyStartFn) error,
	opts ...Opt,
) ChildSpec {
	spec := ChildSpec{}

	if name == "" {
		panic("Child cannot have empty name")
	}
	spec.name = name

	if childMain == nil {
		panic("Child cannot have empty start function")
	}

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}
	spec.start = childMain

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
	terminateCh chan struct{},
) func(Shutdown) error {
	return func(shutdown Shutdown) error {
		switch shutdown.tag {
		case infinityT:
			// We wait forever for the result
			<-terminateCh
			return nil
		case timeoutT:
			// we wait until some duration
			select {
			case <-terminateCh:
				return nil
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
	notifyResult func(runtimeChildName, error),
) (Child, error) {

	runtimeName := strings.Join([]string{parentName, cs.name}, "/")
	childCtx, cancelFn := context.WithCancel(context.Background())

	startCh := make(chan error)
	terminateCh := make(chan struct{})

	// Child Goroutine is bootstraped
	go func() {
		// TODO: Recover from panics

		// we tell the spawner this child thread has stopped
		defer close(terminateCh)

		// we cancel the childCtx on regular termination
		defer cancelFn()

		// client logic starts here, and waits until an error (or lack of) is
		// reported
		notifyResult(
			runtimeName,
			cs.start(childCtx, func(err error) {
				// we tell the spawner this child thread has started running
				if err != nil {
					startCh <- err
				}
				close(startCh)
			}),
		)
	}()

	// Wait until child thread notifies it has started
	err := <-startCh
	if err != nil {
		return Child{runtimeName: runtimeName, spec: cs}, err
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

// Wait blocks the execution of the current goroutine until the child finishes
// it execution.
func (c Child) Wait() error {
	return c.wait(c.spec.shutdown)
}

// Stop is a synchronous procedure that halts the execution of the child
func (c Child) Stop() error {
	c.cancel()
	return c.wait(c.spec.shutdown)
}
