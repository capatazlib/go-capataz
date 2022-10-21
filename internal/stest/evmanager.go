package stest

import (
	"context"
	"sync"

	"github.com/capatazlib/go-capataz/cap"
)

////////////////////////////////////////////////////////////////////////////////

// eventMsg is an envelope for supervision events, it is used to ensure the
// events are stored on the EventManager buffer in the expected order (no
// concurrency jitter).
type eventMsg struct {
	ev     cap.Event
	doneCh chan struct{}
}

func newEventMsg(ev cap.Event) eventMsg {
	doneCh := make(chan struct{})
	return eventMsg{
		ev:     ev,
		doneCh: doneCh,
	}
}

func (msg eventMsg) getEvent() cap.Event {
	return msg.ev
}

func (msg eventMsg) eventProcessed() {
	close(msg.doneCh)
}

func (msg eventMsg) waitForProcessing() {
	<-msg.doneCh
}

// EventManager provides an API that allows to block a goroutine for particular
// events in a test system
type EventManager struct {
	evCh         chan eventMsg
	evDone       bool
	evBufferCond *sync.Cond
	evBuffer     *[]cap.Event
}

// EventIterator represents a single iteration over the list of events that have
// been collected by the EventManager that created it.
type EventIterator struct {
	evIx      int
	evManager *EventManager
}

////////////////////////////////////////////////////////////////////////////////

func (em *EventManager) storeEvent(ev cap.Event) {
	em.evBufferCond.L.Lock()
	defer em.evBufferCond.L.Unlock()
	*em.evBuffer = append(*em.evBuffer, ev)
	em.evBufferCond.Broadcast()
}

// Snapshot returns all the events that this EventManager has collected from the
// supervision system
func (em EventManager) Snapshot() []cap.Event {
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
func (em EventManager) EventCollector(ctx context.Context) func(ev cap.Event) {
	return func(ev cap.Event) {
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
func (ei *EventIterator) foldl(
	zero interface{},
	stepFn func(interface{}, cap.Event) (bool, interface{}),
) interface{} {
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

// SkipTill blocks until an event from the supervision system returns true for
// the given predicate
// TODO: Change name to WaitTill
func (ei *EventIterator) SkipTill(pred EventP) {
	_ = ei.foldl(nil, func(_ interface{}, ev cap.Event) (bool, interface{}) {
		if pred.Call(ev) {
			return false, nil
		}
		return true, nil
	})
}

// TakeTill takes all the events that have been collected since the current
// index until the given predicate returns true
func (ei *EventIterator) TakeTill(pred EventP) []cap.Event {
	zero := make([]cap.Event, 0, 100)
	iresult := ei.foldl(zero, func(iacc interface{}, ev cap.Event) (bool, interface{}) {
		acc, _ := iacc.([]cap.Event)
		if pred.Call(ev) {
			return false, acc
		}
		acc = append(acc, ev)
		return true, acc
	})
	result, _ := iresult.([]cap.Event)
	return result
}

// Iterator returns an iterator over the collected events. This iterator
// will block waiting for new events
func (em EventManager) Iterator() EventIterator {
	it := EventIterator{evIx: 0, evManager: &em}
	return it
}

// GetEventIx returns the nth event that got emitted by a supervision system, if
// the given index is greater than the event buffer length, this function will
// wait until that index is reached. If the index is never reached, the second
// return value will be false.
func (em EventManager) GetEventIx(evIx int) (cap.Event, bool) {
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
		return cap.Event{}, false
	}
	return (*em.evBuffer)[evIx], true
}

// NewEventManager returns an EventManager instance that can be used to wait for
// events to happen on the observed supervision system
func NewEventManager() EventManager {
	var evBufferMux sync.Mutex
	evCh := make(chan eventMsg)
	evBuffer := make([]cap.Event, 0, 1000)
	em := EventManager{
		evCh:         evCh,
		evBufferCond: sync.NewCond(&evBufferMux),
		evBuffer:     &evBuffer,
	}
	return em
}
