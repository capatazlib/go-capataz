package s

import (
	"context"
	"errors"
	"strings"

	"github.com/capatazlib/go-capataz/c"
)

func WithOrder(o Order) Opt {
	return func(spec *Spec) {
		spec.order = o
	}
}

func WithStrategy(s Strategy) Opt {
	return func(spec *Spec) {
		spec.strategy = s
	}
}

func WithNotifier(en EventNotifier) Opt {
	return func(spec *Spec) {
		spec.eventNotifier = en
	}
}

func WithChildren(children ...c.Spec) Opt {
	return func(spec *Spec) {
		spec.children = append(spec.children, children...)
	}
}

func WithSubtree(subtree Spec, copts ...c.Opt) Opt {
	return func(spec *Spec) {
		cspec, _ := spec.Subtree(subtree, copts...)
		WithChildren(cspec)(spec)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Supervisor (dynamic tree) functionality

func (sup Supervisor) handleChildResult() func(string, error) {
	// The function bellow gets called in the child goroutine
	return func(childName string, err error) {
		if err != nil {
			// TODO report failed
		} else {
			// TODO report finished
		}
	}
}

func (sup Supervisor) Stop() error {
	sup.cancel()
	err := sup.wait()
	sup.spec.getEventNotifier().ProcessStopped(sup.Name(), err)
	return err
}

func (sup Supervisor) Wait() error {
	return sup.wait()
}

func (sup Supervisor) Name() string {
	return sup.spec.Name()
}

////////////////////////////////////////////////////////////////////////////////
// Spec (static tree) functionality

func emptyEventNotifier(_ Event) {}

func (spec Spec) getEventNotifier() EventNotifier {
	if spec.eventNotifier == nil {
		return emptyEventNotifier
	}
	return spec.eventNotifier
}

func subtreeMain(spec Spec) func(context.Context) error {
	return func(parentCtx context.Context) error {
		// NOTE: in this function we use the private versions of start and wait
		// given we don't want to signal the eventNotifier more than once
		// on sub-trees

		ctx, cancelFn := context.WithCancel(parentCtx)
		defer cancelFn()
		sup, err := spec.start(ctx)
		if err != nil {
			return err
		}
		return sup.wait()
	}
}

func (spec Spec) Subtree(subtreeSpec Spec, copts ...c.Opt) (c.Spec, error) {
	name := subtreeSpec.name
	subtreeSpec.eventNotifier = spec.eventNotifier
	// Child does prefix at runtime
	runtimeName := strings.Join([]string{spec.name, subtreeSpec.name}, "/")
	// NOTE: we need the subtreeSpec.name to be the runtime name for it'spec
	// childrens to contain the correct prefix
	subtreeSpec.name = runtimeName
	return c.New(name, subtreeMain(subtreeSpec), copts...)
}

func (spec Spec) start(parentCtx context.Context) (Supervisor, error) {
	// cancelFn is used when Stop is requested
	ctx, cancelFn := context.WithCancel(parentCtx)

	// evCh is used to keep track of errors from children
	// evCh := make(chan ChildEvent)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	// ctrlCh := make(chan ControlMsg)

	// startCh is used to track when the supervisor loop thread has started
	startCh := make(chan struct{})

	// terminateCh is used when waiting for cancelFn to complete
	terminateCh := make(chan struct{})

	// errCh is used when supervisor gives up on error handling
	errCh := make(chan error)

	eventNotifier := spec.getEventNotifier()

	sup := Supervisor{
		spec:     spec,
		children: make(map[string]c.Child, len(spec.children)),
		cancel:   cancelFn,
		wait: func() error {
			select {
			case err := <-errCh:
				return err
			case <-terminateCh:
				return nil
			}
		},
	}

	// stopChildrenFn is used on the shutdown of the supervisor tree, stops children in
	// desired order
	stopChildrenFn := func() {
		children := spec.order.Stop(spec.children)
		for _, cs := range children {
			c := sup.children[cs.Name()]
			err := c.Stop()
			name := strings.Join([]string{spec.name, c.Name()}, "/")
			eventNotifier.ProcessStopped(name, err)
		}
	}

	go func() {
		defer close(terminateCh)

		// Start children
		for _, cs := range spec.order.Start(spec.children) {
			c := cs.SyncStart(spec.name, sup.handleChildResult())
			runtimeName := strings.Join([]string{spec.name, c.Name()}, "/")
			eventNotifier.ProcessStarted(runtimeName)
			sup.children[cs.Name()] = c
		}

		// Once children have been spawned we notify the supervisor thread has
		// started
		close(startCh)

		// Supervisor Loop
	supervisorLoop:
		for {
			select {
			case <-ctx.Done():
				// parent context is done
				stopChildrenFn()
				break supervisorLoop
				// case ev := <-evCh:
				// case msg := <-ctrlCh:
			}
		}
	}()

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout
	<-startCh

	return sup, nil
}

func (spec Spec) Name() string {
	return spec.name
}

func (spec Spec) Start(parentCtx context.Context) (Supervisor, error) {
	sup, err := spec.start(parentCtx)
	if err != nil {
		return Supervisor{}, err
	}
	spec.getEventNotifier().ProcessStarted(spec.name)
	return sup, nil
}

func New(name string, opts ...Opt) (Spec, error) {
	spec := Spec{
		children: make([]c.Spec, 0, 10),
	}

	// Check name cannot be empty
	if name == "" {
		return spec, errors.New("Supervisor cannot have empty name")
	}
	spec.name = name

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}

	// return spec
	return spec, nil
}
