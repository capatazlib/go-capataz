package s_test

//
// NOTE: If you feel it is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
	. "github.com/capatazlib/go-capataz/stest"
)

func TestTemporaryOneForOneSingleFailingChildDoesNotRecover(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, failChild1 := FailOnSignalChild(1, "child1", c.WithRestart(c.Temporary))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{
			s.WithChildren(child1),
		},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failChild1()
			// 3) Wait till first restart
			evIt.SkipTill(WorkerFailed("root/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			SupervisorStarted("root"),
			// ^^^ 1) failChild1 starts executing here
			WorkerFailed("root/child1"),
			// ^^^ 2) We see the failure, and then nothing else of this child
			SupervisorStopped("root"),
		},
	)
}

func TestTemporaryOneForOneNestedFailingChildDoesNotRecover(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, failChild1 := FailOnSignalChild(1, "child1", c.WithRestart(c.Temporary))
	tree1 := s.New("subtree1", s.WithChildren(child1))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{s.WithSubtree(tree1)},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			failChild1()
			// 3) Wait till first restart
			evIt.SkipTill(WorkerFailed("root/subtree1/child1"))
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
			// ^^^ 2) We see the failure, and then nothing else of this child
			SupervisorStopped("root/subtree1"),
			SupervisorStopped("root"),
		},
	)
}

func TestTemporaryOneForOneSingleCompleteChildDoesNotRestart(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, completeChild1 := CompleteOnSignalChild("child1", c.WithRestart(c.Temporary))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{
			s.WithChildren(child1),
		},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the complete behavior of child1
			completeChild1()
			// 3) Wait till first restart
			evIt.SkipTill(WorkerCompleted("root/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/child1"),
			SupervisorStarted("root"),
			// ^^^ 1) completeChild1 starts executing here
			WorkerCompleted("root/child1"),
			// ^^^ 2) We see completion, and then nothing else of this child
			SupervisorStopped("root"),
		},
	)
}

func TestTemporaryOneForOneNestedCompleteChildDoesNotRestart(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, completeChild1 := CompleteOnSignalChild("child1", c.WithRestart(c.Temporary))
	tree1 := s.New("subtree1", s.WithChildren(child1))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{s.WithSubtree(tree1)},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted("root"))
			// 2) Start the failing behavior of child1
			completeChild1()
			// 3) Wait till first restart
			evIt.SkipTill(WorkerCompleted("root/subtree1/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			WorkerStarted("root/subtree1/child1"),
			SupervisorStarted("root/subtree1"),
			SupervisorStarted("root"),
			// ^^^ 1) completeChild1 starts executing here
			WorkerCompleted("root/subtree1/child1"),
			// ^^^ 2) We see completion, and then nothing else of this child
			SupervisorStopped("root/subtree1"),
			SupervisorStopped("root"),
		},
	)
}
