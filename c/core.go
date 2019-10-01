package c

import (
	"context"
	"errors"
	"strings"
	"time"
)

func WithRestart(r Restart) Opt {
	return func(spec *Spec) {
		spec.restart = r
	}
}

var Infinity = Shutdown{tag: infinityT}

func WithShutdown(s Shutdown) Opt {
	return func(spec *Spec) {
		spec.shutdown = s
	}
}

////////////////////////////////////////////////////////////////////////////////

func New(name string, start func(context.Context) error, opts ...Opt) (Spec, error) {
	spec := Spec{}

	if name == "" {
		return spec, errors.New("Child cannot have empty name")
	}
	spec.name = name

	if start == nil {
		return spec, errors.New("Child cannot have empty start function")
	}
	spec.start = start

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}
	spec.start = start

	// return spec
	return spec, nil
}

func (cs Spec) Name() string {
	return cs.name
}

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
			panic("Invalid shutdown value received")
		}
	}
}

func (cs Spec) SyncStart(
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

		// we tell the spawner this child thread has started running
		close(startCh)

		// client logic starts here, and waits until an error (or lack of) is
		// reported
		notifyResult(
			runtimeName,
			cs.start(childCtx),
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

func (c Child) RuntimeName() string {
	return c.runtimeName
}

func (c Child) Name() string {
	return c.spec.Name()
}

func (c Child) Wait() error {
	return c.wait(c.spec.shutdown)
}

func (c Child) Stop() error {
	c.cancel()
	return c.wait(c.spec.shutdown)
}
