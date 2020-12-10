package c

import (
	"time"
)

// ErrorToleranceReached is an error that gets reported when a supervisor has
// restarted a child so many times over a period of time that it does not make
// sense to keep restarting.
type ErrorToleranceReached struct {
	failedChildName        string
	failedChildErrCount    uint32
	failedChildErrDuration time.Duration
	err                    error
}

// KVs returns a data bag map that may be used in structured logging
func (err *ErrorToleranceReached) KVs() map[string]interface{} {
	kvs := make(map[string]interface{})
	kvs["node.name"] = err.failedChildName
	if err.err != nil {
		kvs["node.error.msg"] = err.err.Error()
		kvs["node.error.count"] = err.failedChildErrCount
		kvs["node.error.duration"] = err.failedChildErrDuration
	}
	return kvs
}

func (err *ErrorToleranceReached) Error() string {
	return "node failures surpassed error tolerance"
}

// Unwrap returns the last error that caused the creation of an
// ErrorToleranceReached error
func (err *ErrorToleranceReached) Unwrap() error {
	return err.err
}
