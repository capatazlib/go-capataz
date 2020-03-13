package c

import (
	"context"
	"time"
)

// Restart specifies when a goroutine gets restarted
type Restart uint32

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

// Tag returns the ChildTag of this ChildSpec
func (cs ChildSpec) Tag() ChildTag {
	return cs.tag
}

// IsWorker indicates if this child is a worker
func (cs ChildSpec) IsWorker() bool {
	return cs.tag == Worker
}

// Child is the runtime representation of an Spec
type Child struct {
	runtimeName  string
	spec         ChildSpec
	restartCount uint32
	cancel       func()
	wait         func(Shutdown) error
}

// RuntimeName returns the name of this child (once started). It will have a
// prefix with the supervisor name
func (c Child) RuntimeName() string {
	return c.runtimeName
}

// Name returns the name of the `ChildSpec` of this child
func (c Child) Name() string {
	return c.spec.name
}

// Spec returns the `ChildSpec` of this child
func (c Child) Spec() ChildSpec {
	return c.spec
}

// IsWorker indicates if this child is a worker
func (c Child) IsWorker() bool {
	return c.spec.IsWorker()
}

// ChildNotification reports when a child has terminated; if it terminated with
// an error, it is set in the err field, otherwise, err will be nil.
type ChildNotification struct {
	name        string
	runtimeName string
	err         error
}

// Name returns the spec name of the child that emitted this notification
func (ce ChildNotification) Name() string {
	return ce.name
}

// RuntimeName returns the runtime name of the child that emitted this
// notification
func (ce ChildNotification) RuntimeName() string {
	return ce.runtimeName
}

// Unwrap returns the error reported by ChildNotification, if any.
func (ce ChildNotification) Unwrap() error {
	return ce.err
}
