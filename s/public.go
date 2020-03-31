package s

// This file contains the implementation of the public methods for the
// Supervisor API

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Supervisor represents the root of a tree of goroutines. A Supervisor may have
// leaf or sub-tree children, where each of the nodes in the tree represent a
// goroutine that gets automatic restart abilities as soon as the parent
// supervisor detects an error has occured. A Supervisor will always be
// generated from a SupervisorSpec
type Supervisor struct {
	runtimeName string
	spec        SupervisorSpec
	children    map[string]c.Child
	cancel      func()
	wait        func(time.Time, startError) error
}

////////////////////////////////////////////////////////////////////////////////
// Public API

// New creates a SupervisorSpec. It requires the name of the supervisor (for
// tracing purposes), all the other settings can be specified via Opt calls
func New(name string, opts ...Opt) SupervisorSpec {
	spec := SupervisorSpec{
		children:      make([]c.ChildSpec, 0, 10),
		eventNotifier: emptyEventNotifier,
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

// Start creates a Supervisor from the SupervisorSpec. A Supervisor is a tree of
// goroutines. The Start algorithm begins with the spawning the leaf worker
// goroutines first. Depending on the SupervisorSpec's order, it will do an
// initialization in pre-order (LeftToRight) or post-order (RightToLeft).
//
// ### Initialization of the tree
//
// Once all the children leafs are initialized and running, the supervisor will
// execute it's supervision monitor logic (listening to failures on its
// children). Invoking this method will block the thread until all the children
// and its sub-tree's childrens have been started.
//
// ### Failure on child initialization
//
// In case one of the tree children fails to start, the Supervisor is going to
// retry a number of times before giving up and returning an error. In case this
// supervisor is a sub-tree, it's parent supervisor will retry the
// initialization until the error tolerance is surpassed; eventually, the errors
// will reach the root supervisor and the program will return a hard error.
//
func (spec SupervisorSpec) Start(parentCtx context.Context) (Supervisor, error) {
	sup, err := spec.rootStart(parentCtx, rootSupervisorName)
	if err != nil {
		return Supervisor{}, err
	}
	return sup, nil
}

// GetName returns the specified name for a Supervisor Spec
func (spec SupervisorSpec) GetName() string {
	return spec.name
}

// Terminate is a synchronous procedure that halts the execution of the whole
// supervision tree.
func (sup Supervisor) Terminate() error {
	stopingTime := time.Now()
	sup.cancel()
	err := sup.wait(stopingTime, nil /* no startErr */)
	return err
}

// Wait blocks the execution of the current goroutine until the Supervisor
// finishes it execution.
func (sup Supervisor) Wait() error {
	return sup.wait(time.Time{}, nil /* no startErr */)
}

// GetName returns the name of the Spec used to start this Supervisor
func (sup Supervisor) GetName() string {
	return sup.spec.GetName()
}
