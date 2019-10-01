package c

import (
	"context"
	"time"
)

type Restart uint32

const (
	// Permanent Restart = iota
	// Transient
	// Temporary
	Transient Restart = iota
)

type ShutdownTag uint32

const (
	infinityT ShutdownTag = iota
	timeoutT
)

// Shutdown indicates how we would like to shutdown a child thread when stopping
// the system.
type Shutdown struct {
	tag      ShutdownTag
	duration time.Duration
}

var Inf Shutdown = Shutdown{tag: infinityT}

func Timeout(d time.Duration) Shutdown {
	return Shutdown{
		tag:      timeoutT,
		duration: d,
	}
}

type Opt func(*Spec)

type Spec struct {
	name     string
	shutdown Shutdown
	restart  Restart
	start    func(context.Context, func() /* notifyStart */) error
}

type Child struct {
	runtimeName string
	spec        Spec
	cancel      func()
	wait        func(Shutdown) error
}
