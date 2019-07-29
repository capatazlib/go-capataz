package capataz

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChildNotifications(t *testing.T) {
	tests := map[string]struct {
		expected childEventType
		fn       ChildFunc
	}{
		"completed": {
			expected: childCompleted,
			fn: func(_ context.Context) error {
				return nil
			},
		},
		"shutdown": {
			expected: childShutdown,
			fn: func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		},
		"failed": {
			expected: childFailed,
			fn: func(_ context.Context) error {
				return errors.New("failed")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			notifier := make(chan childEvent, 1)
			spec := MakeFuncWorkerChildSpec("test", tc.fn)
			spawnChild(spec, notifier)

			startEvent := <-notifier
			assert.Equal(t, childStarted.String(), startEvent.eventType.String())

			startEvent.child.stop()

			stopEvent := <-notifier
			assert.Equal(t, tc.expected.String(), stopEvent.eventType.String())
		})
	}
}
