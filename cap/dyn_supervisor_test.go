package cap_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestDynStartSingleChild(t *testing.T) {
	events, errs := ObserveDynSupervisor(
		context.TODO(),
		"root",
		[]cap.Node{WaitDoneWorker("one")},
		// []cap.Node{},
		[]cap.Opt{},
		func(cap.DynSupervisor, EventManager) {},
	)
	assert.Empty(t, errs)

	AssertExactMatch(t, events,
		[]EventP{
			SupervisorStarted("root"),
			WorkerStarted("root/one"),
			WorkerTerminated("root/one"),
			SupervisorTerminated("root"),
		})
}

// Test a supervision tree with three children start and stop in the default
// order (LeftToRight)
func TestDynStartMutlipleChildrenLeftToRight(t *testing.T) {
	events, errs := ObserveDynSupervisor(
		context.TODO(),
		"root",
		[]cap.Node{
			WaitDoneWorker("child0"),
			WaitDoneWorker("child1"),
			WaitDoneWorker("child2"),
		},
		[]cap.Opt{},
		func(cap.DynSupervisor, EventManager) {},
	)

	assert.Empty(t, errs)
	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				SupervisorStarted("root"),
				WorkerStarted("root/child0"),
				WorkerStarted("root/child1"),
				WorkerStarted("root/child2"),
				WorkerTerminated("root/child2"),
				WorkerTerminated("root/child1"),
				WorkerTerminated("root/child0"),
				SupervisorTerminated("root"),
			})
	})
}

// Test a supervision tree with three children start and stop in the default
// order (LeftToRight)
func TestDynStartMutlipleChildrenRightToLeft(t *testing.T) {
	events, errs := ObserveDynSupervisor(
		context.TODO(),
		"root",
		[]cap.Node{
			WaitDoneWorker("child0"),
			WaitDoneWorker("child1"),
			WaitDoneWorker("child2"),
		},
		[]cap.Opt{
			cap.WithOrder(cap.RightToLeft),
		},
		func(cap.DynSupervisor, EventManager) {},
	)

	assert.Empty(t, errs)

	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				SupervisorStarted("root"),
				WorkerStarted("root/child0"),
				WorkerStarted("root/child1"),
				WorkerStarted("root/child2"),
				WorkerTerminated("root/child0"),
				WorkerTerminated("root/child1"),
				WorkerTerminated("root/child2"),
				SupervisorTerminated("root"),
			})
	})
}

func TestDynStartFailedChild(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []cap.Node{
		WaitDoneWorker("child0"),
		WaitDoneWorker("child1"),
		WaitDoneWorker("child2"),
		// NOTE: FailStartWorker here
		FailStartWorker("child3"),
		WaitDoneWorker("child4"),
	}

	b0 := cap.NewSupervisorSpec(b0n, cap.WithNodes(cs[0], cs[1]))
	b1 := cap.NewSupervisorSpec(b1n, cap.WithNodes(cs[2], cs[3], cs[4]))

	events, errs := ObserveDynSupervisor(
		context.TODO(),
		parentName,
		[]cap.Node{
			cap.Subtree(b0),
			cap.Subtree(b1),
		},
		[]cap.Opt{},
		func(cap.DynSupervisor, EventManager) {},
	)

	assert.NotEmpty(t, errs)

	AssertExactMatch(t, events,
		[]EventP{
			SupervisorStarted("root"),
			// start children from left to right
			WorkerStarted("root/branch0/child0"),
			WorkerStarted("root/branch0/child1"),
			SupervisorStarted("root/branch0"),
			WorkerStarted("root/branch1/child2"),
			//
			// Note child3 fails at this point
			//
			WorkerStartFailed("root/branch1/child3"),
			//
			// After a failure a few things will happen:
			//
			// * The `child4` worker initialization is skipped because of an error on
			// previous sibling
			//
			// * Previous sibling children get stopped in reversed order
			//
			// * The start function returns an error
			//
			WorkerTerminated("root/branch1/child2"),
			SupervisorStartFailed("root/branch1"),
			WorkerTerminated("root/branch0/child1"),
			WorkerTerminated("root/branch0/child0"),
			SupervisorTerminated("root/branch0"),
			SupervisorTerminated("root"),
		},
	)
}

func TestDynTerminateFailedChild(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []cap.Node{
		WaitDoneWorker("child0"),
		WaitDoneWorker("child1"),
		// NOTE: There is a NeverTerminateWorker here
		NeverTerminateWorker("child2"),
		WaitDoneWorker("child3"),
	}

	b0 := cap.NewSupervisorSpec(b0n, cap.WithNodes(cs[0], cs[1]))
	b1 := cap.NewSupervisorSpec(b1n, cap.WithNodes(cs[2], cs[3]))

	events, errs := ObserveDynSupervisor(
		context.TODO(),
		parentName,
		[]cap.Node{
			cap.Subtree(b0),
			cap.Subtree(b1),
		},
		[]cap.Opt{},
		func(cap.DynSupervisor, EventManager) {},
	)

	assert.NotEmpty(t, errs)

	AssertExactMatch(t, events,
		[]EventP{
			SupervisorStarted("root"),
			// start children from left to right
			WorkerStarted("root/branch0/child0"),
			WorkerStarted("root/branch0/child1"),
			SupervisorStarted("root/branch0"),
			WorkerStarted("root/branch1/child2"),
			WorkerStarted("root/branch1/child3"),
			SupervisorStarted("root/branch1"),
			// NOTE: From here, the stop of the supervisor begins
			WorkerTerminated("root/branch1/child3"),
			// NOTE: the child2 never stops and fails with a timeout
			WorkerFailed("root/branch1/child2"),
			// NOTE: The supervisor branch1 fails because of child2 timeout
			SupervisorFailed("root/branch1"),
			WorkerTerminated("root/branch0/child1"),
			WorkerTerminated("root/branch0/child0"),
			SupervisorTerminated("root/branch0"),
			SupervisorFailed("root"),
		},
	)
}

func TestDynSpawnAfterTerminate(t *testing.T) {
	sup, err := cap.NewDynSupervisor(context.TODO(), "root")
	assert.NoError(t, err)

	sup.Terminate()
	_, err = sup.Spawn(WaitDoneWorker("one"))

	assert.Error(t, err)
}

func TestDynSpawnAfterCrashedSupervisor(t *testing.T) {
	failingNode, failWorker := FailOnSignalWorker(
		1, "failing", cap.WithTolerance(0, 1*time.Millisecond),
	)

	events, errs := ObserveDynSupervisor(
		context.TODO(),
		"root",
		[]cap.Node{},
		[]cap.Opt{},
		func(sup cap.DynSupervisor, em EventManager) {
			_, err := sup.Spawn(failingNode)
			assert.NoError(t, err)

			failWorker(false)

			evIt := em.Iterator()
			evIt.SkipTill(WorkerFailed("root/failing"))

			_, err = sup.Spawn(failingNode)
			assert.Error(t, err)
		},
	)

	assert.Empty(t, errs)

	AssertExactMatch(t, events,
		[]EventP{
			SupervisorStarted("root"),
			// start children from left to right
			WorkerStarted("root/failing"),
			WorkerFailed("root/failing"),
			SupervisorFailed("root"),
		},
	)
}
