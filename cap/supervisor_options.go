package cap

// Opt is a type used to configure a SupervisorSpec
type Opt func(*SupervisorSpec)

// WithOrder is an Opt that specifies the start/stop order of a supervisor's
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
func WithOrder(o Order) Opt {
	return func(spec *SupervisorSpec) {
		spec.order = o
	}
}

// WithStrategy is an Opt that specifies how children nodes of a supervisor get
// restarted when one of the node fails
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
