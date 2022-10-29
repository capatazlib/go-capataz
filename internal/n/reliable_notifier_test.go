// please follow the comments as close as possible and create a ticket if any
// test does not make sense.
package n_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

// newBlockingNotifier creates an EventNotifier and a callback function that
// will block until the given number of notifications have happened
func newBlockingNotifier(total int32) (cap.EventNotifier, func()) {
	doneCh := make(chan struct{})
	evCount := int32(0)
	evNotifier := func(cap.Event) {
		current := atomic.LoadInt32(&evCount)
		if current < total-1 {
			atomic.AddInt32(&evCount, 1)
			return
		}
		close(doneCh)
	}
	doneSignal := func() {
		<-doneCh
	}
	return evNotifier, doneSignal
}

// newPanicBlockingNotifier creates an EventNotifier and a callback function
// that will block until the given number of panic calls have happened
func newPanicBlockingNotifier(total int32) (cap.EventNotifier, func()) {
	doneCh := make(chan struct{})
	evCount := int32(0)
	evNotifier := func(ev cap.Event) {
		current := atomic.LoadInt32(&evCount)
		if current < total-1 {
			atomic.AddInt32(&evCount, 1)
			panic("this is taking down the tree")
		}
		close(doneCh)
	}
	doneSignal := func() {
		<-doneCh
	}
	return evNotifier, doneSignal
}

func slowEvNotifier(delay time.Duration) cap.EventNotifier {
	return func(cap.Event) {
		time.Sleep(delay)
	}
}

// TestReliableNotifierHappyPath checks functionality of the ReliableNotifier
// when there are no panics on the notifiers
func TestReliableNotifierHappyPath(t *testing.T) {

	// these are the output events we are expecting from a test supervision tree
	outEvents := []EventP{
		WorkerStarted("root/child0"),
		WorkerStarted("root/child1"),
		WorkerStarted("root/child2"),
		SupervisorStarted("root"),
		WorkerTerminated("root/child2"),
		WorkerTerminated("root/child1"),
		WorkerTerminated("root/child0"),
		SupervisorTerminated("root"),
	}

	// create multiple event notifiers with their expected number of times to be
	// called
	expectedCount := int32(len(outEvents))
	notifier1, done1 := newBlockingNotifier(expectedCount)
	notifier2, done2 := newBlockingNotifier(expectedCount)
	notifier3, done3 := newBlockingNotifier(expectedCount)

	// create the reliable event notifier that broadcasts to notifiers created in
	// step above
	evNotifier, cancelEvNotifier, err := cap.NewReliableNotifier(
		map[string]cap.EventNotifier{
			"notifier1": notifier1,
			"notifier2": notifier2,
			"notifier3": notifier3,
		},
	)

	// assert reliable notifier started without errors
	assert.NoError(t, err)
	defer cancelEvNotifier()

	// run a supervision tree such that it returns the output events declared above
	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		"root",
		cap.WithNodes(
			WaitDoneWorker("child0"),
			WaitDoneWorker("child1"),
			WaitDoneWorker("child2"),
		),
		[]cap.Opt{},
		[]cap.EventNotifier{
			evNotifier,
		},
		func(EventManager) {},
	)
	assert.NoError(t, err)

	// assert the events from the input supervision tree are the expected ones
	AssertExactMatch(t, events, outEvents)

	// if any of the calls bellow blocks, the test failed
	// wait for notifier1/2/3 to receive expected event count
	done1()
	done2()
	done3()
}

