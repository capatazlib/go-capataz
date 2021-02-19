package cap

import "github.com/capatazlib/go-capataz/internal/s"

// ErrKVs is an utility interface used to get key-values out of Capataz errors
//
// Since: 0.0.0
type ErrKVs = s.ErrKVs

// SupervisorTerminationError wraps errors returned by a child node that failed
// to terminate (io errors, timeouts, etc.), enhancing it with supervisor
// information. Note, the only way to have a valid SupervisorTerminationError is
// for one of the child nodes to fail or the supervisor cleanup operation fails.
//
// Since: 0.0.0
type SupervisorTerminationError = s.SupervisorTerminationError

// SupervisorBuildError wraps errors returned from a client provided function
// that builds the supervisor nodes, enhancing it with supervisor information
//
// Since: 0.0.0
type SupervisorBuildError = s.SupervisorBuildError

// SupervisorStartError wraps an error reported on the initialization of a child
// node, enhancing it with supervisor information and possible termination errors
// on other siblings
//
// Since: 0.0.0
type SupervisorStartError = s.SupervisorStartError

// SupervisorRestartError wraps an error tolerance surpassed error from a child
// node, enhancing it with supervisor information and possible termination errors
// on other siblings
//
// Since: 0.0.0
type SupervisorRestartError = s.SupervisorRestartError

// RestartToleranceReached is an error that gets reported when a supervisor has
// restarted a child so many times over a period of time that it does not make
// sense to keep restarting.
//
// Since: 0.0.0
type RestartToleranceReached = s.RestartToleranceReached

// ExplainError is a utility function that explains capataz errors in a human-friendly
// way. Defaults to a call to error.Error() if the underlying error does not come from
// the capataz library.
//
// Since: 0.1.0
var ExplainError = s.ExplainError
