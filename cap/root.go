package cap

// This file contains the implementation of the supervision start
//
// * Creation of Supervisor from SupervisorSpec
// * Children bootstrap
// * Monitor Loop
//

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// rootSupervisorName is the name the root supervisor has, this is used to
// compare the process current name to the rootSupervisorName
var rootSupervisorName = ""

// notifyTerminationFn is a callback that gets called when a supervisor is
// terminating (with or without an error).
type notifyTerminationFn = func(terminateError)

// buildRuntimeName creates the runtimeName of a Supervisor from the parent name
// and the spec name
func buildRuntimeName(spec SupervisorSpec, parentName string) string {
	var runtimeName string
	if parentName == rootSupervisorName {
		// We are the root supervisor, no need to add prefix
		runtimeName = spec.GetName()
	} else {
		runtimeName = strings.Join([]string{parentName, spec.GetName()}, "/")
	}
	return runtimeName
}

// rootStart is routine that contains the main logic of a Supervisor. This
// function:
//
// 1) spawns a new goroutine for the supervisor loop
//
// 2) spawns each child goroutine in the correct order
//
// 3) stops all the spawned children in the correct order once it gets a stop
// signal
//
// 4) it monitors and reacts to errors reported by the supervised children
//
func (spec SupervisorSpec) rootStart(
	parentCtx context.Context,
	parentName string,
) (Supervisor, error) {
	// cancelFn is used when Terminate is requested
	ctx, cancelFn := context.WithCancel(parentCtx)

	// notifyCh is used to keep track of errors from children
	notifyCh := make(chan c.ChildNotification)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	ctrlCh := make(chan ctrlMsg)

	// startCh is used to track when the supervisor loop thread has started
	startCh := make(chan startError)

	// terminateCh is used when waiting for cancelFn to complete
	terminateCh := make(chan terminateError)

	supRuntimeName := buildRuntimeName(spec, parentName)

	eventNotifier := spec.getEventNotifier()

	// Build childrenSpec and resource cleanup
	childrenSpecs, supRscCleanup, rscAllocError := spec.buildChildrenSpecs()

	// Do not even start the monitor loop if we find an error on the resource
	// allocation logic
	if rscAllocError != nil {
		cancelFn()
		eventNotifier.supervisorStartFailed(supRuntimeName, rscAllocError)
		return Supervisor{}, rscAllocError
	}

	// The variables bellow are used to detect the termination of a supervisor.
	// This is necessary because when we run a DynSupervisor, we need to make sure
	// that we *do not* spawn workers on a terminated supervisor, otherwise we run
	// the risk of getting a panic error, not good.
	var mux sync.Mutex

	var terminatedVal = false
	var terminated = &terminatedVal

	var terminateErrVal error
	var terminateErr = &terminateErrVal

	sup := Supervisor{
		mux:          &mux,
		runtimeName:  supRuntimeName,
		ctrlCh:       ctrlCh,
		terminateCh:  terminateCh,
		terminated:   terminated,
		terminateErr: terminateErr,
		spec:         spec,
		children:     make(map[string]c.Child, len(childrenSpecs)),
		cancel:       cancelFn,
		wait: func(stopingTime time.Time, startErr error) error {

			// We check if there was an start error reported, if this is the case, we
			// notify that the supervisor start failed
			if startErr != nil {
				eventNotifier.supervisorStartFailed(supRuntimeName, startErr)
				return startErr
			}

			// Let us wait for the Supervisor goroutine to terminate, if there are
			// errors in the termination (e.g. Timeout of child, error tolerance
			// surpassed, etc.), the terminateCh is going to return an error,
			// otherwise it will return nil
			_, supErr := getCrashError(
				eventNotifier,
				supRuntimeName,
				&mux,
				terminateCh,
				terminated,
				terminateErr,
				stopingTime,
				true, /* block */
			)

			if supErr != nil {
				return supErr
			}

			return nil
		},
	}

	onStart := func(err startError) {
		if err != nil {
			startCh <- err
		}
		close(startCh)
	}

	onTerminate := func(err terminateError) {
		if err != nil {
			terminateCh <- err
		}
		close(ctrlCh)
		close(terminateCh)
	}

	// spawn goroutine with supervisor monitorLoop
	go func() {
		// NOTE: we ignore the returned error as that is being handled by the
		// onStart and onTerminate callbacks
		startTime := time.Now()
		_ = runMonitorLoop(
			ctx,
			spec,
			childrenSpecs,
			supRuntimeName,
			supRscCleanup,
			notifyCh,
			ctrlCh,
			startTime,
			onStart,
			onTerminate,
		)
	}()

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout

	// We check if there was an start error reported from the monitorLoop, if this
	// is the case, we wait for the termination of started children and return the
	// reported error
	startErr := <-startCh
	if startErr != nil {
		// Let's wait for the supervisor to stop all children before returning the
		// final error
		stopingTime := time.Now()
		_ /* err */ = sup.wait(stopingTime, startErr)

		return Supervisor{}, startErr
	}

	return sup, nil
}
