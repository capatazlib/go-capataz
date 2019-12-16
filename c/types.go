package c

import (
	"context"
	"time"
)

// supervisorName represents the runtime name of the supervisor that is spawning
// the current child
type runtimeChildName = string

// Restart specifies when a goroutine gets restarted
type Restart uint32

// ChildTag specifies the type of Child that is running, this is a closed
// set given we only will support workers and supervisors
type ChildTag uint32

const (
	Worker ChildTag = iota
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

const (
	// Permanent Restart = iota
	// Temporary

	// Transient specifies that the goroutine should be restarted if and only if
	// the goroutine failed with an error. If the goroutine finishes without
	// errors it is not restarted again.
	Transient Restart = iota
)

// ShutdownTag specifies the type of Shutdown strategy that is used when
// stopping a goroutine
type ShutdownTag uint32

const (
	infinityT ShutdownTag = iota
	timeoutT
)

// Shutdown indicates how the parent supervisor will handle the stoppping of the
// child goroutine.
type Shutdown struct {
	tag      ShutdownTag
	duration time.Duration
}

// Inf specifies the parent supervisor must wait until Infinity for child
// goroutine to stop executing
var Inf = Shutdown{tag: infinityT}

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

// Opt is used to configure a child's specification
type Opt func(*ChildSpec)

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
// construction of a worker goroutine. The ChildSpec record is used in conjunction
// with the supervisor's ChildSpec.
type ChildSpec struct {
	name     string
	tag      ChildTag
	shutdown Shutdown
	restart  Restart
	start    func(context.Context, NotifyStartFn) error
}

func (cs ChildSpec) Tag() ChildTag {
	return cs.tag
}

// Child is the runtime representation of an Spec
type Child struct {
	runtimeName  string
	spec         ChildSpec
	cancel       func()
	wait         func(Shutdown) error
}

// IsWorker indicates if this child is a worker
func (c Child) IsWorker() bool {
	return c.spec.tag == Worker
}

// ChildNotification reports when a child has terminated; if it terminated with
// an error, it is set in the err field, otherwise, err will be nil.
type ChildNotification struct {
	runtimeName string
	err         error
}

// RuntimeName returns the runtime name of the child that emitted this exit
// notification
func (ce ChildNotification) RuntimeName() string {
	return ce.runtimeName
}

// Unwrap returns the error reported by ChildNotification, if any.
func (ce ChildNotification) Unwrap() error {
	return ce.err
}
