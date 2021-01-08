package cap_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions below, check stest/README.md
//

import (
	"context"
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
			// The subtree-spawner starts first, as it is the initial logic
			// of the DynSubTree worker
			SupervisorStarted("root/one/subtree-spawner"),
			WorkerStarted("root/one/subtree-spawner/uno"),
			// Every spawn call for the subtree-spawner blocks until it is received
			WorkerStarted("root/one"),
			SupervisorStarted("root"),
			WorkerTerminated("root/one/subtree-spawner/uno"),
			SupervisorTerminated("root/one/subtree-spawner"),
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
			SupervisorStarted("root/one/subtree-spawner"),
			// then the dyn supervisor children
			WorkerStarted("root/one/subtree-spawner/uno"),
			WorkerStarted("root/one/subtree-spawner/dos"),
			WorkerStarted("root/one/subtree-spawner/tres"),
			// then dyn subtree worker
			WorkerStarted("root/one"),
			// finally the root supervisor
			SupervisorStarted("root"),

			// *signal of error starts here*

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/one/subtree-spawner/tres"),
			WorkerTerminated("root/one/subtree-spawner/dos"),
			WorkerTerminated("root/one/subtree-spawner/uno"),
			SupervisorTerminated("root/one/subtree-spawner"),

			// then sub tree worker terminated
			WorkerFailed("root/one"),

			// * restart starts here *
			SupervisorStarted("root/one/subtree-spawner"),
			WorkerStarted("root/one/subtree-spawner/uno"),
			WorkerStarted("root/one/subtree-spawner/dos"),
			WorkerStarted("root/one/subtree-spawner/tres"),
			WorkerStarted("root/one"),

			// After 1st (re)start we stop
			WorkerTerminated("root/one/subtree-spawner/tres"),
			WorkerTerminated("root/one/subtree-spawner/dos"),
			WorkerTerminated("root/one/subtree-spawner/uno"),
			SupervisorTerminated("root/one/subtree-spawner"),
			WorkerTerminated("root/one"),
			SupervisorTerminated("root"),
		},
	)
}
