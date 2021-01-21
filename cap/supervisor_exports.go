package cap

import (
	"github.com/capatazlib/go-capataz/internal/s"
)

// Node represents a tree node in a supervision tree, it could either be a
// Subtree or a Worker
type Node = s.Node

// Order specifies the order in which a supervisor is going to start its node
// children. The stop order is the reverse of the start order.
type Order = s.Order

// LeftToRight is an Order that specifies children start from left to right
var LeftToRight = s.LeftToRight

// RightToLeft is an Order that specifies children start from right to left
var RightToLeft = s.RightToLeft

// Strategy specifies how children get restarted when one of them reports an
// error
type Strategy = s.Strategy

// OneForOne is an Strategy that tells the Supervisor to only restart the
// child process that errored
var OneForOne = s.OneForOne

// CleanupResourcesFn is a function that cleans up resources that were
// allocated in a BuildNodesFn function.
//
// Check the documentation of NewSupervisorSpec for more details and examples
type CleanupResourcesFn = s.CleanupResourcesFn

// BuildNodesFn is a function that returns a list of nodes
//
// Check the documentation of NewSupervisorSpec for more details and examples.
type BuildNodesFn = s.BuildNodesFn

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
type SupervisorSpec = s.SupervisorSpec

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
//       cap.WithStartOrder(cap.RightToLeft),
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
//       cap.WithStartOrder(cap.RightToLeft),
//     )
//
// Dealing with errors
//
// Given resources can involve IO allocations, using this functionality opens
// the door to a few error scenarios:
//
// 1) Resource allocation returns an error
//
// In this scenario, the supervision start procedure will fail and it will
// follow the regular shutdown procedure: the already started nodes will be
// terminated and an error will be returned immediately.
//
// 2) Resource cleanup returns an error
//
// In this scenario, the termination procedure will collect the error and report
// it in the returned SupervisorError.
//
// 3) Resource allocation/cleanup hangs
//
// This library does not handle this scenario. Is the responsibility of the user
// of the API to implement start timeouts and cleanup timeouts inside the given
// BuildNodesFn and CleanupResourcesFn functions.
//
var NewSupervisorSpec = s.NewSupervisorSpec

// Opt is a type used to configure a SupervisorSpec
type Opt = s.Opt

// WithStartOrder is an Opt that specifies the start/stop order of a supervisor's
// children nodes
//
// Possible values may be:
//
// * LeftToRight -- Start children nodes from left to right, stop them from
// right to left
//
// * RightToLeft -- Start children nodes from right to left, stop them from left
// to right
//
var WithStartOrder = s.WithStartOrder

// WithOrder is a backwards compatible alias to WithStartOrder
//
// Deprecated: Use WithStartOrder instead
var WithOrder = WithStartOrder

// WithStrategy is an Opt that specifies how children nodes of a supervisor get
// restarted when one of the nodes fails
//
// Possible values may be:
//
// * OneForOne -- Only restart the failing child
//
// * OneForAll (Not Implemented Yet) -- Restart the failing child and all its
// siblings[*]
//
// [*] This option may come handy when all the other siblings depend on one another
// to work correctly.
//
var WithStrategy = s.WithStrategy

// WithNotifier is an Opt that specifies a callback that gets called whenever
// the supervision system reports an Event
//
// This function may be used to observe the behavior of all the supervisors in
// the systems, and it is a great place to hook in monitoring services like
// logging, error tracing and metrics gatherers
//
var WithNotifier = s.WithNotifier

// WithNodes allows the registration of child nodes in a SupervisorSpec. Node
// records passed to this function are going to be supervised by the Supervisor
// created from a SupervisorSpec.
//
// Check the documentation of NewSupervisorSpec for more details and examples.
var WithNodes = s.WithNodes

// WithRestartTolerance is a Opt that specifies how many errors the supervisor
// should be willing to tolerate before giving up restarting and fail.
//
// If the tolerance is met, the supervisor is going to fail, if this is a
// sub-tree, this error is going to be handled by a grand-parent supervisor,
// restarting the tolerance again.
//
// Example
//
//   // Tolerate 10 errors every 5 seconds
//   //
//   // - if there is 11 errors in a 5 second window, it makes the supervisor fail
//   //
//   WithRestartTolerance(10, 5 * time.Second)
//
var WithRestartTolerance = s.WithRestartTolerance

// Subtree transforms SupervisorSpec into a Node. This function allows you to
// insert a black-box sub-system into a bigger supervised system.
//
// Note the subtree SupervisorSpec is going to inherit the event notifier from
// its parent supervisor.
//
// Example:
//
//   // Initialized a SupervisorSpec that doesn't know anything about other
//   // parts of the systems (e.g. is self-contained)
//   networkingSubsystem := cap.NewSupervisorSpec("net", ...)
//
//   // Another self-contained system
//   filesystemSubsystem := cap.NewSupervisorSpec("fs", ...)
//
//   // SupervisorSpec that is started in your main.go
//   cap.NewSupervisorSpec("root",
//    cap.WithNodes(
//      cap.Subtree(networkingSubsystem),
//      cap.Subtree(filesystemSubsystem),
//    ),
//   )
//
var Subtree = s.Subtree

// DynSupervisor is a supervisor that can spawn workers in a procedural way.
type DynSupervisor = s.DynSupervisor

// NewDynSupervisor creates a DynamicSupervisor which can start workers at
// runtime in a procedural manner. It receives a context and the supervisor name
// (for tracing purposes).
//
//
// When to use a DynSupervisor?
//
// If you want to run supervised worker routines on dynamic inputs. This is
// something that a regular Supervisor cannot do, as it needs to know the
// children nodes at construction time.
//
// Differences to Supervisor
//
// As opposed to a Supervisor, a DynSupervisor:
//
// * Cannot receive node specifications to start them in an static fashion
//
// * It is able to spawn workers dynamically
//
// * In case of a hard crash and following restart, it will start with an empty
//   list of children
//
var NewDynSupervisor = s.NewDynSupervisor

// Supervisor represents the root of a tree of goroutines. A Supervisor may have
// leaf or sub-tree children, where each of the nodes in the tree represent a
// goroutine that gets automatic restart abilities as soon as the parent
// supervisor detects an error has occured. A Supervisor will always be
// generated from a SupervisorSpec
type Supervisor = s.Supervisor
