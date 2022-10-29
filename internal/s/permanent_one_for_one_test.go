package s_test

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

func TestPermanentOneForOneSingleCompleteWorker(t *testing.T) {
	parentName := "root"
	child1, completeWorker1 := CompleteOnSignalWorker(
		3,
		"child1",
		cap.WithRestart(cap.Permanent),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(child1),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1

			completeWorker1()
			evIt.SkipTill(WorkerStarted("root/child1"))
			// ^^^ Wait till 1st restart

			completeWorker1()
			evIt.SkipTill(WorkerStarted("root/child1"))
			// ^^^ Wait till 2nd restart

			completeWorker1()
			evIt.SkipTill(WorkerStarted("root/child1"))
			// ^^^ Wait till 3rd restart

			completeWorker1()
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			SupervisorStarted("root"),
			// ^^^ 1) completeWorker1 starts executing here
			WorkerCompleted("root/child1"),
			WorkerStarted("root/child1"),
			// ^^^ 1st restart
			WorkerCompleted("root/child1"),
			WorkerStarted("root/child1"),
			// ^^^ 2nd restart
			WorkerCompleted("root/child1"),
			WorkerStarted("root/child1"),
			// ^^^ 3rd restart
			WorkerTerminated("root/child1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForOneSinglefailingWorkerRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, failWorker1 := FailOnSignalWorker(1, "child1", cap.WithRestart(cap.Permanent))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(child1),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failWorker1(true /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			SupervisorStarted("root"),
			// ^^^ 1) failWorker1 starts executing here
			WorkerFailed("root/child1"),
			// ^^^ 2) And then we see a new (re)start of it
			WorkerStarted("root/child1"),
			// ^^^ 3) After 1st (re)start we stop
			WorkerTerminated("root/child1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForOneNestedFailingWorkerRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, failWorker1 := FailOnSignalWorker(1, "child1", cap.WithRestart(cap.Permanent))
	tree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(child1))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(cap.Subtree(tree1)),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failWorker1(true /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ 1) Wait till root starts
			WorkerFailed("root/subtree1/child1"),
			// ^^^ 2) We see the failWorker1 causing the error
			WorkerStarted("root/subtree1/child1"),
			// ^^^ 3) After 1st (re)start we stop
			WorkerTerminated("root/subtree1/child1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForOneSingleFailingWorkerReachThreshold(t *testing.T) {
	parentName := "root"
	child1, failWorker1 := FailOnSignalWorker(
		3,
		"child1",
		cap.WithRestart(cap.Permanent),
	)
	child2 := WaitDoneWorker("child2")

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		cap.WithNodes(child1, child2),
		[]cap.Opt{
			cap.WithRestartTolerance(2, 10*time.Second),
		},
		func(em EventManager) {
			evIt := em.Iterator()

			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/child1"))
			// ^^^ Wait till first restart

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/child1"))
			// ^^^ Wait till second restart

			failWorker1(true /* done */)
			evIt.SkipTill(WorkerFailed("root/child1"))
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
			SupervisorStarted("root"),
			// ^^^ failWorker1 starts executing here

			WorkerFailed("root/child1"),
			WorkerStarted("root/child1"),
			// ^^^ first restart

			WorkerFailed("root/child1"),
			WorkerStarted("root/child1"),
			// ^^^ second restart

			// 3rd err
			WorkerFailed("root/child1"),
			// ^^^ Error that indicates treshold has been met

			WorkerTerminated("root/child2"),
			// ^^^ Terminating all other workers because supervisor failed

			SupervisorFailed("root"),
			// ^^^ Finish with SupervisorFailed because no parent supervisor will
			// recover it
		},
	)
}

func TestPermanentOneForOneNestedFailingWorkerReachThreshold(t *testing.T) {
	parentName := "root"
	child1, failWorker1 := FailOnSignalWorker(
		3, // 3 errors, 2 tolerance
		"child1",
		cap.WithRestart(cap.Permanent),
	)
	child2 := WaitDoneWorker("child2")

	tree1 := cap.NewSupervisorSpec(
		"subtree1",
		cap.WithNodes(child1, child2),
		cap.WithRestartTolerance(2, 10*time.Second),
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

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// ^^^ Wait till first restart

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// ^^^ Wait till second restart

			failWorker1(true /* done */) // 3 failures
			evIt.SkipTill(WorkerFailed("root/subtree1/child1"))
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
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ Wait till root starts

			// 1st err
			WorkerFailed("root/subtree1/child1"),
			// ^^^ We see failWorker1 causing the error
			WorkerStarted("root/subtree1/child1"),
			// ^^^ Wait failWorker1 restarts

			// 2nd err
			WorkerFailed("root/subtree1/child1"),
			// ^^^ After 1st (re)start we stop
			WorkerStarted("root/subtree1/child1"),
			// ^^^ Wait failWorker1 restarts (2nd)

			// 3rd err
			WorkerFailed("root/subtree1/child1"),
			// ^^^ Error that indicates treshold has been met

			WorkerTerminated("root/subtree1/child2"),
			// ^^^ IMPORTANT: Supervisor failure stops other children
			SupervisorFailed("root/subtree1"),
			// ^^^ Supervisor child surpassed error

			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			// ^^^ IMPORTANT: Restarted Supervisor signals restart of child first
			SupervisorStarted("root/subtree1"),
			// ^^^ Supervisor restarted again

			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForOneNestedFailingWorkerErrorCountResets(t *testing.T) {
	parentName := "root"
	child1, failWorker1 := FailOnSignalWorker(
		2, // 3 errors, 2 tolerance
		"child1",
		cap.WithRestart(cap.Permanent),
	)
	child2 := WaitDoneWorker("child2")
	tree1 := cap.NewSupervisorSpec(
		"subtree1",
		cap.WithNodes(child1, child2),
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

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// ^^^ Wait till first restart

			// Waiting 3 times more than tolerance window
			time.Sleep(300 * time.Microsecond)
			failWorker1(true /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// ^^^ Wait till second restart
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ Wait till root starts

			// 1st err
			WorkerFailed("root/subtree1/child1"),
			// ^^^ We see failWorker1 causing the error
			WorkerStarted("root/subtree1/child1"),
			// ^^^ Wait failWorker1 restarts

			// 2nd err -- even though we only tolerate one error, the second error happens
			// after the 100 microseconds window, and it restarts
			WorkerFailed("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child1"),
			// ^^^ Wait failWorker1 restarts (2nd)

			WorkerTerminated("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestPermanentOneForOneSiblingTerminationFailOnRestart(t *testing.T) {
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
		cap.WithRestartTolerance(2, 10*time.Second),
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

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// ^^^ Wait till first restart

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// ^^^ Wait till second restart

			failWorker1(true /* done */) // 3 failures
			evIt.SkipTill(WorkerFailed("root/subtree1/child1"))
			// ^^^ Wait till worker failure

			evIt.SkipTill(SupervisorFailed("root/subtree1"))
			// ^^^ Wait till supervisor failure (no more WorkerStarted)
			evIt.SkipTill(SupervisorStarted("root/subtree1"))
			// ^^^ Wait till supervisor restarted
		},
	)

	assert.Error(t, err)

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
		"supervisor 'root/subtree1' crashed due to restart tolerance surpassed.\n\tworker node 'root/subtree1/child1' was restarted more than 2 times in a 10s window.\n\tthe original error reported was:\n\t\t> failing child (1 out of 3)\n\tthe last error reported was:\n\t\t> failing child (3 out of 3)\nalso, some siblings failed to terminate while restarting\n\tworker node 'root/subtree1/child2' failed to terminate\n\t\t> child2 termination fail",
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
			WorkerFailed("root/subtree1/child1"),
			// ^^^ We see failWorker1 causing the error
			WorkerStarted("root/subtree1/child1"),
			// ^^^ Wait failWorker1 restarts

			// 2nd err
			WorkerFailed("root/subtree1/child1"),
			// ^^^ After 1st (re)start we stop
			WorkerStarted("root/subtree1/child1"),
			// ^^^ Wait failWorker1 restarts (2nd)

			// 3rd err
			WorkerFailed("root/subtree1/child1"),
			// ^^^ Error that indicates treshold has been met

			WorkerFailed("root/subtree1/child2"),
			// ^^^ IMPORTANT: Supervisor failure stops other children
			SupervisorFailed("root/subtree1"),
			// ^^^ Supervisor child surpassed error

			WorkerStarted("root/subtree1/child1"),
			WorkerStarted("root/subtree1/child2"),
			// ^^^ IMPORTANT: Restarted Supervisor signals restart of child first
			SupervisorStarted("root/subtree1"),
			// ^^^ Supervisor restarted again

			WorkerFailed("root/subtree1/child2"),
			WorkerTerminated("root/subtree1/child1"),
			SupervisorFailed("root/subtree1"),
			SupervisorFailed("root"),
		},
	)
}
