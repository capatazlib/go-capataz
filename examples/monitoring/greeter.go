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
func newGreeter(log *logrus.Entry, name string, delay time.Duration) c.ChildSpec {
	ticker := time.NewTicker(delay)
	// NOTE: When the supervisor stops or restarts this Child, it's going to
	// cancel the given `context.Context`. It is _essential_ you keep track of the
	// `ctx.Done()` value so that the application runtime doesn't hang.
	return c.New(name, func(ctx context.Context) error {
		for {
			log.Infof("Hello %s", name)
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	})
}

// newGreeterTree allows you to run a group of greeter workers in the same
// supervision tree
func newGreeterTree(log *logrus.Entry, name string, specs ...greeterSpec) s.SupervisorSpec {
	greeters := make([]c.ChildSpec, 0, len(specs))
	for _, spec := range specs {
		greeters = append(greeters, newGreeter(log, spec.name, spec.delay))
	}
	return s.New(name, s.WithChildren(greeters...))
}
