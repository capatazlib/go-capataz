package s

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
	// ignore zero value of iota
	_ EventTag = iota
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
	nodeTag            c.ChildTag
	processRuntimeName string
	err                error
	created            time.Time
	duration           time.Duration
}

// GetTag returns the EventTag from an Event
func (e Event) GetTag() EventTag {
	return e.tag
}

// GetNodeTag returns the c.ChildTag from an Event
func (e Event) GetNodeTag() c.ChildTag {
	return e.nodeTag
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
	buffer.WriteString(fmt.Sprintf(", nodeTag: %10s", e.nodeTag))
	buffer.WriteString(fmt.Sprintf(", processRuntime: %s", e.processRuntimeName))
	if e.err != nil {
		buffer.WriteString(fmt.Sprintf(", err: %+v", e.err))
	}
	buffer.WriteString("}")
	return buffer.String()
}

// EventNotifier is a function that is used for reporting events from the from
// the supervision system.
//
// Check the documentation of WithNotifier for more details.
type EventNotifier func(Event)

// EventNotifiers is a collection of notifiers.
//
// See EventNotifier.
type EventNotifiers []EventNotifier

// processTerminated reports an event with an EventTag of ProcessTerminated
func (en EventNotifier) processTerminated(
	nodeTag c.ChildTag,
	name string,
	stopTime time.Time,
) {
	createdTime := time.Now()
	stopDuration := createdTime.Sub(stopTime)

	en(Event{
		tag:                ProcessTerminated,
		nodeTag:            nodeTag,
		processRuntimeName: name,
		created:            createdTime,
		duration:           stopDuration,
	})
}

// supervisorTerminated reports an event with an EventTag of ProcessTerminated
func (en EventNotifier) supervisorTerminated(name string, stopTime time.Time) {
	en.processTerminated(c.Supervisor, name, stopTime)
}

// workerCompleted reports an event with an EventTag of ProcessCompleted
func (en EventNotifier) workerCompleted(name string) {
	en(Event{
		tag:                ProcessCompleted,
		nodeTag:            c.Worker,
		processRuntimeName: name,
		created:            time.Now(),
	})
}

// processFailed reports an event with an EventTag of ProcessFailed
func (en EventNotifier) processFailed(
	nodeTag c.ChildTag,
	name string,
	err error,
) {
	en(Event{
		tag:                ProcessFailed,
		nodeTag:            nodeTag,
		processRuntimeName: name,
		err:                err,
		created:            time.Now(),
	})
}

// supervisorFailed reports a supervisor event with an EventTag of ProcessFailed
func (en EventNotifier) supervisorFailed(name string, err error) {
	en.processFailed(c.Supervisor, name, err)
}

// workerFailed reports a worker event with an EventTag of ProcessFailed
func (en EventNotifier) workerFailed(name string, err error) {
	en.processFailed(c.Worker, name, err)
}

// workerFailed reports an event with an EventTag of ProcessFailed
// func (en EventNotifier) workerFailed(name string, err error) {
//	en.processFailed(c.Worker, name, err)
// }

// processStartFailed reports an event with an EventTag of ProcessStartFailed
func (en EventNotifier) processStartFailed(
	nodeTag c.ChildTag,
	name string,
	err error,
) {
	en(Event{
		tag:                ProcessStartFailed,
		nodeTag:            nodeTag,
		processRuntimeName: name,
		err:                err,
	})
}

// supervisorStartFailed reports an event with an EventTag of ProcessFailed
func (en EventNotifier) supervisorStartFailed(name string, err error) {
	en.processStartFailed(c.Supervisor, name, err)
}

// // workerStartFailed reports an event with an EventTag of ProcessFailed
// func (en EventNotifier) workerStartFailed(name string, err error) {
//	en.processStartFailed(c.Worker, name, err)
// }

func processStarted(en EventNotifier, nodeTag c.ChildTag, name string, startTime time.Time) {
	createdTime := time.Now()
	startDuration := createdTime.Sub(startTime)
	en(Event{
		tag:                ProcessStarted,
		nodeTag:            nodeTag,
		processRuntimeName: name,
		err:                nil,
		created:            createdTime,
		duration:           startDuration,
	})
}

// supervisorStarted reports an event with an EventTag of ProcessStarted
func (en EventNotifier) supervisorStarted(name string, startTime time.Time) {
	processStarted(en, c.Supervisor, name, startTime)
}

// workerStarted reports an event with an EventTag of ProcessStarted
func (en EventNotifier) workerStarted(name string, startTime time.Time) {
	processStarted(en, c.Worker, name, startTime)
}

// processTerminated reports an event with an EventTag of ProcessTerminated
func (ens EventNotifiers) processTerminated(nodeTag c.ChildTag, name string, stopTime time.Time) {
	for _, en := range ens {
		en.processTerminated(nodeTag, name, stopTime)
	}
}

// supervisorTerminated reports an event with an EventTag of ProcessTerminated
func (ens EventNotifiers) supervisorTerminated(name string, stopTime time.Time) {
	for _, en := range ens {
		en.supervisorTerminated(name, stopTime)
	}
}

// workerCompleted reports an event with an EventTag of ProcessCompleted
func (ens EventNotifiers) workerCompleted(name string) {
	for _, en := range ens {
		en.workerCompleted(name)
	}
}

// processFailed reports an event with an EventTag of ProcessFailed
func (ens EventNotifiers) processFailed(nodeTag c.ChildTag, name string, err error) {
	for _, en := range ens {
		en.processFailed(nodeTag, name, err)
	}
}

// supervisorFailed reports a supervisor event with an EventTag of ProcessFailed
func (ens EventNotifiers) supervisorFailed(name string, err error) {
	for _, en := range ens {
		en.supervisorFailed(name, err)
	}
}

// workerFailed reports a worker event with an EventTag of ProcessFailed
func (ens EventNotifiers) workerFailed(name string, err error) {
	for _, en := range ens {
		en.workerFailed(name, err)
	}
}

// processStartFailed reports an event with an EventTag of ProcessStartFailed
func (ens EventNotifiers) processStartFailed(nodeTag c.ChildTag, name string, err error) {
	for _, en := range ens {
		en.processStartFailed(nodeTag, name, err)
	}
}

// supervisorStartFailed reports an event with an EventTag of ProcessFailed
func (ens EventNotifiers) supervisorStartFailed(name string, err error) {
	for _, en := range ens {
		en.supervisorStartFailed(name, err)
	}
}

// supervisorStarted reports an event with an EventTag of ProcessStarted
func (ens EventNotifiers) supervisorStarted(name string, startTime time.Time) {
	for _, en := range ens {
		en.supervisorStarted(name, startTime)
	}
}

// workerStarted reports an event with an EventTag of ProcessStarted
func (ens EventNotifiers) workerStarted(name string, startTime time.Time) {
	for _, en := range ens {
		en.workerStarted(name, startTime)
	}
}

// emptyEventNotifier is an utility function that works as a default value
// whenever an EventNotifier is not specified on the Supervisor Spec
func emptyEventNotifier(_ Event) {}
