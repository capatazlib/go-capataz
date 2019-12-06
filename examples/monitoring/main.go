package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/capatazlib/go-capataz/s"
)

// default port for prometheus metrics server
const metricsHTTPAddr = "0.0.0.0:8080"

// stopOnSignal will wait for the program to receive a SIGTERM and stop the
// given supervisor
func stopOnSignal(sup s.Supervisor) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM)
	<-done
	err := sup.Stop()
	fmt.Println(err)
}

// This program runs a prometheus metrics server and a few goroutines that print
// greetings in the standard output. It showcases how to build an app by
// composing all the components in a tree shape. All the wiring happens before
// we run any business logic.
//
// This API gives restart logic for any failures that happen in the goroutines
// and also forces us to build our application as a group of sub-systems and
// goroutine workers.
//
func main() {
	// Setup the logging mechanisms
	log, logEventNotifier := newLogEventNotifier()

	// Build a supervision tree that runs the Prometheus HTTP server
	prometheusSpec, promEventNotifier := newPrometheusSpec("prometheus", metricsHTTPAddr)

	// Build a supervision tree that runs 3 greeters
	greetersSpec := newGreeterTree(
		log,
		"greeters",
		greeterSpec{name: "greeter1", delay: 5 * time.Second},
		greeterSpec{name: "greeter2", delay: 7 * time.Second},
		greeterSpec{name: "greeter3", delay: 10 * time.Second},
	)

	// We _build_ the application structure, composing all the sub-components
	// together
	app := s.New(
		// The name of the topmost supervisor in our application
		"root",
		// Setup the supervision system EventNotifier
		//
		// The EventNotifier will emit an Event everytime the supervisor:
		//
		// * starts a worker
		// * stops a worker
		// * detectes a worker failure
		//
		// This callback is the ideal place to add traceability to our supervision
		// system, in this particular `s.EventNotifer`, we log each event and send
		// metrics to prometheus to check our application health
		//
		s.WithNotifier(func(ev s.Event) {
			logEventNotifier(ev)
			promEventNotifier(ev)
		}),
		// When the "root" supervisor starts, it's going to start these two
		// supervisors (sub-branches). Our root supervisor is not concerned
		// about what these sub-systems do.
		//
		// In this particular example we have two sub-systems:
		//
		// * prometheusSpec: runs the prometheus infrastructure for our application
		//
		// * greetersSpec: runs a bunch of worker goroutines that greet over the
		// terminal
		//
		// Given each of them is a sub-tree supervisor, if any of the leaf workers
		// fail, they are going to get restarted by their direct supervisor. If the
		// number of failures passes a treshold, then its going to report the error
		// to the root supervisor, which can decide to restart the whole system, or
		// just that particular sub-tree.
		//
		// In case the root supervisor error treshold is reached, then the
		// application will fail hard.
		s.WithSubtree(prometheusSpec),
		s.WithSubtree(greetersSpec),
	)

	// Start the application, this will spawn the goroutines of the following supervision tree
	//
	// root (supervisor that restarts children gorotines)
	// |
	// ` prometheus (supervisor that restarts http goroutines)
	// | |
	// | ` listen-and-serve (goroutine that runs http.ListenAndServe)
	// | |
	// | ` wait-server (goroutine that calls http.Shutdown on supervisor shutdown)
	// |
	// ` greeters (supervisor that restarts greeters goroutines)
	//   |
	//   ` greeter1 (goroutine that greets on terminal)
	//   |
	//   ` greeter2 (goroutine that greets on terminal)
	//   |
	//   ` greeter3 (goroutine that greets on terminal)
	//
	//
	// The Start method will start each sub-component in the order above. For
	// example, the greeters sub-tree will not start until the prometheus tree has
	// reported it started. When using `c.New`, the start notification happens as
	// soon as the child goroutine executes, if you use `c.NewWithStart` you can
	// signal to the supervisor when the supervisor has started, this is useful
	// when you need to wait for some setup before your system is "running".
	sup, err := app.Start(context.Background())
	if err != nil {
		panic(err)
	}

	// We block, waiting for signals from the OS to stop the supervisors
	// gracefully.
	//
	// When we terminate the supervisor, it will terminate each sub-tree and leaf
	// child in the reverse order, meaning, if the child "greeter3" was the last
	// to start, then it will be the first child that is going to be terminated.
	//
	// Note, goroutine termination relies on the `context.Context` given in the
	// `c.New` call.
	stopOnSignal(sup)
}
