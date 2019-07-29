package capataz

import (
	"context"
	"sync"
	"time"
)

// Child represents a child process which could be either a worker
// or a supervisor
type Child interface {
	Start(context.Context) error
}

// ChildFunc allows creating children passing in a start function
type ChildFunc func(context.Context) error

// Start it is required to implement the Child interface
func (f ChildFunc) Start(ctx context.Context) error {
	return f(ctx)
}

// ChildFactory represents a factory function for child processes
type ChildFactory func() Child

// ChildType can be either a WorkerType or a SupervisorType
type ChildType uint8

const (
	// WorkerType identifies a child as a worker
	WorkerType ChildType = iota

	// SupervisorType identifies a child as a supervisor
	SupervisorType
)

// ChildSpec represents the specification of a child process
type ChildSpec struct {
	ID      string
	Factory ChildFactory
	Type    ChildType
}

// MakeWorkerChildSpec creates a instance of ChildSpec
func MakeWorkerChildSpec(id string, factory ChildFactory) ChildSpec {
	return ChildSpec{ID: id, Type: WorkerType, Factory: factory}
}

// MakeFuncWorkerChildSpec creates a instance of ChildSpec passing in a start
// function
func MakeFuncWorkerChildSpec(id string, f ChildFunc) ChildSpec {
	return MakeWorkerChildSpec(id, func() Child { return f })
}

type childEventType uint8

const (
	childStarted childEventType = iota
	childCompleted
	childShutdown
	childFailed
)

func (et childEventType) String() string {
	switch et {
	case childStarted:
		return "CHILD_STARTED"
	case childCompleted:
		return "CHILD_COMPLETED"
	case childShutdown:
		return "CHILD_SHUTDOWN"
	case childFailed:
		return "CHILD_FAILED"
	default:
		return "UNKNOWN_CHILD_EVENT"
	}
}

type childEvent struct {
	eventType childEventType
	child     *child
	time      time.Time
	err       error
}

type childEventNotifier chan<- childEvent

func (n childEventNotifier) notify(eventType childEventType, child *child, err error) {
	n <- childEvent{eventType, child, time.Now(), err}
}

type child struct {
	spec      ChildSpec
	cancelCtx context.CancelFunc
	wg        sync.WaitGroup
}

func spawnChild(spec ChildSpec, eventNotifier childEventNotifier) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	c := &child{spec: spec, cancelCtx: cancelCtx}

	c.wg.Add(1)
	eventNotifier.notify(childStarted, c, nil)

	go func() {
		err := c.spec.Factory().Start(ctx)
		c.wg.Done()

		switch err {
		case nil:
			eventNotifier.notify(childCompleted, c, nil)
		case ctx.Err():
			eventNotifier.notify(childShutdown, c, nil)
		default:
			eventNotifier.notify(childFailed, c, err)
		}
	}()
}

func (c *child) stop() {
	// TODO Add timeout or this can block forever
	c.cancelCtx()
	c.wg.Wait()
}
