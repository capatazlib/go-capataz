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

// This is an s.EventNotifier that registers capataz' Events to prometheus
func promEventNotifier(ev s.Event) {
	gauge := eventGauge.WithLabelValues(ev.GetTag().String(), ev.GetProcessRuntimeName())
	if ev.GetTag() == s.ProcessStarted {
		gauge.Inc()
	} else {
		gauge.Dec()
	}
}

////////////////////////////////////////////////////////////////////////////////

// listenAndServeHTTPWorker blocks on this server until another goroutine cals
// the shutdown method
func listenAndServeHTTPWorker(server *http.Server) c.ChildSpec {
	return c.New("listen-and-serve", func(ctx context.Context) error {
		// NOTE: we ignore the given context because we cannot use it on go's HTTP
		// API to stop the server. When we call the server.Shutdown method (which is
		// done in waitUntilDoneHTTPWorker) the following line is going to return.
		// Just to be safe, we do a `<-ctx.Done()` check, but is not necessary.
		err := server.ListenAndServe()
		<-ctx.Done()
		return err
	})
}

// waitUntilDoneHTTPWorker waits for a supervisor tree signal to shutdown the
// given server
func waitUntilDoneHTTPWorker(server *http.Server) c.ChildSpec {
	return c.New("wait-server", func(ctx context.Context) error {
		<-ctx.Done()
		return server.Shutdown(ctx)
	})
}

////////////////////////////////////////////////////////////////////////////////

// buildPrometheusHTTPServer builds an HTTP Server that has a handler that spits
// out prometheus stats
func buildPrometheusHTTPServer(addr string) *http.Server {
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
func newPrometheusSpec(name, addr string) s.SupervisorSpec {
	return s.New(
		name,
		// this function builds an HTTP Server, this functionality requires more
		func() ([]s.Node, s.CleanupResourcesFn) {
			server := buildPrometheusHTTPServer(addr)

			// CAUTION: The order here matters, we need waitUntilDone to start last so
			// that it can terminate first, if this is not the case the
			// listenAndServeHTTPWorker child will never terminate.
			//
			// DISCLAIMER: The caution above _is not_ a capataz requirement, but a
			// requirement of net/https' API
			nodes := []s.Node{
				s.Worker(listenAndServeHTTPWorker(server)),
				s.Worker(waitUntilDoneHTTPWorker(server)),
			}

			cleanupServer := func() error {
				return server.Close()
			}

			return nodes, cleanupServer
		},
	)
}
