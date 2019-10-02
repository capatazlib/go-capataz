package s_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

func ExampleNew() {
	// Build a supervision tree with two branches
	rootSpec := s.New(
		"root",
		// first sub-tree
		s.WithSubtree(
			s.New("sub-tree-1",
				s.WithChildren(
					c.New("child-1", func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					}),
					c.New("child-2", func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					}),
				),
			),
		),
		// second sub-tree
		s.WithSubtree(
			s.New("sub-tree-2",
				s.WithChildren(
					c.New("child-3", func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					}),
					c.New("child-4", func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					}),
				),
			),
		),
	)

	// Spawn goroutines of supervision tree
	sup, err := rootSpec.Start(context.Background())
	if err != nil {
		fmt.Printf("Error starting system: %v\n", err)
	}
	// Wait for supervision tree to exit, this happens only if the errors are not
	// recoverable
	err = sup.Wait()
	if err != nil {
		fmt.Printf("Supervisor failed with error: %v\n", err)
	}
}

// Test a supervision tree with a single child starts and stops
func TestStartSingleChild(t *testing.T) {
	cs := waitDoneChild("one")

	events, err := observeSupervisor(
		"root",
		[]s.Opt{
			s.WithChildren(cs),
		},
		noWait,
	)
	assert.Nil(t, err)

	assertExactMatch(t, events,
		[]EventP{
			ProcessStarted("root/one"),
			ProcessStarted("root"),
			ProcessStopped("root/one"),
			ProcessStopped("root"),
		})
}

// Test a supervision tree with three children start and stop in the default
// order (LeftToRight)
func TestStartMutlipleChildren(t *testing.T) {
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

	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		assertExactMatch(t, events,
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
	})

	t.Run("asserts there is a single start/stop event per go-routine", func(t *testing.T) {
		for _, name := range []string{
			"root/child0",
			"root/child1",
			"root/child2",
			"root",
		} {
			assertPredMatchesN(t, 1, events, ProcessStarted(name))
			assertPredMatchesN(t, 1, events, ProcessStopped(name))
		}
	})

}

// Test a supervision tree with two sub-trees start and stop childrens in the
// default order _always_ (LeftToRight)
func TestStartNestedSupervisors(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []c.Spec{
		waitDoneChild("child0"),
		waitDoneChild("child1"),
		waitDoneChild("child2"),
		waitDoneChild("child3"),
	}

	b0 := s.New(b0n, s.WithChildren(cs[0], cs[1]))
	b1 := s.New(b1n, s.WithChildren(cs[2], cs[3]))

	events, err := observeSupervisor(
		parentName,
		[]s.Opt{
			s.WithSubtree(b0),
			s.WithSubtree(b1),
		},
		noWait,
	)
	assert.Nil(t, err)

	t.Run("starts and stops routines in the correct order", func(t *testing.T) {
		assertExactMatch(t, events,
			[]EventP{
				// start children from left to right
				ProcessStarted("root/branch0/child0"),
				ProcessStarted("root/branch0/child1"),
				ProcessStarted("root/branch0"),
				ProcessStarted("root/branch1/child2"),
				ProcessStarted("root/branch1/child3"),
				ProcessStarted("root/branch1"),
				ProcessStarted("root"),
				// stops children from right to left
				ProcessStopped("root/branch1/child3"),
				ProcessStopped("root/branch1/child2"),
				ProcessStopped("root/branch1"),
				ProcessStopped("root/branch0/child1"),
				ProcessStopped("root/branch0/child0"),
				ProcessStopped("root/branch0"),
				ProcessStopped("root"),
			},
		)
	})

	t.Run("asserts there is a single start/stop event per go-routine", func(t *testing.T) {
		for _, name := range []string{
			"root/branch0/child0",
			"root/branch0/child1",
			"root/branch1/child2",
			"root/branch1/child3",
			"root/branch0",
			"root/branch1",
			"root",
		} {
			assertPredMatchesN(t, 1, events, ProcessStarted(name))
			assertPredMatchesN(t, 1, events, ProcessStopped(name))
		}
	})
}
