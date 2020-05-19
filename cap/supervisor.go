package cap

// This file contains the implementation of the public methods for the
// Supervisor API

import (
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Supervisor represents the root of a tree of goroutines. A Supervisor may have
// leaf or sub-tree children, where each of the nodes in the tree represent a
// goroutine that gets automatic restart abilities as soon as the parent
// supervisor detects an error has occured. A Supervisor will always be
// generated from a SupervisorSpec
type Supervisor struct {
	runtimeName string
	spec        SupervisorSpec
	children    map[string]c.Child
	cancel      func()
	wait        func(time.Time, startError) error
}

////////////////////////////////////////////////////////////////////////////////
// Public API

const defaultSupShutdownTimeout = 5 * time.Second

// Terminate is a synchronous procedure that halts the execution of the whole
// supervision tree.
func (sup Supervisor) Terminate() error {
	stopingTime := time.Now()
	sup.cancel()
	err := sup.wait(stopingTime, nil /* no startErr */)
	return err
}

// Wait blocks the execution of the current goroutine until the Supervisor
// finishes it execution.
func (sup Supervisor) Wait() error {
	return sup.wait(time.Time{}, nil /* no startErr */)
}

// GetName returns the name of the Spec used to start this Supervisor
func (sup Supervisor) GetName() string {
	return sup.spec.GetName()
}
