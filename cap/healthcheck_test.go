package cap

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNothingToDo(t *testing.T) {

	healthcheckMonitor := NewHealthcheckMonitor(1*time.Millisecond, 1)

	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestHappyPath(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(1*time.Millisecond, 1)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerStarted("w2", time.Now())
	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestMaxFailure(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(1*time.Millisecond, 0)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerStarted("w2", time.Now())
	assert.True(t, healthcheckMonitor.IsHealthy())

	notifier.workerFailed("w1", errors.New("w1 error"))
	assert.False(t, healthcheckMonitor.IsHealthy())
}

func TestHealthyReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(1*time.Millisecond, 0)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())

	hr := healthcheckMonitor.GetHealthReport()
	assert.True(t, hr.IsHealthyReport())
}

func TestUnhealthyFailuresReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(1000*time.Millisecond, 0)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerFailed("w1", errors.New("w1 error"))

	hr := healthcheckMonitor.GetHealthReport()
	assert.False(t, hr.IsHealthyReport())

	assert.EqualValues(t, 1, len(hr.GetFailedProcesses()))
	assert.EqualValues(t, 0, len(hr.GetDelayedRestartProcesses()))
}

func TestUnhealthyDelaysReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(0*time.Millisecond, 0)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerFailed("w1", errors.New("w1 error"))

	hr := healthcheckMonitor.GetHealthReport()
	assert.False(t, hr.IsHealthyReport())

	assert.EqualValues(t, 1, len(hr.GetFailedProcesses()))
	assert.EqualValues(t, 1, len(hr.GetDelayedRestartProcesses()))
}
