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
		"dyn", []cap.Opt{}, []cap.WorkerOpt{}, WaitDoneWorker("uno"),
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
			// The dyn-subtree subtree sub-tree starts always first
			SupervisorStarted("root/dyn/subtree"),
			WorkerStarted("root/dyn/subtree/uno"),
			// Then the dyn-subtree worker
			WorkerStarted("root/dyn/spawner"),
			// Then dyn-subtree as a whole is started
			SupervisorStarted("root/dyn"),
			SupervisorStarted("root"),
			// stop occurs in reverse order of start
			WorkerTerminated("root/dyn/spawner"),
			WorkerTerminated("root/dyn/subtree/uno"),
			SupervisorTerminated("root/dyn/subtree"),
			SupervisorTerminated("root/dyn"),
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
			evIt.WaitTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failSubtree1(true /* done */)
			// 3) Wait till first restart
			evIt.WaitTill(WorkerStarted("root/one/spawner"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree subtree sub-tree starts always first
			SupervisorStarted("root/one/subtree"),
			// then, because of the FailOnSignalDynSubtree implementation the dyn
			// supervisor children start first
			WorkerStarted("root/one/subtree/uno"),
			WorkerStarted("root/one/subtree/dos"),
			WorkerStarted("root/one/subtree/tres"),
			// then dyn-subtree worker
			WorkerStarted("root/one/spawner"),
			// the the dyn-subtree as a whole
			SupervisorStarted("root/one"),
			// finally the root supervisor
			SupervisorStarted("root"),

			// signal of error starts here
			WorkerFailed("root/one/spawner"),
			// the dyn sub-tree has a wired strategy of OneForAll, so the dyn-subtree
			// subtree also gets terminated on error
			WorkerTerminated("root/one/subtree/tres"),
			WorkerTerminated("root/one/subtree/dos"),
			WorkerTerminated("root/one/subtree/uno"),
			SupervisorTerminated("root/one/subtree"),

			// the subtree sub-tree gets started first after a restart
			SupervisorStarted("root/one/subtree"),
			WorkerStarted("root/one/subtree/uno"),
			WorkerStarted("root/one/subtree/dos"),
			WorkerStarted("root/one/subtree/tres"),
			WorkerStarted("root/one/spawner"),

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/one/spawner"),
			WorkerTerminated("root/one/subtree/tres"),
			WorkerTerminated("root/one/subtree/dos"),
			WorkerTerminated("root/one/subtree/uno"),
			SupervisorTerminated("root/one/subtree"),
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
			evIt.WaitTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failWorker1(false /* done */)
			// 3) Wait till first restart
			evIt.WaitTill(WorkerStarted("root/one/subtree/child1"))
			// 4) fail again to surpass default error threshold (2 in 5 seconds)
			failWorker1(true /* done */)
			// one_for_all strategy makes the whole dyn-subtree starts
			// 5) wait for the whole dyn-subtree one gets started
			evIt.WaitTill(WorkerStarted("root/one/spawner"))
		},
	)

	// assert that error contains all information required to assess what went wrong
	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree subtree sub-tree starts always first
			SupervisorStarted("root/one/subtree"),
			WorkerStarted("root/one/subtree/child0"),
			WorkerStarted("root/one/subtree/child1"),
			WorkerStarted("root/one/subtree/child2"),
			WorkerStarted("root/one/spawner"),
			SupervisorStarted("root/one"),
			SupervisorStarted("root"),

			// failure one starts here
			WorkerFailed("root/one/subtree/child1"),
			WorkerStarted("root/one/subtree/child1"),
			// failure two starts here, one_for_all got triggered here
			WorkerFailed("root/one/subtree/child1"),
			WorkerTerminated("root/one/subtree/child2"),
			WorkerTerminated("root/one/subtree/child0"),
			SupervisorFailed("root/one/subtree"),
			WorkerTerminated("root/one/spawner"),

			// one_for_all start here
			//
			// remember, supervisor starts first because it's children are started
			// dynamically by the WaitDoneDynSubtree implementation
			SupervisorStarted("root/one/subtree"),
			WorkerStarted("root/one/subtree/child0"),
			WorkerStarted("root/one/subtree/child1"),
			WorkerStarted("root/one/subtree/child2"),
			WorkerStarted("root/one/spawner"),

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/one/spawner"),
			WorkerTerminated("root/one/subtree/child2"),
			WorkerTerminated("root/one/subtree/child1"),
			WorkerTerminated("root/one/subtree/child0"),
			SupervisorTerminated("root/one/subtree"),
			SupervisorTerminated("root/one"),
			SupervisorTerminated("root"),
		},
	)
}

