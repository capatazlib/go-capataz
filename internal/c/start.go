package c

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// capatazKey is an internal type for the capataz keys
type capatazKey string

// nodeNameKey is an internal representation of the worker name in the
// worker context. If you reverse engineer, you are on your own.
var nodeNameKey capatazKey = "__capataz.node.runtime_name__"

// GetNodeName gets a capataz worker name from a context
func GetNodeName(ctx context.Context) (string, bool) {
	if val := ctx.Value(nodeNameKey); val != nil {
		result, ok := val.(string)
		return result, ok
	}
	return "", false
}

// setNodeName allows to add a capataz worker name to a context
func setNodeName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nodeNameKey, name)
}

// waitTimeout is the internal function used by Child to wait for the execution
// of it's thread to stop.
func waitTimeout(
	terminateCh <-chan ChildNotification,
) func(Shutdown) (bool, error) {
	return func(shutdown Shutdown) (bool, error) {
		switch shutdown.tag {
		case indefinitelyT:
			// We wait forever for the result
			childNotification, ok := <-terminateCh
			if !ok {
				return false, nil
			}
			// A child may have terminated with an error
			return true, childNotification.Unwrap()
		case timeoutT:
			// we wait until some duration
			select {
			case childNotification, ok := <-terminateCh:
				if !ok {
					return false, nil
				}
				// A child may have terminated with an error
				return true, childNotification.Unwrap()
			case <-time.After(shutdown.duration):
				return true, errors.New("child shutdown timeout")
			}
		default:
			// This should never happen if we use the already defined Shutdown types
			panic("invalid shutdown value received; check waitTimeout implementation")
		}
	}
}

// sendNotificationToSup creates a ChildNotification record and sends it to the
// assigned supervisor for this child.
func sendNotificationToSup(
	err error,
	chSpec ChildSpec,
	chRuntimeName string,
	supNotifyChan chan<- ChildNotification,
	terminateCh chan<- ChildNotification,
) {
	chNotification := ChildNotification{
		name:        chSpec.GetName(),
		tag:         chSpec.GetTag(),
		runtimeName: chRuntimeName,
		err:         err,
	}

	// We send the chNotification that got created to our parent supervisor.
	//
	// There are two ways the supervisor could receive this notification:
	//
	// 1) If the supervisor is running it's supervision loop (e.g. normal
	// execution), the notification will be received over the `supNotifyChan`
	// channel; this will execute the restart mechanisms.
	//
	// 2) If the supervisor is shutting down, it won't be reading the
	// `supNotifyChan`, but instead is going to be executing the `stopChildren`
	// function, which calls the `child.Terminate` method for each of the supervised
	// internally, this function reads the `terminateCh`.
	//
	select {
	// (1)
	case supNotifyChan <- chNotification:
	// (2)
	case terminateCh <- chNotification:
	}
}

// DoStart spawns a new goroutine that will execute the `Start` attribute of the
// ChildSpec, this function will block until the spawned goroutine notifies it
// has been initialized.
//
// ### The supNotifyChan value
//
// Messages sent to this channel notify the supervisor that the child's
// goroutine has finished (either with or without an error). The runtime name of
// the child is also given so that the supervisor can use the spec for that
// child when restarting.
func (chSpec ChildSpec) DoStart(
	startCtx context.Context,
	supName string,
	supNotifyChan chan<- ChildNotification,
) (Child, error) {

	chRuntimeName := strings.Join([]string{supName, chSpec.GetName()}, "/")

	// we remove the cancel from the context received on the start call so that we
	// don't end up canceling the children at a non-appropiate time
	ctx := WithoutCancel(startCtx)

	// we allow a node to know it's name so as to allow subtrees to report
	// events with it's full name
	childCtx, cancelFn := context.WithCancel(setNodeName(ctx, chRuntimeName))

	startCh := make(chan startError)
	terminateCh := make(chan ChildNotification)

	// Child Goroutine is bootstraped
	go func() {
		// we tell the spawner this child thread has stopped. We want to
		// close this channel after the worker is done so that on the
		// scenario the termination logic is called again, the call
		// returns immediatelly and without errors
		defer close(terminateCh)

		// we cancel the childCtx on regular termination
		defer cancelFn()

		defer func() {
			if chSpec.DoesCapturePanic() {
				panicVal := recover()
				// if there is a panicVal in the recover, we should handle this as an
				// error
				if panicVal == nil {
					return
				}

				panicErr, ok := panicVal.(error)
				if !ok {
					panicErr = fmt.Errorf("panic error: %v", panicVal)
				}

				startCh <- panicErr

				sendNotificationToSup(
					panicErr,
					chSpec,
					chRuntimeName,
					supNotifyChan,
					terminateCh,
				)
			}
		}()

		// client logic starts here, despite the call here being a "start", we will
		// block and wait here until an error (or lack of) is reported from the
		// client code
		err := chSpec.Start(childCtx, func(err error) {
			// we tell the spawner this child thread has started running. err may be
			// nil
			startCh <- err
		})

		sendNotificationToSup(
			err,
			chSpec,
			chRuntimeName,
			supNotifyChan,
			terminateCh,
		)
	}()

	// Wait until child thread notifies it has started or failed with an error
	err := <-startCh
	if err != nil {
		return Child{}, err
	}

	return Child{
		runtimeName: chRuntimeName,
		createdAt:   time.Now(),
		spec:        chSpec,
		cancel:      cancelFn,
		wait:        waitTimeout(terminateCh),
	}, nil
}
