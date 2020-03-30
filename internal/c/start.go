package c

import (
	"context"
	"errors"
	"strings"
	"time"
)

// waitTimeout is the internal function used by Child to wait for the execution
// of it's thread to stop.
func waitTimeout(
	terminateCh <-chan ChildNotification,
) func(Shutdown) error {
	return func(shutdown Shutdown) error {
		switch shutdown.tag {
		case infinityT:
			// We wait forever for the result
			childNotification, ok := <-terminateCh
			if !ok {
				return nil
			}
			// A child may have terminated with an error
			return childNotification.Unwrap()
		case timeoutT:
			// we wait until some duration
			select {
			case childNotification, ok := <-terminateCh:
				if !ok {
					return nil
				}
				// A child may have terminated with an error
				return childNotification.Unwrap()
			case <-time.After(shutdown.duration):
				return errors.New("Child shutdown timeout")
			}
		default:
			// This should never happen if we use the already defined Shutdown types
			panic("Invalid shutdown value received")
		}
	}
}

// DoStart spawns a new goroutine that will execute the `Start` attribute of the
// ChildSpec, this function will block until the spawned goroutine notifies it
// has been initialized.
//
// ### The notifyResult callback
//
// This callback notifies this child's supervisor that the goroutine has
// finished (either with or without an error). The runtime name of the child is
// also given so that the supervisor can use the spec for that child when
// restarting.
//
// #### Why a callback?
//
// By using a callback we avoid coupling the Supervisor types to the Child
// logic.
//
func (chSpec ChildSpec) DoStart(
	parentName string,
	supNotifyCh chan<- ChildNotification,
) (Child, error) {

	runtimeName := strings.Join([]string{parentName, chSpec.GetName()}, "/")
	childCtx, cancelFn := context.WithCancel(context.Background())

	startCh := make(chan startError)
	terminateCh := make(chan ChildNotification)

	// Child Goroutine is bootstraped
	go func() {
		// TODO: Recover from panics

		// we tell the spawner this child thread has stopped
		defer close(terminateCh)

		// we cancel the childCtx on regular termination
		defer cancelFn()

		// client logic starts here, despite the call here being a "start", we will
		// block and wait here until an error (or lack of) is reported from the
		// client code
		err := chSpec.Start(childCtx, func(err error) {
			// we tell the spawner this child thread has started running
			if err != nil {
				startCh <- err
			}
			close(startCh)
		})

		childNotification := ChildNotification{
			name:        chSpec.GetName(),
			tag:         chSpec.GetTag(),
			runtimeName: runtimeName,
			err:         err,
		}

		// We send the childNotification that got created to our parent supervisor.
		//
		// There are two ways the supervisor could receive this notification:
		//
		// 1) If the supervisor is running it's supervision loop (e.g. normal
		// execution), the notification will be received over the `supNotifyCh`
		// channel; this will execute the restart mechanisms.
		//
		// 2) If the supervisor is shutting down, it won't be reading the
		// `supNotifyCh`, but instead is going to be executing the `stopChildren`
		// function, which calls the `child.Stop` method for each of the supervised
		// internally, this function reads the `terminateCh`.
		//
		select {
		// (1)
		case supNotifyCh <- childNotification:
		// (2)
		case terminateCh <- childNotification:
		}

	}()

	// Wait until child thread notifies it has started or failed with an error
	err := <-startCh
	if err != nil {
		return Child{}, err
	}

	return Child{
		runtimeName: runtimeName,
		createdAt:   time.Now(),
		spec:        chSpec,
		cancel:      cancelFn,
		wait:        waitTimeout(terminateCh),
	}, nil
}
