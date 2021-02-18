package cap

import (
	"github.com/capatazlib/go-capataz/internal/n"
	"github.com/capatazlib/go-capataz/internal/s"
)

// EventTag specifies the type of Event that gets notified from the supervision
// system
//
// Since: 0.0.0
type EventTag = s.EventTag

// ProcessStarted is an Event that indicates a process started
//
// Since: 0.0.0
var ProcessStarted = s.ProcessStarted

// ProcessTerminated is an Event that indicates a process was stopped by a
// parent supervisor
//
// Since: 0.0.0
var ProcessTerminated = s.ProcessTerminated

// ProcessStartFailed is an Event that indicates a process failed to start
//
// Since: 0.0.0
var ProcessStartFailed = s.ProcessStartFailed

// ProcessFailed is an Event that indicates a process reported an error
//
// Since: 0.0.0
var ProcessFailed = s.ProcessFailed

// ProcessCompleted is an Event that indicates a process finished without errors
//
// Since: 0.0.0
var ProcessCompleted = s.ProcessCompleted

// Event is a record emitted by the supervision system. The events are used for
// multiple purposes, from testing to monitoring the healthiness of the
// supervision system.
//
// Since: 0.0.0
type Event = s.Event

// EventNotifier is a function that is used for reporting events from the from
// the supervision system.
//
// Check the documentation of WithNotifier for more details.
//
// Since: 0.0.0
type EventNotifier = s.EventNotifier

// ReliableNotifierOpt allows clients to tweak the behavior of an EventNotifier
// instance built with NewReliableNotifier
//
// Since: 0.1.0
type ReliableNotifierOpt = n.ReliableNotifierOpt

// NewReliableNotifier is an EventNotifier that guarantees it will never panic
// the execution of its caller, and that it will continue sending events to
// notifiers despite previous panics
//
// Since: 0.1.0
var NewReliableNotifier = n.NewReliableNotifier

// WithOnNotifierTimeout sets callback that gets executed when a given notifier drops
// an event because it cannot process it.
//
// Since: 0.1.0
var WithOnNotifierTimeout = n.WithOnNotifierTimeout

// WithNotifierTimeout sets the maximum allowed time the reliable notifier is going to
// wait for a notifier function to be ready to receive an event (defaults to 10 millis).
//
// since: 0.1.0
var WithNotifierTimeout = n.WithNotifierTimeout

// WithOnReliableNotifierFailure sets a callback that gets executed when a failure
// occurs on the event broadcasting logic
//
// Since: 0.1.0
var WithOnReliableNotifierFailure = n.WithOnReliableNotifierFailure
