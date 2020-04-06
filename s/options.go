package s

// This file all the constructor options for a SupervisorSpec

// Opt is used to configure a supervisor's specification
type Opt func(*SupervisorSpec)

// WithOrder specifies the start/stop order of a supervisor's children
func WithOrder(o Order) Opt {
	return func(spec *SupervisorSpec) {
		spec.order = o
	}
}

// WithStrategy specifies how children get restarted when one of them fails
func WithStrategy(s Strategy) Opt {
	return func(spec *SupervisorSpec) {
		spec.strategy = s
	}
}

// WithNotifier specifies a callback that gets called whenever the supervision
// system reports an Event
func WithNotifier(en EventNotifier) Opt {
	return func(spec *SupervisorSpec) {
		spec.eventNotifier = en
	}
}

// WithChildren allows the registration of child nodes in a SupervisorSpec. Node
// records passed to this function are going to be supervised by the Supervisor
// created from the SupervisorSpec
func WithChildren(nodes ...Node) BuildNodesFn {
	emptyCleanupResources := func() error { return nil }
	return func() ([]Node, CleanupResourcesFn, error) {
		return nodes, emptyCleanupResources, nil
	}
}
