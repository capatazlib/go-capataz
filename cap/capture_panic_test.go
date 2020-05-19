package cap_test

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
		panicChild1, signalPanic1 := PanicOnSignalWorker(
			1,
			"child1",
		)

		events, err := ObserveSupervisor(
			context.TODO(),
			parentName,
			cap.WithNodes(panicChild1),
			[]cap.Opt{},
			func(em EventManager) {
				evIt := em.Iterator()
				evIt.SkipTill(SupervisorStarted("root"))
				signalPanic1(true /* done */)
				evIt.SkipTill(WorkerStarted("root/child1"))
			},
		)

		assert.NoError(t, err)

		AssertExactMatch(t, events,
			[]EventP{
				// start children from left to right
				WorkerStarted("root/child1"),
				SupervisorStarted("root"),
				WorkerFailedWith("root/child1", "Panicking child (1 out of 1)"),
				WorkerStarted("root/child1"),
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
				evIt.SkipTill(SupervisorStarted("root"))
				signalPanic1(true /* done */)
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
				WorkerFailedWith("root/subtree1/child1", "Panicking child (1 out of 1)"),
				WorkerStarted("root/subtree1/child1"),
				WorkerTerminated("root/subtree1/child1"),
				SupervisorTerminated("root/subtree1"),
				SupervisorTerminated("root"),
			},
		)
	})
}
