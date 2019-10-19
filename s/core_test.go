package s_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/capatazlib/go-capataz/c"
	"github.com/capatazlib/go-capataz/s"
)

// ExampleNew showcases an example on how to define a supervision tree system.
//
// In this trivial example we create a system with two sub-systems.
//
// 1) The "file-system" sub-tree supervises file system errors
//
// 2) The "service-a" sub-tree supervises interactions with the service-a API
//
// Note that if there are errors in the "file-system" sub-tree, its supervisor
// will deal with transient errors without affecting the "service-a" sub-tree.
//
// If errors on the file-system go beyond an specified treshold, the errors are
// raised to the root supervisor and the root supervisor will restart both
// sub-trees.
//
// If the errors still continue on the "file-system" sub-tree, it will
// eventually surpass the root tree error treshold and the system will return an
// error on the root supervisor Wait call.
//
func ExampleNew() {
	// // Channels used for communication between the different children of the
	// // supervision tree
	// fileChangedCh := make(chan string)
	writeFileCh := make(chan string)

	// Build a supervision tree with two branches
	rootSpec := s.New(
		"root",
		// first sub-tree
		s.WithSubtree(
			s.New("file-system",
				s.WithChildren(
					c.New("file-watcher", func(ctx context.Context) error {
						fmt.Println("Start File Watch Functionality")
						// Use inotify and send messages to the fileChangedCh
						<-ctx.Done()
						return nil
					}),
					c.New("file-writer-manager", func(ctx context.Context) error {
						// assume this function has access to a request chan from a closure
						for {
							select {
							case <-ctx.Done():
								return ctx.Err()
							case _ /* content */, ok := <-writeFileCh:
								if !ok {
									err := fmt.Errorf("writeFileCh ownership compromised")
									return err
								}
								fmt.Println("Write contents to a file")
							}
						}
					}),
				),
			),
		),
		// second sub-tree
		s.WithSubtree(
			s.New("service-a",
				s.WithChildren(
					c.New("fetch-service-data-ticker", func(ctx context.Context) error {
						// assume this function has access to a request and response
						// channnels from a closure
						fmt.Println("Perform requests to API to gather data and submit it to a channel")
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

	// Wait for supervision tree to exit, this will only happen when errors cannot
	// be recovered by the supervision system because they reached the given
	// treshold of failure.
	//
	// TODO: Add reference to treshold settings documentation once it is
	// implemented
	err = sup.Wait()
	if err != nil {
		fmt.Printf("Supervisor failed with error: %v\n", err)
	}
}

// Test a supervision tree with a single child starts and stops
func TestStartSingleChild(t *testing.T) {
	cs := waitDoneChild("one")

	events := observeSupervisor(
		"root",
		[]s.Opt{
			s.WithChildren(cs),
		},
		noWait,
	)

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

	events := observeSupervisor(
		"root",
		[]s.Opt{
			s.WithChildren(c0, c1, c2),
		},
		noWait,
	)

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
}

// Test a supervision tree with two sub-trees start and stop children in the
// default order _always_ (LeftToRight)
func TestStartNestedSupervisors(t *testing.T) {
	parentName := "root"
	b0n := "branch0"
	b1n := "branch1"

	cs := []c.ChildSpec{
		waitDoneChild("child0"),
		waitDoneChild("child1"),
		waitDoneChild("child2"),
		waitDoneChild("child3"),
	}

	b0 := s.New(b0n, s.WithChildren(cs[0], cs[1]))
	b1 := s.New(b1n, s.WithChildren(cs[2], cs[3]))

	events := observeSupervisor(
		parentName,
		[]s.Opt{
			s.WithSubtree(b0),
			s.WithSubtree(b1),
		},
		noWait,
	)

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
}
