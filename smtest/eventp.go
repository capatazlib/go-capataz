package smtest

import (
	"strings"
)

////////////////////////////////////////////////////////////////////////////////

// EventP represents a predicate function that allows us to assert properties of
// an Event signaled by the supervision system
type EventP[A any] interface {

	// Call will execute the logic of this event predicate
	Call(A) bool

	// Returns an string representation of this event predicate (for debugging
	// purposes)
	String() string
}

// AndP is a predicate that builds the conjunction of a group EventP predicates
// (e.g. join EventP predicates with &&)
type AndP[A any] struct {
	Preds []EventP[A]
}

// Call will try and verify that all it's grouped predicates return true, if any
// returns false, this predicate function will return false
func (p AndP[A]) Call(ev A) bool {
	acc := true
	for _, pred := range p.Preds {
		acc = acc && pred.Call(ev)
		if !acc {
			return acc
		}
	}
	return acc
}

func (p AndP[A]) String() string {
	acc := make([]string, 0, len(p.Preds))
	for _, pred := range p.Preds {
		acc = append(acc, pred.String())
	}
	return strings.Join(acc, " && ")
}

// OrP is a predicate that builds the adjunction of a group EventP predicates
// (e.g. join EventP predicates with ||)
type OrP[A any] struct {
	Preds []EventP[A]
}

// Call will try and verify that all it's grouped predicates return true, if any
// returns false, this predicate function will return false
func (p OrP[A]) Call(ev A) bool {
	if len(p.Preds) == 0 {
		return true
	}
	acc := false

	for _, pred := range p.Preds {
		acc = acc || pred.Call(ev)
		if acc {
			return acc
		}
	}
	return acc
}

func (p OrP[A]) String() string {
	acc := make([]string, 0, len(p.Preds))
	for _, pred := range p.Preds {
		acc = append(acc, pred.String())
	}
	return strings.Join(acc, " || ")
}
