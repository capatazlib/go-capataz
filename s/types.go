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
func (o Order) SortStart(input0 []c.Spec) []c.Spec {
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
func (o Order) SortStop(input0 []c.Spec) []c.Spec {
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
	tag     EventTag
	name    string
	err     error
	created time.Time
}

// Tag returns the EventTag from an Event
func (e Event) Tag() EventTag {
	return e.tag
}

// Name returns the given name of a process that emitted this event
func (e Event) Name() string {
	return e.name
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
	return fmt.Sprintf("Event{tag: %s, name: %s, created: %v}", e.tag, e.name, e.created)
}

// EventNotifier is a function that is used for reporting events from the from
// the supervision system
type EventNotifier func(Event)

// ProcessStopped reports an event with an EventTag of ProcessStopped
func (en EventNotifier) ProcessStopped(name string, err error) {
	tag := ProcessStopped
	if err != nil {
		tag = ProcessFailed
	}

	en(Event{
		tag:     tag,
		name:    name,
		err:     err,
		created: time.Now(),
	})
}

// ProcessStarted reports an event with an EventTag of ProcessStarted
func (en EventNotifier) ProcessStarted(name string) {
	en(Event{
		tag:     ProcessStarted,
		name:    name,
		err:     nil,
		created: time.Now(),
	})
}

// Opt is used to configure a supervisor's specification
type Opt func(*Spec)

// Spec represents the specification of a Supervisor; it serves as a template
// for the construction of supervision trees. Using an Spec you can create
// Supervisor records
type Spec struct {
	name          string
	order         Order
	strategy      Strategy
	children      []c.Spec
	eventNotifier EventNotifier
}

// Supervisor represents a runtime supervision tree. You can either join the
// Supervisor goroutine with the current goroutine, or stop it altogether.
type Supervisor struct {
	runtimeName string
	spec        Spec
	children    map[string]c.Child
	cancel      func()
	wait        func() error
}
