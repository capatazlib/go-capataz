package cap

import (
	"time"
)

// HealthcheckMonitor listens to the events of a supervision tree events, and
// assess if the supervisor is healthy or not
type HealthcheckMonitor struct {
	maxAllowedRestartDuration time.Duration
	failedEvs                 map[string]Event
}

// NewHealthcheckMonitor offers a way to monitor a supervision tree health from
// events emitted by it. The given duration is the amount of time that indicates
// a goroutine is taking too much time to restart.
func NewHealthcheckMonitor(maxAllowedRestartDuration time.Duration) HealthcheckMonitor {
	return HealthcheckMonitor{
		maxAllowedRestartDuration: maxAllowedRestartDuration,
		failedEvs:                 make(map[string]Event),
	}
}

// HandleEvent is a function that receives supervision events and assess if the
// supervisor sending these events is healthy or not
func (h *HealthcheckMonitor) HandleEvent(ev Event) {
	switch ev.GetTag() {
	case ProcessFailed:
		h.failedEvs[ev.GetProcessRuntimeName()] = ev
	case ProcessStarted:
		delete(h.failedEvs, ev.GetProcessRuntimeName())
	}
}

// GetUnhealthyReason returns a string that indicates why a the system
// is unhealthy. Returns empty if everything is ok.
func (h HealthcheckMonitor) GetUnhealthyReason() string {
	// if there are no failures, things are healthy
	if len(h.failedEvs) == 0 {
		return ""
	}

	// if you have more than one process failing, then you are not healthy
	if len(h.failedEvs) > 1 {
		return "multiple process restarting"
	}

	// if we are getting a process taking more time to restart than expected
	// notify as unhealthy
	currentTime := time.Now()
	for _, ev := range h.failedEvs {
		dur := currentTime.Sub(ev.GetCreated())

		if dur < h.maxAllowedRestartDuration {
			return ""
		}

		return "process is pending to restart"
	}
	return ""
}

// IsHealthy retursn true when the system is in a healthy state, meaning, no
// processes restarting at the moment
func (h HealthcheckMonitor) IsHealthy() bool {
	return h.GetUnhealthyReason() == ""
}
