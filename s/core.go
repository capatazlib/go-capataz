package s

import (
	"context"
	"strings"

	"github.com/capatazlib/go-capataz/c"
)

// WithOrder specifies the start/stop order of a supervisor's children
func WithOrder(o Order) Opt {
	return func(spec *Spec) {
		spec.order = o
	}
}

// WithStrategy specifies how children get restarted when one fails
func WithStrategy(s Strategy) Opt {
	return func(spec *Spec) {
		spec.strategy = s
	}
}

// WithNotifier specifies a callback that gets called whenever the supervision
// system reports an Event
func WithNotifier(en EventNotifier) Opt {
	return func(spec *Spec) {
		spec.eventNotifier = en
	}
}

// WithChildren specifies a list of child Spec that will get started when the
// supervisor starts
func WithChildren(children ...c.Spec) Opt {
	return func(spec *Spec) {
		spec.children = append(spec.children, children...)
	}
}

// WithSubtree specifies a supervisor sub-tree. Is intended to be used when
// composing sub-systems in a supervision tree.
func WithSubtree(subtree Spec, copts ...c.Opt) Opt {
	return func(spec *Spec) {
		cspec := spec.Subtree(subtree, copts...)
		WithChildren(cspec)(spec)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Supervisor (dynamic tree) functionality

// handleChildResult returns a callback function that gets called by a
// supervised child whenever it finishes it main execution function.
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

// Stop is a synchronous procedure that halts the execution of the whole
// supervision tree.
func (sup Supervisor) Stop() error {
	sup.cancel()
	err := sup.wait()
	sup.spec.getEventNotifier().ProcessStopped(sup.Name(), err)
	return err
}

// Wait blocks the execution of the current goroutine until the Supervisor
// finishes it execution.
func (sup Supervisor) Wait() error {
	return sup.wait()
}

// Name returns the name of the Spec used to start this Supervisor
func (sup Supervisor) Name() string {
	return sup.spec.Name()
}

////////////////////////////////////////////////////////////////////////////////
// Spec (static tree) functionality

// emptyEventNotifier is an utility function that works as a default value
// whenever an EventNotifier is not specified on the Supervisor Spec
func emptyEventNotifier(_ Event) {}

// getEventNotifier returns the configured EventNotifier or emptyEventNotifier
// (if none is given via WithEventNotifier)
func (spec Spec) getEventNotifier() EventNotifier {
	if spec.eventNotifier == nil {
		return emptyEventNotifier
	}
	return spec.eventNotifier
}

// subtreeMain contains the main logic of the Child spec that runs a supervision
// sub-tree. It returns an error if the child supervisor fails to start.
func subtreeMain(spec Spec) func(context.Context, func()) error {
	// we use the start version that receives the notifyChildStart callback, this
	// is essential, as we need this callback to signal the sub-tree children have
	// started before signaling we have started
	return func(parentCtx context.Context, notifyChildStart func()) error {
		// in this function we use the private versions of start and wait
		// given we don't want to signal the eventNotifier more than once
		// on sub-trees

		ctx, cancelFn := context.WithCancel(parentCtx)
		defer cancelFn()
		sup, err := spec.start(ctx)
		if err != nil {
			return err
		}
		notifyChildStart()
		return sup.wait()
	}
}

// Subtree allows to register a Supervisor Spec as a sub-tree of a bigger
// Supervisor Spec. It returns an error if the child creation fails.
func (spec Spec) Subtree(subtreeSpec Spec, copts ...c.Opt) c.Spec {
	name := subtreeSpec.name
	subtreeSpec.eventNotifier = spec.eventNotifier

	// Child does prefix at runtime
	runtimeName := strings.Join([]string{spec.name, subtreeSpec.name}, "/")

	// we need the subtreeSpec.name to be the runtime name for it'spec childrens
	// to contain the correct prefix
	subtreeSpec.name = runtimeName
	return c.New1(name, subtreeMain(subtreeSpec), copts...)
}

// start is routine that contains the main logic of a Supervisor. This function:
//
// 1) spawns a new goroutine for the supervisor loop
//
// 2) spawns each child goroutine in the correct order
//
// 3) stops all the spawned children in the correct order once it gets a stop
// signal
//
// 4) it monitors and reacts to errors reported by the supervised children
//
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
		children := spec.order.SortStop(spec.children)
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
		for _, cs := range spec.order.SortStart(spec.children) {
			c := cs.Start(spec.name, sup.handleChildResult())
			eventNotifier.ProcessStarted(c.RuntimeName())
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
				// TODO: Deal with errors on children
				// case msg := <-ctrlCh:
				// TODO: Deal with public facing API calls
			}
		}
	}()

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout
	<-startCh

	return sup, nil
}

// Name returns the specified name for a Supervisor Spec
func (spec Spec) Name() string {
	return spec.name
}

// Start transforms a Spec into a Supervisor record, once this function return,
// a new supervision tree is guaranteed to be initialized and running.
func (spec Spec) Start(parentCtx context.Context) (Supervisor, error) {
	sup, err := spec.start(parentCtx)
	if err != nil {
		return Supervisor{}, err
	}
	spec.getEventNotifier().ProcessStarted(spec.name)
	return sup, nil
}

// New creates an Spec for a Supervisor. It requires the name of the supervisor
// (for tracing purposes), all the other settings can be specified via Opt calls
func New(name string, opts ...Opt) Spec {
	spec := Spec{
		children: make([]c.Spec, 0, 10),
	}

	// Check name cannot be empty
	if name == "" {
		panic("Supervisor cannot have empty name")
	}
	spec.name = name

	// apply options
	for _, optFn := range opts {
		optFn(&spec)
	}

	// return spec
	return spec
}
