package reviewui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
)

func statusEvent(message string) review.Event {
	return review.NewEvent(review.EventStatus, review.StatusData{Message: message})
}

func TestEventLogAssignsSequenceAndSince(t *testing.T) {
	eventLog := NewEventLog()
	var emit review.EmitFunc = eventLog.Emit
	for index := 0; index < 5; index++ {
		emit(statusEvent("step"))
	}
	all := eventLog.Since(0)
	if len(all) != 5 {
		t.Fatalf("expected 5 events, got %d", len(all))
	}
	for index, event := range all {
		if event.Seq != index+1 {
			t.Errorf("event %d has seq %d", index, event.Seq)
		}
	}
	if tail := eventLog.Since(3); len(tail) != 2 || tail[0].Seq != 4 {
		t.Errorf("Since(3) = %+v", tail)
	}
	if empty := eventLog.Since(9); len(empty) != 0 {
		t.Errorf("Since past the end should be empty, got %d", len(empty))
	}
}

func TestEventLogDropsAfterClose(t *testing.T) {
	eventLog := NewEventLog()
	eventLog.Emit(statusEvent("one"))
	eventLog.Close()
	eventLog.Close() // idempotent
	if dropped := eventLog.Append(statusEvent("late")); dropped.Seq != 0 {
		t.Errorf("append after close should be dropped, got seq %d", dropped.Seq)
	}
	if eventLog.LastSeq() != 1 {
		t.Errorf("LastSeq = %d", eventLog.LastSeq())
	}
}

func TestSubscribeReplaysThenStreamsThenCloses(t *testing.T) {
	eventLog := NewEventLog()
	eventLog.Emit(statusEvent("before-1"))
	eventLog.Emit(statusEvent("before-2"))

	events := eventLog.Subscribe(context.Background())
	expectSeq(t, events, 1)
	expectSeq(t, events, 2)

	eventLog.Emit(statusEvent("live"))
	expectSeq(t, events, 3)

	eventLog.Close()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected channel to close after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber channel did not close")
	}
}

func TestSubscribeStopsOnContextCancel(t *testing.T) {
	eventLog := NewEventLog()
	ctx, cancel := context.WithCancel(context.Background())
	events := eventLog.Subscribe(ctx)
	cancel()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("unexpected event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not stop on cancel")
	}
}

func TestSlowSubscriberDoesNotBlockEmitters(t *testing.T) {
	eventLog := NewEventLog()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = eventLog.Subscribe(ctx) // never read

	done := make(chan struct{})
	go func() {
		for index := 0; index < subscriberBuffer*10; index++ {
			eventLog.Emit(statusEvent("flood"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("emitters blocked on a slow subscriber")
	}
}

func TestEventLogConcurrentEmittersAndSubscribers(t *testing.T) {
	const emitters, perEmitter, subscribers = 8, 200, 4
	eventLog := NewEventLog()

	var readers sync.WaitGroup
	counts := make([]int, subscribers)
	for reader := 0; reader < subscribers; reader++ {
		readers.Add(1)
		go func(reader int) {
			defer readers.Done()
			last := 0
			for event := range eventLog.Subscribe(context.Background()) {
				if event.Seq != last+1 {
					t.Errorf("subscriber %d: seq %d after %d", reader, event.Seq, last)
				}
				last = event.Seq
				counts[reader]++
			}
		}(reader)
	}

	var writers sync.WaitGroup
	for writer := 0; writer < emitters; writer++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for index := 0; index < perEmitter; index++ {
				eventLog.Emit(statusEvent("concurrent"))
				_ = eventLog.Since(index)
			}
		}()
	}
	writers.Wait()
	eventLog.Close()
	readers.Wait()

	for reader, count := range counts {
		if count != emitters*perEmitter {
			t.Errorf("subscriber %d saw %d events, want %d", reader, count, emitters*perEmitter)
		}
	}
}

func expectSeq(t *testing.T, events <-chan review.Event, want int) {
	t.Helper()
	select {
	case event := <-events:
		if event.Seq != want {
			t.Fatalf("got seq %d, want %d", event.Seq, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for seq %d", want)
	}
}
