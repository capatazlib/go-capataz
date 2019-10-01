package s_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/s"
)

func TestSingleChild(t *testing.T) {
	cs := waitDoneChild("one")

	events, err := observeSupervisor(
		"root",
		[]s.Opt{
			s.WithChildren(cs),
		},
		noWait,
	)
	assert.Nil(t, err)

	assertPartialMatch(t, events,
		[]EventP{
			ProcessStarted("root/one"),
			ProcessStarted("root"),
			ProcessStopped("root/one"),
			ProcessStopped("root"),
		})
}

func TestMutlipleChild(t *testing.T) {
	c0 := waitDoneChild("child0")
	c1 := waitDoneChild("child1")
	c2 := waitDoneChild("child2")

	events, err := observeSupervisor(
		"root",
		[]s.Opt{
			s.WithChildren(c0, c1, c2),
		},
		noWait,
	)
	assert.Nil(t, err)

	assertPartialMatch(t, events,
		[]EventP{
			ProcessStarted("root/child0"),
			ProcessStarted("root/child1"),
			ProcessStarted("root/child2"),
			ProcessStarted("root"),
			ProcessStopped("root/child2"),
			ProcessStopped("root/child1"),
			ProcessStopped("root/child0"),
			ProcessStopped("root"),
		})
}

func TestNestedSupervisor(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs, waitSignal := orderedChidren(4)

	b0, _ := s.New(b0n, s.WithChildren(cs[0], cs[1]))
	b1, _ := s.New(b1n, s.WithChildren(cs[2], cs[3]))

	events, err := observeSupervisor(
		parentName,
		[]s.Opt{
			s.WithSubtree(b0),
			s.WithSubtree(b1),
		},
		waitSignal,
	)
	assert.Nil(t, err)

	assertPartialMatch(t, events,
		[]EventP{
			ProcessStarted("root/branch0/child0"),
			ProcessStarted("root/branch0/child1"),
			ProcessStarted("root/branch0"),
			ProcessStarted("root/branch1/child2"),
			ProcessStarted("root/branch1/child3"),
			ProcessStarted("root/branch1"),
			ProcessStarted("root"),
			///
			ProcessStopped("root/branch1/child3"),
			ProcessStopped("root/branch1/child2"),
			ProcessStopped("root/branch1"),
			ProcessStopped("root/branch0/child1"),
			ProcessStopped("root/branch0/child0"),
			ProcessStopped("root/branch0"),
			ProcessStopped("root"),
		},
	)
}
