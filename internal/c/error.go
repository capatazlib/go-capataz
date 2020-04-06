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

func (err *ErrorToleranceReached) String() string {
	return fmt.Sprintf("Child failures surpassed error tolerance")
}

func (err *ErrorToleranceReached) Error() string {
	return err.String()
}

// Unwrap returns the last error that caused the creation of an
// ErrorToleranceReached error
func (err *ErrorToleranceReached) Unwrap() error {
	return err.err
}
