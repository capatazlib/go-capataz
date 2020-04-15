package s

import (
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

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
		panic("Invalid s.Order value")
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
		panic("Invalid s.Order value")
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
