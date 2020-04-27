package stest

import (
	"fmt"
	"strings"

	"github.com/capatazlib/go-capataz/cap"
	"github.com/capatazlib/go-capataz/internal/c"
)

////////////////////////////////////////////////////////////////////////////////

// EventP represents a predicate function that allows us to assert properties of
// an Event signaled by the supervision system
type EventP interface {

	// Call will execute the logic of this event predicate
	Call(cap.Event) bool

	// Returns an string representation of this event predicate (for debugging
	// purposes)
	String() string
}

// EventTagP is a predicate that asserts the `cap.EventTag` of a given `capataz.Event`
// matches an expected `cap.EventTag`
type EventTagP struct {
	tag cap.EventTag
}

// Call will execute predicate that checks tag name of event
func (p EventTagP) Call(ev cap.Event) bool {
	return ev.GetTag() == p.tag
}

func (p EventTagP) String() string {
	return fmt.Sprintf("tag == %s", p.tag.String())
}

// ProcessNameP is a predicate that asserts the name of the `Child` that
// triggered the event matches the expected name
type ProcessNameP struct {
	name string
}

// Call will execute predicate that checks the name of the process that
// triggered the event
func (p ProcessNameP) Call(ev cap.Event) bool {
	return ev.GetProcessRuntimeName() == p.name
}

func (p ProcessNameP) String() string {
	return fmt.Sprintf("name == %s", p.name)
}

// ProcessChildTagP is a predicate that asserts the ChildTag of the `Child` that
// triggered the event matches the expect ChildTag
type ProcessChildTagP struct {
	childTag c.ChildTag
}

// Call will execute predicate that checks the ChildTag of the process that
// triggered the event matches the expected ChildTag
func (p ProcessChildTagP) Call(ev cap.Event) bool {
	return ev.GetChildTag() == p.childTag
}

func (p ProcessChildTagP) String() string {
	return fmt.Sprintf("childTag == %s", p.childTag)
}

// AndP is a predicate that builds the conjunction of a group EventP predicates
// (e.g. join EventP predicates with &&)
type AndP struct {
	preds []EventP
}

// Call will try and verify that all it's grouped predicates return true, if any
// returns false, this predicate function will return false
func (p AndP) Call(ev cap.Event) bool {
	acc := true
	for _, pred := range p.preds {
		acc = acc && pred.Call(ev)
		if !acc {
			return acc
		}
	}
	return acc
}

func (p AndP) String() string {
	acc := make([]string, 0, len(p.preds))
	for _, pred := range p.preds {
		acc = append(acc, pred.String())
	}
	return strings.Join(acc, " && ")
}

// ProcessName is a predicate to assert an event was triggered by the given
// runtime process name
func ProcessName(name string) EventP {
	return ProcessNameP{name: name}
}

// SupervisorStarted is a predicate to assert an event represents a process that
// got started
func SupervisorStarted(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessStarted},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Supervisor},
		},
	}
}

// WorkerStarted is a predicate to assert an event represents a process that
// got started
func WorkerStarted(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessStarted},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Worker},
		},
	}
}

// WorkerCompleted is a predicate to assert an event represents a worker process
// that got completed
func WorkerCompleted(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessCompleted},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Worker},
		},
	}
}

// SupervisorTerminated is a predicate to assert an event represents a process that
// got stopped by its parent supervisor
func SupervisorTerminated(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessTerminated},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Supervisor},
		},
	}
}

// WorkerTerminated is a predicate to assert an event represents a process that
// got stopped by its parent supervisor
func WorkerTerminated(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessTerminated},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Worker},
		},
	}
}

// SupervisorFailed is a predicate to assert an event represents a process that
// failed
func SupervisorFailed(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessFailed},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Supervisor},
		},
	}
}

// WorkerFailed is a predicate to assert an event represents a process that
// failed
func WorkerFailed(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessFailed},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Worker},
		},
	}
}

// SupervisorStartFailed is a predicate to assert an event represents a process
// that failed on start
func SupervisorStartFailed(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessStartFailed},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Supervisor},
		},
	}
}

// WorkerStartFailed is a predicate to assert an event represents a process
// that failed on start
func WorkerStartFailed(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: cap.ProcessStartFailed},
			ProcessNameP{name: name},
			ProcessChildTagP{childTag: c.Worker},
		},
	}
}
