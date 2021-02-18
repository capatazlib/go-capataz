package n

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/s"
)

// rootName is the name of the ReliableNotifier supervisor root tree node
var rootName = "reliable-notifier"

// notifierSettings contains settings and callbacks for a ReliableNotifier
// instance
type notifierSettings struct {
	entrypointBufferSize    uint
	notifierTimeoutDuration time.Duration

	onReliableNotifierFailure func(error)
	onNotifierTimeout         func(string)
}

// ReliableNotifierOpt allows clients to tweak the behavior of a
// ReliableNotifier instance
type ReliableNotifierOpt func(*notifierSettings)

// newNotifierWorker runs a worker that listens to a channel dedicated to the
// given event notifier. In the situation the notifierFn panics, the worker gets
// restarted.
func newNotifierWorker(
	_ notifierSettings,
	name string,
	notifierFn s.EventNotifier,
) (chan s.Event, s.Node) {
	// ch := make(chan s.Event, settings.notifierBufferSize)
	ch := make(chan s.Event)
	return ch, s.NewWorker(
		name,
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case ev := <-ch:
					notifierFn(ev)
				}
			}
		},
	)
}

// newNotifierSubTree runs a subtree of notifier workers. It restarts all
// notifiers if one of them fails above restart threshold.
func newNotifierSubTree(
	settings notifierSettings,
	notifierFns map[string]s.EventNotifier,
) (s.Node, map[string](chan s.Event)) {

	workers := make([]s.Node, 0, len(notifierFns))
	notifierChans := make(map[string](chan s.Event))

	for name, notifierFn := range notifierFns {
		ch, worker := newNotifierWorker(settings, name, notifierFn)
		notifierChans[name] = ch
		workers = append(workers, worker)
	}
	notifierTree := s.Subtree(s.NewSupervisorSpec("notifiers", s.WithNodes(workers...)))

	return notifierTree, notifierChans
}

// runEntrypointListener listens to a channel for events, and it then broadcast
// in a non-blockin fashion the same event across multiple workers.
func runEntrypointListener(
	ctx context.Context,
	settings notifierSettings,
	entrypointCh chan s.Event,
	notifierChans map[string](chan s.Event),
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case ev := <-entrypointCh:
			for name, ch := range notifierChans {
				notifyCtx, stopTimer := context.WithTimeout(
					context.Background(),
					settings.notifierTimeoutDuration,
				)
				select {
				case <-notifyCtx.Done():
					settings.onNotifierTimeout(name)

				case ch <- ev:
				}
				stopTimer()
			}
		}
	}
}

// WithOnNotifierTimeout sets callback that gets executed when a given notifier
// is so slow to get an event that it gets skipped.
func WithOnNotifierTimeout(cb func(string)) ReliableNotifierOpt {
	return func(settings *notifierSettings) {
		settings.onNotifierTimeout = cb
	}
}

// WithOnReliableNotifierFailure sets a callback that gets executed when a failure
// occurs on the event broadcasting logic
func WithOnReliableNotifierFailure(cb func(error)) ReliableNotifierOpt {
	return func(settings *notifierSettings) {
		settings.onReliableNotifierFailure = cb
	}
}

// WithNotifierTimeout sets the maximum allowed time the reliable notifier is going to
// wait for a notifier function to be ready to receive an event (defaults to 10 millis).
func WithNotifierTimeout(ts time.Duration) ReliableNotifierOpt {
	return func(settings *notifierSettings) {
		settings.notifierTimeoutDuration = ts
	}
}

// notifyRootFailure builds an EventNotifier that executes the
// onReliableNotifierFailure callback from the given notifierSettings
func notifyRootFailure(settings notifierSettings) s.EventNotifier {
	failingNode := strings.Join([]string{rootName, "notifiers"}, s.NodeSepToken)
	return func(ev s.Event) {
		if ev.GetTag() == s.ProcessFailed && ev.GetProcessRuntimeName() == failingNode {
			settings.onReliableNotifierFailure(ev.Err())
		}
	}
}

// NewReliableNotifier is an EventNotifier that guarantees it will never panic
// the execution of its caller, and that it will continue sending events to
// notifiers despite previous panics
func NewReliableNotifier(
	notifierFns map[string]s.EventNotifier,
	opts ...ReliableNotifierOpt,
) (s.EventNotifier, context.CancelFunc, error) {

	// default notifier settings
	settings := notifierSettings{
		entrypointBufferSize:      0,
		notifierTimeoutDuration:   10 * time.Millisecond,
		onReliableNotifierFailure: func(error) {},
		onNotifierTimeout:         func(string) {},
	}

	for _, optFn := range opts {
		optFn(&settings)
	}

	// the entrypoint channel is built outside the "notifier supervision tree"
	// given the chan is referenced from the outside.
	// entrypointCh := make(chan s.Event, settings.entrypointBufferSize)
	entrypointCh := make(chan s.Event)

	//
	reliableNotifierSpec := s.NewSupervisorSpec(
		rootName,
		func() ([]s.Node, s.CleanupResourcesFn, error) {

			notifierSubtree, notifierChans := newNotifierSubTree(settings, notifierFns)
			entrypointWorker := s.NewWorker(
				"entrypoint",
				func(ctx context.Context) error {
					return runEntrypointListener(ctx, settings, entrypointCh, notifierChans)
				},
			)

			emptyCleanup := func() error { return nil }

			return []s.Node{entrypointWorker, notifierSubtree}, emptyCleanup, nil
		},
		// we set an impossible restart tolerance, as we always want to keep
		// this logic running (and failing in the background)
		s.WithRestartTolerance(100, 1*time.Second),
		s.WithNotifier(notifyRootFailure(settings)),
		// TODO: Add OneForAll strategy here
	)

	reliableNotifier, startErr := reliableNotifierSpec.Start(context.Background())
	if startErr != nil {
		return nil, nil, fmt.Errorf("could not start reliable notifier: %w", startErr)
	}

	// this is the eventNotifier that the main supervision tree is going to use to
	// send notifications.
	eventNotifier := func(ev s.Event) {
		entrypointCh <- ev
	}

	cancelFn := func() {
		_ = reliableNotifier.Terminate()
	}

	return eventNotifier, cancelFn, nil
}
