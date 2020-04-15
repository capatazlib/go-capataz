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
	rscCleanupErr  error
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

// KVs returns a data bag map that may be used in structured logging
func (se *SupervisorTerminationError) KVs() map[string]interface{} {
	kvs := make(map[string]interface{})
	kvs["supervisor.name"] = se.supRuntimeName
	for chKey, chErr := range se.childErrMap {
		kvs[fmt.Sprintf("supervisor.child.%v.stop.error", chKey)] = chErr.Error()
	}
	if se.childErr != nil {
		kvs["supervisor.termination.error"] = se.childErr.Error()
	}
	if se.rscCleanupErr != nil {
		kvs["supervisor.cleanup.error"] = se.rscCleanupErr.Error()
	}
	return kvs
}

// Error returns an error message
func (se *SupervisorTerminationError) Error() string {
	// NOTE: We are not reporting error details on the string given we want to
	// rely on structured logging via KVs
	if (se.childErr != nil || len(se.childErrMap) > 0) && se.rscCleanupErr != nil {
		return fmt.Sprintf(
			"supervisor children failed to terminate " +
				"(and resource cleanup failed as well)",
		)
	} else if se.childErr != nil {
		return fmt.Sprintf("supervisor child failed to terminate")
	} else if se.rscCleanupErr != nil {
		return fmt.Sprintf("supervisor failed to cleanup resources")
	}
	// NOTE: this case never happens, an invariant condition of this type has not
	// been respected. If we are here, it means we manually created a wrong
	// SupervisorTerminationError value (implementation error).
	panic(
		errors.New("invalid SupervisorTerminationError was created"),
	)
}

// SupervisorRestartError wraps an error tolerance surpassed error from a
// children, enhancing it with supervisor information and possible shutdown
// errors on other siblings
type SupervisorRestartError struct {
	supRuntimeName string
	childErr       *c.ErrorToleranceReached
	terminateErr   *SupervisorTerminationError
}

// KVs returns a data bag map that may be used in structured logging
func (se *SupervisorRestartError) KVs() map[string]interface{} {
	kvs := make(map[string]interface{})
	terminateKvs := se.terminateErr.KVs()
	childErrKvs := se.childErr.KVs()

	for k, v := range terminateKvs {
		kvs[k] = v
	}

	for k, v := range childErrKvs {
		kvs[k] = v
	}

	return kvs
}

func (err *SupervisorRestartError) Error() string {
	// NOTE: We are not reporting error details on the string given we want to
	// rely on structured logging via KVs
	if err.childErr != nil && err.terminateErr != nil {
		return fmt.Sprintf(
			"supervisor child surpassed error threshold, " +
				"(and other children failed to terminate as well)",
		)
	} else if err.childErr != nil {
		return fmt.Sprintf("supervisor child surpassed error tolerance")
	} else if err.terminateErr != nil {
		return fmt.Sprintf("supervisor children failed to terminate")
	}
	// NOTE: this case never happens, an invariant condition of this type is that
	// it only hold values with a childErr. If we are here, it means we manually
	// created a wrong SupervisorRestartError value (implementation error).
	panic(
		errors.New("invalid SupervisorRestartError was created"),
	)
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
