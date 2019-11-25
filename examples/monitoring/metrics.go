package main

import (
	"context"
	"fmt"
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

func metricEventHandler(ev s.Event) {
	fmt.Printf("Event received: %s\n", ev.String())
	if ev.Tag() == s.ProcessStarted {
		eventGauge.WithLabelValues(ev.Tag().String(), ev.ProcessRuntimeName()).Inc()
	} else {
		eventGauge.WithLabelValues(ev.Tag().String(), ev.ProcessRuntimeName()).Dec()
	}
}

func newMetricsSpec(addr string) (s.SupervisorSpec, s.EventNotifier) {
	handle := http.NewServeMux()
	handle.Handle("/metrics", promhttp.Handler())

	// NOTE: adding a buffer to this chan to account initial events of the
	// supervisor system
	evCh := make(chan s.Event, 10)
	server := &http.Server{Addr: addr, Handler: handle}

	evHandler := func(ev s.Event) {
		evCh <- ev
	}

	// This sub-system will contain all things related to HTTP metrics
	return s.New(
		"http-metrics",
		s.WithChildren(
			// this is the goroutine that will run the HTTP server
			c.New("listen-and-serve", func(_ context.Context) error {
				fmt.Printf("http-metrics starting\n")
				err := server.ListenAndServe()
				fmt.Printf("http-metrics failed:\n%+v\n", err)
				return err
			}),
			// stop-handler notifies the http server to stop, we need an extra thread
			// for this
			c.New("stop-handler", func(ctx context.Context) error {
				<-ctx.Done()
				fmt.Println("Shutting down metrics server")
				err := server.Shutdown(ctx)
				if err != nil {
					fmt.Printf("Shutting down failed: %+v\n", err)
				}
				return err
			}),
			c.New("event-registerer", func(ctx context.Context) error {
				defer close(evCh)
			loop:
				for {
					select {
					case <-ctx.Done():
						break loop
					case ev := <-evCh:
						metricEventHandler(ev)
					}
				}
				return nil
			}),
		),
	), evHandler
}
