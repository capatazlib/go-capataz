package stest

import (
	"github.com/capatazlib/go-capataz/cap"
	"github.com/capatazlib/go-capataz/smtest"
)

// EventManager provides an API that allows to block a goroutine for particular
// events in a test system
type EventManager = smtest.EventManager[cap.Event]

// EventP represents a predicate function that allows us to assert properties of
// an Event signaled by the supervision system
type EventP = smtest.EventP[cap.Event]

// AndP is a predicate that builds the conjunction of a group EventP predicates
// (e.g. join EventP predicates with &&)
type AndP = smtest.AndP[cap.Event]

// AssertExactMatch is an assertion that checks the input slice of EventP
// predicate match 1 to 1 with a given list of supervision system events.
var AssertExactMatch = smtest.AssertExactMatch[cap.Event]

// NewEventManager returns an EventManager instance that can be used to wait for
// events to happen on the observed supervision system
var NewEventManager = smtest.NewEventManager[cap.Event]
