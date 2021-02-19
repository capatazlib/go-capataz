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

// EAnd joins a slice of EventCriteria with an "and" statement. A call without
// arguments will accept all given events.
func EAnd(crits ...EventCriteria) EventCriteria {
	return func(ev s.Event) bool {
		for _, crit := range crits {
			if !crit(ev) {
				return false
			}
		}
		return true
	}
}

// EOr joins a slice of EventCriteria with an "or" statement. A call without
// arguments wil accept all given events.
func EOr(crits ...EventCriteria) EventCriteria {
	return func(ev s.Event) bool {
		if len(crits) == 0 {
			return true
		}

		for _, crit := range crits {
			if crit(ev) {
				return true
			}
		}
		return false
	}
}

// ENot negates the result from a given EventCriteria.
func ENot(crit EventCriteria) EventCriteria {
	return func(ev s.Event) bool {
		return !crit(ev)
	}
}

// EInSubtree allows to filter s.Event that were sent from an specific subtree.
func EInSubtree(rawName string) EventCriteria {
	// ensure internal token is not coupled to this API
	tokens := strings.Split(rawName, "/")
	prefix := strings.Join(tokens, s.NodeSepToken)
	return func(ev s.Event) bool {
		return strings.HasPrefix(ev.GetProcessRuntimeName(), prefix)
	}
}

// EIsFailure returns true if the event represent a node failure.
var EIsFailure EventCriteria = func(ev s.Event) bool {
	return ev.GetTag() == s.ProcessFailed
}

// EIsWorkerFailure returns true if the event represents a worker failure.
var EIsWorkerFailure EventCriteria = func(ev s.Event) bool {
	return EIsFailure(ev) && ev.GetNodeTag() == c.Worker
}

// EIsSupervisorRestartError returns true if the event represents a restart
// tolerance reached error
var EIsSupervisorRestartError EventCriteria = func(ev s.Event) bool {
	if ev.GetTag() == s.ProcessFailed && ev.GetNodeTag() == c.Supervisor {
		var restartErr *s.SupervisorRestartError
		return errors.As(ev.Err(), &restartErr)
	}
	return false
}

// EHasRuntimeName returns true if the runtime name of the node that emitted the event
// matches the given name
func EHasRuntimeName(runtimeName string) EventCriteria {
	// ensure internal token is not coupled to this API
	tokens := strings.Split(runtimeName, "/")
	name := strings.Join(tokens, s.NodeSepToken)

	return func(ev s.Event) bool {
		return ev.GetProcessRuntimeName() == name
	}
}

// EHasNameSuffix returns true if the runtime name of the node that emitted the
// event matches the given suffix
func EHasNameSuffix(rawSuffix string) EventCriteria {
	// ensure internal token is not coupled to this API
	tokens := strings.Split(rawSuffix, "/")
	suffix := strings.Join(tokens, s.NodeSepToken)

	return func(ev s.Event) bool {
		return strings.HasSuffix(ev.GetProcessRuntimeName(), suffix)
	}
}

// ApplyEventCriteria forwards Event records that match positively the given
// criteria to the given EventNotifier
func ApplyEventCriteria(crit EventCriteria, notifier s.EventNotifier) s.EventNotifier {
	return func(ev s.Event) {
		if crit(ev) {
			notifier(ev)
		}
	}
}
