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

// WithOnNotifierTimeout sets callback that gets executed when a given notifier
// is so slow to get an event that it gets skipped. You need to ensure the given
// callback does not block.
//
// Since: 0.1.0
var WithOnNotifierTimeout = n.WithOnNotifierTimeout

// WithNotifierTimeout sets the maximum allowed time the reliable notifier is going to
// wait for a notifier function to be ready to receive an event (defaults to 10 millis).
//
// since: 0.1.0
var WithNotifierTimeout = n.WithNotifierTimeout

// WithNotifierBufferSize sets the buffer size for each notifier.
//
// since: 0.3.0
var WithNotifierBufferSize = n.WithNotifierBufferSize

// WithEntrypointBufferSize sets the buffer size for entrypoint of the reliable
// notifier.
//
// since: 0.3.0
var WithEntrypointBufferSize = n.WithEntrypointBufferSize

// WithOnReliableNotifierFailure sets a callback that gets executed when a
// failure occurs on the event broadcasting logic. You need to ensure the given
// callback does not block.
//
// Since: 0.1.0
var WithOnReliableNotifierFailure = n.WithOnReliableNotifierFailure

// EventCriteria is an utility that allows us to specify a matching criteria to
// a specific supervision event
//
// Since: 0.1.0
type EventCriteria = n.EventCriteria

// EAnd joins a slice of EventCriteria with an "and" statement. A call without
// arguments will accept all given events.
//
// Since: 0.1.0
var EAnd = n.EAnd

// EOr joins a slice of EventCriteria with an "or" statement. A call without
// arguments wil reject all given events.
//
// Since: 0.1.0
var EOr = n.EOr

// ENot negates the result from a given EventCriteria
//
// Since: 0.1.0
var ENot = n.ENot

// EInSubtree allows to filter s.Event that were sent from an specific subtree
//
// Since: 0.1.0
var EInSubtree = n.EInSubtree

// EIsFailure returns true if the event represent a node failure
//
// Since: 0.1.0
var EIsFailure = n.EIsFailure

// EIsWorkerFailure returns true if the event represents a worker failure
//
// Since: 0.1.0
var EIsWorkerFailure = n.EIsWorkerFailure

// EIsSupervisorRestartError returns true if the event represents a restart
// tolerance reached error
//
// Since: 0.1.0
var EIsSupervisorRestartError = n.EIsSupervisorRestartError

// EHasRuntimeName returns true if the runtime name of the node that emitted the event
// matches the given name
//
// Since: 0.1.0
var EHasRuntimeName = n.EHasRuntimeName

// EHasNameSuffix returnes true if the runtime name of the node that emitted the event
// matches the given suffix
//
// Since: 0.1.0
var EHasNameSuffix = n.EHasNameSuffix

// ApplyEventCriteria forwards Event records that match positively the given
// criteria to the given EventNotifier
//
// Since: 0.1.0
var ApplyEventCriteria = n.ApplyEventCriteria
