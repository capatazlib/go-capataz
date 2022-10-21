package cap

import "github.com/capatazlib/go-capataz/internal/s"

// HealthReport contains a report for the HealthMonitor
//
// Since: 0.0.0
type HealthReport = s.HealthReport

// HealthyReport represents a healthy report
//
// Since: 0.0.0
var HealthyReport = s.HealthyReport

// HealthcheckMonitor listens to the events of a supervision tree events, and
// assess if the supervisor is healthy or not
//
// Since: 0.0.0
type HealthcheckMonitor = s.HealthcheckMonitor

// NewHealthcheckMonitor offers a way to monitor a supervision tree health from
// events emitted by it.
//
// maxAllowedFailures: the threshold beyond which the environment is considered
//
//	unhealthy.
//
// maxAllowedRestartDuration: the restart threshold, which if exceeded, indicates
//
//	an unhealthy environment. Any process that fails
//	to restart under the threshold results in an
//	unhealthy report
//
// Since: 0.0.0
var NewHealthcheckMonitor = s.NewHealthcheckMonitor
