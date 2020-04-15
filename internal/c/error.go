package c

import (
	"fmt"
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
	kvs["child.name"] = err.failedChildName
	kvs["child.error"] = err.err.Error()
	kvs["child.error.count"] = err.failedChildErrCount
	kvs["child.error.duration"] = err.failedChildErrDuration
	return kvs
}

func (err *ErrorToleranceReached) Error() string {
	return fmt.Sprintf("Child failures surpassed error tolerance")
}

// Unwrap returns the last error that caused the creation of an
// ErrorToleranceReached error
func (err *ErrorToleranceReached) Unwrap() error {
	return err.err
}
