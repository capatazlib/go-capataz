package c

import "time"

// WithRestart specifies how the parent supervisor should restart this worker
// after an error is encountered.
func WithRestart(r Restart) Opt {
	return func(spec *ChildSpec) {
		spec.Restart = r
	}
}

// WithCapturePanic specifies if panics raised by this worker should be treated
// as errors. restartable errors.
func WithCapturePanic(capture bool) Opt {
	return func(spec *ChildSpec) {
		spec.CapturePanic = capture
	}
}

// WithShutdown specifies how the shutdown of the worker is going to be handled.
// Read `Indefinitely` and `Timeout` shutdown values documentation for details.
func WithShutdown(s Shutdown) Opt {
	return func(spec *ChildSpec) {
		spec.Shutdown = s
	}
}

// WithTolerance specifies to the supervisor monitor of this worker how many
// errors it should be willing to tolerate before giving up restarting it and
// fail.
//
func WithTolerance(maxErrCount uint32, errWindow time.Duration) Opt {
	return func(spec *ChildSpec) {}
}

// WithTag sets the given c.ChildTag on a c.ChildSpec
func WithTag(t ChildTag) Opt {
	return func(spec *ChildSpec) {
		spec.Tag = t
	}
}