func TestDynSubtreeFailedTermination(t *testing.T) {
	subtree := WaitDoneDynSubtree(
		"dyn",
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
				// The dyn-subtree subtree sub-tree starts always first
				SupervisorStarted("root/dyn/subtree"),
				WorkerStarted("root/dyn/subtree/child0"),
				WorkerStarted("root/dyn/subtree/child1"),
				WorkerStarted("root/dyn/subtree/child2"),
				WorkerStarted("root/dyn/spawner"),
				SupervisorStarted("root/dyn"),
				SupervisorStarted("root"),

				// dyn subtree terminates in reverse order
				WorkerTerminated("root/dyn/spawner"),
				WorkerTerminated("root/dyn/subtree/child2"),
				WorkerFailedWith("root/dyn/subtree/child1", "child1 failed"),
				WorkerTerminated("root/dyn/subtree/child0"),
				SupervisorFailed("root/dyn/subtree"),
				SupervisorFailed("root/dyn"),
				SupervisorFailed("root"),
			})
	})
}

func TestDynSubtreeWorkerStartStopStart(t *testing.T) {
	rootStarted := make(chan struct{})
	subtree := cap.NewDynSubtreeWithNotifyStart(
		"dyn",
		func(ctx context.Context, notifyStart cap.NotifyStartFn, subtree cap.Spawner) error {
			_, _ = subtree.Spawn(WaitDoneWorker("child0"))
			cancelWorker, err := subtree.Spawn(WaitDoneWorker("child1"))
			_, _ = subtree.Spawn(WaitDoneWorker("child2"))

			assert.NoError(t, err)
			err = cancelWorker()
			assert.NoError(t, err)

			cancelWorker, err = subtree.Spawn(WaitDoneWorker("child1"))
			assert.NoError(t, err)
			err = cancelWorker()
			assert.NoError(t, err)

			// we use notifyStart to make sure there are no race conditions in the
			// event assertion list bellow
			notifyStart(nil)

			// let us start/stop some workers after the root start to showcase that we
			// can start workers at any time
			<-rootStarted

			cancelWorker, err = subtree.Spawn(WaitDoneWorker("child3"))
			_, _ = subtree.Spawn(WaitDoneWorker("child4"))
			assert.NoError(t, err)
			err = cancelWorker()
			assert.NoError(t, err)

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
		func(EventManager) {
			// signal that root started to the dynamic subtree worker
			close(rootStarted)
		},
	)
	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// The dyn-subtree subtree sub-tree starts always first
			SupervisorStarted("root/dyn/subtree"),
			WorkerStarted("root/dyn/subtree/child0"),
			WorkerStarted("root/dyn/subtree/child1"),
			WorkerStarted("root/dyn/subtree/child2"),
			WorkerTerminated("root/dyn/subtree/child1"),
			WorkerStarted("root/dyn/subtree/child1"),
			WorkerTerminated("root/dyn/subtree/child1"),

			WorkerStarted("root/dyn/spawner"),
			SupervisorStarted("root/dyn"),
			SupervisorStarted("root"),

			// rootStarted signal gets triggered in dyn subtree worker
			WorkerStarted("root/dyn/subtree/child3"),
			WorkerStarted("root/dyn/subtree/child4"),
			WorkerTerminated("root/dyn/subtree/child3"),

			// dyn subtree terminates in reverse order
			WorkerTerminated("root/dyn/spawner"),
			WorkerTerminated("root/dyn/subtree/child4"),
			// child3 is not here because it was shut down already
			WorkerTerminated("root/dyn/subtree/child2"),
			// child1 is not here because it was shut down already
			WorkerTerminated("root/dyn/subtree/child0"),
			SupervisorTerminated("root/dyn/subtree"),
			SupervisorTerminated("root/dyn"),
			SupervisorTerminated("root"),
		})
}
