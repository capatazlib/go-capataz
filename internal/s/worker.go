package s

import (
	"context"

	"github.com/capatazlib/go-capataz/internal/c"
)

// NotifyStartFn is a function given to worker nodes that allows them to notify
// the parent supervisor that they are officialy started.
//
// The argument contains an error if there was a failure, nil otherwise.
//
// See the documentation of NewWorkerWithNotifyStart for more details
type NotifyStartFn = c.NotifyStartFn

// childToNode transforms a c.ChildSpec into a Node.
func childToNode(chSpec c.ChildSpec) Node {
	return func(_ SupervisorSpec) c.ChildSpec {
		return chSpec
	}
}

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
func NewWorker(name string, startFn func(context.Context) error, opts ...c.Opt) Node {
	return childToNode(c.New(name, startFn, opts...))
}

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
func NewWorkerWithNotifyStart(
	name string,
	startFn func(context.Context, NotifyStartFn) error,
	opts ...c.Opt,
) Node {
	return childToNode(c.NewWithNotifyStart(name, startFn, opts...))
}
