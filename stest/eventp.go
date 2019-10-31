package stest

import (
	"fmt"
	"strings"

	"github.com/capatazlib/go-capataz/s"
)

////////////////////////////////////////////////////////////////////////////////

// EventP represents a predicate function that allows us to assert properties of
// an Event signaled by the supervision system
type EventP interface {

	// Call will execute the logic of this event predicate
	Call(s.Event) bool

	// Returns an string representation of this event predicate (for debugging
	// purposes)
	String() string
}

// EventTagP is a predicate that asserts the `s.EventTag` of a given `s.Event`
// matches an expected `s.EventTag`
type EventTagP struct {
	tag s.EventTag
}

// Call will execute predicate that checks tag name of event
func (p EventTagP) Call(ev s.Event) bool {
	return ev.Tag() == p.tag
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
func (p ProcessNameP) Call(ev s.Event) bool {
	return ev.ProcessRuntimeName() == p.name
}

func (p ProcessNameP) String() string {
	return fmt.Sprintf("name == %s", p.name)
}

// AndP is a predicate that builds the conjunction of a group EventP predicates
// (e.g. join EventP predicates with &&)
type AndP struct {
	preds []EventP
}

// Call will try and verify that all it's grouped predicates return true, if any
// returns false, this predicate function will return false
func (p AndP) Call(ev s.Event) bool {
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

// ProcessStarted is a predicate to assert an event represents a process that
// got started
func ProcessStarted(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessStarted},
			ProcessNameP{name: name},
		},
	}
}

// ProcessStopped is a predicate to assert an event represents a process that
// got stopped by its parent supervisor
func ProcessStopped(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessStopped},
			ProcessNameP{name: name},
		},
	}
}

// ProcessFailed is a predicate to assert an event represents a process that
// failed
func ProcessFailed(name string) EventP {
	return AndP{
		preds: []EventP{
			EventTagP{tag: s.ProcessFailed},
			ProcessNameP{name: name},
		},
	}
}
