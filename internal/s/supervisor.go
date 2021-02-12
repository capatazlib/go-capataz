package s

// This file contains the implementation of the public methods for the
// Supervisor API

import (
	"sync"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// terminationManager offers an API to do thread-safe tracking of the
// termination state of a Supervisor. This record is designed to hide the
// required internal mutability on the Supervisor record.
//
// This is a necessary type; when we run a DynSupervisor, we need to make sure
// that we *do not* spawn workers on a terminated supervisor, otherwise we run
// the risk of getting a panic error.
type terminationManager struct {
	mux          *sync.Mutex
	terminated   bool
	terminateErr error
}

// newTerminationManager creates a new terminationManager
func newTerminationManager() *terminationManager {
	var mux sync.Mutex

	return &terminationManager{
		mux:          &mux,
		terminated:   false,
		terminateErr: nil,
	}
}

// getTerminateErr is a (concurrent-safe) function that allows us to query if a
// Supervisor was terminated.
func (tm *terminationManager) getTerminateErr() (bool, error) {
	tm.mux.Lock()
	defer tm.mux.Unlock()
	if tm.terminated {
		// we already did the termination, just return the result
		return tm.terminated, tm.terminateErr
	}
	// has not terminated, there cannot be error in this scenario
	return false, nil
}

// setTerminationErr is a concurrent-safe function that registers the final
// state of a Supervisor.
func (tm *terminationManager) setTerminationErr(err error) {
	tm.mux.Lock()
	defer tm.mux.Unlock()
	tm.terminated = true
	tm.terminateErr = err
}

// restartToleranceManager contains the information required to lear if a surpervisor
// surpassed error tolerance
type restartToleranceManager struct {
	restartTolerance restartTolerance
	restartCount     uint32
	restartBeginTime time.Time
}

// checkTolerance adds a new failure on the error tolerance calculation, if the
// number of errors is enough to surpass tolerance, it will return false,
// otherwise it will modify it's restart count and return true.
func (mgr *restartToleranceManager) checkTolerance() bool {
	if mgr.restartBeginTime == (time.Time{}) {
		mgr.restartBeginTime = time.Now()
	}

	restartTolerance := mgr.restartTolerance
	check := restartTolerance.check(mgr.restartCount, mgr.restartBeginTime)

	switch check {
	case restartToleranceSurpassed:
		return false
	case incRestartCount:
		mgr.restartCount++
		return true
	case resetRestartCount:
		// not zero given we need to account for the error that just happened
		mgr.restartCount = 1
		mgr.restartBeginTime = time.Now()
		return true
	default:
		panic("Invalid implementation of restartTolerance values")
	}
}

// Supervisor represents the root of a tree of goroutines. A Supervisor may have
// leaf or sub-tree children, where each of the nodes in the tree represent a
// goroutine that gets automatic restart abilities as soon as the parent
// supervisor detects an error has occured. A Supervisor will always be
// generated from a SupervisorSpec
type Supervisor struct {
	runtimeName string

	ctrlCh      chan ctrlMsg
	terminateCh chan error

	terminateManager        *terminationManager
	restartToleranceManager *restartToleranceManager

	spec     SupervisorSpec
	children map[string]c.Child
	cancel   func()
	wait     func(time.Time, startNodeError) error
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

// storeTerminationError is responsible of registering the final state of the
// supervisor and to signal the event notifications system
func storeTerminationErr(
	eventNotifiers EventNotifiers,
	supRuntimeName string,
	tm *terminationManager,
	err error,
	stopingTime time.Time,
) {
	tm.setTerminationErr(err)
	if err != nil {
		eventNotifiers.supervisorFailed(supRuntimeName, err)
		return
	}

	// stopingTime is only relevant when we call the internal wait function
	// from the Terminate() public API; if we just called from Wait(), we don't
	// need to keep track of the stop duration
	if stopingTime == (time.Time{}) {
		stopingTime = time.Now()
	}
	eventNotifiers.supervisorTerminated(supRuntimeName, stopingTime)
}

// getCrashError will return an error if the supervisor crashed, otherwise
// returns nil.
func getCrashError(
	block bool,
	eventNotifiers EventNotifiers,
	supRuntimeName string,
	terminateCh <-chan error,
	tm *terminationManager,
	stopingTime time.Time,
) (bool, error) {

	if terminatedVal, terminateErrVal := tm.getTerminateErr(); terminatedVal {
		return terminatedVal, terminateErrVal
	}

	if block {
		terminateErr := <-terminateCh
		storeTerminationErr(
			eventNotifiers,
			supRuntimeName,
			tm,
			terminateErr,
			stopingTime,
		)
		return true, terminateErr
	}

	select {
	case terminateErr := <-terminateCh:
		storeTerminationErr(
			eventNotifiers,
			supRuntimeName,
			tm,
			terminateErr,
			time.Time{},
		)
		return true, terminateErr

	default:
		return false, nil
	}
}

// GetCrashError is a non-blocking function that returns a crash error if there
// is one, the first parameter indicates if the supervisor is running or not. If
// the returned error is not nil, the first result will always be true.
func (sup Supervisor) GetCrashError(block bool) (bool, error) {
	return getCrashError(
		false, /* block */
		sup.spec.eventNotifiers,
		sup.runtimeName,
		sup.terminateCh,
		sup.terminateManager,
		time.Time{},
	)
}
