package s

// This file contains the implementation of the supervision start
//
// * Creation of Supervisor from SupervisorSpec
// * Children bootstrap
// * Monitor Loop
//

import (
	"context"
	"errors"
	"strings"

	"github.com/capatazlib/go-capataz/c"
)

// runMonitorLoop does the initialization of supervisor's children and then runs
// an infinite loop that monitors each child error.
//
// This function is used for both async and sync strategies, given this, we
// receive an onStart and onTerminate callbacks that behave differently
// depending on which strategy is used:
//
// 1) When called with the async strategy, these callbacks will interact with
// gochans that communicate with the spawner goroutine.
//
// 2) When called with the sync strategy, these callbacks will return the given
// error, note this implementation returns the result of the callback calls
//
func runMonitorLoop(
	ctx context.Context,
	spec SupervisorSpec,
	runtimeName string,
	notifyCh chan c.ChildNotification,
	onStart c.NotifyStartFn,
	onTerminate func(terminateError),
) error {
	// Start children
	children, err := startChildren(spec, runtimeName, notifyCh)
	if err != nil {
		// in case we run in the async strategy we notify the spawner that we
		// started with an error
		onStart(err)
		// We signal that we terminated, the error is not reported here because
		// it was reported in the onStart
		onTerminate(nil)
		return err
	}

	// Once children have been spawned we notify the supervisor main loop has
	// started, we ignore the return given onStart usually returns the given
	// parameter
	onStart(nil)

	// Supervisor Loop
	for {
		select {
		// parent context is done
		case <-ctx.Done():
			childErrMap := stopChildren(spec, children, false /* starting? */)
			// If any of the children fails to stop, we should report that as an
			// error
			if len(childErrMap) > 0 {
				// On async strategy, we notify that the spawner terminated with an
				// error
				err := SupervisorError{
					err:         errors.New("Supervisor stop error"),
					runtimeName: runtimeName,
					childErrMap: childErrMap,
				}
				onTerminate(err)
				// On sync strategy, we return the error
				return err
			}
			onTerminate(nil)
			return nil
		case /* childNotification = */ <-notifyCh:
			// TODO: Deal with errors on children
			// case msg := <-ctrlCh:
			// TODO: Deal with public facing API calls
		}
	}
}

// buildRuntimeName creates the runtimeName of a Supervisor from the parent name
// and the spec name
func buildRuntimeName(spec SupervisorSpec, parentName string) string {
	var runtimeName string
	if parentName == rootSupervisorName {
		// We are the root supervisor, no need to add prefix
		runtimeName = spec.Name()
	} else {
		runtimeName = strings.Join([]string{parentName, spec.Name()}, "/")
	}
	return runtimeName
}

// start is routine that contains the main logic of a Supervisor. This function:
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
func (spec SupervisorSpec) start(
	parentCtx context.Context,
	parentName string,
) (Supervisor, error) {
	// cancelFn is used when Stop is requested
	ctx, cancelFn := context.WithCancel(parentCtx)

	// notifyCh is used to keep track of errors from children
	notifyCh := make(chan c.ChildNotification)

	// ctrlCh is used to keep track of request from client APIs (e.g. spawn child)
	// ctrlCh := make(chan ControlMsg)

	// startCh is used to track when the supervisor loop thread has started
	startCh := make(chan startError)

	// terminateCh is used when waiting for cancelFn to complete
	terminateCh := make(chan terminateError)

	runtimeName := buildRuntimeName(spec, parentName)

	sup := Supervisor{
		runtimeName: runtimeName,
		spec:        spec,
		children:    make(map[string]c.Child, len(spec.children)),
		cancel:      cancelFn,
		wait: func() error {
			// Let's us wait for the Supervisor goroutine to terminate, if there are
			// errors in the termination (e.g. Timeout of child, error treshold
			// reached, etc.), the terminateCh is going to return an error, otherwise
			// it will nil
			err := <-terminateCh
			return err
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
		close(terminateCh)
	}

	// spawn goroutine with supervisor monitorLoop
	go func() {
		// NOTE: we ignore the returned error as that is being handled by the
		// onStart and onTerminate callbacks
		_ = runMonitorLoop(
			ctx,
			spec,
			runtimeName,
			notifyCh,
			// ctrlCh,
			onStart,
			onTerminate,
		)
	}()

	// TODO: Figure out stop before start finish
	// TODO: Figure out start with timeout

	// We check if there was an start error reported from the monitorLoop, if this
	// is the case, we wait for the termination of started children and return the
	// reported error
	err := <-startCh
	if err != nil {
		// Let's wait for the supervisor to stop all children before returning the
		// final error
		_ /* err */ = sup.wait()
		return Supervisor{}, err
	}

	return sup, nil
}
