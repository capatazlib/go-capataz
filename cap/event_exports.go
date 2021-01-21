package cap

import "github.com/capatazlib/go-capataz/internal/s"

// EventTag specifies the type of Event that gets notified from the supervision
// system
type EventTag = s.EventTag

// ProcessStarted is an Event that indicates a process started
var ProcessStarted = s.ProcessStarted

// ProcessTerminated is an Event that indicates a process was stopped by a
// parent supervisor
var ProcessTerminated = s.ProcessTerminated

// ProcessStartFailed is an Event that indicates a process failed to start
var ProcessStartFailed = s.ProcessStartFailed

// ProcessFailed is an Event that indicates a process reported an error
var ProcessFailed = s.ProcessFailed

// ProcessCompleted is an Event that indicates a process finished without errors
var ProcessCompleted = s.ProcessCompleted

// Event is a record emitted by the supervision system. The events are used for
// multiple purposes, from testing to monitoring the healthiness of the
// supervision system.
type Event = s.Event

// EventNotifier is a function that is used for reporting events from the from
// the supervision system.
//
// Check the documentation of WithNotifier for more details.
type EventNotifier = s.EventNotifier
