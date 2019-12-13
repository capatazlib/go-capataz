package main

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

type greeterSpec struct {
	name  string
	delay time.Duration
}

// newGreeter returns a worker goroutine that prints the given name every delay
// duration of time
func newGreeter(log *logrus.Entry, spec greeterSpec) c.ChildSpec {
	ticker := time.NewTicker(spec.delay)
	// NOTE: When the supervisor stops or restarts this Child, it's going to
	// cancel the given `context.Context`. It is _essential_ you keep track of the
	// `ctx.Done()` value so that the application runtime doesn't hang.
	return c.New(spec.name, func(ctx context.Context) error {
		for {
			log.Infof("Hello %s", spec.name)
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	})
}

// newGreeterTreeSpec allows you to run a group of greeter workers in the same
// supervision tree
func newGreeterTreeSpec(log *logrus.Entry, name string, specs ...greeterSpec) s.SupervisorSpec {
	greeters := make([]c.ChildSpec, 0, len(specs))
	for _, spec := range specs {
		greeters = append(greeters, newGreeter(log, spec))
	}
	return s.New(name, s.WithChildren(greeters...))
}
