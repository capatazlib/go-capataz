package example

import (
	sup "github.com/capatazlib/go-capataz/s"
	sup "github.com/capatazlib/go-capataz/w"
)

var myWorker := w.Go(
)

var mySubSystem := s.NewSupervisor("UDP connection manager")

func newMyWorker(index int) {
	return w.Go(fmt.Sprintf("worker-%d", index))
}

func logSupervisorEventWorker(c chan ProcessEvent) w.Worker {
	return w.Go("event-logger",
		func(ctx context.Context) error {
			for ev := range c {
				select {
					case <-ctx.Done():
					  return ctx.Err()
					default:
					fmt.Printf("%v\n", ev)
				}
			}
		})
}

type ProcessEvent struct {}

func main() {
	c := make(chan ProcessEvent, 100)
	x := c.NewSupervisor(
		"main",
		s.WithObserverChan(c)
		s.WithContext(context.Background()),
		s.WithStrategy(s.AllForOne),
		s.WithWorkerBootJitter(10 * time.Milliseconds)
		s.WithTolerance(10, 10 * time.Second),
		s.WithStartOrder(s.LeftToRight),
		s.WithPanicRescue(true),
		s.WithProcesses(
			logSupervisorEventWorker(c),
			w.Go("foo",
				func(ctx context.Context) { },
				w.WithBackoff(c.ExponentialBackoff),
				w.WithCancelTimeout(500 * time.Millisecond),
				w.WithRunTimeout(1 * time.Minute),
				w.WithWorkerStrategy(c.Permanent),
				w.WithWorkerPanicRescue(true),
				w.WithOnError(func(err error) {

				})
			),
			s.NewSupervisor(
				"child-1"
				s.WithStrategy(c.AllForOne),
				s.WithWorkerBootJitter(10 * time.Milliseconds)
				s.WithTolerance(10, 10 * time.Second),
			),
			mySubSystem,
			myWorker,
		)
	)
	x.Start()
}
