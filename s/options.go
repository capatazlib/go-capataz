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

// // WithChildren specifies a list of child Spec that will get started when the
// // supervisor starts
// func WithChildren(children ...c.ChildSpec) Opt {
//	return func(spec *SupervisorSpec) {
//		spec.children = append(spec.children, children...)
//	}
// }

// // WithSubtree specifies a supervisor sub-tree. Is intended to be used when
// // composing sub-systems in a supervision tree.
// func WithSubtree(subtree SupervisorSpec, copts ...c.Opt) Opt {
//	return func(spec *SupervisorSpec) {
//		cspec := spec.subtree(subtree, copts...)
//		WithChildren(cspec)(spec)
//	}
// }

// WithChildren allows the registration of child nodes in a SupervisorSpec. Node
// records passed to this function are going to be supervised by the Supervisor
// created from the SupervisorSpec
func WithChildren(nodes ...Node) BuildNodesFn {
	emptyCleanupResources := func() error { return nil }
	return func() ([]Node, CleanupResourcesFn) {
		return nodes, emptyCleanupResources
	}
}
