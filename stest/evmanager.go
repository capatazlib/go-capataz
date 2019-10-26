package stest

import (
	// "context"
	// "sync"

	"context"
	"sync"

	"github.com/capatazlib/go-capataz/s"
)

////////////////////////////////////////////////////////////////////////////////

type EventManager struct {
	evCh         chan s.Event
	evDone       bool
	evBufferCond *sync.Cond
	evBuffer     *[]s.Event
}

////////////////////////////////////////////////////////////////////////////////

func (em *EventManager) storeEvent(ev s.Event) {
	em.evBufferCond.L.Lock()
	defer em.evBufferCond.L.Unlock()
	*em.evBuffer = append(*em.evBuffer, ev)
	em.evBufferCond.Broadcast()
}

func (em EventManager) Snapshot() []s.Event {
	em.evBufferCond.L.Lock()
	defer em.evBufferCond.L.Unlock()
	return append((*em.evBuffer)[:0:0], *em.evBuffer...)
}

// StartCollector starts the goroutine that reads the evCh that gets filled
// by the supervision system
func (em EventManager) StartCollector(ctx context.Context) {
	go func() {
		for {
			select {
			// if context is done, return
			case <-ctx.Done():
				return
			case ev, ok := <-em.evCh:
				// if the channel was closed, return
				if !ok {
					em.evDone = true
					return
				}
				em.storeEvent(ev)
			}
		}
	}()
}

// EventCollector is used as an event notifier of a supervision system
func (em EventManager) EventCollector(ctx context.Context) func(ev s.Event) {
	return func(ev s.Event) {
		select {
		case <-ctx.Done():
		case em.evCh <- ev:
		}
	}
}

// foldl will iterate over a list of events emitted by a supervision system and
// block when waiting for new events to happen
func (em EventManager) foldl(
	zero interface{},
	stepFn func(interface{}, s.Event) (bool, interface{}),
) interface{} {
	evIx := 0

	shouldContinue := true
	acc := zero

	for {
		for {
			em.evBufferCond.L.Lock()
			if evIx >= len(*em.evBuffer) && !em.evDone {
				em.evBufferCond.Wait()
				em.evBufferCond.L.Unlock()
			} else {
				break
			}
		}
		shouldContinue, acc = stepFn(acc, (*em.evBuffer)[evIx])
		em.evBufferCond.L.Unlock()

		evIx++

		if !shouldContinue {
			break
		}
	}

	return acc
}

// WaitTill blocks until the given predicate returns true
func (em EventManager) WaitTill(pred EventP) {
	_ = em.foldl(nil, func(_ interface{}, ev s.Event) (bool, interface{}) {
		if pred.Call(ev) {
			return false, nil
		}
		return true, nil
	})
}

func NewEventManager() EventManager {
	var evBufferMux sync.Mutex
	evCh := make(chan s.Event)
	evBuffer := make([]s.Event, 0, 1000)
	em := EventManager{
		evCh:         evCh,
		evBufferCond: sync.NewCond(&evBufferMux),
		evBuffer:     &evBuffer,
	}
	return em
}
