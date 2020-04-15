package capataz_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/capataz"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestTransientOneForOneSingleFailingWorkerRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	worker1, failWorker1 := FailOnSignalWorker(1, "worker1", capataz.WithRestart(capataz.Transient))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		capataz.WithChildren(worker1),
		[]capataz.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of worker1
			failWorker1(true /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/worker1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/worker1"),
			SupervisorStarted("root"),
			// ^^^ 1) failWorker1 starts executing here
			WorkerFailed("root/worker1"),
			// ^^^ 2) And then we see a new (re)start of it
			WorkerStarted("root/worker1"),
			// ^^^ 3) After 1st (re)start we stop
			WorkerTerminated("root/worker1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestTransientOneForOneNestedFailingWorkerRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	worker1, failWorker1 := FailOnSignalWorker(1, "worker1", capataz.WithRestart(capataz.Transient))
	tree1 := capataz.NewSupervisor("subtree1", capataz.WithChildren(worker1))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		capataz.WithChildren(capataz.Subtree(tree1)),
		[]capataz.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of worker1
			failWorker1(true /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/subtree1/worker1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/worker1"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ 1) Wait till root starts
			WorkerFailed("root/subtree1/worker1"),
			// ^^^ 2) We see the failWorker1 causing the error
			WorkerStarted("root/subtree1/worker1"),
			// ^^^ 3) After 1st (re)start we stop
			WorkerTerminated("root/subtree1/worker1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestTransientOneForOneSingleCompleteWorker(t *testing.T) {
	parentName := "root"
	// Fail only one time
	worker1, completeWrorker1 := CompleteOnSignalWorker(1, "worker1", capataz.WithRestart(capataz.Transient))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		capataz.WithChildren(worker1),
		[]capataz.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the complete behavior of worker1
			completeWrorker1()
			// 3) Wait till first restart
			evIt.SkipTill(WorkerCompleted("root/worker1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/worker1"),
			SupervisorStarted("root"),
			// ^^^ completeWrorker1 starts executing here
			WorkerCompleted("root/worker1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestTransientOneForOneNestedCompleteWorker(t *testing.T) {
	parentName := "root"
	// Fail only one time
	worker1, completeWrorker1 := CompleteOnSignalWorker(1, "worker1", capataz.WithRestart(capataz.Transient))
	tree1 := capataz.NewSupervisor("subtree1", capataz.WithChildren(worker1))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		capataz.WithChildren(capataz.Subtree(tree1)),
		[]capataz.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of worker1
			completeWrorker1()
			// 3) Wait till first restart
			evIt.SkipTill(WorkerCompleted("root/subtree1/worker1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/worker1"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ 1) Wait till root starts
			WorkerCompleted("root/subtree1/worker1"),
			// ^^^ 2) We see the completeWrorker1 causing the completion
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}

func TestTransientOneForOneSingleFailingWorkerReachThreshold(t *testing.T) {
	parentName := "root"
	worker1, failWorker1 := FailOnSignalWorker(
		3,
		"worker1",
		capataz.WithRestart(capataz.Transient),
		capataz.WithTolerance(2, 10*time.Second),
	)
	worker2 := WaitDoneWorker("worker2")

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		capataz.WithChildren(worker1, worker2),
		[]capataz.Opt{},
		func(em EventManager) {
			evIt := em.Iterator()

			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/worker1"))
			// ^^^ Wait till first restart

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/worker1"))
			// ^^^ Wait till second restart

			failWorker1(true /* done */)
			evIt.SkipTill(WorkerFailed("root/worker1"))
			// ^^^ Wait till third failure
		},
	)

	// This should return an error given there is no other supervisor that will
	// rescue us when error threshold reached in a child.
	assert.Error(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/worker1"),
			WorkerStarted("root/worker2"),
			SupervisorStarted("root"),
			// ^^^ failWorker1 starts executing here

			WorkerFailed("root/worker1"),
			WorkerStarted("root/worker1"),
			// ^^^ first restart

			WorkerFailed("root/worker1"),
			WorkerStarted("root/worker1"),
			// ^^^ second restart

			// 3rd err
			WorkerFailed("root/worker1"),
			// ^^^ Error that indicates treshold has been met

			WorkerTerminated("root/worker2"),
			// ^^^ Terminating all other workers because supervisor failed

			SupervisorFailed("root"),
			// ^^^ Finish with SupervisorFailed because no parent supervisor will
			// recover it
		},
	)
}

func TestTransientOneForOneNestedFailingWorkerReachThreshold(t *testing.T) {
	parentName := "root"
	worker1, failWorker1 := FailOnSignalWorker(
		3, // 3 errors, 2 tolerance
		"worker1",
		capataz.WithRestart(capataz.Transient),
		capataz.WithTolerance(2, 10*time.Second),
	)
	worker2 := WaitDoneWorker("worker2")
	tree1 := capataz.NewSupervisor("subtree1", capataz.WithChildren(worker1, worker2))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		capataz.WithChildren(capataz.Subtree(tree1)),
		[]capataz.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			evIt.SkipTill(SupervisorStarted("root"))
			// ^^^ Wait till all the tree is up

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/worker1"))
			// ^^^ Wait till first restart

			failWorker1(false /* done */)
			evIt.SkipTill(WorkerStarted("root/subtree1/worker1"))
			// ^^^ Wait till second restart

			failWorker1(true /* done */) // 3 failures
			evIt.SkipTill(WorkerFailed("root/subtree1/worker1"))
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
			WorkerStarted("root/subtree1/worker1"),
			WorkerStarted("root/subtree1/worker2"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ Wait till root starts

			// 1st err
			WorkerFailed("root/subtree1/worker1"),
			// ^^^ We see failWorker1 causing the error
			WorkerStarted("root/subtree1/worker1"),
			// ^^^ Wait failWorker1 restarts

			// 2nd err
			WorkerFailed("root/subtree1/worker1"),
			// ^^^ After 1st (re)start we stop
			WorkerStarted("root/subtree1/worker1"),
			// ^^^ Wait failWorker1 restarts (2nd)

			// 3rd err
			WorkerFailed("root/subtree1/worker1"),
			// ^^^ Error that indicates treshold has been met

			WorkerTerminated("root/subtree1/worker2"),
			// ^^^ IMPORTANT: Supervisor failure stops other children
			SupervisorFailed("root/subtree1"),
			// ^^^ Supervisor child surpassed error

			WorkerStarted("root/subtree1/worker1"),
			WorkerStarted("root/subtree1/worker2"),
			// ^^^ IMPORTANT: Restarted Supervisor signals restart of child first
			SupervisorStarted("root/subtree1"),
			// ^^^ Supervisor restarted again

			WorkerTerminated("root/subtree1/worker2"),
			WorkerTerminated("root/subtree1/worker1"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}
