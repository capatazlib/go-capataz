package cap_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestHealthyHappyPath(t *testing.T) {
	ctx := context.TODO()

	healthcheckMonitor := cap.NewHealthcheckMonitor(1 * time.Millisecond)


	supSpec := cap.NewSupervisorSpec(
		"root",
		cap.WithNodes(
			WaitDoneWorker("one"),
			WaitDoneWorker("two"),
			),
		cap.WithNotifier(
			func(ev cap.Event) {
				healthcheckMonitor.HandleEvent(ev)
			},
		),
	)

	// We always want to start the supervisor for test purposes, so this is
	// embedded in the ObserveSupervisor call
	sup, startErr := supSpec.Start(ctx)
	assert.NoError(t, startErr)

	assert.True(t, healthcheckMonitor.IsHealthy())

	_ = sup.Terminate()

	assert.True(t, healthcheckMonitor.IsHealthy())
}
