package cap_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestHealthStartSingleChild(t *testing.T) {
	healthcheckMonitor := cap.NewHealthcheckMonitor(0, 0*time.Millisecond)

	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		"root",
		cap.WithNodes(WaitDoneWorker("one")),
		[]cap.Opt{},
		[]cap.EventNotifier{func(ev cap.Event) { healthcheckMonitor.HandleEvent(ev) }},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/one"),
			SupervisorStarted("root"),
			WorkerTerminated("root/one"),
			SupervisorTerminated("root"),
		})

	// No failures ever
	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestHealthStartNestedSupervisors(t *testing.T) {
	healthcheckMonitor := cap.NewHealthcheckMonitor(0, 0*time.Millisecond)

	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []cap.Node{
		WaitDoneWorker("child0"),
		WaitDoneWorker("child1"),
		WaitDoneWorker("child2"),
		WaitDoneWorker("child3"),
	}

	b0 := cap.NewSupervisorSpec(b0n, cap.WithNodes(cs[0], cs[1]))
	b1 := cap.NewSupervisorSpec(b1n, cap.WithNodes(cs[2], cs[3]))

	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		parentName,
		cap.WithNodes(
			cap.Subtree(b0),
			cap.Subtree(b1),
		),
		[]cap.Opt{},
		[]cap.EventNotifier{func(ev cap.Event) { healthcheckMonitor.HandleEvent(ev) }},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				// start children from left to right
				WorkerStarted("root/branch0/child0"),
				WorkerStarted("root/branch0/child1"),
				SupervisorStarted("root/branch0"),
				WorkerStarted("root/branch1/child2"),
				WorkerStarted("root/branch1/child3"),
				SupervisorStarted("root/branch1"),
				SupervisorStarted("root"),
				// stops children from right to left
				WorkerTerminated("root/branch1/child3"),
				WorkerTerminated("root/branch1/child2"),
				SupervisorTerminated("root/branch1"),
				WorkerTerminated("root/branch0/child1"),
				WorkerTerminated("root/branch0/child0"),
				SupervisorTerminated("root/branch0"),
				SupervisorTerminated("root"),
			},
		)
	})

	// No failures ever
	assert.True(t, healthcheckMonitor.IsHealthy())
}

func TestHealthTerminateFailedChild(t *testing.T) {
	healthcheckMonitor := cap.NewHealthcheckMonitor(0, 0*time.Millisecond)

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

	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		parentName,
		cap.WithNodes(
			cap.Subtree(b0),
			cap.Subtree(b1),
		),
		[]cap.Opt{},
		[]cap.EventNotifier{func(ev cap.Event) { healthcheckMonitor.HandleEvent(ev) }},
		func(em EventManager) {},
	)

	assert.Error(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/branch0/child0"),
			WorkerStarted("root/branch0/child1"),
			SupervisorStarted("root/branch0"),
			WorkerStarted("root/branch1/child2"),
			WorkerStarted("root/branch1/child3"),
			SupervisorStarted("root/branch1"),
			SupervisorStarted("root"),
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

	// Not healthy
	assert.False(t, healthcheckMonitor.IsHealthy())

	hr := healthcheckMonitor.GetHealthReport()
	failedProcesses := hr.GetFailedProcesses()
	delayedRestartProcesses := hr.GetDelayedRestartProcesses()
	// Failures are over tolerance. Failed supervisor, branch and child
	assert.EqualValues(t, 3, len(failedProcesses))
	assert.True(t, failedProcesses["root/branch1/child2"])
	assert.True(t, failedProcesses["root/branch1"])
	assert.True(t, failedProcesses["root"])
	// restart delays are under tolerance. Nobody restarted
	assert.EqualValues(t, 3, len(delayedRestartProcesses))
	assert.True(t, delayedRestartProcesses["root/branch1/child2"])
	assert.True(t, delayedRestartProcesses["root/branch1"])
	assert.True(t, delayedRestartProcesses["root"])
}

func TestHealthPermanentOneForOneNestedFailingWorkerRecovers(t *testing.T) {
	healthcheckMonitor := cap.NewHealthcheckMonitor(0, 0*time.Millisecond)

	parentName := "root"
	// Fail only one time
	child1, failWorker1 := FailOnSignalWorker(1, "child1", cap.WithRestart(cap.Permanent))
	tree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(child1))

	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		parentName,
		cap.WithNodes(cap.Subtree(tree1)),
		[]cap.Opt{},
		[]cap.EventNotifier{func(ev cap.Event) { healthcheckMonitor.HandleEvent(ev) }},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failWorker1(true /* done */)
			// 3) Wait until we can catch the failure
			evIt.SkipTill(WorkerFailed("root/subtree1/child1"))
			// 4) We are unhealthy
			assert.False(t, healthcheckMonitor.IsHealthy())
			// 5) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/subtree1/child1"))
			// 6) We are healthy again
			assert.True(t, healthcheckMonitor.IsHealthy())
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

	// Healthy after recovery
	assert.True(t, healthcheckMonitor.IsHealthy())
}
