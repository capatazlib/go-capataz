package s_test

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
	subtree := WaitDoneDynSubtree(
		"one", []cap.Opt{}, []cap.WorkerOpt{}, WaitDoneWorker("uno"),
	)

	events, errs := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(subtree),
		[]cap.Opt{},
		func(EventManager) {},
	)
	assert.Empty(t, errs)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree spawner sub-tree starts always first
			SupervisorStarted("root/one/spawner"),
			WorkerStarted("root/one/spawner/uno"),
			// Then the dyn-subtree worker
			WorkerStarted("root/one/worker"),
			// Then dyn-subtree as a whole is started
			SupervisorStarted("root/one"),
			SupervisorStarted("root"),
			// stop occurs in reverse order of start
			WorkerTerminated("root/one/worker"),
			WorkerTerminated("root/one/spawner/uno"),
			SupervisorTerminated("root/one/spawner"),
			SupervisorTerminated("root/one"),
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
			evIt.SkipTill(WorkerStarted("root/one/worker"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree spawner sub-tree starts always first
			SupervisorStarted("root/one/spawner"),
			// then, because of the FailOnSignalDynSubtree implementation the dyn
			// supervisor children start first
			WorkerStarted("root/one/spawner/uno"),
			WorkerStarted("root/one/spawner/dos"),
			WorkerStarted("root/one/spawner/tres"),
			// then dyn-subtree worker
			WorkerStarted("root/one/worker"),
			// the the dyn-subtree as a whole
			SupervisorStarted("root/one"),
			// finally the root supervisor
			SupervisorStarted("root"),

			// signal of error starts here
			WorkerFailed("root/one/worker"),
			// the dyn sub-tree has a wired strategy of OneForAll, so the dyn-subtree
			// spawner also gets terminated on error
			WorkerTerminated("root/one/spawner/tres"),
			WorkerTerminated("root/one/spawner/dos"),
			WorkerTerminated("root/one/spawner/uno"),
			SupervisorTerminated("root/one/spawner"),

			// the spawner sub-tree gets started first after a restart
			SupervisorStarted("root/one/spawner"),
			WorkerStarted("root/one/spawner/uno"),
			WorkerStarted("root/one/spawner/dos"),
			WorkerStarted("root/one/spawner/tres"),
			WorkerStarted("root/one/worker"),

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/one/worker"),
			WorkerTerminated("root/one/spawner/tres"),
			WorkerTerminated("root/one/spawner/dos"),
			WorkerTerminated("root/one/spawner/uno"),
			SupervisorTerminated("root/one/spawner"),
			SupervisorTerminated("root/one"),
			SupervisorTerminated("root"),
		},
	)
}

func TestDynSubtreeFailedSpawnedWorker(t *testing.T) {
	worker1, failWorker1 := FailOnSignalWorker(2, "child1")
	subtree := WaitDoneDynSubtree(
		"one",
		[]cap.Opt{},
		[]cap.WorkerOpt{},
		WaitDoneWorker("child0"),
		worker1,
		WaitDoneWorker("child2"),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(subtree),
		[]cap.Opt{},
		func(em EventManager) {
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failWorker1(false /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/one/spawner/child1"))
			// 4) fail again to surpass default error threshold (2 in 5 seconds)
			failWorker1(true /* done */)
			// one_for_all strategy makes the whole dyn-subtree starts
			// 5) wait for the whole dyn-subtree one gets started
			evIt.SkipTill(WorkerStarted("root/one/worker"))
		},
	)

	// assert that error contains all information required to assess what went wrong
	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree spawner sub-tree starts always first
			SupervisorStarted("root/one/spawner"),
			WorkerStarted("root/one/spawner/child0"),
			WorkerStarted("root/one/spawner/child1"),
			WorkerStarted("root/one/spawner/child2"),
			WorkerStarted("root/one/worker"),
			SupervisorStarted("root/one"),
			SupervisorStarted("root"),

			// failure one starts here
			WorkerFailed("root/one/spawner/child1"),
			WorkerStarted("root/one/spawner/child1"),
			// failure two starts here, one_for_all got triggered here
			WorkerFailed("root/one/spawner/child1"),
			WorkerTerminated("root/one/spawner/child2"),
			WorkerTerminated("root/one/spawner/child0"),
			SupervisorFailed("root/one/spawner"),
			WorkerTerminated("root/one/worker"),

			// one_for_all start here
			//
			// remember, supervisor starts first because it's children are started
			// dynamically by the WaitDoneDynSubtree implementation
			SupervisorStarted("root/one/spawner"),
			WorkerStarted("root/one/spawner/child0"),
			WorkerStarted("root/one/spawner/child1"),
			WorkerStarted("root/one/spawner/child2"),
			WorkerStarted("root/one/worker"),

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/one/worker"),
			WorkerTerminated("root/one/spawner/child2"),
			WorkerTerminated("root/one/spawner/child1"),
			WorkerTerminated("root/one/spawner/child0"),
			SupervisorTerminated("root/one/spawner"),
			SupervisorTerminated("root/one"),
			SupervisorTerminated("root"),
		},
	)
}

