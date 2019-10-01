package s

import (
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/c"
)

type Order uint32

const (
	LeftToRight Order = iota
	RightToLeft
)

func (o Order) Start(input0 []c.Spec) []c.Spec {
	input := append(input0[:0:0], input0...)
	switch o {
	case LeftToRight:
		return input
	case RightToLeft:
		for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
			input[i], input[j] = input[j], input[i]
		}
		return input
	default:
		panic("Invalid s.Order value")
	}
}

func (o Order) Stop(input0 []c.Spec) []c.Spec {
	input := append(input0[:0:0], input0...)
	switch o {
	case LeftToRight:
		for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
			input[i], input[j] = input[j], input[i]
		}
		return input
	case RightToLeft:
		return input
	default:
		panic("Invalid s.Order value")
	}
}

type Strategy uint32

const (
	OneForOne Strategy = iota
	// OneForAll
	// RestForOne
)

type EventTag uint32

const (
	eventNoop EventTag = iota
	ProcessStarted
	ProcessStopped
	ProcessFailed
)

func (tag EventTag) String() string {
	switch tag {
	case ProcessStarted:
		return "ProcessStarted"
	case ProcessStopped:
		return "ProcessStopped"
	case ProcessFailed:
		return "ProcessFailed"
	default:
		return "<Unknown>"
	}
}

type Event struct {
	tag     EventTag
	name    string
	err     error
	created time.Time
}

func (e Event) Tag() EventTag {
	return e.tag
}

func (e Event) Name() string {
	return e.name
}

func (e Event) Err() error {
	return e.err
}

func (e Event) Created() time.Time {
	return e.created
}

func (e Event) String() string {
	return fmt.Sprintf("Event{tag: %s, name: %s, created: %v}", e.tag, e.name, e.created)
}

type EventNotifier func(Event)

func (en EventNotifier) ProcessStopped(name string, err error) {
	tag := ProcessStopped
	if err != nil {
		tag = ProcessFailed
	}

	en(Event{
		tag:     tag,
		name:    name,
		err:     err,
		created: time.Now(),
	})
}

func (en EventNotifier) ProcessStarted(name string) {
	en(Event{
		tag:     ProcessStarted,
		name:    name,
		err:     nil,
		created: time.Now(),
	})
}

type Opt func(*Spec)

type Spec struct {
	name          string
	order         Order
	strategy      Strategy
	children      []c.Spec
	eventNotifier EventNotifier
}

type Supervisor struct {
	runtimeName string
	spec        Spec
	children    map[string]c.Child
	cancel      func()
	wait        func() error
}
