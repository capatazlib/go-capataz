package cap

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Node represents a tree node in a supervision tree, it could either be a
// Subtree or a Worker
type Node func(SupervisorSpec) c.ChildSpec

// Order specifies the order in which a supervisor is going to start its node
// children. The stop order is the reverse of the start order.
type Order uint32

const (
	// LeftToRight is an Order that specifies children start from left to right
	LeftToRight Order = iota
	// RightToLeft is an Order that specifies children start from right to left
	RightToLeft
)

// sortStart returns children sorted for the supervisor start
func (o Order) sortStart(input0 []c.ChildSpec) []c.ChildSpec {
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
		panic("Invalid cap.Order value")
	}
}

// sortTermination returns children sorted for the supervisor stop
func (o Order) sortTermination(input0 []c.ChildSpec) []c.ChildSpec {
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
		panic("Invalid cap.Order value")
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

// CleanupResourcesFn is a function that cleans up resources that were
// allocated in a BuildNodesFn function.
//
// Check the documentation of NewSupervisorSpec for more details and examples
type CleanupResourcesFn = func() error

// BuildNodesFn is a function that returns a list of nodes
//
// Check the documentation of NewSupervisorSpec for more details and examples.
type BuildNodesFn = func() ([]Node, CleanupResourcesFn, error)

// SupervisorSpec represents the specification of a static supervisor; it serves
// as a template for the construction of a runtime supervision tree. In the
// SupervisorSpec you can specify settings like:
//
// * The children (workers or sub-trees) you want spawned in your system when it
// starts
//
// * The order in which the supervised node children get started
//
// * Notifies the supervisor to restart a child node (and, if specified all its
// siblings as well) when the node fails in unexpected ways.

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

// NewSupervisorSpec creates a SupervisorSpec. It requires the name of the
// supervisor (for tracing purposes) and some children nodes to supervise.
//
// Monitoring children that do not share resources
//
// This is intended for situations where you need worker goroutines that are
// self-contained running in the background.
//
// To specify a group of children nodes, you need to use the WithNodes utility
// function. This function may receive Subtree or Worker nodes.
//
// Example:
//
//     cap.NewSupervisorSpec("root",
//
//       // (1)
//       // Specify child nodes to spawn when this supervisor starts
//       cap.WithNodes(
//         cap.Subtree(subtreeSupervisorSpec),
//         workerChildSpec,
//       ),
//
//       // (2)
//       // Specify child nodes start from right to left (reversed order) and
//       // stop from left to right.
//       cap.WithOrder(cap.RightToLeft),
//     )
//
//
// Monitoring nodes that share resources
//
// Sometimes, you want a group of children nodes to interact between each other
// via some shared resource that only the workers know about (for example, a
// gochan, a db datapool, etc).
//
// You are able to specify a custom function (BuildNodesFn) that allocates and
// releases these resources.
//
// This function should return:
//
// * The children nodes of the supervision tree
//
// * A function that cleans up the allocated resources (CleanupResourcesFn)
//
// * An error, but only in the scenario where a resource initialization failed
//
// Example:
//
//     cap.NewSupervisorSpec("root",
//
//       // (1)
//       // Implement a function that return all nodes to be supervised.
//       // When this supervisor gets (re)started, this function will be called.
//       // Imagine this function as a factory for it's children.
//       func() ([]cap.Node, cap.CleanupResourcesFn, error) {
//
//         // In this example, child nodes have a shared resource (a gochan)
//         // and it gets passed to their constructors.
//         buffer := make(chan MyType)
//         nodes := []cap.Node{
//           producerWorker(buffer),
//           consumerWorker(buffer),
//         }
//
//         // We create a function that gets executed when the supervisor
//         // shuts down.
//         cleanup := func() {
//           close(buffer)
//         }
//
//         // We return the allocated Node records and the cleanup function
//         return nodes, cleanup, nil
//       },
//
//       // (2)
//       cap.WithOrder(cap.RightToLeft),
//     )
//
// Dealing with errors
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
// of the API to implement start timeouts and cleanup timeouts inside the given
// BuildNodesFn and CleanupResourcesFn functions.
//
func NewSupervisorSpec(name string, buildNodes BuildNodesFn, opts ...Opt) SupervisorSpec {
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

// Start creates a Supervisor from this SupervisorSpec.
//
// A Supervisor is a tree of workers and/or sub-trees. The Start algorithm
// spawns the leaf worker goroutines first and then it will go up into the
// supervisor sub-trees. Depending on the SupervisorSpec's order, it will do an
// initialization in pre-order (LeftToRight) or post-order (RightToLeft).
//
// Supervisor Tree Initialization
//
// Once all the children leafs are initialized and running, the supervisor will
// execute its supervision monitor logic (listening to failures on its
// children). Invoking this method will block the thread until all the children
// and its sub-tree's childrens have been started.
//
// Failures on Child Initialization
//
// In the scenario that one of the child nodes fails to start (IO error, etc.),
// the Start algorithm is going to abort the start routine, and is going to stop
// in reverse order all the child nodes that have been started, finally
// returning an error value.
//
func (spec SupervisorSpec) Start(parentCtx context.Context) (Supervisor, error) {
	sup, err := spec.rootStart(parentCtx, rootSupervisorName)
	if err != nil {
		return Supervisor{}, err
	}
	return sup, nil
}

// GetName returns the given name of the supervisor spec (not a runtime name)
func (spec SupervisorSpec) GetName() string {
	return spec.name
}
