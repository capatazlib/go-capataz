package cap

import (
	"github.com/capatazlib/go-capataz/internal/c"
	"github.com/capatazlib/go-capataz/internal/s"
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
// Deprecated: Use WithRestartTolerance instead.
var WithTolerance = c.WithTolerance

// GetWorkerName returns the runtime name of a supervised goroutine by plucking it
// up from the given context.
var GetWorkerName = c.GetNodeName

// NotifyStartFn is a function given to worker nodes that allows them to notify
// the parent supervisor that they are officialy started.
//
// The argument contains an error if there was a failure, nil otherwise.
//
// See the documentation of NewWorkerWithNotifyStart for more details
type NotifyStartFn = c.NotifyStartFn

// NewWorker creates a Node that represents a worker goroutine. It requires two
// arguments: a name that is used for runtime tracing and a startFn function.
//
// The name argument
//
// A name argument must not be empty nor contain forward slash characters (e.g.
// /), otherwise, the system will panic[*].
//
// [*] This method is preferred as opposed to return an error given it is considered
// a bad implementation (ideally a compilation error).
//
// The startFn argument
//
// The startFn function is where your business logic should be located. This
// function will be running on a new supervised goroutine.
//
// The startFn function will receive a context.Context record that *must* be
// used inside your business logic to accept stop signals from its parent
// supervisor.
//
// Depending on the Shutdown values used with the WithShutdown settings of the
// worker, if the `startFn` function does not respect the given context, the
// parent supervisor will either block forever or leak goroutines after a
// timeout has been reached.
//
var NewWorker = s.NewWorker

// NewWorkerWithNotifyStart accomplishes the same goal as NewWorker with the
// addition of passing an extra argument (notifyStart callback) to the startFn
// function parameter.
//
// The NotifyStartFn argument
//
// Sometimes you want to consider a goroutine started after certain
// initialization was done; like doing a read from a Database or API, or some
// socket is bound, etc. The NotifyStartFn is a callback that allows the spawned
// worker goroutine to signal when it has officially started.
//
// It is essential to call this callback function in your business logic as soon
// as you consider the worker is initialized, otherwise the parent supervisor
// will block and eventually fail with a timeout.
//
// Report a start error on NotifyStartFn
//
// If for some reason, a child node is not able to start correctly (e.g. DB
// connection fails, network is kaput), the node may call the given
// NotifyStartFn function with the impending error as a parameter. This will
// cause the whole supervision system start procedure to abort.
//
var NewWorkerWithNotifyStart = s.NewWorkerWithNotifyStart
