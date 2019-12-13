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

// monitorLoop does the initialization of supervisor's children and then runs an
// infinite loop that monitors each child error.
func (sup Supervisor) monitorLoop(
	ctx context.Context,
	startCh chan startError,
	terminateCh chan terminateError,
	notifyCh chan c.ChildNotification,
) {
	defer close(terminateCh)

	// Start children
	err := sup.startChildren(notifyCh)
	if err != nil {
		startCh <- err
		return
	}

	// Once children have been spawned we notify the supervisor thread has
	// started
	close(startCh)

	// Supervisor Loop
	for {
		select {
		// parent context is done
		case <-ctx.Done():
			childErrMap := sup.stopChildren(false /* starting? */)
			// If any of the children fails to stop, we should report that as an
			// error
			if len(childErrMap) > 0 {
				terminateCh <- SupervisorError{
					err:         errors.New("Supervisor stop error"),
					runtimeName: sup.runtimeName,
					childErrMap: childErrMap,
				}
			}
			return
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
func (spec SupervisorSpec) start(parentCtx context.Context, parentName string) (Supervisor, error) {
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

	// spawn goroutine with supervisor monitorLoop
	go sup.monitorLoop(ctx, startCh, terminateCh, notifyCh)

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
