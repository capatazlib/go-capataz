package capataz_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/capataz"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestSupervisorWithErroredBuildNodesFn(t *testing.T) {
	t.Run("on one-level tree", func(t *testing.T) {
		events, err := ObserveSupervisor(
			context.TODO(),
			"root",
			func() ([]capataz.Node, capataz.CleanupResourcesFn, error) {
				return []capataz.Node{}, nil, errors.New("resource alloc error")
			},
			[]capataz.Opt{},
			func(EventManager) {},
		)

		assert.Error(t, err)

		AssertExactMatch(t, events,
			[]EventP{
				SupervisorStartFailed("root"),
			})
	})
	t.Run("on multi-level tree", func(t *testing.T) {
		subtree1 := capataz.NewSupervisorSpec("subtree1", capataz.WithNodes(WaitDoneWorker("worker1")))

		failingSubtree2 := capataz.NewSupervisorSpec("subtree2", func() ([]capataz.Node, capataz.CleanupResourcesFn, error) {
			return []capataz.Node{}, nil, errors.New("resource alloc error")
		})

		subtree3 := capataz.NewSupervisorSpec("subtree3", capataz.WithNodes(WaitDoneWorker("worker2")))

		events, err := ObserveSupervisor(
			context.TODO(),
			"root",
			capataz.WithNodes(
				capataz.Subtree(subtree1),
				capataz.Subtree(failingSubtree2),
				capataz.Subtree(subtree3),
			),
			[]capataz.Opt{},
			func(EventManager) {},
		)

		assert.Error(t, err)

		AssertExactMatch(t, events,
			[]EventP{
				WorkerStarted("root/subtree1/worker1"),
				SupervisorStarted("root/subtree1"),
				SupervisorStartFailed("root/subtree2"),
				// On start failure, we abort immediately (no restart type logic
				// in-place)
				WorkerTerminated("root/subtree1/worker1"),
				SupervisorTerminated("root/subtree1"),
				SupervisorStartFailed("root"),
			})
	})
}

func TestSupervisorWithErroredCleanupResourcesFn(t *testing.T) {
	t.Run("on one-level tree", func(t *testing.T) {
		events, err := ObserveSupervisor(
			context.TODO(),
			"root",
			func() ([]capataz.Node, capataz.CleanupResourcesFn, error) {
				nodes := []capataz.Node{WaitDoneWorker("worker1")}
				cleanup := func() error {
					return errors.New("cleanup resources err")
				}
				return nodes, cleanup, nil
			},
			[]capataz.Opt{},
			func(EventManager) {},
		)

		assert.Error(t, err, "supervisor failed to cleanup resources")

		AssertExactMatch(t, events,
			[]EventP{
				WorkerStarted("root/worker1"),
				SupervisorStarted("root"),
				WorkerTerminated("root/worker1"),
				SupervisorFailed("root"),
			})
	})
	t.Run("on multi-level tree", func(t *testing.T) {
		subtree1 := capataz.NewSupervisorSpec("subtree1", capataz.WithNodes(WaitDoneWorker("worker1")))

		failingSubtree2 := capataz.NewSupervisorSpec("subtree2", func() ([]capataz.Node, capataz.CleanupResourcesFn, error) {
			nodes := []capataz.Node{WaitDoneWorker("worker2")}
			cleanup := func() error {
				return errors.New("cleanup resources err")
			}
			return nodes, cleanup, nil
		})

		subtree3 := capataz.NewSupervisorSpec("subtree3", capataz.WithNodes(WaitDoneWorker("worker3")))

		events, err := ObserveSupervisor(
			context.TODO(),
			"root",
			capataz.WithNodes(
				capataz.Subtree(subtree1),
				capataz.Subtree(failingSubtree2),
				capataz.Subtree(subtree3),
			),
			[]capataz.Opt{},
			func(EventManager) {},
		)

		assert.Error(t, err, "supervisor failed to cleanup resources")

		AssertExactMatch(t, events,
			[]EventP{
				WorkerStarted("root/subtree1/worker1"),
				SupervisorStarted("root/subtree1"),
				WorkerStarted("root/subtree2/worker2"),
				SupervisorStarted("root/subtree2"),
				WorkerStarted("root/subtree3/worker3"),
				SupervisorStarted("root/subtree3"),
				SupervisorStarted("root"),
				// Termination starts here
				WorkerTerminated("root/subtree3/worker3"),
				SupervisorTerminated("root/subtree3"),
				WorkerTerminated("root/subtree2/worker2"),
				SupervisorFailed("root/subtree2"),
				// ^ supervisor failed, but shutdown still continues
				WorkerTerminated("root/subtree1/worker1"),
				SupervisorTerminated("root/subtree1"),
				SupervisorFailed("root"),
			})
	})
}