// TestReliableNotifierFailureCallback verifies that the failure callback gets called
// when a given event notifier is panicking
func TestReliableNotifierFailureCallback(t *testing.T) {

	// these are the output events we are expecting from a test supervision tree
	outEvents := []EventP{
		WorkerStarted("root/child0"),
		WorkerStarted("root/child1"),
		WorkerStarted("root/child2"),
		SupervisorStarted("root"),
		WorkerTerminated("root/child2"),
		WorkerTerminated("root/child1"),
		WorkerTerminated("root/child0"),
		SupervisorTerminated("root"),
	}

	// panicCount needs to be double the expected callback calls, given that
	// notifier need to fail at least twice in a 5 second period to report a
	// callback call
	expectedCallbackCalls := int32(3)
	panicCount := expectedCallbackCalls * 2

	// create multiple event notifiers with their expected number of times to be
	// called
	panicEvNotifier, waitTillFailuresDone := newPanicBlockingNotifier(panicCount)
	notifier1, done1 := newBlockingNotifier(int32(len(outEvents)))
	notifier2, done2 := newBlockingNotifier(int32(len(outEvents)))

	// build a callback function for event notifier errors
	callbackDone := make(chan struct{})
	errCount := int32(0)
	errCallback := func(err error) {
		current := atomic.LoadInt32(&errCount)
		if current < expectedCallbackCalls-1 {
			atomic.AddInt32(&errCount, 1)
			return
		}
		close(callbackDone)
	}

	// create the reliable event notifier that broadcasts to notifiers created in
	// step above
	evNotifier, cancelEvNotifier, err := cap.NewReliableNotifier(
		map[string]cap.EventNotifier{
			"notifier1": notifier1,
			"notifier2": notifier2,
			"panic":     panicEvNotifier,
		},
		cap.WithOnReliableNotifierFailure(errCallback),
	)
	defer cancelEvNotifier()

	// assert reliable notifier started without errors
	assert.NoError(t, err)

	// run a supervision tree such that it returns the output events declared above
	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		"root",
		cap.WithNodes(
			WaitDoneWorker("child0"),
			WaitDoneWorker("child1"),
			WaitDoneWorker("child2"),
		),
		[]cap.Opt{},
		[]cap.EventNotifier{
			evNotifier,
		},
		func(EventManager) {},
	)
	assert.NoError(t, err)

	// assert the events from the input supervision tree are the expected ones
	AssertExactMatch(t, events, outEvents)

	// we have to wait for the event notifier to process all the notifications
	waitTillFailuresDone()
	done1()
	done2()
	<-callbackDone
}

// TestReliableNotifierSlowNotifier verifies that the failure callback gets called
// when a given event notifier is panicking
func TestReliableNotifierSlowNotifier(t *testing.T) {

	// these are the output events we are expecting from a test supervision tree
	outEvents := []EventP{
		WorkerStarted("root/child0"),
		SupervisorStarted("root"),
		WorkerTerminated("root/child0"),
		SupervisorTerminated("root"),
	}

	evNotifier0 := slowEvNotifier(12 * time.Second)
	// setup a concurrent safe check for timeout callback calls
	callbacksDone := make(chan struct{})
	// all events but the first one should timeout
	expectedCallbackCalls := int32(len(outEvents) - 1)
	callbackCounter := int32(0)
	timeoutCallback := func(name string) {
		assert.Equal(t, "slow", name)
		current := atomic.LoadInt32(&callbackCounter)
		// buffer size is set to 1, so expect one less timedout callback due to a
		// notification sitting in the channel buffer
		if current < expectedCallbackCalls-2 {
			atomic.AddInt32(&callbackCounter, 1)
			return
		}
		close(callbacksDone)
	}

	notifier1, done1 := newBlockingNotifier(int32(len(outEvents)))

	// create the reliable event notifier that broadcasts to notifiers created in
	// step above
	evNotifier, cancelEvNotifier, err := cap.NewReliableNotifier(
		map[string]cap.EventNotifier{
			"slow":     evNotifier0,
			"notifier": notifier1,
		},
		// use a very small timeout to make the test run fast
		cap.WithNotifierTimeout(100*time.Microsecond),
		cap.WithOnNotifierTimeout(timeoutCallback),
		cap.WithNotifierBufferSize(1),
		cap.WithEntrypointBufferSize(1),
	)

	// assert reliable notifier started without errors
	assert.NoError(t, err)

	// run a supervision tree such that it returns the output events declared above
	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		"root",
		cap.WithNodes(
			WaitDoneWorker("child0"),
		),
		[]cap.Opt{},
		[]cap.EventNotifier{
			evNotifier,
		},
		func(EventManager) {},
	)
	assert.NoError(t, err)

	// assert the events from the input supervision tree are the expected ones
	AssertExactMatch(t, events, outEvents)

	// wait for notifier1 to receive all events
	done1()
	<-callbacksDone

	// this process is slow on unresponsive workers, so doing an async call for that
	go cancelEvNotifier()
}
