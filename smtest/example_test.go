package smtest_test

import (
	"context"
	"fmt"
	"testing"
	"github.com/capatazlib/go-capataz/smtest"
)

type Event struct {
	eventName string
}

func (ev Event) String() string {
	return fmt.Sprintf("Event{%v}", ev.eventName)
}

type myState struct {
	// ...
}

func myStateMachine(
	ctx context.Context,
	reqChan chan any,
	notifyEvent func(Event),
) {
	_ = myState{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-reqChan:
			// use req to modify myState and create an event from state
			// transitions. Your state machine may create various kind of events.
			fooHappened := Event{
				eventName: "foo",
			}
			notifyEvent(fooHappened)
		}
	}
}

// FooP is a predicate type that receives every event reported by the state
// machine. If the predicate matches the event, it's Call function should return
// true.
type FooP struct {}

func (p FooP) Call(ev Event) bool {
	return ev.eventName == "foo"
}

func (p FooP) String() string {
	return "foop"
}

func TestMyStateMachine(t *testing.T) {
	reqChan := make(chan any)
	ctx, stopApp := context.WithCancel(context.Background())
	defer stopApp()
	evManager := smtest.NewEventManager[Event]()
	evManager.StartCollector(ctx)
	go myStateMachine(ctx, reqChan, evManager.EventCollector(ctx))
	reqChan <- "hello world"
	it := evManager.Iterator()
	// block until FooP happens
	it.WaitTill(FooP{})
	// verify that the only event reported by the state machine is Foo
	smtest.AssertExactMatch(t, evManager.Snapshot(), []smtest.EventP[Event]{FooP{}})
}

