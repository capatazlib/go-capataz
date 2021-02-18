package n

import (
	"errors"
	"strings"

	"github.com/capatazlib/go-capataz/internal/c"
	"github.com/capatazlib/go-capataz/internal/s"
)

// EventCriteria is an utility that allows us to specify a matching criteria to
// a specific supervision event
type EventCriteria func(s.Event) bool

// EAnd joins a slice of EventCriteria with an and statement
func EAnd(crits ...EventCriteria) EventCriteria {
	return func(ev s.Event) bool {
		result := true
		for _, crit := range crits {
			result = result && crit(ev)
			if !result {
				return result
			}
		}
		return result
	}
}

// EOr joins a slice of EventCriteria with an or statement
func EOr(crits ...EventCriteria) EventCriteria {
	return func(ev s.Event) bool {
		result := false
		for _, crit := range crits {
			result = result || crit(ev)
			if result {
				return result
			}
		}
		return result
	}
}

// ENot negates the result from a given EventCriteria
func ENot(crit EventCriteria) EventCriteria {
	return func(ev s.Event) bool {
		return !crit(ev)
	}
}

// EInSubtree allows to filter s.Event that were sent from an specific subtree
func EInSubtree(names ...string) EventCriteria {
	prefix := strings.Join(names, s.NodeSepToken)
	return func(ev s.Event) bool {
		return strings.HasPrefix(ev.GetProcessRuntimeName(), prefix)
	}
}

// EIsFailure returns true if the event represent a node failure
var EIsFailure EventCriteria = func(ev s.Event) bool {
	return ev.GetTag() == s.ProcessFailed
}

// EIsWorkerFailure returns true if the event represents a worker failure
var EIsWorkerFailure EventCriteria = func(ev s.Event) bool {
	return EIsFailure(ev) && ev.GetNodeTag() == c.Worker
}

// EIsSupervisorRestartError returns true if the event represents a restart
// tolerance reached error
var EIsSupervisorRestartError EventCriteria = func(ev s.Event) bool {
	if ev.GetTag() == s.ProcessFailed && ev.GetNodeTag() == c.Supervisor {
		return errors.Is(ev.Err(), &s.SupervisorRestartError{})
	}
	return false
}

// EHasName returns true if the runtime name of the node that emitted the event
// matches the given name
func EHasName(names ...string) EventCriteria {
	name := strings.Join(names, s.NodeSepToken)
	return func(ev s.Event) bool {
		return ev.GetProcessRuntimeName() == name
	}
}

// SelectEventByCriteria forwards Event records that match positively the given
// criteria to the given EventNotifier
func SelectEventByCriteria(crit EventCriteria, notifier s.EventNotifier) s.EventNotifier {
	return func(ev s.Event) {
		if crit(ev) {
			notifier(ev)
		}
	}
}
