package example

import (
	sup "github.com/capatazlib/go-capataz/s"
	sup "github.com/capatazlib/go-capataz/w"
)

var myWorker := w.Go(
)

var mySubSystem := s.NewSupervisor("UDP connection manager",
....
)

func newMyWorker(index int) {
	return w.Go(fmt.Sprintf("worker-%d", index))
}

func main() {
	x := c.NewSupervisor(
		"main",
		s.WithContext(context.Background()),
		s.WithStrategy(c.AllForOne),
		s.WithWorkerBootJitter(10 * time.Milliseconds)
		s.WithTolerance(10, 10 * time.Second),
		s.WithStartOrder(c.LeftToRight),
		s.WithPanicRescue(true),
		s.WithProcesses(
			w.Go("foo",
				func(ctx context.Context) { },
				w.WithBackoff(c.ExponentialBackoff),
				w.WithCancelTimeout(500 * time.Millisecond),
				w.WithRunTimeout(1 * time.Minute),
				w.WithWorkerStrategy(c.Permanent),
				w.WithWorkerPanicRescue(true),
			),
			s.NewSupervisor(
				"child-1"
				c.WithSupStrategy(c.AllForOne),
				c.WithWorkerBootJitter(10 * time.Milliseconds)
				c.WithTolerance(10, 10 * time.Second),
			),
			mySubSystem,
			myWorker,
		)
	)
	x.Start()
}
