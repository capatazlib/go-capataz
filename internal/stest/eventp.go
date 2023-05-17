package stest

import (
	"fmt"

	"github.com/capatazlib/go-capataz/cap"
	"github.com/capatazlib/go-capataz/smtest"
	"github.com/capatazlib/go-capataz/internal/c"
)

////////////////////////////////////////////////////////////////////////////////

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

// ProcessNodeTagP is a predicate that asserts the ChildTag of the `Child` that
// triggered the event matches the expect ChildTag
type ProcessNodeTagP struct {
	nodeTag cap.NodeTag
}

// Call will execute predicate that checks the ChildTag of the process that
// triggered the event matches the expected ChildTag
func (p ProcessNodeTagP) Call(ev cap.Event) bool {
	return ev.GetNodeTag() == p.nodeTag
}

func (p ProcessNodeTagP) String() string {
	return fmt.Sprintf("nodeTag == %s", p.nodeTag)
}

// ErrorMsgP is a predicate that asserts the message of an error is the one
// specified
type ErrorMsgP struct {
	errMsg string
}

// Call will try and verify that the event has an error attribute with the
// specified message
func (p ErrorMsgP) Call(ev cap.Event) bool {
	err := ev.Err()
	return err != nil && err.Error() == p.errMsg
}

func (p ErrorMsgP) String() string {
	return fmt.Sprintf("err == %s", p.errMsg)
}

////////////////////////////////////////////////////////////////////////////////

// ProcessName is a predicate to assert an event was triggered by the given
// runtime process name
func ProcessName(name string) EventP {
	return ProcessNameP{name: name}
}

// SupervisorStarted is a predicate to assert an event represents a process that
// got started
func SupervisorStarted(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessStarted},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Supervisor},
		},
	}
}

// WorkerStarted is a predicate to assert an event represents a process that
// got started
func WorkerStarted(name string) smtest.EventP[cap.Event] {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessStarted},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Worker},
		},
	}
}

// WorkerCompleted is a predicate to assert an event represents a worker process
// that got completed
func WorkerCompleted(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessCompleted},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Worker},
		},
	}
}

// SupervisorTerminated is a predicate to assert an event represents a process that
// got stopped by its parent supervisor
func SupervisorTerminated(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessTerminated},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Supervisor},
		},
	}
}

// WorkerTerminated is a predicate to assert an event represents a process that
// got stopped by its parent supervisor
func WorkerTerminated(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessTerminated},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Worker},
		},
	}
}

// SupervisorFailed is a predicate to assert an event represents a process that
// failed
func SupervisorFailed(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessFailed},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Supervisor},
		},
	}
}

// WorkerFailed is a predicate to assert an event represents a process that
// failed
func WorkerFailed(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessFailed},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Worker},
		},
	}
}

// WorkerFailedWith is a predicate to assert an event represents a process that
// failed
func WorkerFailedWith(name, errMsg string) smtest.EventP[cap.Event] {
	return smtest.AndP[cap.Event]{
		Preds: []smtest.EventP[cap.Event]{
			EventTagP{tag: cap.ProcessFailed},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Worker},
			ErrorMsgP{errMsg: errMsg},
		},
	}
}

// SupervisorStartFailed is a predicate to assert an event represents a process
// that failed on start
func SupervisorStartFailed(name string) EventP {
	return AndP{
		Preds: []EventP{
			EventTagP{tag: cap.ProcessStartFailed},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Supervisor},
		},
	}
}

// WorkerStartFailed is a predicate to assert an event represents a process
// that failed on start
func WorkerStartFailed(name string) smtest.EventP[cap.Event] {
	return smtest.AndP[cap.Event]{
		Preds: []smtest.EventP[cap.Event]{
			EventTagP{tag: cap.ProcessStartFailed},
			ProcessNameP{name: name},
			ProcessNodeTagP{nodeTag: c.Worker},
		},
	}
}
