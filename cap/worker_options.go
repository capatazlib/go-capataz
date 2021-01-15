package cap

import (
	"github.com/capatazlib/go-capataz/internal/c"
)

// Restart specifies when a goroutine gets restarted
type Restart = c.Restart

// Permanent specifies that a goroutine should be restarted whether or not there
// are errors.
//
// You can specify this option using the WithRestart function
var Permanent = c.Permanent

// Transient specifies that a goroutine should be restarted if and only if the
// goroutine failed with an error. If the goroutine finishes without errors it
// is not restarted again.
//
// You can specify this option using the WithRestart function
var Transient = c.Transient

// Temporary specifies that the goroutine should not be restarted under any
// circumstances
//
// You can specify this option using the WithRestart function
var Temporary = c.Temporary

// Shutdown is an enum type that indicates how the parent supervisor will handle
// the stoppping of the worker goroutine
//
type Shutdown = c.Shutdown

// Indefinitely is a Shutdown value that specifies the parent supervisor must
// wait indefinitely for the worker goroutine to stop executing. You can specify
// this option using the WithShutdown function
var Indefinitely = c.Indefinitely

// Timeout is a Shutdown function that returns a value that indicates the time
// that the supervisor will wait before "force-killing" a worker goroutine. This
// function receives a time.Duration value. You can specify this option using
// the WithShutdown function.
//
// * Warning
//
// Is important to emphasize that golang **does not** provide a "force-kill"
// mechanism for goroutines.
//
// There is no known way to kill a goroutine via a signal other than using
// context.Done, which the supervised goroutine must observe and respect.
//
// If the timeout is reached and the goroutine does not stop (because the worker
// goroutine is not using the offered context value), the supervisor will
// continue with the shutdown procedure (reporting a shutdown error), possibly
// leaving the goroutine running in memory (e.g. memory leak)
var Timeout = c.Timeout

// NodeTag specifies the type of node that is running. This is a closed set
// given we will only support workers and supervisors
type NodeTag = c.ChildTag

// WorkerT is a NodeTag used to indicate a goroutine is a worker that run some
// business-logic
var WorkerT = c.Worker

// SupervisorT is a NodeTag used to indicate a goroutine is running another
// supervision tree
var SupervisorT = c.Supervisor

// WorkerOpt is used to configure a Worker node spec
type WorkerOpt = c.Opt

// WithRestart is a WorkerOpt that specifies how the parent supervisor should
// restart this worker after an error is encountered.
//
// Possible values may be:
//
// * Permanent -- Always restart worker goroutine
//
// * Transient -- Only restart worker goroutine if it fails
//
// * Temporary -- Never restart a worker goroutine (go keyword behavior)
//
var WithRestart = c.WithRestart

// WithShutdown is a WorkerOpt that specifies how the shutdown of the worker is
// going to be handled. Read Indefinitely and Timeout shutdown values
// documentation for details.
//
// Possible values may be:
//
// * Indefinitely -- Wait forever for the shutdown of this worker goroutine
//
// * Timeout(time.Duration) -- Wait for a duration of time before giving up
// shuting down this worker goroutine
//
var WithShutdown = c.WithShutdown

// WithCapturePanic is a WorkerOpt that specifies if panics raised by
// this worker should be treated as errors.
var WithCapturePanic = c.WithCapturePanic

// WithTag is a WorkerOpt that sets the given NodeTag on Worker.
//
// Do not use this function if you are not extending capataz' API.
var WithTag = c.WithTag

// WithTolerance is a WorkerOpt that specifies how many errors the supervisor
// should be willing to tolerate before giving up restarting and fail.
//
// Deprecated: Use WithErrTolerance instead.
var WithTolerance = c.WithTolerance

// GetWorkerName returns the runtime name of a supervised goroutine by plucking it
// up from the given context.
var GetWorkerName = c.GetNodeName
