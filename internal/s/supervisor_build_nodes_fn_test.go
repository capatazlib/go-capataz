package s_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestSupervisorWithErroredBuildNodesFn(t *testing.T) {
	t.Run("on one-level tree", func(t *testing.T) {
		events, err := ObserveSupervisor(
			context.TODO(),
			"root",
			func() ([]cap.Node, cap.CleanupResourcesFn, error) {
				return []cap.Node{}, nil, errors.New("resource alloc error")
			},
			[]cap.Opt{},
			func(EventManager) {},
		)

		assert.Error(t, err)

		AssertExactMatch(t, events,
			[]EventP{
				SupervisorStartFailed("root"),
			})
	})
	t.Run("on multi-level tree", func(t *testing.T) {
		subtree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(WaitDoneWorker("worker1")))

		failingSubtree2 := cap.NewSupervisorSpec(
			"subtree2",
			func() ([]cap.Node, cap.CleanupResourcesFn, error) {
				return []cap.Node{}, nil, errors.New("resource alloc error")
			})

		subtree3 := cap.NewSupervisorSpec("subtree3", cap.WithNodes(WaitDoneWorker("worker2")))

		events, err := ObserveSupervisor(
			context.TODO(),
			"root",
			cap.WithNodes(
				cap.Subtree(subtree1),
				cap.Subtree(failingSubtree2),
				cap.Subtree(subtree3),
			),
			[]cap.Opt{},
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

func TestSupervisorWithPanicBuildNodesFnOnSingleTree(t *testing.T) {
	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		func() ([]cap.Node, cap.CleanupResourcesFn, error) {
			panic("single tree panic")
		},
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.Error(t, err)
	errKVs := err.(cap.ErrKVs)
	kvs := errKVs.KVs()
	assert.Equal(t, "supervisor build nodes function failed", err.Error())
	assert.Equal(t, "root", kvs["supervisor.name"])
	assert.Contains(t, fmt.Sprint(kvs["supervisor.build.error"]), "single tree panic")

	explanation := cap.ExplainError(err)
	assert.Contains(
		t,
		explanation,
		"supervisor 'root' build nodes function failed\n\t> single tree panic",
	)

	AssertExactMatch(t, events,
		[]EventP{
			SupervisorStartFailed("root"),
		})
}

func TestSupervisorWithPanicBuildNodesFnOnNestedTree(t *testing.T) {
	subtree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(WaitDoneWorker("worker1")))

	failingSubtree2 := cap.NewSupervisorSpec(
		"subtree2",
		func() ([]cap.Node, cap.CleanupResourcesFn, error) {
			panic("sub-tree panic")
		})

	subtree3 := cap.NewSupervisorSpec("subtree3", cap.WithNodes(WaitDoneWorker("worker3")))

	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(
			cap.Subtree(subtree1),
			cap.Subtree(failingSubtree2),
			cap.Subtree(subtree3),
		),
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.Error(t, err)
	errKVs := err.(cap.ErrKVs)
	kvs := errKVs.KVs()
	assert.Equal(t, "supervisor node failed to start", err.Error())
	assert.Equal(t, "root/subtree2", kvs["supervisor.subtree.name"])
	assert.Contains(t, fmt.Sprint(kvs["supervisor.subtree.build.error"]), "sub-tree panic")

	explanation := cap.ExplainError(err)
	assert.Contains(
		t,
		explanation,
		"supervisor 'root/subtree2' build nodes function failed\n\t> sub-tree panic",
	)

	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/subtree1/worker1"),
			SupervisorStarted("root/subtree1"),
			SupervisorStartFailed("root/subtree2"),
			// ^ supervisor panicked on creation
			WorkerTerminated("root/subtree1/worker1"),
			// ^ termination in reverse order starts
			SupervisorTerminated("root/subtree1"),
			SupervisorStartFailed("root"),
		})
}

func TestSupervisorWithErroredCleanupResourcesFnOnSingleTree(t *testing.T) {
	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		func() ([]cap.Node, cap.CleanupResourcesFn, error) {
			nodes := []cap.Node{WaitDoneWorker("worker1")}
			cleanup := func() error {
				return errors.New("cleanup resources err")
			}
			return nodes, cleanup, nil
		},
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.Error(t, err)
	errKvs := err.(cap.ErrKVs)
	kvs := errKvs.KVs()
	assert.Equal(t, "supervisor terminated with failures", err.Error())
	assert.Equal(t, "root", kvs["supervisor.name"])
	assert.Equal(
		t,
		"cleanup resources err",
		fmt.Sprint(kvs["supervisor.termination.cleanup.error"]),
	)

	explanation := cap.ExplainError(err)
	assert.Equal(
		t,
		"supervisor 'root' cleanup failed on termination\n\t> cleanup resources err",
		explanation,
	)

	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/worker1"),
			SupervisorStarted("root"),
			WorkerTerminated("root/worker1"),
			SupervisorFailed("root"),
		})
}

func TestSupervisorWithErroredCleanupResourcesFnOnNestedTree(t *testing.T) {
	subtree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(WaitDoneWorker("worker1")))

	failingSubtree2 := cap.NewSupervisorSpec(
		"subtree2",
		func() ([]cap.Node, cap.CleanupResourcesFn, error) {
			nodes := []cap.Node{WaitDoneWorker("worker2")}
			cleanup := func() error {
				return errors.New("cleanup resources err")
			}
			return nodes, cleanup, nil
		})

	subtree3 := cap.NewSupervisorSpec("subtree3", cap.WithNodes(WaitDoneWorker("worker3")))

	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		cap.WithNodes(
			cap.Subtree(subtree1),
			cap.Subtree(failingSubtree2),
			cap.Subtree(subtree3),
		),
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.Error(t, err)
	errKvs := err.(cap.ErrKVs)
	kvs := errKvs.KVs()
	assert.Equal(t, "supervisor terminated with failures", err.Error())
	assert.Equal(t, "root", kvs["supervisor.name"])
	assert.Equal(t, "root/subtree2", kvs["supervisor.subtree.0.name"])
	assert.Equal(
		t,
		"cleanup resources err",
		fmt.Sprint(kvs["supervisor.subtree.0.termination.cleanup.error"]),
	)

	explanation := cap.ExplainError(err)
	assert.Equal(
		t,
		"supervisor 'root/subtree2' cleanup failed on termination\n\t> cleanup resources err",
		explanation,
	)

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

}
