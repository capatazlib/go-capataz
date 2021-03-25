package c_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/internal/c"
	"github.com/capatazlib/go-capataz/internal/s"

	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestWorkerDoubleTermination(t *testing.T) {
	t.Run("on healthy worker", func(t *testing.T) {
		// internal way to transform a Node to a WorkerSpec
		wspec := WaitDoneWorker("worker")(s.SupervisorSpec{})

		supNotifyChan := make(chan c.ChildNotification)
		c, err := wspec.DoStart(context.Background(), "test", supNotifyChan)
		assert.NoError(t, err)
		// first time, is going to call the termination logic, and it should not retrun an error
		isFirstTime, err := c.Terminate()
		assert.NoError(t, err)
		assert.True(t, isFirstTime)

		isFirstTime, err = c.Terminate()
		assert.NoError(t, err)
		assert.Nil(t, err)
		assert.False(t, isFirstTime)
	})
	t.Run("on termination failing worker", func(t *testing.T) {
		// internal way to transform a Node to a WorkerSpec
		wspec := FailTerminationWorker(
			"worker", fmt.Errorf("failing worker"),
		)(s.SupervisorSpec{})

		supNotifyChan := make(chan c.ChildNotification)
		c, err := wspec.DoStart(context.Background(), "test", supNotifyChan)
		assert.NoError(t, err)
		// first time, is going to call the termination logic, and it should not
		// return an error
		isFirstTime, err := c.Terminate()
		assert.Error(t, err)
		assert.True(t, isFirstTime)
		// at this point, the internal termination channel is closed, and this
		// returns an error only the first time it is called.
		isFirstTime, err = c.Terminate()
		assert.NoError(t, err)
		assert.Nil(t, err)
		assert.False(t, isFirstTime)

	})
}
