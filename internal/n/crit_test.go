package n_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/cap"
	. "github.com/capatazlib/go-capataz/internal/stest"

	"github.com/capatazlib/go-capataz/internal/n"
	"github.com/capatazlib/go-capataz/internal/s"
)

var alwaysTrue n.EventCriteria = func(s.Event) bool { return true }
var alwaysFalse n.EventCriteria = func(s.Event) bool { return false }

func TestEAnd(t *testing.T) {
	t.Run("all values are true", func(t *testing.T) {
		counter := 0
		evNotifier0 := func(s.Event) {
			counter++
		}
		crit := n.EAnd(alwaysTrue, alwaysTrue, alwaysTrue)
		evNotifier := n.SelectEventByCriteria(crit, evNotifier0)
		evNotifier(s.Event{})
		assert.Equal(t, 1, counter)
	})

	t.Run("one value is false", func(t *testing.T) {
		counter := 0
		evNotifier0 := func(s.Event) {
			counter++
		}
		crit := n.EAnd(alwaysTrue, alwaysFalse, alwaysTrue)
		evNotifier := n.SelectEventByCriteria(crit, evNotifier0)
		evNotifier(s.Event{})
		assert.Equal(t, 0, counter)
	})
}

func TestEOr(t *testing.T) {
	t.Run("all values are false", func(t *testing.T) {
		counter := 0
		evNotifier0 := func(s.Event) {
			counter++
		}
		crit := n.EOr(alwaysFalse, alwaysFalse, alwaysFalse)
		evNotifier := n.SelectEventByCriteria(crit, evNotifier0)
		evNotifier(s.Event{})
		assert.Equal(t, 0, counter)
	})

	t.Run("one value is true", func(t *testing.T) {
		counter := 0
		evNotifier0 := func(s.Event) {
			counter++
		}
		crit := n.EOr(alwaysFalse, alwaysTrue, alwaysFalse)
		evNotifier := n.SelectEventByCriteria(crit, evNotifier0)
		evNotifier(s.Event{})
		assert.Equal(t, 1, counter)
	})
}

func TestENot(t *testing.T) {
	crit1 := n.ENot(alwaysTrue)
	crit2 := n.ENot(alwaysFalse)

	counter := 0
	evNotifier0 := func(s.Event) {
		counter++
	}

	evNotifier1 := n.SelectEventByCriteria(crit1, evNotifier0)
	evNotifier1(s.Event{})
	assert.Equal(t, 0, counter)

	evNotifier2 := n.SelectEventByCriteria(crit2, evNotifier0)
	evNotifier2(s.Event{})
	assert.Equal(t, 1, counter)
}

func TestEInSubtree(t *testing.T) {
	counter := 0
	evNotifier0 := func(s.Event) {
		counter++
	}

	rootName := "root"
	b0n := "branch0"
	b1n := "branch1"

	evNotifier := n.SelectEventByCriteria(n.EInSubtree("root/branch0"), evNotifier0)

	b0 := s.NewSupervisorSpec(b0n, s.WithNodes(WaitDoneWorker("child0")))
	b1 := s.NewSupervisorSpec(b1n, s.WithNodes(WaitDoneWorker("child1")))

	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		rootName,
		s.WithNodes(
			s.Subtree(b0),
			s.Subtree(b1),
		),
		[]s.Opt{},
		[]s.EventNotifier{evNotifier},
		func(EventManager) {},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			// 1
			WorkerStarted("root/branch0/child0"),
			// 2
			SupervisorStarted("root/branch0"),
			WorkerStarted("root/branch1/child1"),
			SupervisorStarted("root/branch1"),
			SupervisorStarted("root"),
			WorkerTerminated("root/branch1/child1"),
			SupervisorTerminated("root/branch1"),
			// 3
			WorkerTerminated("root/branch0/child0"),
			// 4
			SupervisorTerminated("root/branch0"),
			SupervisorTerminated("root"),
		},
	)

	assert.Equal(t, 4, counter)
}

func TestEIsFailure(t *testing.T) {
	counter0 := 0
	evNotifier0 := func(s.Event) {
		counter0++
	}

	counter1 := 0
	evNotifier1 := func(s.Event) {
		counter1++
	}

	evNotifierA := n.SelectEventByCriteria(n.EIsFailure, evNotifier0)
	evNotifierB := n.SelectEventByCriteria(n.EIsWorkerFailure, evNotifier1)

	rootName := "root"
	// Fail only one time
	child1, failWorker1 := FailOnSignalWorker(1, "child1", cap.WithRestart(cap.Permanent))

	events, err := ObserveSupervisorWithNotifiers(
		context.TODO(),
		rootName,
		cap.WithNodes(child1),
		[]s.Opt{},
		[]s.EventNotifier{evNotifierA, evNotifierB},
		func(em EventManager) {
			// NOTE: we won't stop the supervisor until the child has failed at least
			// once
			evIt := em.Iterator()
			// 1) Wait till all the tree is up
			evIt.SkipTill(SupervisorStarted(rootName))
			// 2) Start the failing behavior of child1
			failWorker1(true /* done */)
			// 3) Wait till first restart
			evIt.SkipTill(WorkerStarted("root/child1"))
		},
	)

	assert.NoError(t, err)

	AssertExactMatch(t, events,
		[]EventP{
			WorkerStarted("root/child1"),
			SupervisorStarted("root"),
			// A, B
			WorkerFailed("root/child1"),
			WorkerStarted("root/child1"),
			WorkerTerminated("root/child1"),
			SupervisorTerminated("root"),
		},
	)

	assert.Equal(t, 1, counter0)
	assert.Equal(t, 1, counter1)

}
