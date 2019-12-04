package main

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

var (
	eventGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "supervisor_event_gauge",
		},
		[]string{"type", "process_name"},
	)
)

////////////////////////////////////////////////////////////////////////////////

func registerEvent(ev s.Event) {
	if ev.Tag() == s.ProcessStarted {
		eventGauge.WithLabelValues(ev.Tag().String(), ev.ProcessRuntimeName()).Inc()
	} else {
		eventGauge.WithLabelValues(ev.Tag().String(), ev.ProcessRuntimeName()).Dec()
	}
}

////////////////////////////////////////////////////////////////////////////////

// listenAndServeHttpWorker blocks on this server until another goroutine cals
// the shutdown method
func listenAndServeHttpWorker(server *http.Server) c.ChildSpec {
	return c.New("listen-and-serve", func(ctx context.Context) error {
		err := server.ListenAndServe()
		<-ctx.Done()
		return err
	})
}

// waitUntilDoneHttpWorker waits for a supervisor tree signal to shutdown the
// given server
func waitUntilDoneHttpWorker(server *http.Server) c.ChildSpec {
	return c.New("wait-server", func(ctx context.Context) error {
		<-ctx.Done()
		return server.Shutdown(ctx)
	})
}

// httpServerTree returns a SupervisorSpec that runs an HTTP Server, this
// functionality requires more than a goroutine given the only way to stop a
// http server is to call the http.Shutdown function on a seperate goroutine
func httpServerTree(name string, server *http.Server) s.SupervisorSpec {
	return s.New(
		name,
		s.WithChildren(
			listenAndServeHttpWorker(server),
			waitUntilDoneHttpWorker(server),
		),
	)
}

////////////////////////////////////////////////////////////////////////////////

// buildPrometheusHttpServer builds an HTTP Server that has a handler that spits
// out prometheus stats
func buildPrometheusHttpServer(addr string) *http.Server {
	handle := http.NewServeMux()
	handle.Handle("/metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: handle}
}

// newPrometheusSpec returns a SupervisorSpec that when started
//
// * Runs an HTTP Server that handles requests from the Prometheus server.
//
// The returned SupervisorSpec executes a tree of the following shape:
//
// + <given-name>
// |
// ` listen-and-serve
// |
// ` wait-server
//
// The function receives:
//
// * name: The sub-tree supervisor name
//
// * addr: The http address
//
// The function also returns a `s.EventNotifier` that may be used on the root
// supervisor.
//
func newPrometheusSpec(name, addr string) (s.SupervisorSpec, s.EventNotifier) {
	server := buildPrometheusHttpServer(addr)
	return httpServerTree(name, server), registerEvent
}
