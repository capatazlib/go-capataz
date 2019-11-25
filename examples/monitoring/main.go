package main

import (
	"context"
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

const metricsAddr = "0.0.0.0:8080"

func newGreeter(delay time.Duration, name string) func(context.Context) error {
	ticker := time.NewTicker(delay)
	return func(ctx context.Context) error {
		for {
			fmt.Printf("Hello %s\n", name)
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	}
}

func main() {
	metricSpec, evNotifier := newMetricsSpec(metricsAddr)
	app := s.New(
		"root",
		s.WithNotifier(evNotifier),
		s.WithSubtree(metricSpec),
		s.WithChildren(
			c.New("greeter", newGreeter(5*time.Second, "capataz")),
		),
	)

	sup, err := app.Start(context.Background())
	if err != nil {
		panic(err)
	}

	err = sup.Wait()
	if err != nil {
		panic(err)
	}
}
