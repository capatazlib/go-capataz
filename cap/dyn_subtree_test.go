package cap_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions below, check stest/README.md
//

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestDynSubtreeStartSingleChild(t *testing.T) {
	events, errs := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(
			WaitDoneDynSubtree(
				"one", []cap.Opt{}, []cap.WorkerOpt{}, WaitDoneWorker("uno"),
			),
		),
		[]cap.Opt{},
		func(EventManager) {},
	)
	assert.Empty(t, errs)

	AssertExactMatch(t, events,
		[]EventP{
			// The spawner starts first, as it is the initial logic
			// of the DynSubTree worker
			SupervisorStarted("root/one/spawner"),
			WorkerStarted("root/one/spawner/uno"),
			// Every spawn call for the spawner blocks until it is received
			WorkerStarted("root/one"),
			SupervisorStarted("root"),
			WorkerTerminated("root/one/spawner/uno"),
			SupervisorTerminated("root/one/spawner"),
			WorkerTerminated("root/one"),
			SupervisorTerminated("root"),
		})
}

func TestDynSubtreeFailing(t *testing.T) {
	// Fail only one time
	child1, failSubtree1 := FailOnSignalDynSubtree(
		1,
		"one",
		[]cap.Opt{},
		[]cap.WorkerOpt{},
		WaitDoneWorker("uno"),
		WaitDoneWorker("dos"),
		WaitDoneWorker("tres"),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(child1),
		[]cap.Opt{},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failSubtree1(true /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/one"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// dynamic supervisor will start before the worker that
			// contains it
			SupervisorStarted("root/one/spawner"),
			// then the dyn supervisor children
			WorkerStarted("root/one/spawner/uno"),
			WorkerStarted("root/one/spawner/dos"),
			WorkerStarted("root/one/spawner/tres"),
			// then dyn subtree worker
			WorkerStarted("root/one"),
			// finally the root supervisor
			SupervisorStarted("root"),

			// *signal of error starts here*

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/one/spawner/tres"),
			WorkerTerminated("root/one/spawner/dos"),
			WorkerTerminated("root/one/spawner/uno"),
			SupervisorTerminated("root/one/spawner"),

			// then sub tree worker terminated
			WorkerFailed("root/one"),

			// * restart starts here *
			SupervisorStarted("root/one/spawner"),
			WorkerStarted("root/one/spawner/uno"),
			WorkerStarted("root/one/spawner/dos"),
			WorkerStarted("root/one/spawner/tres"),
			WorkerStarted("root/one"),

			// After 1st (re)start we stop
			WorkerTerminated("root/one/spawner/tres"),
			WorkerTerminated("root/one/spawner/dos"),
			WorkerTerminated("root/one/spawner/uno"),
			SupervisorTerminated("root/one/spawner"),
			WorkerTerminated("root/one"),
			SupervisorTerminated("root"),
		},
	)
}

func TestDynSubtreeFailedTermination(t *testing.T) {
	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(
			WaitDoneDynSubtree(
				"one",
				[]cap.Opt{},
				[]cap.WorkerOpt{},
				WaitDoneWorker("child0"),
				FailTerminationWorker("child1", fmt.Errorf("child1 failed")),
				WaitDoneWorker("child2"),
			),
		),
		[]cap.Opt{},
		func(EventManager) {},
	)

	// assert that error contains all information required to assess what went wrong
	assert.Error(t, err)
	assert.Equal(t, "supervisor terminated with failures", err.Error())

	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				SupervisorStarted("root/one/spawner"),
				WorkerStarted("root/one/spawner/child0"),
				WorkerStarted("root/one/spawner/child1"),
				WorkerStarted("root/one/spawner/child2"),
				WorkerStarted("root/one"),
				SupervisorStarted("root"),
				WorkerTerminated("root/one/spawner/child2"),
				WorkerFailedWith("root/one/spawner/child1", "child1 failed"),
				WorkerTerminated("root/one/spawner/child0"),
				SupervisorFailed("root/one/spawner"),
				WorkerFailed("root/one"),
				SupervisorFailed("root"),
			})
	})
}
