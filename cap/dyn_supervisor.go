package cap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// ctrlMsg executes some control logic on the Supervisor goroutine
type ctrlMsg interface {
	// processMsg receives all the required supervisor state to fullfill
	// its purpose
	processMsg(
		evNotifier EventNotifier,
		spec SupervisorSpec,
		specChildren []c.ChildSpec,
		supRuntimeName string,
		supChildren map[string]c.Child,
		supNotifyCh chan c.ChildNotification,
	) ([]c.ChildSpec, map[string]c.Child)
}

type startChildResult struct {
	childName string
	startErr  startError
}

// startChildMsg is a message sent from clients to tell a supervisor to spawn
// on-demand a worker routine.
type startChildMsg struct {
	node Node
	// result could either be a startError or a string with the runtime name
	resultChan chan<- startChildResult
}

func (scm startChildMsg) processMsg(
	evNotifier EventNotifier,
	spec SupervisorSpec,
	specChildren []c.ChildSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
) ([]c.ChildSpec, map[string]c.Child) {
	// REMEMBER: WE ARE RUNNING THIS CODE IN THE SUPERVISOR THREAD

	childSpec := scm.node(spec)

	ch, startErr := startChildNode(spec, supRuntimeName, supNotifyCh, childSpec)
	if startErr != nil {
		// When we fail, we send an error to the supNotifyCh and return the error,
		// this doesn't have any detrimental consequence in static supervisors,
		// but on dynamic supervisors, it means the monitor loop will get bothered
		// with an error that it should not really handle. We are going to read it
		// out and return after that.
		_ = <-supNotifyCh
		// do not block waiting for a read
		select {
		case scm.resultChan <- startChildResult{
			childName: "",
			startErr:  startErr,
		}:
		default:
		}

		return specChildren, supChildren

	}

	// We store the child to the spec list because we need to terminate them
	// when the supervisor is terminated in the correct order. This won't have
	// unintended side-effects because a DynSupervisor once terminated, cannot
	// be started again.
	specChildren = append(specChildren, childSpec)
	supChildren[ch.GetName()] = ch

	select {
	case scm.resultChan <- startChildResult{
		childName: ch.GetName(),
		startErr:  nil,
	}:
	// We return the regular name (not the runtime one), given that we store that
	// in the childrenMap of the supervisor
	default:
	}

	return specChildren, supChildren
}

var _ ctrlMsg = startChildMsg{}

// terminateChildMsg is a message sent from clients to tell a supervisor to close a
// previously spawned worker routine.
type terminateChildMsg struct {
	nodeName   string
	resultChan chan<- terminateError
}

func (tcm terminateChildMsg) processMsg(
	evNotifier EventNotifier,
	spec SupervisorSpec,
	specChildren []c.ChildSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
) ([]c.ChildSpec, map[string]c.Child) {
	// REMEMBER: WE ARE RUNNING THIS CODE IN THE SUPERVISOR THREAD

	ch, ok := supChildren[tcm.nodeName]
	if !ok {
		errMsg := fmt.Sprintf("worker %s not found", tcm.nodeName)
		// do not block waiting for a read
		select {
		case tcm.resultChan <- errors.New(errMsg):
		default:
		}

		return specChildren, supChildren
	}

	// we call our basic terminateChildNode function that is found in the
	// monitor.go file
	terminateErr := terminateChildNode(evNotifier, ch)

	// do not block waiting for a read
	select {
	case tcm.resultChan <- terminateErr:
	default:
	}

	// we remove the terminated child from the spec and the runtime children to
	// avoid shutting it down on supervisor termination
	for i, chSpec := range specChildren {
		if chSpec.GetName() == ch.GetName() {
			specChildren = append(specChildren[:i], specChildren[i+1:]...)
			delete(supChildren, ch.GetName())
		}
	}

	return specChildren, supChildren
}

var _ ctrlMsg = terminateChildMsg{}

// DynSupervisor is a supervisor that can spawn workers in a procedural way.
type DynSupervisor struct {
	sup            Supervisor
	terminated     bool
	terminationErr error
}

// handleCtrlMsg is used in the supervisor monitor loop to operator over public
// API calls like Spawn or Cancel a child node.
func handleCtrlMsg(
	eventNotifier EventNotifier,
	spec SupervisorSpec,
	specChildren []c.ChildSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	msg ctrlMsg,
) ([]c.ChildSpec, map[string]c.Child) {
	return msg.processMsg(
		eventNotifier,
		spec,
		specChildren,
		supRuntimeName,
		supChildren,
		supNotifyCh,
	)
}

