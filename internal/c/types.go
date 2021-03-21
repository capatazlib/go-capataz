package c

import (
	"time"
)

// Child is the runtime representation of a Spec
type Child struct {
	runtimeName  string
	spec         ChildSpec
	restartCount uint32
	createdAt    time.Time
	cancel       func()
	wait         func(Shutdown) (error, bool)
}

// GetRuntimeName returns the name of this child (once started). It will have a
// prefix with the supervisor name
func (c Child) GetRuntimeName() string {
	return c.runtimeName
}

// GetName returns the name of the `ChildSpec` of this child
func (c Child) GetName() string {
	return c.spec.GetName()
}

// GetSpec returns the `ChildSpec` of this child
func (c Child) GetSpec() ChildSpec {
	return c.spec
}

// IsWorker indicates if this child is a worker
func (c Child) IsWorker() bool {
	return c.spec.IsWorker()
}

// GetTag returns the ChildTag of this ChildSpec
func (c Child) GetTag() ChildTag {
	return c.spec.GetTag()
}

// ChildNotification reports when a child has terminated; if it terminated with
// an error, it is set in the err field, otherwise, err will be nil.
type ChildNotification struct {
	name        string
	tag         ChildTag
	runtimeName string
	err         error
}

// GetName returns the spec name of the child that emitted this notification
func (ce ChildNotification) GetName() string {
	return ce.name
}

// RuntimeName returns the runtime name of the child that emitted this
// notification
func (ce ChildNotification) RuntimeName() string {
	return ce.runtimeName
}

// Unwrap returns the error reported by ChildNotification, if any.
func (ce ChildNotification) Unwrap() error {
	return ce.err
}
