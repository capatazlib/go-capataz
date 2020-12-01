package cap

import (
	"time"
)

// HealthReport contains a report for the HealthMonitor
type HealthReport struct {
	failedProcesses         []string
	delayedRestartProcesses []string
}

// HealthyReport represents a healthy report
var HealthyReport = HealthReport{}

// HealthcheckMonitor listens to the events of a supervision tree events, and
// assess if the supervisor is healthy or not
type HealthcheckMonitor struct {
	maxAllowedRestartDuration time.Duration
	maxAllowedFailures        uint32
	failedEvs                 map[string]Event
}

// GetFailedProcesses returns a list of the failed processes
func (hr HealthReport) GetFailedProcesses() []string {
	return hr.failedProcesses
}

// GetDelayedRestartProcesses returns a list of the failed processes
func (hr HealthReport) GetDelayedRestartProcesses() []string {
	return hr.delayedRestartProcesses
}

// IsHealthyReport returns a list of the failed processes
func (hr HealthReport) IsHealthyReport() bool {
	return len(hr.failedProcesses) == 0 && len(hr.delayedRestartProcesses) == 0
}

// NewHealthcheckMonitor offers a way to monitor a supervision tree health from
// events emitted by it. The given duration is the amount of time that indicates
// a goroutine is taking too much time to restart.
func NewHealthcheckMonitor(
	maxAllowedFailures uint32,
	maxAllowedRestartDuration time.Duration,
) HealthcheckMonitor {
	return HealthcheckMonitor{
		maxAllowedRestartDuration: maxAllowedRestartDuration,
		maxAllowedFailures:        maxAllowedFailures,
		failedEvs:                 make(map[string]Event),
	}
}

// HandleEvent is a function that receives supervision events and assess if the
// supervisor sending these events is healthy or not
func (h HealthcheckMonitor) HandleEvent(ev Event) {
	switch ev.GetTag() {
	case ProcessFailed:
		h.failedEvs[ev.GetProcessRuntimeName()] = ev
	case ProcessStarted:
		delete(h.failedEvs, ev.GetProcessRuntimeName())
	}
}

// GetHealthReport returns a string that indicates why a the system
// is unhealthy. Returns empty if everything is ok.
func (h HealthcheckMonitor) GetHealthReport() HealthReport {
	// if there is an acceptable number of failures, things are healthy
	if uint32(len(h.failedEvs)) <= h.maxAllowedFailures {
		return HealthyReport
	}

	hr := HealthReport{
		failedProcesses:         make([]string, 0, len(h.failedEvs)),
		delayedRestartProcesses: make([]string, 0, len(h.failedEvs)),
	}

	currentTime := time.Now()
	for processName, ev := range h.failedEvs {
		// Capture all failures
		hr.failedProcesses = append(hr.failedProcesses, processName)

		dur := currentTime.Sub(ev.GetCreated())

		if dur <= h.maxAllowedRestartDuration {
			continue
		}

		// Capture all failures that are taking too long to recover
		hr.delayedRestartProcesses = append(hr.delayedRestartProcesses, processName)
	}
	return hr
}

// IsHealthy return true when the system is in a healthy state, meaning, no
// processes restarting at the moment
func (h HealthcheckMonitor) IsHealthy() bool {
	return h.GetHealthReport().IsHealthyReport()
}
