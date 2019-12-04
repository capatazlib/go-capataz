package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

// default port for prometheus metrics server
const metricsAddr = "0.0.0.0:8080"

// newGreeter returns a worker goroutine that prints the given name every delay
// duration of time
func newGreeter(log *logrus.Entry, name string, delay time.Duration) c.ChildSpec {
	ticker := time.NewTicker(delay)
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

// stopOnSignal will wait for the program to receive a SIGTERM and stop the
// given supervisor
func stopOnSignal(sup s.Supervisor) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM)
	<-done
	err := sup.Stop()
	fmt.Println(err)
}

func newLogEventNotifier() (*logrus.Entry, s.EventNotifier) {
	log := logrus.New()
	log.Out = os.Stdout
	log.Level = logrus.DebugLevel
	log.SetFormatter(&logrus.JSONFormatter{})

	ll := log.WithFields(logrus.Fields{})

	return ll, func(ev s.Event) {
		if ev.Err() != nil {
			ll = log.WithError(ev.Err())
		}
		ll.WithFields(logrus.Fields{
			"process_runtime_name": ev.ProcessRuntimeName(),
			"created_at":           ev.Created(),
		}).Debug(ev.Tag().String())
	}
}

func main() {
	log, logEventNotifier := newLogEventNotifier()

	// We build a supervision tree that runs the Prometheus HTTP server
	prometheusSpec, promEventNotifier := newPrometheusSpec("prometheus", metricsAddr)

	// Run the root supervisor
	app := s.New(
		"root",
		// Use the EventNotifier that reports metrics to Prometheus
		s.WithNotifier(func(ev s.Event) {
			logEventNotifier(ev)
			promEventNotifier(ev)
		}),
		// Run the prometheus supervision sub-tree
		s.WithSubtree(prometheusSpec),
		// Run the greeter goroutines
		s.WithChildren(
			newGreeter(log, "greeter1", 5*time.Second),
			newGreeter(log, "greeter2", 7*time.Second),
			newGreeter(log, "greeter3", 10*time.Second),
		),
	)

	// Start the application, this will spawn the goroutines of the following supervision tree
	//
	// root (supervisor that restarts children gorotines)
	// |
	// ` prometheus (supervisor that restarts children goroutines)
	// | |
	// | ` listen-and-serve (goroutine that runs http.ListenAndServe)
	// | |
	// | ` wait-server (goroutine that calls http.Shutdown on supervisor shutdown)
	// |
	// ` greeter1 (goroutine that greets on terminal)
	// |
	// ` greeter2 (goroutine that greets on terminal)
	// |
	// ` greeter3 (goroutine that greets on terminal)
	//
	sup, err := app.Start(context.Background())
	if err != nil {
		panic(err)
	}

	// We wait for signals from the OS to stop the supervisor
	stopOnSignal(sup)
}
