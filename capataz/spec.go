package capataz

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Node represents a tree node in a supervision tree, it could either be a
// Subtree or a Worker
type Node func(SupervisorSpec) c.ChildSpec

// Order specifies the order in which a supervision tree is going to start and
// stop its children. The stop ordering is always the reverse of the start
// ordering.
type Order uint32

const (
	// LeftToRight is an Order that specifies children start from left to right
	LeftToRight Order = iota
	// RightToLeft is an Order that specifies children start from right to left
	RightToLeft
)

// SortStart returns children sorted for the supervisor start
func (o Order) SortStart(input0 []c.ChildSpec) []c.ChildSpec {
	input := append(input0[:0:0], input0...)
	switch o {
	case LeftToRight:
		return input
	case RightToLeft:
		for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
			input[i], input[j] = input[j], input[i]
		}
		return input
	default:
		panic("Invalid capataz.Order value")
	}
}

// SortTermination returns children sorted for the supervisor stop
func (o Order) SortTermination(input0 []c.ChildSpec) []c.ChildSpec {
	input := append(input0[:0:0], input0...)
	switch o {
	case LeftToRight:
		for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
			input[i], input[j] = input[j], input[i]
		}
		return input
	case RightToLeft:
		return input
	default:
		panic("Invalid capataz.Order value")
	}
}

// Strategy specifies how children get restarted when one of them reports an
// error
type Strategy uint32

const (
	// OneForOne is an Strategy that tells the Supervisor to only restart the
	// child process that errored
	OneForOne Strategy = iota
	// OneForAll
	// RestForOne
)

// getEventNotifier returns the configured EventNotifier or emptyEventNotifier
// (if none is given via WithEventNotifier)
func (spec SupervisorSpec) getEventNotifier() EventNotifier {
	if spec.eventNotifier == nil {
		return emptyEventNotifier
	}
	return spec.eventNotifier
}

// CleanupResourcesFn is a function that cleans up resources that were initialized
// in a BuildNodesFn function.
type CleanupResourcesFn = func() error

// BuildNodesFn is a function that returns a list of nodes
type BuildNodesFn = func() ([]Node, CleanupResourcesFn, error)

// SupervisorSpec represents the specification of a Supervisor; it serves as a
// template for the construction of supervision trees. In the SupervisorSpec you
// can specify settings like:
//
// - The children (workers or sub-trees) you want spawned in your system when it
// gets started
//
// - The order in which the supervised children get started
//
// - When a failure occurs, if the supervisor restarts the failing child, or all
// its children
//
type SupervisorSpec struct {
	name            string
	buildNodes      BuildNodesFn
	order           Order
	strategy        Strategy
	shutdownTimeout time.Duration
	eventNotifier   EventNotifier
}

// buildChildren constructs the childSpec records that the Supervisor is going
// to monitor at runtime.
func (spec SupervisorSpec) buildChildrenSpecs() ([]c.ChildSpec, CleanupResourcesFn, error) {
	nodes, cleanup, err := spec.buildNodes()
	if err != nil {
		return []c.ChildSpec{}, cleanup, err
	}

	children := make([]c.ChildSpec, 0, len(nodes))
	for _, buildChildSpec := range nodes {
		children = append(children, buildChildSpec(spec))
	}
	return children, cleanup, nil
}

// NewSupervisor creates a SupervisorSpec. It requires the name of the
// supervisor (for tracing purposes) and some children nodes to supervise.
//
// ## How to provide a capataz.BuildNodesFn
//
// There are two possible use cases:
//
// ### Monitoring children that do not share resources
//
// To specify a static group of children nodes, you need to use the
// capataz.WithChildren utility function. This function may receive capataz.Subtree or
// capataz.Worker nodes.
//
// #### Example:
//
// > capataz.NewSupervisor("root", capataz.WithChildren(
// >     capataz.Subtree(subtreeSupervisorSpec),
// >     workerChildSpec,
// >   ),
// >   capataz.WithOrder(capataz.RightToLeft),
// > )
//
//
// ### Monitoring nodes that share resources
//
// Sometimes, you want a tree of workers interacting between each other, and
// these nodes share a resource that only the workers should know about (for
// example, a gochan, a db datapool, etc). You are able to specify a function
// that allocates and releases these kind of resources.
//
// #### Example:
//
// > capataz.NewSupervisor("root",
// >   func() ([]capataz.Node, capataz.CleanupResourcesFn, error) {
// >     buffer := make(chan MyType)
// >     nodes := []capataz.Node{
// >       producerWorker(buffer),
// >       consumerWorker(buffer),
// >     }
// >     cleanup := func() {
// >       close(buffer)
// >     }
// >     return nodes, cleanup, nil
// >   },
// >   capataz.WithOrder(capataz.RightToLeft),
// > )
//
// #### Dealing with errors
//
// Given resources can involve IO allocations, using this functionality opens
// the door to a few error scenarios:
//
// 1) Resource allocation returns an error
//
// On this scenario, the supervision start procedure will fail and it will
// follow the regular shutdown procedure, the already started nodes will be
// terminated and an error will be returned immediately.
//
// 2) Resource cleanup returns an error
//
// On this scenario, the termination procedure will collect the error and report
// it in the returned SupervisorTerminationError.
//
// 3) Resource allocation/cleanup hangs
//
// This library does not handle this scenario. Is the responsibility of the user
// of the API to ensure a start timeouts and cleanup timeouts inside the
// capataz.BuildNodesFn and capataz.CleanupResourcesFn respectively.
//
func NewSupervisor(name string, buildNodes BuildNodesFn, opts ...Opt) SupervisorSpec {
	spec := SupervisorSpec{
		buildNodes:      buildNodes,
		shutdownTimeout: defaultSupShutdownTimeout,
		eventNotifier:   emptyEventNotifier,
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
