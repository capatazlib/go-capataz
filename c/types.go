package c

import (
	"context"
	"time"
)

// Restart specifies when a goroutine gets restarted
type Restart uint32

const (
	// Permanent Restart = iota
	// Temporary

	// Transient specifies that the goroutine should be restarted if and only if
	// the goroutine failed with an error. If the goroutine finishes without
	// errors it is not restarted again.
	Transient Restart = iota
)

// ShutdownTag specifies the type of Shutdown strategy that is used when
// stopping a goroutine
type ShutdownTag uint32

const (
	infinityT ShutdownTag = iota
	timeoutT
)

// Shutdown indicates how the parent supervisor will handle the stoppping of the
// child goroutine.
type Shutdown struct {
	tag      ShutdownTag
	duration time.Duration
}

// Inf specifies the parent supervisor must wait until Infinity for child
// goroutine to stop executing
var Inf = Shutdown{tag: infinityT}

// Timeout specifies a duration of time the parent supervisor will wait for the
// child goroutine to stop executing
func Timeout(d time.Duration) Shutdown {
	return Shutdown{
		tag:      timeoutT,
		duration: d,
	}
}

// Opt is used to configure a child's specification
type Opt func(*Spec)

// Spec represents the specification of a child; it serves as a template for the
// construction of worker goroutine. The Spec type is used in conjunction with
// the supervisor's Spec.
type Spec struct {
	name     string
	shutdown Shutdown
	restart  Restart
	start    func(context.Context, func() /* notifyStart */) error
}

// Child is the runtime representation of an Spec
type Child struct {
	runtimeName string
	spec        Spec
	cancel      func()
	wait        func(Shutdown) error
}
