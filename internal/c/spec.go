package c

import (
	"context"
	"time"
)

// Opt is used to configure a child's specification
type Opt func(*ChildSpec)

// ChildTag specifies the type of Child that is running, this is a closed
// set given we only will support workers and supervisors
type ChildTag uint32

const (
	// Worker is used for a c.Child that run a business-logic goroutine
	Worker ChildTag = iota
	// Supervisor is used for a c.Child that runs another supervision tree
	Supervisor
)

func (ct ChildTag) String() string {
	switch ct {
	case Worker:
		return "Worker"
	case Supervisor:
		return "Supervisor"
	default:
		return "<Unknown>"
	}
}

// Restart specifies when a goroutine gets restarted
type Restart uint32

const (
	// Permanent specifies that the goroutine should be restarted any time there
	// is an error. If the goroutine is finished without errors, it is restarted
	// again.
	Permanent Restart = iota

	// Transient specifies that the goroutine should be restarted if and only if
	// the goroutine failed with an error. If the goroutine finishes without
	// errors it is not restarted again.
	Transient

	// Temporary specifies that the goroutine should not be restarted, not even
	// when the goroutine fails
	Temporary
)

func (r Restart) String() string {
	switch r {
	case Permanent:
		return "Permanent"
	case Transient:
		return "Transient"
	case Temporary:
		return "Temporary"
	default:
		return "<Unknown>"
	}
}

// ShutdownTag specifies the type of Shutdown strategy that is used when
// stopping a goroutine
type ShutdownTag uint32

const (
	indefinitelyT ShutdownTag = iota
	timeoutT
)

// Shutdown indicates how the parent supervisor will handle the stoppping of the
// child goroutine.
type Shutdown struct {
	tag      ShutdownTag
	duration time.Duration
}

// Indefinitely specifies the parent supervisor must wait indefinitely for child
// goroutine to stop executing
var Indefinitely = Shutdown{tag: indefinitelyT}

// Timeout specifies a duration of time the parent supervisor will wait for the
// child goroutine to stop executing
//
// ### WARNING:
//
// A point worth bringing up is that golang *does not* provide a hard kill
// mechanism for goroutines. There is no known way to kill a goroutine via a
// signal other than using `context.Done` and the goroutine respecting this
// mechanism. If the timeout is reached and the goroutine does not stop, the
// supervisor will continue with the shutdown procedure, possibly leaving the
// goroutine running in memory (e.g. memory leak).
func Timeout(d time.Duration) Shutdown {
	return Shutdown{
		tag:      timeoutT,
		duration: d,
	}
}

// startError is the error reported back to a Supervisor when the start of a
// Child fails
type startError = error

// NotifyStartFn is a function given to supervisor children to notify the
// supervisor that the child has started.
//
// ### Notify child's start failure
//
// In case the child cannot get started it should call this function with an
// error value different than nil.
//
type NotifyStartFn = func(startError)

// ChildSpec represents a Child specification; it serves as a template for the
// construction of a goroutine. The ChildSpec record is used in conjunction with
// the supervisor's SupervisorSpec.
//
// # A note about ChildTag
//
// An approach that we considered was to define a type heriarchy for
// SupervisorChildSpec and WorkerChildSpec to deal with differences between
// Workers and Supervisors rather than having a value you can use in a switch
// statement. In reality, the differences between the two are minimal (only
// behavior change happens when sending notifications to the events system). If
// this changes, we may consider a design where we have a ChildSpec interface
// and we have different implementations.
type ChildSpec struct {
	Name         string
	Tag          ChildTag
	Shutdown     Shutdown
	Restart      Restart
	ErrTolerance ErrTolerance
	CapturePanic bool

	Start func(context.Context, NotifyStartFn) error
}

// GetTag returns the ChildTag of this ChildSpec
func (chSpec ChildSpec) GetTag() ChildTag {
	return chSpec.Tag
}

// IsWorker indicates if this child is a worker
func (chSpec ChildSpec) IsWorker() bool {
	return chSpec.Tag == Worker
}

// GetRestart returns the Restart setting for this ChildSpec
func (chSpec ChildSpec) GetRestart() Restart {
	return chSpec.Restart
}

// DoesCapturePanic indicates if this child handles panics
func (chSpec ChildSpec) DoesCapturePanic() bool {
	return chSpec.CapturePanic
}
