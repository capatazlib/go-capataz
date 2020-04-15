package capataz

import (
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// EventTag specifies the type of Event that gets notified from the supervision
// system
type EventTag uint32

const (
	eventNoop EventTag = iota
	// ProcessStarted is an Event that indicates a process started
	ProcessStarted
	// ProcessTerminated is an Event that indicates a process was stopped by a parent
	// supervisor
	ProcessTerminated
	// ProcessStartFailed is an Event that indicates a process failed to start
	ProcessStartFailed
	// ProcessFailed is an Event that indicates a process reported an error
	ProcessFailed
	// ProcessCompleted is an Event that indicates a process finished without errors
	ProcessCompleted
)

// String returns a string representation of the current EventTag
func (tag EventTag) String() string {
	switch tag {
	case ProcessStarted:
		return "ProcessStarted"
	case ProcessTerminated:
		return "ProcessTerminated"
	case ProcessStartFailed:
		return "ProcessStartFailed"
	case ProcessFailed:
		return "ProcessFailed"
	case ProcessCompleted:
		return "ProcessCompleted"
	default:
		return "<Unknown>"
	}
}

// Event is a record emitted by the supervision system. The events are used for
// multiple purposes, from testing to monitoring the healthiness of the
// supervision system.
type Event struct {
	tag                EventTag
	childTag           c.ChildTag
	processRuntimeName string
	err                error
	created            time.Time
	duration           time.Duration
}

// GetTag returns the EventTag from an Event
func (e Event) GetTag() EventTag {
	return e.tag
}

// GetChildTag returns the ChildTag from an Event
func (e Event) GetChildTag() c.ChildTag {
	return e.childTag
}

// GetProcessRuntimeName returns the given name of a process that emitted this event
func (e Event) GetProcessRuntimeName() string {
	return e.processRuntimeName
}

// Err returns an error reported by the process that emitted this event
func (e Event) Err() error {
	return e.err
}

// GetCreated returns a timestamp of the creation of the event by the process
func (e Event) GetCreated() time.Time {
	return e.created
}

// String returns an string representation for the Event
func (e Event) String() string {
	var buffer strings.Builder
	buffer.WriteString("Event{")
	buffer.WriteString(fmt.Sprintf("created: %55s", e.created.String()))
	buffer.WriteString(fmt.Sprintf(", tag: %20s", e.tag))
	buffer.WriteString(fmt.Sprintf(", childTag: %10s", e.childTag))
	buffer.WriteString(fmt.Sprintf(", processRuntime: %s", e.processRuntimeName))
	if e.err != nil {
		buffer.WriteString(fmt.Sprintf(", err: %+v", e.err))
	}
	buffer.WriteString("}")
	return buffer.String()
}

// EventNotifier is a function that is used for reporting events from the from
// the supervision system
type EventNotifier func(Event)

// ProcessTerminated reports an event with an EventTag of ProcessTerminated
func (en EventNotifier) ProcessTerminated(
	childTag c.ChildTag,
	name string,
	stopTime time.Time,
) {
	createdTime := time.Now()
	stopDuration := createdTime.Sub(stopTime)

	en(Event{
		tag:                ProcessTerminated,
		childTag:           childTag,
		processRuntimeName: name,
		created:            createdTime,
		duration:           stopDuration,
	})
}

// SupervisorTerminated reports an event with an EventTag of ProcessTerminated
func (en EventNotifier) SupervisorTerminated(name string, stopTime time.Time) {
	en.ProcessTerminated(c.Supervisor, name, stopTime)
}

// WorkerCompleted reports an event with an EventTag of ProcessCompleted
func (en EventNotifier) WorkerCompleted(name string) {
	en(Event{
		tag:                ProcessCompleted,
		childTag:           c.Worker,
		processRuntimeName: name,
		created:            time.Now(),
	})
}

// ProcessFailed reports an event with an EventTag of ProcessStartFailed
func (en EventNotifier) ProcessFailed(
	childTag c.ChildTag,
	name string,
	err error,
) {
	en(Event{
		tag:                ProcessFailed,
		childTag:           childTag,
		processRuntimeName: name,
		err:                err,
		created:            time.Now(),
	})
}

// SupervisorFailed reports an event with an EventTag of ProcessFailed
func (en EventNotifier) SupervisorFailed(name string, err error) {
	en.ProcessFailed(c.Supervisor, name, err)
}

// WorkerFailed reports an event with an EventTag of ProcessFailed
func (en EventNotifier) WorkerFailed(name string, err error) {
	en.ProcessFailed(c.Worker, name, err)
}

// ProcessStartFailed reports an event with an EventTag of ProcessStartFailed
func (en EventNotifier) ProcessStartFailed(
	childTag c.ChildTag,
	name string,
	err error,
) {
	en(Event{
		tag:                ProcessStartFailed,
		childTag:           childTag,
		processRuntimeName: name,
		err:                err,
	})
}

// SupervisorStartFailed reports an event with an EventTag of ProcessFailed
func (en EventNotifier) SupervisorStartFailed(name string, err error) {
	en.ProcessStartFailed(c.Supervisor, name, err)
}

// WorkerStartFailed reports an event with an EventTag of ProcessFailed
func (en EventNotifier) WorkerStartFailed(name string, err error) {
	en.ProcessStartFailed(c.Worker, name, err)
}

func processStarted(en EventNotifier, childTag c.ChildTag, name string, startTime time.Time) {
	createdTime := time.Now()
	startDuration := createdTime.Sub(startTime)
	en(Event{
		tag:                ProcessStarted,
		childTag:           childTag,
		processRuntimeName: name,
		err:                nil,
		created:            createdTime,
		duration:           startDuration,
	})
}

// SupervisorStarted reports an event with an EventTag of ProcessStarted
func (en EventNotifier) SupervisorStarted(name string, startTime time.Time) {
	processStarted(en, c.Supervisor, name, startTime)
}

// WorkerStarted reports an event with an EventTag of ProcessStarted
func (en EventNotifier) WorkerStarted(name string, startTime time.Time) {
	processStarted(en, c.Worker, name, startTime)
}

// emptyEventNotifier is an utility function that works as a default value
// whenever an EventNotifier is not specified on the Supervisor Spec
func emptyEventNotifier(_ Event) {}
