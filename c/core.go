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
	return func(spec *Spec) {
		spec.restart = r
	}
}

// WithShutdown specifies how the shutdown of the child is going to be handled.
func WithShutdown(s Shutdown) Opt {
	return func(spec *Spec) {
		spec.shutdown = s
	}
}

////////////////////////////////////////////////////////////////////////////////

// New creates an Spec for a worker goroutine.
//
// It requires two arguments:
//
// - name: The name of the worker (for tracing purposes)
//
// - start: The main function that is going to be executing in this child
// goroutine
//
// The start function will receive a context.Context record that must be used to
// receive a stop signal from the parent supervisor. Depending on the Shutdown
// settings, if the start function does not respect the given context, the
// supervision system will either block forever or leak goroutines after a
// timeout has been reached.
func New(name string, start func(context.Context) error, opts ...Opt) Spec {
	return New1(name, func(ctx context.Context, notifyChildStart func()) error {
		notifyChildStart()
		return start(ctx)
	}, opts...)
}

// New1 is similar to New, with the difference that the start function receives
// a `notifyStart` callback on the child routine, this callback allows the child
// goroutine to signal when it has officially started. This is essential when
// you want to guarantee some bootstrap on thread initialization.
func New1(
	name string,
	start func(context.Context, func()) error,
	opts ...Opt,
) Spec {
	spec := Spec{}

	if name == "" {
		// If the name is empty, the program should not start. If I return an error
		// to deal with this at runtime the API gets awful quickly
		panic("Child cannot have empty name")
	}
	spec.name = name

	if start == nil {
		// ditto to the previous comment
		panic("Child cannot have empty start function")
	}

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}
	spec.start = start

	// return spec
	return spec
}

// Name returns the specified name for a Child Spec
func (cs Spec) Name() string {
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

// Start does a synchronous initialization of the child goroutine, this function
// will block until the spawned goroutine notifies it has been initialized.
func (cs Spec) Start(
	parentName string,
	notifyResult func(string, error),
) Child {

	runtimeName := strings.Join([]string{parentName, cs.name}, "/")
	childCtx, cancelFn := context.WithCancel(context.Background())

	startCh := make(chan struct{})
	terminateCh := make(chan struct{})

	// Child Goroutine is bootstraped
	go func() {
		// TODO: Recover from panics

		// we tell the spawner this child thread has stopped
		defer close(terminateCh)

		// we kill the cancelFn on regular termination
		defer cancelFn()

		// client logic starts here, and waits until an error (or lack of) is
		// reported
		notifyResult(
			runtimeName,
			cs.start(childCtx, func() {
				// we tell the spawner this child thread has started running
				close(startCh)
			}),
		)
	}()

	// Wait until child thread notifies it has started
	<-startCh

	return Child{
		runtimeName: runtimeName,
		spec:        cs,
		cancel:      cancelFn,
		wait:        waitTimeout(terminateCh),
	}
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
