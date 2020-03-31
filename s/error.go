package s

import (
	"errors"
	"fmt"

	"github.com/capatazlib/go-capataz/internal/c"
)

// startError is the error reported back to a Supervisor when
// the start of a Child fails
type startError = error

// terminateError is the error reported back to a Supervisor when
// the termination of a Child fails
type terminateError = error

// SupervisorTerminationError wraps a termination error from a supervised
// worker, enhancing it with supervisor information and possible shutdown errors
// on other siblings
type SupervisorTerminationError struct {
	supRuntimeName string
	childErr       error
	childErrMap    map[string]error
}

// Unwrap returns a error from a supervised goroutine (if any)
func (se *SupervisorTerminationError) Unwrap() error {
	return se.childErr
}

// Cause returns a error from a supervised goroutine (if any)
func (se *SupervisorTerminationError) Cause() error {
	return se.childErr
}

// GetRuntimeName returns the name of the supervisor that failed
func (se *SupervisorTerminationError) GetRuntimeName() string {
	return se.supRuntimeName
}

// ChildFailCount returns the number of children that failed to terminate
// correctly. Note if a goroutine fails to terminate because of a shutdown
// timeout, the failed goroutines may leak. This happens because go doesn't
// offer any true way to _kill_ a goroutine.
func (se *SupervisorTerminationError) ChildFailCount() int {
	return len(se.childErrMap)
}

// Error returns an error message
func (se *SupervisorTerminationError) Error() string {
	return "Supervisor termination failure"
}

// SupervisorRestartError wraps an error tolerance surpassed error from a
// children, enhancing it with supervisor information and possible shutdown
// errors on other siblings
type SupervisorRestartError struct {
	supRuntimeName string
	childErr       *c.ErrorToleranceReached
	terminateErr   *SupervisorTerminationError
}

func (err *SupervisorRestartError) String() string {
	if err.childErr != nil && err.terminateErr != nil {
		return fmt.Sprintf(
			"Supervisor child surpassed error threshold, " +
				"(and other children failed to terminate as well)",
		)
	} else if err.childErr != nil {
		return fmt.Sprintf("Supervisor child surpassed error tolerance")
	} else if err.terminateErr != nil {
		return fmt.Sprintf("Supervisor children failed to terminate")
	}
	// NOTE: this case never happens, an invariant condition of this type is that
	// it only hold values with a childErr. If we are here, it means we manually
	// created a wrong SupervisorRestartError value (implementation error).
	panic(
		errors.New("invalid SupervisorRestartError was created"),
	)
}

func (err *SupervisorRestartError) Error() string {
	return err.String()
}

// Unwrap returns a child error or a termination error
func (err *SupervisorRestartError) Unwrap() error {
	// it should never be nil
	if err.childErr != nil {
		return err.childErr.Unwrap()
	}
	if err.terminateErr != nil {
		return err.terminateErr
	}
	return nil
}

// Cause returns a child error or a termination error
func (err *SupervisorRestartError) Cause() error {
	// it should never be nil
	if err.childErr != nil {
		return err.childErr.Unwrap()
	}
	if err.terminateErr != nil {
		return err.terminateErr
	}
	return nil
}
