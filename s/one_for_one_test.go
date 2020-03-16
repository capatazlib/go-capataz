package s_test

//
// NOTE: If you feel is counter-intuitive to have workers start before
// supervisors in the assertions bellow, check stest/README.md
//

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/s"
	. "github.com/capatazlib/go-capataz/stest"
)

func TestSingleFailingChildRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, startFailingChild1 := FailingChild(1, "child1")

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
			// 1) Wait till all the three is up
			evIt.SkipTill(ProcessStarted("root"))
			// 2) Start the failing behavior of child1
			startFailingChild1()
			// 3) Wait till first restart
			evIt.SkipTill(ProcessStarted("root/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			ProcessStarted("root/child1"),
			ProcessStarted("root"),
			// ^^^ 1) startFailingChild1 starts executing here
			ProcessFailed("root/child1"),
			// ^^^ 2) And then we see a new (re)start of it
			ProcessStarted("root/child1"),
			// ^^^ 3) After 1st (re)start we stop
			ProcessStopped("root/child1"),
			ProcessStopped("root"),
		},
	)
}

func TestNestedFailingChildRecovers(t *testing.T) {
	parentName := "root"
	// Fail only one time
	child1, startFailingChild1 := FailingChild(1, "child1")
	tree1 := s.New("subtree1", s.WithChildren(child1))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{s.WithSubtree(tree1)},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the three is up
			evIt.SkipTill(ProcessStarted("root"))
			// 2) Start the failing behavior of child1
			startFailingChild1()
			// 3) Wait till first restart
			evIt.SkipTill(ProcessStarted("root/subtree1/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// start children from left to right
			ProcessStarted("root/subtree1/child1"),
			ProcessStarted("root/subtree1"),
			ProcessStarted("root"),
			// ^^^ 1) Wait till root starts
			ProcessFailed("root/subtree1/child1"),
			// ^^^ 2) We see the startFailingChild1 causing the error
			ProcessStarted("root/subtree1/child1"),
			// ^^^ 3) After 1st (re)start we stop
			ProcessStopped("root/subtree1/child1"),
			ProcessStopped("root/subtree1"),
			ProcessStopped("root"),
		},
	)
}
