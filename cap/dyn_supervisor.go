package cap

import (
	"context"
	"errors"
	"fmt"

	"github.com/capatazlib/go-capataz/internal/c"
)

// startChildMsg is a message sent from clients to tell a supervisor to spawn
// on-demand a worker routine.
type startChildMsg struct {
	node Node
	// result could either be a startError or a string with the runtime name
	resultChan chan<- interface{}
}

// terminateChildMsg is a message sent from clients to tell a supervisor to close a
// previously spawned worker routine.
type terminateChildMsg struct {
	nodeName   string
	resultChan chan<- terminateError
}

// ctlrMsgTag tells us which control message we need to cast/handle
type ctrlMsgTag uint32

const (
	startChild ctrlMsgTag = iota
	terminateChild
)

// ctlrMsg is a message sent from clients to a supervisor via a public API
// method
type ctrlMsg struct {
	tag ctrlMsgTag
	msg interface{}
}

// DynSupervisor is a supervisor that can spawn workers in a procedural way.
type DynSupervisor struct {
	sup            Supervisor
	terminated     bool
	terminationErr error
}

func handleCtrlMsg(
	eventNotifier EventNotifier,
	supSpec SupervisorSpec,
	supChildrenSpec []c.ChildSpec,
	supRuntimeName string,
	supChildren map[string]c.Child,
	supNotifyCh chan c.ChildNotification,
	msg ctrlMsg,
) ([]c.ChildSpec, map[string]c.Child) {
	switch msg.tag {
	case startChild:
		startcm, ok := msg.msg.(startChildMsg)
		if !ok {
			panic("Expected startChildMsg; got something else. Invalid Supervisor implementation")
		}
		childSpec := startcm.node(supSpec)

		ch, startErr := startChildNode(supSpec, supRuntimeName, supNotifyCh, childSpec)
		if startErr != nil {
			// When we fail, we send an error to the supNotifyCh and return the error,
			// this doesn't have any detrimental consequence in static supervisors,
			// but on dynamic supervisors, it means the monitor loop will get bothered
			// with an error that it should not really handle. We are going to read it
			// out and return after that.
			_ = <-supNotifyCh
			// do not block waiting for a read
			select {
			case startcm.resultChan <- startErr:
			default:
			}

			return supChildrenSpec, supChildren
		}

		// We store the child to the spec list because we need to terminate them
		// when the supervisor is terminated in the correct order. This won't have
		// unintended side-effects because a DynSupervisor once terminated, cannot
		// be started again.
		supChildrenSpec = append(supChildrenSpec, childSpec)
		supChildren[ch.GetName()] = ch

		select {
		case startcm.resultChan <- ch.GetName():
			// We return the regular name (not the runtime one), given that we store that
			// in the childrenMap of the supervisor
		default:
		}

		return supChildrenSpec, supChildren

	case terminateChild:
		terminatecm, ok := msg.msg.(terminateChildMsg)
		ch, ok := supChildren[terminatecm.nodeName]
		if !ok {
			errMsg := fmt.Sprintf("worker %s not found", terminatecm.nodeName)
			// do not block waiting for a read
			select {
			case terminatecm.resultChan <- errors.New(errMsg):
			default:
			}

			return supChildrenSpec, supChildren
		}

		// we call our basic terminateChildNode function that is found in the
		// monitor.go file
		terminateErr := terminateChildNode(eventNotifier, ch)
		// do not block waiting for a read
		select {
		case terminatecm.resultChan <- terminateErr:
		default:
		}

		return supChildrenSpec, supChildren

	default:
		panic("Unknown control message received. Invalid Supervisor implementation.")
	}
}

func (dyn *DynSupervisor) terminateNode(nodeName string) context.CancelFunc {
	resultCh := make(chan terminateError)
	terminatemsg := terminateChildMsg{
		nodeName:   nodeName,
		resultChan: resultCh,
	}
	return func() {
		// block until the supervisor can handle the request, in case the
		// supervisor is stopped, this line is going to panic
		// TODO: be extra paranoid and add a timeout here
		dyn.sup.ctrlCh <- ctrlMsg{
			tag: terminateChild,
			msg: terminatemsg,
		}

		_ = <-resultCh
	}
}

// Spawn creates a new worker routine from the given node specification. It
// either returns a cancel/shutdown callback or an error in the scenario the
// start of this worker failed. This function blocks until the worker is
// started.
func (dyn *DynSupervisor) Spawn(nodeFn Node) (context.CancelFunc, error) {
	// if we already registered a terminationErr, return it
	if dyn.terminated {
		return func() {}, fmt.Errorf("supervisor already terminated: %w", dyn.terminationErr)
	}

	// if the underlying supervisor is kaput, return the error
	if terminationErr := dyn.sup.GetCrashError(); terminationErr != nil {
		dyn.terminated = true
		dyn.terminationErr = terminationErr
		return func() {}, fmt.Errorf("supervisor already terminated: %w", terminationErr)
	}

	resultCh := make(chan interface{})
	startmsg := startChildMsg{
		node:       nodeFn,
		resultChan: resultCh,
	}

	// block until the supervisor can handle the request, in case the
	// supervisor is stopped, this line is going to panic
	// TODO: be extra paranoid and add a timeout here
	dyn.sup.ctrlCh <- ctrlMsg{
		tag: startChild,
		msg: startmsg,
	}

	// blocks until worker start, the worker already has a timeout mechanism
	// so we rely on that to not get stuck here forever
	result, ok := <-resultCh

	if !ok {
		panic("could not get the result of a spawn call. Implementation error")
	}

	switch v := result.(type) {
	// successful case
	case string:
		return dyn.terminateNode(v), nil

	// error case
	case error:
		return nil, v

	// unknown case
	default:
		panic("did not get valid response value from control message. Implementation error")
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
// * Cannot receive node specifications to start then in an static fashion
//
// * It is able to spawn workers dynamically
//
// * In case of a hard crash and following restart, it will start with an empty
//   list of children
//
func NewDynSupervisor(ctx context.Context, name string, opts ...Opt) (DynSupervisor, error) {
	spec := NewSupervisorSpec(name, withNodes([]Node{}), opts...)
	sup, err := spec.Start(ctx)
	if err != nil {
		return DynSupervisor{}, err
	}
	return DynSupervisor{sup: sup}, nil
}
