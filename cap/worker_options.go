package cap

import (
	"github.com/capatazlib/go-capataz/internal/c"
)

// Restart specifies when a goroutine gets restarted
type Restart = c.Restart

// Permanent specifies that a goroutine should be restarted whether or not there
// are errors.
var Permanent = c.Permanent

// Transient specifies that a goroutine should be restarted if and only if the
// goroutine failed with an error. If the goroutine finishes without errors it
// is not restarted again.
var Transient = c.Transient

// Temporary specifies that the goroutine should not be restarted under any
// circumstances
var Temporary = c.Temporary

// Shutdown indicates how the parent supervisor will handle the stoppping of the
// worker goroutine.
type Shutdown = c.Shutdown

// Indefinitely specifies the parent supervisor must wait indefinitely for the
// worker goroutine to stop executing
var Indefinitely = c.Indefinitely

// NodeTag specifies the type of node that is running, this is a closed set
// given we will only support workers and supervisors
type NodeTag = c.ChildTag

// Worker is used for a worker that run a business-logic goroutine
var WorkerT = c.Worker

// Supervisor is used for a worker that runs another supervision tree
var SupervisorT = c.Supervisor

// Timeout specifies a duration of time the parent supervisor will wait for the
// worker goroutine to stop executing
//
// ### WARNING:
//
// Is important to emphasize that golang *does not* provide a hard kill
// mechanism for goroutines. There is no known way to kill a goroutine via a
// signal other than using `context.Done` which the supervised goroutine must
// observe and respect. If the timeout is reached and the goroutine does not
// stop, the supervisor will continue with the shutdown procedure (reporting a
// shutdown error), possibly leaving the goroutine running in memory (e.g.
// memory leak).
var Timeout = c.Timeout

// WorkerOpt is used to configure a worker's specification
type WorkerOpt = c.Opt

// WithRestart specifies how the parent supervisor should restart this worker
// after an error is encountered.
var WithRestart = c.WithRestart

// WithShutdown specifies how the shutdown of the worker is going to be handled.
// Read `Indefinitely` and `Timeout` shutdown values documentation for details.
var WithShutdown = c.WithShutdown

// WithTag sets the given NodeTag on a Worker
var WithTag = c.WithTag

// WithTolerance specifies to the supervisor monitor of this worker how many
// errors it should be willing to tolerate before giving up restarting it and
// fail.
var WithTolerance = c.WithTolerance
