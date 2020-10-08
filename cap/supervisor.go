package cap

// This file contains the implementation of the public methods for the
// Supervisor API

import (
	"sync"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Supervisor represents the root of a tree of goroutines. A Supervisor may have
// leaf or sub-tree children, where each of the nodes in the tree represent a
// goroutine that gets automatic restart abilities as soon as the parent
// supervisor detects an error has occured. A Supervisor will always be
// generated from a SupervisorSpec
type Supervisor struct {
	runtimeName string
	ctrlCh      chan ctrlMsg

	mux          *sync.Mutex
	terminateCh  chan error
	terminated   *bool
	terminateErr *error

	spec     SupervisorSpec
	children map[string]c.Child
	cancel   func()
	wait     func(time.Time, startError) error
}

////////////////////////////////////////////////////////////////////////////////
// Public API

const defaultSupShutdownTimeout = 5 * time.Second

// Terminate is a synchronous procedure that halts the execution of the whole
// supervision tree.
func (sup Supervisor) Terminate() error {
	stopingTime := time.Now()
	sup.cancel()
	err := sup.wait(stopingTime, nil /* no startErr */)
	return err
}

// Wait blocks the execution of the current goroutine until the Supervisor
// finishes it execution.
func (sup Supervisor) Wait() error {
	return sup.wait(time.Time{}, nil /* no startErr */)
}

// GetName returns the name of the Spec used to start this Supervisor
func (sup Supervisor) GetName() string {
	return sup.spec.GetName()
}

// getTerminateErr is an utility function that allows us to query if the
// Supervisor was terminated in a concurrent-safe manner. If error is not nil,
// then first bool must be true
func getTerminateErr(
	mux *sync.Mutex,
	terminated *bool,
	terminateErr *error,
) (bool, error) {
	mux.Lock()
	defer mux.Unlock()
	// We already did the termination, just return the result
	if terminated != nil {
		return *terminated, *terminateErr
	}
	return false, nil
}

// storeTerminationError is responsible of registering the final state of the supervisor
func storeTerminationErr(
	eventNotifier EventNotifier,
	supRuntimeName string,
	mux *sync.Mutex,
	terminated *bool,
	terminateErr *error,
	err error,
	stopingTime time.Time,
) {
	mux.Lock()
	defer mux.Unlock()
	*terminated = true
	*terminateErr = err
	if err != nil {
		eventNotifier.supervisorFailed(supRuntimeName, err)
		return
	}

	// stopingTime is only relevant when we call the internal wait function
	// from the Terminate() public API; if we just called from Wait(), we don't
	// need to keep track of the stop duration
	if stopingTime == (time.Time{}) {
		stopingTime = time.Now()
	}
	eventNotifier.supervisorTerminated(supRuntimeName, stopingTime)
}

// GetCrashError will return an error if the supervisor crashed, otherwise
// returns nil.
func getCrashError(
	eventNotifier EventNotifier,
	supRuntimeName string,
	mux *sync.Mutex,
	terminateCh chan error,
	terminated *bool,
	terminateErr *error,
	stopingTime time.Time,
	block bool,
) (bool, error) {

	if terminatedVal, terminateErrVal := getTerminateErr(mux, terminated, terminateErr); terminatedVal {
		return terminatedVal, terminateErrVal
	}

	if block {
		terminateErrVal := <-terminateCh
		storeTerminationErr(
			eventNotifier,
			supRuntimeName,
			mux,
			terminated,
			terminateErr,
			terminateErrVal,
			stopingTime,
		)
		return true, terminateErrVal
	}

	select {
	case terminateErrVal := <-terminateCh:
		storeTerminationErr(
			eventNotifier,
			supRuntimeName,
			mux,
			terminated,
			terminateErr,
			terminateErrVal,
			time.Time{},
		)
		return true, terminateErrVal

	default:
		return false, nil
	}
}

// GetCrashError returns a crash error if there is one, the second parameter
// indicates if the supervisor is running or not. If the returned error is not
// nil, the second result will always be true.
func (sup Supervisor) GetCrashError(block bool) (bool, error) {
	return getCrashError(
		sup.spec.eventNotifier,
		sup.runtimeName,
		sup.mux,
		sup.terminateCh,
		sup.terminated,
		sup.terminateErr,
		time.Time{},
		false, /* block */
	)
}
