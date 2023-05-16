package smtest

import (
	"context"
	"sync"
)

////////////////////////////////////////////////////////////////////////////////

// eventMsg is an envelope for supervision events, it is used to ensure the
// events are stored on the EventManager buffer in the expected order (no
// concurrency jitter).
type eventMsg[A any] struct {
	ev     A
	doneCh chan struct{}
}

func newEventMsg[A any](ev A) eventMsg[A] {
	doneCh := make(chan struct{})
	return eventMsg[A]{
		ev:     ev,
		doneCh: doneCh,
	}
}

func (msg eventMsg[A]) getEvent() A {
	return msg.ev
}

func (msg eventMsg[A]) eventProcessed() {
	close(msg.doneCh)
}

func (msg eventMsg[A]) waitForProcessing() {
	<-msg.doneCh
}

// EventManager provides an API that allows to block a goroutine for particular
// events in a test system
type EventManager[A any] struct {
	evCh         chan eventMsg[A]
	evDone       bool
	evBufferCond *sync.Cond
	evBuffer     *[]A
}

// EventIterator represents a single iteration over the list of events that have
// been collected by the EventManager that created it.
type EventIterator[A any] struct {
	evIx      int
	evManager *EventManager[A]
}

////////////////////////////////////////////////////////////////////////////////

func (em *EventManager[A]) storeEvent(ev A) {
	em.evBufferCond.L.Lock()
	defer em.evBufferCond.L.Unlock()
	*em.evBuffer = append(*em.evBuffer, ev)
	em.evBufferCond.Broadcast()
}

// Snapshot returns all the events that this EventManager has collected from the
// supervision system
func (em EventManager[A]) Snapshot() []A {
	em.evBufferCond.L.Lock()
	defer em.evBufferCond.L.Unlock()
	return append((*em.evBuffer)[:0:0], *em.evBuffer...)
}

// StartCollector starts the goroutine that reads the evCh that gets filled
// by the supervision system
func (em EventManager[A]) StartCollector(ctx context.Context) {
	go func() {
		for {
			select {
			// if context is done, return
			case <-ctx.Done():
				return
			case evMsg, ok := <-em.evCh:
				// if the channel was closed, return
				if !ok {
					em.evDone = true
					defer evMsg.eventProcessed()
					return
				}
				em.storeEvent(evMsg.getEvent())
				evMsg.eventProcessed()
			}
		}
	}()
}

// EventCollector is used as an event notifier of a supervision system
func (em EventManager[A]) EventCollector(ctx context.Context) func(ev A) {
	return func(ev A) {
		// NOTE: DO NOT REMOVE LINE BELLOW (DEBUG tool)
		// fmt.Printf("%+v\n", ev)
		evMsg := newEventMsg(ev)
		select {
		case <-ctx.Done():
		case em.evCh <- evMsg:
			// we are going to block until we know for sure that it was inserted in
			// the buffer of events
			evMsg.waitForProcessing()
		}
	}
}

// foldl will do a functional fold left (reduce) over a list of events emitted
// by a supervision system and block when waiting for new events to happen.
func foldl[A any, ACC any](
	ei *EventIterator[A],
	zero ACC,
	stepFn func(ACC, A) (bool, ACC),
) ACC {
	var shouldContinue bool
	acc := zero

	for {
		ev, ok := ei.evManager.GetEventIx(ei.evIx)
		if !ok {
			// we will never reach that index, stop here
			break
		}
		shouldContinue, acc = stepFn(acc, ev)

		ei.evIx++

		if !shouldContinue {
			// the reduce step function told us to stop, let's stop
			break
		}
	}

	return acc
}

// WaitTill blocks until an event from the supervision system returns true for
// the given predicate
func (ei *EventIterator[A]) WaitTill(pred EventP[A]) {
	_ = foldl(ei, nil, func(_ interface{}, ev A) (bool, interface{}) {
		if pred.Call(ev) {
			return false, nil
		}
		return true, nil
	})
}

// TakeTill takes all the events that have been collected since the current
// index until the given predicate returns true
func (ei *EventIterator[A]) TakeTill(pred EventP[A]) []A {
	zero := make([]A, 0, 100)
	result := foldl(ei, zero, func(acc []A, ev A) (bool, []A) {
		if pred.Call(ev) {
			return false, acc
		}
		acc = append(acc, ev)
		return true, acc
	})
	return result
}

// Iterator returns an iterator over the collected events. This iterator
// will block waiting for new events
func (em EventManager[A]) Iterator() EventIterator[A] {
	it := EventIterator[A]{evIx: 0, evManager: &em}
	return it
}

// GetEventIx returns the nth event that got emitted by a supervision system, if
// the given index is greater than the event buffer length, this function will
// wait until that index is reached. If the index is never reached, the second
// return value will be false.
func (em EventManager[A]) GetEventIx(evIx int) (A, bool) {
	defer em.evBufferCond.L.Unlock()

	for {
		// All the events that the parent EventManager collects come from a channel
		// that is read on a dedicated goroutine, and, in order to iterate over them
		// many times (on different iterator instances), the parent EventManager
		// must collect the events on a buffer. We iterate over this buffer with
		// this iterator index, and at the moment the iterator index is greater than
		// the buffer size, this means we need to wait for this buffer to get new
		// events in it. We break out of this inner loop when new entries are in the
		// buffer
		em.evBufferCond.L.Lock()
		if evIx >= len(*em.evBuffer) && !em.evDone {
			em.evBufferCond.Wait()
			em.evBufferCond.L.Unlock()
		} else {
			break
		}
	}
	// if the events are done, it means we did not reach the input evIx so we
	// should return an ok false
	if em.evDone {
		var empty A
		return empty, false
	}
	return (*em.evBuffer)[evIx], true
}

// NewEventManager returns an EventManager instance that can be used to wait for
// events to happen on the observed supervision system
func NewEventManager[A any]() EventManager[A] {
	var evBufferMux sync.Mutex
	evCh := make(chan eventMsg[A])
	evBuffer := make([]A, 0, 1000)
	em := EventManager[A]{
		evCh:         evCh,
		evBufferCond: sync.NewCond(&evBufferMux),
		evBuffer:     &evBuffer,
	}
	return em
}
