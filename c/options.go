package c

import (
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Restart specifies when a goroutine gets restarted
type Restart = c.Restart

// Permanent specifies that the goroutine should be restarted any time there is
// an error. If the goroutine is finished without errors, it is restarted again.
var Permanent = c.Permanent

// Transient specifies that the goroutine should be restarted if and only if the
// goroutine failed with an error. If the goroutine finishes without errors it
// is not restarted again.
var Transient = c.Transient

// Temporary specifies that the goroutine should not be restarted, not even when
// the goroutine fails
var Temporary = c.Temporary

// Shutdown indicates how the parent supervisor will handle the stoppping of the
// child goroutine.
type Shutdown = c.Shutdown

// Inf specifies the parent supervisor must wait until Infinity for child
// goroutine to stop executing
var Inf = c.Inf

// ChildTag specifies the type of Child that is running, this is a closed
// set given we only will support workers and supervisors
type ChildTag = c.ChildTag

// Worker is used for a c.Child that run a business-logic goroutine
var Worker = c.Worker

// Supervisor is used for a c.Child that runs another supervision tree
var Supervisor = c.Supervisor

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
var Timeout = c.Timeout

// Opt is used to configure a child's specification
type Opt func(*c.ChildSpec)

// WithRestart specifies how the parent supervisor should restart this child
// after an error is encountered.
func WithRestart(r Restart) Opt {
	return func(spec *c.ChildSpec) {
		spec.Restart = r
	}
}

// WithShutdown specifies how the shutdown of the child is going to be handled.
// Read `Inf` and `Timeout` Shutdown values documentation for details.
func WithShutdown(s Shutdown) Opt {
	return func(spec *c.ChildSpec) {
		spec.Shutdown = s
	}
}

// WithTag sets the given c.ChildTag on a c.ChildSpec
func WithTag(t ChildTag) Opt {
	return func(spec *c.ChildSpec) {
		spec.Tag = t
	}
}

// WithTolerance specifies to the supervisor monitor of this child how many
// errors it should be willing to tolerate before giving up restarting it and
// fail.
func WithTolerance(maxErrCount uint32, errWindow time.Duration) Opt {
	return func(spec *c.ChildSpec) {
		spec.ErrTolerance = c.ErrTolerance{MaxErrCount: maxErrCount, ErrWindow: errWindow}
	}
}
