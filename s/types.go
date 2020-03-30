package s

import (
	"errors"
	"fmt"
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

// SortStop returns children sorted for the supervisor stop
func (o Order) SortStop(input0 []c.ChildSpec) []c.ChildSpec {
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

// Opt is used to configure a supervisor's specification
type Opt func(*SupervisorSpec)

// SupervisorSpec represents the specification of a Supervisor; it serves as a
// template for the construction of supervision trees. In the SupervisorSpec
// you can specify settings like:
//
// - The children (workers or sub-trees) you want spawned in your system when it
// gets started
//
// - The order in which the supervised children get started
//
// - When a failure occurs, if the supervisor restarts the failing child, or all it's children
//
type SupervisorSpec struct {
	name          string
	order         Order
	strategy      Strategy
	children      []c.ChildSpec
	eventNotifier EventNotifier
}

// Supervisor represents a tree of Child records. A Supervisor may have leaf or
// sub-tree children, each of the Child values represent a goroutine that gets
// automatic restart abilities as soon as the parent supervisor detects an error
// has occured. A Supervisor will always be generated from a SupervisorSpec
type Supervisor struct {
	runtimeName string
	spec        SupervisorSpec
	children    map[string]c.Child
	cancel      func()
	wait        func(time.Time, startError) error
}

// SupervisorError wraps an error from a children, enhancing it with supervisor
// information and possible shutdown errors on other siblings
type SupervisorError struct {
	err         error
	runtimeName string
	childErrMap map[string]error
}

// Unwrap returns a child error (if any)
func (se SupervisorError) Unwrap() error {
	return se.err
}

// GetRuntimeName returns the name of the supervisor that failed
func (se SupervisorError) GetRuntimeName() string {
	return se.runtimeName
}

// ChildFailCount returns the number of children that failed to terminate
// correctly. Note if a child fails to terminate because of a shutdown timeout,
// the failed goroutines may leak at runtime. This happens because go doesn't
// offer any true way to _kill_ a goroutine.
func (se SupervisorError) ChildFailCount() int {
	return len(se.childErrMap)
}

func (se SupervisorError) Error() string {
	if se.err != nil {
		return se.err.Error()
	}
	return "Supervisor failure"
}

// startError is the error reported back to a Supervisor when
// the start of a Child fails
type startError = error

// terminateError is the error reported back to a Supervisor when
// the termination of a Child fails
type terminateError = error

// rootSupervisorName is the name the root supervisor has, this is used to
// compare the process current name to the rootSupervisorName
var rootSupervisorName = ""

// childSepToken is the token use to separate sub-trees and child names in the
// supervision tree
const childSepToken = "/"

type supervisionError struct {
	supRuntimeName string
	childErr       *c.ErrorToleranceReached
	terminateErr   terminateError
}

func (err supervisionError) String() string {
	if err.childErr != nil && err.terminateErr != nil {
		return fmt.Sprintf(
			"Supervisor child surpassed error threshold, " +
				"(and other children failed to terminate as well)",
		)
	} else if err.childErr != nil {
		return fmt.Sprintf("Supervisor child surpassed error tolerance")
	}
	// NOTE: this case never happens, an invariant condition of this type is that
	// it only hold values with a childErr. If we are here, it means we manually
	// created a wrong supervisionError value (implementation error).
	panic(
		errors.New("invalid supervisionError was created"),
	)
}

func (err supervisionError) Error() string {
	return err.String()
}

func (err supervisionError) Unwrap() error {
	// it should never be nil
	if err.childErr != nil {
		return err.childErr.Unwrap()
	}
	return nil
}
