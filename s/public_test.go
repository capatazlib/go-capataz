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
	. "github.com/capatazlib/go-capataz/internal/stest"
	"github.com/capatazlib/go-capataz/s"
)

func TestStartSingleChild(t *testing.T) {
	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		[]s.Opt{
			s.WithChildren(WaitDoneChild("one")),
		},
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
}

// Test a supervision tree with three children start and stop in the default
// order (LeftToRight)
func TestStartMutlipleChildrenLeftToRight(t *testing.T) {
	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		[]s.Opt{
			s.WithChildren(
				WaitDoneChild("child0"),
				WaitDoneChild("child1"),
				WaitDoneChild("child2"),
			),
		},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				WorkerStarted("root/child0"),
				WorkerStarted("root/child1"),
				WorkerStarted("root/child2"),
				SupervisorStarted("root"),
				WorkerTerminated("root/child2"),
				WorkerTerminated("root/child1"),
				WorkerTerminated("root/child0"),
				SupervisorTerminated("root"),
			})
	})
}

// Test a supervision tree with three children start and stop in the default
// order (LeftToRight)
func TestStartMutlipleChildrenRightToLeft(t *testing.T) {
	events, err := ObserveSupervisor(
		context.TODO(),
		"root",
		[]s.Opt{
			s.WithChildren(
				WaitDoneChild("child0"),
				WaitDoneChild("child1"),
				WaitDoneChild("child2"),
			),
			s.WithOrder(s.RightToLeft),
		},
		func(EventManager) {},
	)

	assert.NoError(t, err)
	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		AssertExactMatch(t, events,
			[]EventP{
				WorkerStarted("root/child2"),
				WorkerStarted("root/child1"),
				WorkerStarted("root/child0"),
				SupervisorStarted("root"),
				WorkerTerminated("root/child0"),
				WorkerTerminated("root/child1"),
				WorkerTerminated("root/child2"),
				SupervisorTerminated("root"),
			})
	})
}

// Test a supervision tree with two sub-trees start and stop children in the
// default order _always_ (LeftToRight)
func TestStartNestedSupervisors(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []c.ChildSpec{
		WaitDoneChild("child0"),
		WaitDoneChild("child1"),
		WaitDoneChild("child2"),
		WaitDoneChild("child3"),
	}

	b0 := s.New(b0n, s.WithChildren(cs[0], cs[1]))
	b1 := s.New(b1n, s.WithChildren(cs[2], cs[3]))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{
			s.WithSubtree(b0),
			s.WithSubtree(b1),
		},
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
}

func TestStartFailedChild(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []c.ChildSpec{
		WaitDoneChild("child0"),
		WaitDoneChild("child1"),
		WaitDoneChild("child2"),
		// NOTE: FailStartChild here
		FailStartChild("child3"),
		WaitDoneChild("child4"),
	}

	b0 := s.New(b0n, s.WithChildren(cs[0], cs[1]))
	b1 := s.New(b1n, s.WithChildren(cs[2], cs[3], cs[4]))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{
			s.WithSubtree(b0),
			s.WithSubtree(b1),
		},
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
			SupervisorStartFailed("root"),
		},
	)
}

func TestTerminateFailedChild(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []c.ChildSpec{
		WaitDoneChild("child0"),
		WaitDoneChild("child1"),
		// NOTE: There is a NeverTerminateChild here
		NeverTerminateChild("child2"),
		WaitDoneChild("child3"),
	}

	b0 := s.New(b0n, s.WithChildren(cs[0], cs[1]))
	b1 := s.New(b1n, s.WithChildren(cs[2], cs[3]))

	events, err := ObserveSupervisor(
		context.TODO(),
		parentName,
		[]s.Opt{
			s.WithSubtree(b0),
			s.WithSubtree(b1),
		},
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
}