func TestDynSubtreeFailedTermination(t *testing.T) {
	subtree := WaitDoneDynSubtree(
		"one",
		[]cap.Opt{},
		[]cap.WorkerOpt{},
		WaitDoneWorker("child0"),
		FailTerminationWorker("child1", fmt.Errorf("child1 failed")),
		WaitDoneWorker("child2"),
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(subtree),
		[]cap.Opt{},
		func(EventManager) {},
	)

	// assert that error contains all information required to assess what went wrong
	assert.Error(t, err)
	assert.Equal(t, "supervisor terminated with failures", err.Error())

	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				// The dyn-subtree spawner sub-tree starts always first
				SupervisorStarted("root/one/spawner"),
				WorkerStarted("root/one/spawner/child0"),
				WorkerStarted("root/one/spawner/child1"),
				WorkerStarted("root/one/spawner/child2"),
				WorkerStarted("root/one/worker"),
				SupervisorStarted("root/one"),
				SupervisorStarted("root"),

				// dyn subtree terminates in reverse order
				WorkerTerminated("root/one/worker"),
				WorkerTerminated("root/one/spawner/child2"),
				WorkerFailedWith("root/one/spawner/child1", "child1 failed"),
				WorkerTerminated("root/one/spawner/child0"),
				SupervisorFailed("root/one/spawner"),
				SupervisorFailed("root/one"),
				SupervisorFailed("root"),
			})
	})
}

func TestDynSubtreeWorkerStartStopStart(t *testing.T) {
	subtree := cap.NewDynSubtreeWithNotifyStart(
		"subtree",
		func(ctx context.Context, notifyStart cap.NotifyStartFn, spawner cap.Spawner) error {
			_, _ = spawner.Spawn(WaitDoneWorker("child0"))
			cancelWorker, err := spawner.Spawn(WaitDoneWorker("child1"))
			_, _ = spawner.Spawn(WaitDoneWorker("child2"))

			assert.NoError(t, err)
			err = cancelWorker()
			assert.NoError(t, err)

			cancelWorker, err = spawner.Spawn(WaitDoneWorker("child1"))
			assert.NoError(t, err)
			err = cancelWorker()
			assert.NoError(t, err)

			// we use notifyStart to make sure there are no race conditions in the
			// event assertion list bellow
			notifyStart(nil)

			<-ctx.Done()
			return nil
		},
		[]cap.Opt{},
	)

	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(subtree),
		[]cap.Opt{},
		func(EventManager) {},
	)
	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree spawner sub-tree starts always first
			SupervisorStarted("root/subtree/spawner"),
			WorkerStarted("root/subtree/spawner/child0"),
			WorkerStarted("root/subtree/spawner/child1"),
			WorkerStarted("root/subtree/spawner/child2"),
			WorkerTerminated("root/subtree/spawner/child1"),
			WorkerStarted("root/subtree/spawner/child1"),
			WorkerTerminated("root/subtree/spawner/child1"),

			WorkerStarted("root/subtree/worker"),
			SupervisorStarted("root/subtree"),
			SupervisorStarted("root"),

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/subtree/worker"),
			WorkerTerminated("root/subtree/spawner/child2"),
			// child1 is not here because it was shut down already
			WorkerTerminated("root/subtree/spawner/child0"),
			SupervisorTerminated("root/subtree/spawner"),
			SupervisorTerminated("root/subtree"),
			SupervisorTerminated("root"),
		})
}
