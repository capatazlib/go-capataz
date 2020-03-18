package s_test

//
// NOTE: If you feel is counter-intuitive to have workers start before
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

func TestTransientOneForOneSingleFailingChildRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, failChild1 := FailOnSignalChild(1, "child1", c.WithRestart(c.Transient))

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
			evIt.SkipTill(WorkerStarted("root/child1"))
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
			// ^^^ 2) And then we see a new (re)start of it
			WorkerStarted("root/child1"),
			// ^^^ 3) After 1st (re)start we stop
			WorkerStopped("root/child1"),
			SupervisorStopped("root"),
		},
	)
}

func TestTransientOneForOneNestedFailingChildRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, failChild1 := FailOnSignalChild(1, "child1", c.WithRestart(c.Transient))
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
			// ^^^ 1) Wait till root starts
			WorkerFailed("root/subtree1/child1"),
			// ^^^ 2) We see the failChild1 causing the error
			WorkerStarted("root/subtree1/child1"),
			// ^^^ 3) After 1st (re)start we stop
			WorkerStopped("root/subtree1/child1"),
			SupervisorStopped("root/subtree1"),
			SupervisorStopped("root"),
		},
	)
}

func TestTransientOneForOneSingleCompleteChild(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, completeChild1 := CompleteOnSignalChild("child1", c.WithRestart(c.Transient))

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
			// ^^^ completeChild1 starts executing here
			WorkerCompleted("root/child1"),
			SupervisorStopped("root"),
		},
	)
}

func TestTransientOneForOneNestedCompleteChild(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, completeChild1 := CompleteOnSignalChild("child1", c.WithRestart(c.Transient))
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
			// ^^^ 1) Wait till root starts
			WorkerCompleted("root/subtree1/child1"),
			// ^^^ 2) We see the completeChild1 causing the completion
			SupervisorStopped("root/subtree1"),
			SupervisorStopped("root"),
		},
	)
}
