package stest

import (
	// "context"
	// "sync"

	"context"
	"sync"

	"github.com/capatazlib/go-capataz/s"
)

////////////////////////////////////////////////////////////////////////////////

// EventManager provides an API that allows to block a goroutine for particular
// events in a test system
type EventManager struct {
	evCh         chan s.Event
	evDone       bool
	evBufferCond *sync.Cond
	evBuffer     *[]s.Event
}

// EventIterator represents a single iteration over the list of events that have
// been collected by the EventManager that created it.
type EventIterator struct {
	evIx      int
	evManager *EventManager
}

////////////////////////////////////////////////////////////////////////////////

func (em *EventManager) storeEvent(ev s.Event) {
	em.evBufferCond.L.Lock()
	defer em.evBufferCond.L.Unlock()
	*em.evBuffer = append(*em.evBuffer, ev)
	em.evBufferCond.Broadcast()
}

// Snapshot returns all the events that this EventManager has collected from the
// supervision system
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
func (ei *EventIterator) foldl(
	zero interface{},
	stepFn func(interface{}, s.Event) (bool, interface{}),
) interface{} {
	em := ei.evManager
	shouldContinue := true
	acc := zero

	for {
		for {
			em.evBufferCond.L.Lock()
			if ei.evIx >= len(*em.evBuffer) && !em.evDone {
				em.evBufferCond.Wait()
				em.evBufferCond.L.Unlock()
			} else {
				break
			}
		}
		shouldContinue, acc = stepFn(acc, (*em.evBuffer)[ei.evIx])
		em.evBufferCond.L.Unlock()

		ei.evIx++

		if !shouldContinue {
			break
		}
	}

	return acc
}

// SkipTill blocks until an event from the supervision system returns true for
// the given predicate
func (ei *EventIterator) SkipTill(pred EventP) {
	_ = ei.foldl(nil, func(_ interface{}, ev s.Event) (bool, interface{}) {
		if pred.Call(ev) {
			return false, nil
		}
		return true, nil
	})
}

// TakeTill takes all the events that have been collected since the current
// index until the given predicate returns true
func (ei *EventIterator) TakeTill(pred EventP) []s.Event {
	zero := make([]s.Event, 0, 100)
	iresult := ei.foldl(zero, func(iacc interface{}, ev s.Event) (bool, interface{}) {
		acc, _ := iacc.([]s.Event)
		if pred.Call(ev) {
			return false, acc
		}
		acc = append(acc, ev)
		return true, acc
	})
	result, _ := iresult.([]s.Event)
	return result
}

// Iterator returns an iterator over the collected events. This iterator
// will block waiting for new events
func (em EventManager) Iterator() EventIterator {
	it := EventIterator{evIx: 0, evManager: &em}
	return it
}

// NewEventManager returns an EventManager instance that can be used to wait for
// events to happen on the observed supervision system
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
