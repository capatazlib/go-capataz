package capataz

import (
	"context"

	"github.com/capatazlib/go-capataz/internal/c"
)

// NotifyStartFn is a function given to supervisor workers to notify the
// supervisor that the worker has started.
//
// ### Notify worker's start failure
//
// In case the worker cannot get started it should call this function with an
// error value different than nil.
//
type NotifyStartFn = c.NotifyStartFn

// childToNode transforms a c.ChildSpec into a Node.
func childToNode(chSpec c.ChildSpec) Node {
	return func(_ SupervisorSpec) c.ChildSpec {
		return chSpec
	}
}

// NewWorker creates a `Node` that represents a worker goroutine. It requires
// two arguments: a `name` that is used for runtime tracing and a `startFn`
// function.
//
// ### The `name` argument
//
// The `name` argument must not be empty nor contain forward slash characters
// (e.g. `/`), otherwise, the system will panic. This method is preferred as
// opposed to return an error given it is considered a bad implementation
// (ideally a compilation error).
//
// ### The `startFn` argument
//
// The `startFn` function where your business logic should be located. This
// attribute of a `Node` is going to be used to spawn a new supervised
// goroutine.
//
// The `startFn` function will receive a `context.Context` record that _must_ be
// used inside your business logic to accept stop signals from it's parent
// supervisor.
//
// Depending on the `Shutdown` values used in the `Node` , if the `startFn`
// function does not respect the given context, the parent supervisor will
// either block forever or leak goroutines after a timeout has been reached.
func NewWorker(name string, startFn func(context.Context) error, opts ...WorkerOpt) Node {
	return childToNode(c.New(name, startFn, opts...))
}

// NewWithNotifyStart accomplishes the same goal as `New` with the addition of
// passing a `notifyStart` callback function to the `start` parameter.
//
// ### The `NotifyStartFn` argument
//
// The `NotifyStartFn` is a callback that allows the spawned worker goroutine to
// signal when it has officially started. Is essential to call this callback
// function in your business logic as soon as you consider the worker is
// initialized, otherwise the parent supervisor will block and eventually fail
// with a timeout.
//
// #### Report a start error on `NotifyStartFn`
//
// If for some reason, a child is not able to start correctly, the child should
// call the `NotifyStartFn` function with the start `error` as a parameter.
func NewWorkerWithNotifyStart(
	name string,
	startFn func(context.Context, NotifyStartFn) error,
	opts ...WorkerOpt,
) Node {
	return childToNode(c.NewWithNotifyStart(name, startFn, opts...))
}
