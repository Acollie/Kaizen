// Package reviewui serves the local web UI for `kaizen review`: a diff-centred
// code view, a changes sidebar, a call graph and a live feed of AI review events.
package reviewui

import (
	"context"
	"sync"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
)

// subscriberBuffer is how many events a subscriber channel holds before the
// subscriber goroutine waits; emitters never wait on subscribers.
const subscriberBuffer = 64

// EventLog is an append-only, replayable log of review events. It is safe for
// concurrent use. Subscribers read from the log at their own pace, so a slow
// subscriber never blocks Append.
type EventLog struct {
	mutex   sync.Mutex
	events  []review.Event
	closed  bool
	changed chan struct{} // closed and replaced whenever the log changes
}

// NewEventLog creates an empty, open event log
func NewEventLog() *EventLog {
	return &EventLog{changed: make(chan struct{})}
}

// Append stores an event, assigning the next sequence number (starting at 1)
// and a timestamp if it has none. Events appended after Close are dropped and
// returned with Seq 0.
func (eventLog *EventLog) Append(event review.Event) review.Event {
	eventLog.mutex.Lock()
	defer eventLog.mutex.Unlock()

	if eventLog.closed {
		event.Seq = 0
		return event
	}
	event.Seq = len(eventLog.events) + 1
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	eventLog.events = append(eventLog.events, event)
	eventLog.notifyLocked()
	return event
}

// Emit appends an event; its method value satisfies review.EmitFunc
func (eventLog *EventLog) Emit(event review.Event) {
	eventLog.Append(event)
}

// Since returns a copy of every event with a sequence number greater than seq
func (eventLog *EventLog) Since(seq int) []review.Event {
	pending, _, _ := eventLog.snapshot(seq)
	return pending
}

// Close marks the log as finished. Subscribers drain the remaining events and
// their channels are then closed.
func (eventLog *EventLog) Close() {
	eventLog.mutex.Lock()
	defer eventLog.mutex.Unlock()

	if eventLog.closed {
		return
	}
	eventLog.closed = true
	eventLog.notifyLocked()
}

// Closed reports whether Close has been called
func (eventLog *EventLog) Closed() bool {
	eventLog.mutex.Lock()
	defer eventLog.mutex.Unlock()
	return eventLog.closed
}

// LastSeq returns the sequence number of the newest event, or 0 if there are none
func (eventLog *EventLog) LastSeq() int {
	eventLog.mutex.Lock()
	defer eventLog.mutex.Unlock()
	return len(eventLog.events)
}

// Subscribe replays every event and then streams new ones until ctx is done
// or the log is closed and drained, at which point the channel is closed.
func (eventLog *EventLog) Subscribe(ctx context.Context) <-chan review.Event {
	return eventLog.SubscribeSince(ctx, 0)
}

// SubscribeSince is Subscribe starting after the given sequence number
func (eventLog *EventLog) SubscribeSince(ctx context.Context, seq int) <-chan review.Event {
	output := make(chan review.Event, subscriberBuffer)
	go eventLog.feed(ctx, seq, output)
	return output
}

// feed copies events after cursor into output, waiting for changes between batches
func (eventLog *EventLog) feed(ctx context.Context, cursor int, output chan<- review.Event) {
	defer close(output)

	for {
		pending, changed, closed := eventLog.snapshot(cursor)
		for _, event := range pending {
			select {
			case output <- event:
				cursor = event.Seq
			case <-ctx.Done():
				return
			}
		}
		if len(pending) > 0 {
			continue
		}
		if closed {
			return
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return
		}
	}
}

// snapshot returns the events after seq, a channel that closes on the next change, and the closed flag
func (eventLog *EventLog) snapshot(seq int) ([]review.Event, <-chan struct{}, bool) {
	eventLog.mutex.Lock()
	defer eventLog.mutex.Unlock()

	if seq < 0 {
		seq = 0
	}
	var pending []review.Event
	if seq < len(eventLog.events) {
		pending = make([]review.Event, len(eventLog.events)-seq)
		copy(pending, eventLog.events[seq:])
	}
	return pending, eventLog.changed, eventLog.closed
}

// notifyLocked wakes every waiting subscriber; the caller holds the mutex
func (eventLog *EventLog) notifyLocked() {
	close(eventLog.changed)
	eventLog.changed = make(chan struct{})
}
