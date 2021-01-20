package cap

import (
	"time"
)

// Opt is a type used to configure a SupervisorSpec
type Opt func(*SupervisorSpec)

// WithStartOrder is an Opt that specifies the start/stop order of a supervisor's
// children nodes
//
// Possible values may be:
//
// * LeftToRight -- Start children nodes from left to right, stop them from
// right to left
//
// * RightToLeft -- Start children nodes from right to left, stop them from left
// to right
//
func WithStartOrder(o Order) Opt {
	return func(spec *SupervisorSpec) {
		spec.order = o
	}
}

// WithOrder is a backwards compatible alias to WithStartOrder
//
// Deprecated: Use WithStartOrder instead
var WithOrder = WithStartOrder

// WithStrategy is an Opt that specifies how children nodes of a supervisor get
// restarted when one of the nodes fails
//
// Possible values may be:
//
// * OneForOne -- Only restart the failing child
//
// * OneForAll (Not Implemented Yet) -- Restart the failing child and all its
// siblings[*]
//
// [*] This option may come handy when all the other siblings depend on one another
// to work correctly.
//
func WithStrategy(s Strategy) Opt {
	return func(spec *SupervisorSpec) {
		spec.strategy = s
	}
}

// WithNotifier is an Opt that specifies a callback that gets called whenever
// the supervision system reports an Event
//
// This function may be used to observe the behavior of all the supervisors in
// the systems, and it is a great place to hook in monitoring services like
// logging, error tracing and metrics gatherers
//
func WithNotifier(en EventNotifier) Opt {
	return func(spec *SupervisorSpec) {
		spec.eventNotifier = en
	}
}

// WithNodes allows the registration of child nodes in a SupervisorSpec. Node
// records passed to this function are going to be supervised by the Supervisor
// created from a SupervisorSpec.
//
// Check the documentation of NewSupervisorSpec for more details and examples.
func WithNodes(nodes ...Node) BuildNodesFn {
	emptyCleanupResources := func() error { return nil }
	return func() ([]Node, CleanupResourcesFn, error) {
		return nodes, emptyCleanupResources, nil
	}
}

// WithRestartTolerance is a Opt that specifies how many errors the supervisor
// should be willing to tolerate before giving up restarting and fail.
//
// If the tolerance is met, the supervisor is going to fail, if this is a
// sub-tree, this error is going to be handled by a grand-parent supervisor,
// restarting the tolerance again.
//
// Example
//
//   // Tolerate 10 errors every 5 seconds
//   //
//   // - if there is 11 errors in a 5 second window, it makes the supervisor fail
//   //
//   WithRestartTolerance(10, 5 * time.Second)
//
func WithRestartTolerance(maxErrCount uint32, errWindow time.Duration) Opt {
	return func(spec *SupervisorSpec) {
		spec.restartTolerance = restartTolerance{
			MaxRestartCount: maxErrCount,
			RestartWindow:   errWindow,
		}
	}
}