func (dyn *DynSupervisor) terminateNode(nodeName string) func() error {
	// REMEMBER: WE ARE RUNNING ON THE CLIENT API THREAD

	// we initialize the resultCh with a buffer of 1, we may store the result
	// before the client is ready to read it.
	resultCh := make(chan terminateError, 1)
	msg := terminateChildMsg{
		nodeName:   nodeName,
		resultChan: resultCh,
	}
	return func() (err error) {
		defer func() {
			panicVal := recover()
			if panicVal == nil {
				return
			}
			switch v := panicVal.(type) {
			case error:
				err = fmt.Errorf("could not talk to supervisor: %w", v)
				return
			default:
				// retrigger panic, this would happen on an implementation error
				panic(panicVal)
			}
		}()
		// block until the supervisor can handle the request, in case the
		// supervisor is stopped, this line is going to panic
		select {
		case dyn.sup.ctrlCh <- msg:
		case _ = <-time.After(1 * time.Second):
			// This scenario can happen when the supervisor is being terminated and the
			// non-blocking sup.GetCrashError happened just before that (race
			// condition).
			err = errors.New("could not talk to supervisor")
			return
		}

		select {
		case err = <-resultCh:
		case <-time.After(1 * time.Second):
			// Not sure when this scenario would happen to be honest :shrug:
			err = errors.New("could not get a cancelation confirmation from worker")
		}
		return
	}
}

// Spawn creates a new worker routine from the given node specification. It
// either returns a cancel/shutdown callback or an error in the scenario the
// start of this worker failed. This function blocks until the worker is
// started.
func (dyn *DynSupervisor) Spawn(nodeFn Node) (func() error, error) {
	// REMEMBER: WE ARE RUNNING ON THE CLIENT API THREAD

	// if we already registered a terminationErr, return it
	if dyn.terminated {
		return nil, fmt.Errorf("supervisor already terminated: %w", dyn.terminationErr)
	}

	// if the underlying supervisor is kaput, return the error
	if terminated, terminationErr := dyn.sup.GetCrashError(false); terminated {
		dyn.terminated = true
		dyn.terminationErr = terminationErr
		return nil, fmt.Errorf("supervisor already terminated: %w", terminationErr)
	}

	// we initialize the resultCh with a buffer of 1, we may store the result
	// before the client is ready to read it.
	resultCh := make(chan startChildResult, 1)
	msg := startChildMsg{
		node:       nodeFn,
		resultChan: resultCh,
	}

	select {
	case dyn.sup.ctrlCh <- msg:
	case _ = <-time.After(1 * time.Second):
		// This scenario can happen when the supervisor is being terminated and the
		// non-blocking sup.GetCrashError happened just before that (race
		// condition).
		return nil, errors.New("could not talk to supervisor")
	}

	select {
	case result, ok := <-resultCh:
		if !ok {
			panic("could not get the result of a spawn call. Implementation error")
		}

		if result.startErr != nil {
			return nil, result.startErr
		}
		return dyn.terminateNode(result.childName), nil
	case <-time.After(1 * time.Second):
		// Paranoid timeout. Better to not hang if this ever happens; to be honest,
		// not sure when this is the case :shrug:
		return nil, errors.New("could not get a creation confirmation from worker")
	}
}

// Terminate is a synchronous procedure that halts the execution of the whole
// supervision tree.
func (dyn *DynSupervisor) Terminate() error {
	dyn.terminationErr = dyn.sup.Terminate()
	dyn.terminated = true
	return dyn.terminationErr
}

// Wait blocks the execution of the current goroutine until the Supervisor
// finishes it execution.
func (dyn DynSupervisor) Wait() error {
	return dyn.sup.Wait()
}

// GetName returns the name of the Spec used to start this Supervisor
func (dyn DynSupervisor) GetName() string {
	return dyn.sup.GetName()
}

// NewDynSupervisor creates a DynamicSupervisor which can start workers at
// runtime in a procedural manner. It receives a context and the supervisor name
// (for tracing purposes).
//
//
// When to use a DynSupervisor?
//
// If you want to run supervised worker routines on dynamic inputs. This is
// something that a regular Supervisor cannot do, as it needs to know the
// children nodes at construction time.
//
// Differences to Supervisor
//
// As opposed to a Supervisor, a DynSupervisor:
//
// * Cannot receive node specifications to start them in an static fashion
//
// * It is able to spawn workers dynamically
//
// * In case of a hard crash and following restart, it will start with an empty
//   list of children
//
func NewDynSupervisor(ctx context.Context, name string, opts ...Opt) (DynSupervisor, error) {
	spec := NewSupervisorSpec(name, WithNodes(), opts...)
	sup, err := spec.Start(ctx)
	if err != nil {
		return DynSupervisor{}, err
	}
	return DynSupervisor{sup: sup}, nil
}
