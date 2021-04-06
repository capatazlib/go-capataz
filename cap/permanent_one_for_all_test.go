package cap_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestPermanentOneForAllSingleCompleteWorker(t *testing.T) {
	parentName := "root"
	child1, completeWorker1 := CompleteOnSignalWorker(
		3,
		"child1",
		cap.WithRestart(cap.Permanent),
	)
	child2 := WaitDoneWorker("child2")

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(child1, child2),
		[]cap.Opt{cap.WithStrategy(cap.OneForAll)},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has completed
			// at least once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1

			completeWorker1()
			evIt.SkipTill(WorkerStarted("root/child2"))
			// ^^^ Wait till 1st restart

			completeWorker1()
			evIt.SkipTill(WorkerStarted("root/child2"))
			// ^^^ Wait till 2nd restart

			completeWorker1()
			evIt.SkipTill(WorkerStarted("root/child2"))
			// ^^^ Wait till 3rd restart

			completeWorker1()
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			SupervisorStarted("root"),
			// ^^^ 1) completeWorker1 starts executing here

			WorkerCompleted("root/child1"),
			WorkerTerminated("root/child2"),
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			// ^^^ 1st restart

			WorkerCompleted("root/child1"),
			WorkerTerminated("root/child2"),
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			// ^^^ 2nd restart

			WorkerCompleted("root/child1"),
			WorkerTerminated("root/child2"),
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			// ^^^ 3rd restart

			WorkerTerminated("root/child2"),
			WorkerTerminated("root/child1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForAllSingleFailingWorkerRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1 := WaitDoneWorker("child1")
	child2, failWorker2 := FailOnSignalWorker(
		1, "child2", cap.WithRestart(cap.Permanent),
	)
	child3 := WaitDoneWorker("child3")

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(child1, child2, child3),
		[]cap.Opt{cap.WithStrategy(cap.OneForAll)},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))

			// 2) Start the failing behavior of child2
			failWorker2(true /* done */)
			evIt.SkipTill(WorkerFailed("root/child2"))

			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/child3"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			WorkerStarted("root/child3"),
			SupervisorStarted("root"),
			// ^^^ 1) failWorker2 starts executing here

			WorkerFailed("root/child2"),
			WorkerTerminated("root/child3"),
			WorkerTerminated("root/child1"),
			// ^^^ 2) And then we see a new (re)start of it

			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			WorkerStarted("root/child3"),
			// ^^^ 3) After 1st (re)start we stop

			WorkerTerminated("root/child3"),
			WorkerTerminated("root/child2"),
			WorkerTerminated("root/child1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForAllNestedFailingWorkerRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1 := WaitDoneWorker("child1")
	child2, failWorker2 := FailOnSignalWorker(1, "child2", cap.WithRestart(cap.Permanent))
	child3 := WaitDoneWorker("child3")
	tree1 := cap.NewSupervisorSpec(
		"subtree1",
		cap.WithNodes(child1, child2, child3),
		cap.WithStrategy(cap.OneForAll),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(cap.Subtree(tree1)),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at
			// least once
			evIt := em.Iterator()

			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))

			// 2) Start the failing behavior of child2
			failWorker2(true /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child2"))

			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/subtree1/child3"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ 1) Wait till root starts

			WorkerFailed("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child1"),
			// ^^^ 2) We see the failWorker2 causing the error

			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			// ^^^ 3) After 1st (re)start we stop

			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForAllSingleFailingWorkerReachThreshold(t *testing.T) {
	parentName := "root"
	child1 := WaitDoneWorker("child1")
	child2, failWorker2 := FailOnSignalWorker(
		1,
		"child2",
		cap.WithRestart(cap.Permanent),
	)
	child3, failWorker3 := FailOnSignalWorker(
		2,
		"child3",
		cap.WithRestart(cap.Permanent),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(child1, child2, child3),
		[]cap.Opt{
			cap.WithRestartTolerance(2, 10*time.Second),
			cap.WithStrategy(cap.OneForAll),
		},
		func(em EventManager) {
			evIt := em.Iterator()

			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			// (1)
			failWorker3(false /* done */)
			evIt.SkipTill(WorkerFailed("root/child3"))
			evIt.SkipTill(WorkerStarted("root/child3"))
			// ^^^ Wait till first restart

			// (2)
			failWorker2(true /* done */)
			evIt.SkipTill(WorkerFailed("root/child2"))
			evIt.SkipTill(WorkerStarted("root/child3"))
			// ^^^ Wait till second restart

			// (3)
			failWorker3(true /* done */)
			evIt.SkipTill(WorkerFailed("root/child3"))
			evIt.SkipTill(WorkerTerminated("root/child1"))
			// ^^^ Wait till third failure
		},
	)

	// This should return an error given there is no other supervisor that will
	// rescue us when error threshold reached in a child.
	assert.Error(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			WorkerStarted("root/child3"),
			SupervisorStarted("root"),
			// ^^^ failWorker3 excutes here (1)

			WorkerFailed("root/child3"),
			WorkerTerminated("root/child2"),
			WorkerTerminated("root/child1"),
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			WorkerStarted("root/child3"),
			// ^^^ first restart

			WorkerFailed("root/child2"),
			WorkerTerminated("root/child3"),
			WorkerTerminated("root/child1"),
			WorkerStarted("root/child1"),
			WorkerStarted("root/child2"),
			WorkerStarted("root/child3"),
			// ^^^ failWorker2 executes here (2)

			// 3rd err
			WorkerFailed("root/child3"),
			WorkerTerminated("root/child2"),
			WorkerTerminated("root/child1"),
			// failWorker3 executes here (3)
			// ^^^ Error that indicates treshold has been met

			SupervisorFailed("root"),
			// ^^^ Finish with SupervisorFailed because no parent supervisor
			// will recover it
		},
	)
}

