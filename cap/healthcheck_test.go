package cap

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNothingToDo(t *testing.T) {

	healthcheckMonitor := NewHealthcheckMonitor(1, 1*time.Millisecond)

	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestHappyPath(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(1, 1*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerStarted("w2", time.Now())
	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestUnderMaxFailure(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(2, 1*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerStarted("w2", time.Now())
	assert.True(t, healthcheckMonitor.IsHealthy())

	notifier.workerFailed("w1", errors.New("w1 error"))
	assert.True(t, healthcheckMonitor.IsHealthy())

	// We tolerate 2 failures
	notifier.workerFailed("w2", errors.New("w2 error"))
	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestOverMaxFailure(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(1, 1*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerStarted("w2", time.Now())
	notifier.workerStarted("w3", time.Now())
	assert.True(t, healthcheckMonitor.IsHealthy())

	// One process running, two failed and only one failure is tolerated
	notifier.workerFailed("w1", errors.New("w1 error"))
	notifier.workerFailed("w2", errors.New("w2 error"))
	assert.False(t, healthcheckMonitor.IsHealthy())
}

func TestHealthyReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(0, 1*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())

	hr := healthcheckMonitor.GetHealthReport()
	assert.True(t, hr.IsHealthyReport())
}

func TestUnhealthyFailuresReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(0, 1000*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerFailed("w1", errors.New("w1 error"))

	hr := healthcheckMonitor.GetHealthReport()
	assert.False(t, hr.IsHealthyReport())

	// Failures are over tolerance
	assert.EqualValues(t, 1, len(hr.GetFailedProcesses()))
	// restart delays are under tolerance
	assert.EqualValues(t, 0, len(hr.GetDelayedRestartProcesses()))
}

func TestUnhealthyDelaysReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(0, 0*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerFailed("w1", errors.New("w1 error"))

	hr := healthcheckMonitor.GetHealthReport()
	assert.False(t, hr.IsHealthyReport())

	// Failures are over tolerance
	assert.EqualValues(t, 1, len(hr.GetFailedProcesses()))
	// restart delays are over tolerance
	assert.EqualValues(t, 1, len(hr.GetDelayedRestartProcesses()))
}

func TestHealthRestoredReport(t *testing.T) {
	healthcheckMonitor := NewHealthcheckMonitor(0, 0*time.Millisecond)

	var notifier EventNotifier = func(ev Event) {
		healthcheckMonitor.HandleEvent(ev)
	}

	notifier.workerStarted("w1", time.Now())
	notifier.workerFailed("w1", errors.New("w1 error"))

	// Failures are over tolerance
	assert.False(t, healthcheckMonitor.GetHealthReport().IsHealthyReport())

	// Failures recovered
	notifier.workerStarted("w1", time.Now())
	assert.True(t, healthcheckMonitor.GetHealthReport().IsHealthyReport())
}
