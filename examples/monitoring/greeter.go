package main

import (
	"context"
	"time"

	"github.com/capatazlib/go-capataz/capataz"
	"github.com/sirupsen/logrus"
)

type greeterSpec struct {
	name  string
	delay time.Duration
}

// newGreeter returns a worker goroutine that prints the given name every delay
// duration of time
func newGreeter(log *logrus.Entry, spec greeterSpec) capataz.Node {
	ticker := time.NewTicker(spec.delay)
	// NOTE: When the supervisor stops or restarts this worker, it's going to
	// cancel the given `context.Context`. It is _essential_ you keep track of the
	// `ctx.Done()` value so that the application runtime doesn't hang.
	return capataz.NewWorker(spec.name, func(ctx context.Context) error {
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
func newGreeterTreeSpec(log *logrus.Entry, name string, specs ...greeterSpec) capataz.SupervisorSpec {
	greeters := make([]capataz.Node, 0, len(specs))
	for _, spec := range specs {
		greeters = append(greeters, newGreeter(log, spec))
	}
	return capataz.NewSupervisor(name, capataz.WithChildren(greeters...))
}
