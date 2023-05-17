package s_test

import (
	"context"
	"testing"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
	"github.com/stretchr/testify/assert"
)

func TestCapturePanic(t *testing.T) {
	t.Run("one-level tree", func(t *testing.T) {
		parentName := "root"
		// We create a worker that fails with a panic only once
		panicChild1, signalPanic1 := PanicOnSignalWorker(
			1, /* 1 panic only */
			"child1",
		)

		events, err := ObserveSupervisor(
			context.TODO(),
			parentName,
			cap.WithNodes(panicChild1),
			[]cap.Opt{},
			func(em EventManager) {
				evIt := em.Iterator()
				evIt.WaitTill(SupervisorStarted("root"))
				signalPanic1(true /* done */)
				evIt.WaitTill(WorkerStarted("root/child1"))
			},
		)

		assert.NoError(t, err)

		AssertExactMatch(t, events,
			[]EventP{
				// *** Supervisor start process ***

				WorkerStarted("root/child1"),
				SupervisorStarted("root"),
				// ^^^ supervisor started successfully, from here, it is supervising the
				// only worker node it has
				WorkerFailedWith("root/child1", "panicking child (1 out of 1)"),
				// ^^^ As specified with the PanicOnSignalWorker, this worker fails
				// only one time (1 out of 1)
				WorkerStarted("root/child1"),
				// ^^^ Worker is (re)started, this time, it won't panic (given we ran)
				// out of panic errors to throw

				// *** Supervisor termination process (via ObserveSupervisor) ***

				// When supervisor is terminated, it starts with it's child nodes, and
				// then itself.
				WorkerTerminated("root/child1"),
				SupervisorTerminated("root"),
			},
		)
	})

	t.Run("multi-level tree", func(t *testing.T) {
		parentName := "root"
		panicChild1, signalPanic1 := PanicOnSignalWorker(
			1,
			"child1",
		)
		subtree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(panicChild1))

		events, err := ObserveSupervisor(
			context.TODO(),
			parentName,
			cap.WithNodes(cap.Subtree(subtree1)),
			[]cap.Opt{},
			func(em EventManager) {
				evIt := em.Iterator()
				evIt.WaitTill(SupervisorStarted("root"))
				signalPanic1(true /* done */)
				evIt.WaitTill(WorkerStarted("root/subtree1/child1"))
			},
		)

		assert.NoError(t, err)

		AssertExactMatch(t, events,
			[]EventP{
				// start children from left to right
				WorkerStarted("root/subtree1/child1"),
				SupervisorStarted("root/subtree1"),
				SupervisorStarted("root"),
				WorkerFailedWith("root/subtree1/child1", "panicking child (1 out of 1)"),
				WorkerStarted("root/subtree1/child1"),
				WorkerTerminated("root/subtree1/child1"),
				SupervisorTerminated("root/subtree1"),
				SupervisorTerminated("root"),
			},
		)
	})
}
