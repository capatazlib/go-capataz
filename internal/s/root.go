package s

// This file contains the implementation of the supervision start
//
// * Creation of Supervisor from SupervisorSpec
// * Children bootstrap
// * Monitor Loop
//

import (
	"context"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// rootSupervisorName is the name the root supervisor has, this is used to
// compare the process current name to the rootSupervisorName
var rootSupervisorName = ""

// notifyTerminationFn is a callback that gets called when a supervisor is
// terminating (with or without an error).
type notifyTerminationFn = func(terminateNodeError)

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

type capatazSupKey string

var eventNotifierKey capatazSupKey = "__capataz.node.event_notifier__"

// withEventNotifier sets the Capataz EventNotifier in the context that is
// thread-through across all capataz logic
func withEventNotifier(ctx context.Context, evNotifier EventNotifier) context.Context {
	return context.WithValue(ctx, eventNotifierKey, evNotifier)
}

// getEventNotifier returns the EventNotifier that is thread-through all the
// capataz API
func getEventNotifier(ctx context.Context) (EventNotifier, bool) {
	val := ctx.Value(eventNotifierKey)
	if val != nil {
		if evNotifier, ok := val.(EventNotifier); ok {
			return evNotifier, true
		}
		return nil, false
	}
	return nil, false
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
func (spec SupervisorSpec) rootStart(
	startCtx context.Context,
	parentName string,
) (Supervisor, error) {
	// cancelFn is used when Terminate is requested
	supCtx, cancelFn := context.WithCancel(startCtx)

	// notifyCh is used to keep track of errors from children
	notifyCh := make(chan c.ChildNotification)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	ctrlCh := make(chan ctrlMsg)

	// startCh is used to track when the supervisor loop thread has started
	startCh := make(chan startNodeError)

	// terminateCh is used when waiting for cancelFn to complete
	terminateCh := make(chan terminateNodeError)

	supRuntimeName := buildRuntimeName(spec, parentName)

	eventNotifier := spec.getEventNotifier()
	supCtx = withEventNotifier(supCtx, eventNotifier)

	// Build childrenSpec and resource cleanup
	childrenSpecs, supRscCleanup, rscAllocError := spec.buildChildrenSpecs(supRuntimeName)

	// Do not even start the monitor loop if we find an error on the resource
	// allocation logic
	if rscAllocError != nil {
		cancelFn()
		eventNotifier.supervisorStartFailed(supRuntimeName, rscAllocError)
		return Supervisor{}, rscAllocError
	}

	tm := newTerminationManager()
	// ^^^ used to detect the termination of a supervisor.

	sup := Supervisor{
		runtimeName: supRuntimeName,
		ctrlCh:      ctrlCh,

		terminateCh:      terminateCh,
		terminateManager: tm,

		spec:     spec,
		children: make(map[string]c.Child, len(childrenSpecs)),

		cancel: cancelFn,
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
				true, /* block */
				eventNotifier,
				supRuntimeName,
				terminateCh,
				tm,
				stopingTime,
			)

			if supErr != nil {
				return supErr
			}

			return nil
		},
	}

	onStart := func(err startNodeError) {
		if err != nil {
			startCh <- err
		}
		close(startCh)
	}

	onTerminate := func(err terminateNodeError) {
		if err != nil {
			terminateCh <- err
		}
		close(ctrlCh)
		close(terminateCh)
	}

	supTolerance := &restartToleranceManager{restartTolerance: spec.restartTolerance}

	// spawn goroutine with supervisor monitorLoop
	go func() {
		// NOTE: we ignore the returned error as that is being handled by the
		// onStart and onTerminate callbacks
		startTime := time.Now()
		_ = runMonitorLoop(
			supCtx,
			spec,
			childrenSpecs,
			supRuntimeName,
			supTolerance,
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
