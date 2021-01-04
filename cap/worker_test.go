package cap_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"
)

func TestWorkerHasContextValuesOnSimpleTree(t *testing.T) {
	ctx := context.Background()

	valueOne := 123
	valueTwo := 456

	ctx = context.WithValue(ctx, "value_one", 123)
	ctx = context.WithValue(ctx, "value_two", 456)

	worker := cap.NewWorker("one", func(ctx context.Context) error {
		v1 := ctx.Value("value_one").(int)
		assert.Equal(t, valueOne, v1)

		v2 := ctx.Value("value_two").(int)
		assert.Equal(t, valueTwo, v2)

		<-ctx.Done()
		return nil
	})

	events, err := ObserveSupervisor(
		ctx,
		"root",
		cap.WithNodes(worker),
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/one"),
			SupervisorStarted("root"),
			WorkerTerminated("root/one"),
			SupervisorTerminated("root"),
		},
	)
}

func TestWorkerHasContextValuesOnNestedTree(t *testing.T) {
	ctx := context.Background()

	valueOne := 123
	valueTwo := 456

	ctx = context.WithValue(ctx, "value_one", 123)
	ctx = context.WithValue(ctx, "value_two", 456)

	worker := cap.NewWorker("one", func(ctx context.Context) error {
		v1 := ctx.Value("value_one").(int)
		assert.Equal(t, valueOne, v1)

		v2 := ctx.Value("value_two").(int)
		assert.Equal(t, valueTwo, v2)

		<-ctx.Done()
		return nil
	})

	tree1 := cap.NewSupervisorSpec("subtree", cap.WithNodes(worker))

	events, err := ObserveSupervisor(
		ctx,
		"root",
		cap.WithNodes(
			cap.Subtree(tree1),
		),
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/subtree/one"),
			SupervisorStarted("root/subtree"),
			SupervisorStarted("root"),
			WorkerTerminated("root/subtree/one"),
			SupervisorTerminated("root/subtree"),
			SupervisorTerminated("root"),
		},
	)
}

func TestWorkerContextCanGetNameOnNestedSubtree(t *testing.T) {
	ctx := context.Background()

	newWorker := func(prefix, expected string) cap.Node {
		worker := cap.NewWorker(expected, func(ctx context.Context) error {
			name, ok := cap.GetWorkerName(ctx)
			assert.True(t, ok)
			assert.Equal(t, fmt.Sprintf("%s/%s", prefix, expected), name)
			<-ctx.Done()
			return nil
		})
		return worker
	}

	worker1 := newWorker("root/subtree1", "one")
	worker2 := newWorker("root/subtree2", "two")
	worker3 := newWorker("root/subtree1/subtree3", "three")

	tree3 := cap.NewSupervisorSpec("subtree3", cap.WithNodes(worker3))
	tree1 := cap.NewSupervisorSpec("subtree1", cap.WithNodes(worker1, cap.Subtree(tree3)))
	tree2 := cap.NewSupervisorSpec("subtree2", cap.WithNodes(worker2))

	events, err := ObserveSupervisor(
		ctx,
		"root",
		cap.WithNodes(
			cap.Subtree(tree1),
			cap.Subtree(tree2),
		),
		[]cap.Opt{},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/subtree1/one"),
			WorkerStarted("root/subtree1/subtree3/three"),
			SupervisorStarted("root/subtree1/subtree3"),
			SupervisorStarted("root/subtree1"),
			WorkerStarted("root/subtree2/two"),
			SupervisorStarted("root/subtree2"),
			SupervisorStarted("root"),
			WorkerTerminated("root/subtree2/two"),
			SupervisorTerminated("root/subtree2"),
			WorkerTerminated("root/subtree1/subtree3/three"),
			SupervisorTerminated("root/subtree1/subtree3"),
			WorkerTerminated("root/subtree1/one"),
			SupervisorTerminated("root/subtree1"),
			SupervisorTerminated("root"),
		},
	)
}
