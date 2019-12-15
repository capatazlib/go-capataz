package s

import (
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/c"
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

// EventTag specifies the type of Event that gets notified from the supervision
// system
type EventTag uint32

const (
	eventNoop EventTag = iota
	// ProcessStarted is an Event that indicates a process started
	ProcessStarted
	// ProcessStopped is an Event that indicates a process was stopped by a parent
	// supervisor
	ProcessStopped
	// ProcessFailed is an Event that indicates a process reported an error
	ProcessFailed
)

// String returns a string representation of the current EventTag
func (tag EventTag) String() string {
	switch tag {
	case ProcessStarted:
		return "ProcessStarted"
	case ProcessStopped:
		return "ProcessStopped"
	case ProcessFailed:
		return "ProcessFailed"
	default:
		return "<Unknown>"
	}
}

// Event is a record emitted by the supervision system. The events are used for
// multiple purposes, from testing to monitoring the healthiness of the
// supervision system.
type Event struct {
	tag                EventTag
	processRuntimeName string
	err                error
	created            time.Time
	duration           time.Duration
}

// Tag returns the EventTag from an Event
func (e Event) Tag() EventTag {
	return e.tag
}

// ProcessRuntimeName returns the given name of a process that emitted this event
func (e Event) ProcessRuntimeName() string {
	return e.processRuntimeName
}

// Err returns an error reported by the process that emitted this event
func (e Event) Err() error {
	return e.err
}

// Created returns a timestamp of the creation of the event by the process
func (e Event) Created() time.Time {
	return e.created
}

// String returns an string representation for the Event
func (e Event) String() string {
	return fmt.Sprintf("Event{tag: %s, processRuntimeName: %s, created: %v}",
		e.tag,
		e.processRuntimeName,
		e.created,
	)
}

// EventNotifier is a function that is used for reporting events from the from
// the supervision system
type EventNotifier func(Event)

// ProcessStopped reports an event with an EventTag of ProcessStopped
func (en EventNotifier) ProcessStopped(name string, stopTime time.Time, err error) {
	tag := ProcessStopped
	if err != nil {
		tag = ProcessFailed
	}

	createdTime := time.Now()
	stopDuration := createdTime.Sub(stopTime)

	en(Event{
		tag:                tag,
		processRuntimeName: name,
		err:                err,
		created:            time.Now(),
		duration:           stopDuration,
	})
}

// ProcessStarted reports an event with an EventTag of ProcessStarted
func (en EventNotifier) ProcessStarted(name string, startTime time.Time) {
	createdTime := time.Now()
	startDuration := createdTime.Sub(startTime)
	en(Event{
		tag:                ProcessStarted,
		processRuntimeName: name,
		err:                nil,
		created:            createdTime,
		duration:           startDuration,
	})
}

// emptyEventNotifier is an utility function that works as a default value
// whenever an EventNotifier is not specified on the Supervisor Spec
func emptyEventNotifier(_ Event) {}

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
	wait        func() error
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

// RuntimeName returns the name of the supervisor that failed
func (se SupervisorError) RuntimeName() string {
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