func TestPermanentOneForAllNestedFailingWorkerReachThreshold(t *testing.T) {

	// remember, error accumulation happens at the supervisor level, not the
	// worker

	parentName := "root"
	child1 := WaitDoneWorker("child1")
	child2, failWorker2 := FailOnSignalWorker(
		2, // 2 errors on this worker, 1 on the other worker, 3 total, 2 tolerance
		"child2",
		cap.WithRestart(cap.Permanent),
	)
	child3, failWorker3 := FailOnSignalWorker(
		1, // 1 error on this worker, 2 on the other worker, 3 total, 2 tolerance
		"child3",
		cap.WithRestart(cap.Permanent),
	)

	tree1 := cap.NewSupervisorSpec(
		"subtree1",
		cap.WithNodes(child1, child2, child3),
		cap.WithStrategy(cap.OneForAll),
		cap.WithRestartTolerance(2, 10*time.Second),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(cap.Subtree(tree1)),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at
			// least once
			evIt := em.Iterator()
			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			// (1)
			failWorker2(false /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child2"))
			evIt.SkipTill(WorkerStarted("root/subtree1/child3"))
			// ^^^ Wait till first restart

			// (2)
			failWorker3(false /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child3"))
			evIt.SkipTill(WorkerStarted("root/subtree1/child3"))
			// ^^^ Wait till second restart

			// (3)
			failWorker2(true /* done */) // 3 failures, tolerance exceeded
			evIt.SkipTill(WorkerFailed("root/subtree1/child2"))
			evIt.SkipTill(WorkerTerminated("root/subtree1/child1"))
			// ^^^ Wait till worker failure

			evIt.SkipTill(SupervisorFailed("root/subtree1"))
			// ^^^ Wait till supervisor failure (no more WorkerStarted)
			evIt.SkipTill(SupervisorStarted("root/subtree1"))
			// ^^^ Wait till supervisor restarted
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ Wait till root starts

			WorkerFailed("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child1"),
			// ^^^ We see failWorker2 causing an error (1)
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			// ^^^ Wait failWorker2 restarts (1st restart)

			WorkerFailed("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			// ^^^ Wee see failWorker3 causing the error (2)
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			// ^^^ Wait failWorker3 restarts (2nd restart)

			WorkerFailed("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child1"),
			// ^^^ We see failWorker2 causing an error (3); threshold has been
			// exceeded

			SupervisorFailed("root/subtree1"),
			// ^^^ Supervisor child surpassed error

			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			// ^^^ IMPORTANT: Restarted Supervisor signals restart of child
			// first
			SupervisorStarted("root/subtree1"),
			// ^^^ Supervisor restarted again

			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForAllNestedFailingWorkerErrorCountResets(t *testing.T) {
	parentName := "root"
	child1 := WaitDoneWorker("child1")
	child2, failWorker2 := FailOnSignalWorker(
		1, /* 1 error allowed */
		"child2",
		cap.WithRestart(cap.Permanent),
	)
	child3, failWorker3 := FailOnSignalWorker(
		1, /* 1 error allowed */
		"child3",
		cap.WithRestart(cap.Permanent),
	)
	tree1 := cap.NewSupervisorSpec(
		"subtree1",
		cap.WithNodes(child1, child2, child3),
		cap.WithStrategy(cap.OneForAll),
		cap.WithRestartTolerance(1, 100*time.Microsecond),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(cap.Subtree(tree1)),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			// (1)
			failWorker3(true /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child3"))
			evIt.SkipTill(WorkerStarted("root/subtree1/child3"))
			// ^^^ Wait till first restart

			// (2)
			// Waiting 3 times more than tolerance window, ideally, we have
			// a way to not rely on time.Sleep, but I can't think of a clever
			// way (like virtual time) yet
			time.Sleep(300 * time.Microsecond)
			failWorker2(true /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child2"))
			evIt.SkipTill(WorkerStarted("root/subtree1/child3"))
			// ^^^ Wait till second restart
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ Wait till root starts

			WorkerFailed("root/subtree1/child3"),
			// ^^^ We see failWorker3 causing an error (1)
			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			// ^^^ Wait failWorker3 restarts

			// 2nd err -- even though we only tolerate one error, the second
			// error happens after the 100 microseconds window
			WorkerFailed("root/subtree1/child2"),
			// ^^^ Wait failWorker2 causing an error (2)
			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			WorkerStarted("root/subtree1/child3"),
			// ^^^ Wait failWorker2 restarts

			WorkerTerminated("root/subtree1/child3"),
			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForAllSiblingTerminationFailOnRestart(t *testing.T) {
	parentName := "root"
	child1, failWorker1 := FailOnSignalWorker(
		3, // 3 errors, 2 tolerance
		"child1",
		cap.WithRestart(cap.Permanent),
	)
	child2 := FailTerminationWorker("child2", errors.New("child2 termination fail"))

	tree1 := cap.NewSupervisorSpec(
		"subtree1",
		cap.WithNodes(child1, child2),
		cap.WithStrategy(cap.OneForAll),
		cap.WithRestartTolerance(2, 10*time.Second),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(cap.Subtree(tree1)),
		[]cap.Opt{},
		func(em EventManager) {
			evIt := em.Iterator()
			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child1"))
			evIt.SkipTill(WorkerStarted("root/subtree1/child2"))
			// ^^^ Wait till first restart

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerFailed("root/subtree1/child1"))
			evIt.SkipTill(WorkerStarted("root/subtree1/child2"))
			// ^^^ Wait till second restart

			failWorker1(true /* done */) // 3 failures
			evIt.SkipTill(SupervisorFailed("root/subtree1"))
			// ^^^ Wait till supervisor failure

			evIt.SkipTill(SupervisorStarted("root/subtree1"))
			// ^^^ Wait till supervisor restarted
		},
	)

	assert.Error(t, err)

	// get supervisor restart error to assert error message properties
	var restartEv *cap.Event = nil
	for _, ev := range events {
		if cap.EIsSupervisorRestartError(ev) {
			restartEv = &ev
			break
		}
	}

	explanation := cap.ExplainError(restartEv.Err())
	assert.Equal(
		t,
		"supervisor 'root/subtree1' crashed due to restart tolerance surpassed.\n\tworker node 'root/subtree1/child1' was restarted more than 2 times in a 10s window.\n\tthe original error reported was:\n\t\t> Failing child (1 out of 3)\n\tthe last error reported was:\n\t\t> Failing child (3 out of 3)\nalso, some siblings failed to terminate while restarting\n\tworker node 'root/subtree1/child2' failed to terminate\n\t\t> child2 termination fail",
		explanation,
	)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ Wait till root starts

			// 1st err
			WorkerFailed("root/subtree1/child1"), // runtime error
			WorkerFailed("root/subtree1/child2"), // termination error
			// ^^^ We see failWorker1 causing the error
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			// ^^^ Wait failWorker1 restarts

			// 2nd err
			WorkerFailed("root/subtree1/child1"), // runtime error
			WorkerFailed("root/subtree1/child2"), // termination error
			// ^^^ After 1st (re)start we stop
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			// ^^^ Wait failWorker1 restarts (2nd)

			// 3rd err
			WorkerFailed("root/subtree1/child1"), //runtime error
			WorkerFailed("root/subtree1/child2"), // termination error
			// ^^^ Error that indicates treshold has been met

			SupervisorFailed("root/subtree1"),
			// ^^^ Supervisor child surpassed error

			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			SupervisorStarted("root/subtree1"),
			// ^^^ Supervisor restarted again

			WorkerFailed("root/subtree1/child2"), // termination error
			WorkerTerminated("root/subtree1/child1"),
			SupervisorFailed("root/subtree1"),
			SupervisorFailed("root"),
		},
	)
}
