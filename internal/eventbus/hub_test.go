package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type tokenMsg struct{ N int }

func countEvents(events []Event, kind Kind) int {
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

func TestPublishIsNonBlockingWhenQueueIsFull(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(8)

	// No consumer at all: 10k publishes must still return and stay accounted
	// for. This is the property OpenCode's blocking fan-out lacks.
	for i := 0; i < 10000; i++ {
		hub.Publish(KindToken, tokenMsg{N: i})
	}

	events := sub.Drain()
	if len(events) != 8 {
		t.Fatalf("expected the bounded queue to hold 8 events, got %d", len(events))
	}
	if got, want := sub.Dropped(), uint64(10000-8); got != want {
		t.Fatalf("dropped = %d, want %d", got, want)
	}
	// The newest events survive because eviction is oldest-first.
	if last, ok := events[len(events)-1].Msg.(tokenMsg); !ok || last.N != 9999 {
		t.Fatalf("expected the newest token to survive, got %#v", events[len(events)-1].Msg)
	}
}

func TestPublishNeverMutatesDeliveredMessages(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(4)
	hub.Publish(KindAgent, "agent-a")
	events := sub.Drain()
	if len(events) != 1 || events[0].Msg != "agent-a" {
		t.Fatalf("unexpected delivery: %#v", events)
	}
	if events[0].Seq != 1 {
		t.Fatalf("expected sequence 1, got %d", events[0].Seq)
	}
}

func TestResizeCoalescesToLatest(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(16)

	hub.PublishSize(80, 24)
	hub.PublishSize(120, 40)
	hub.PublishSize(189, 56)

	events := sub.Drain()
	if len(events) != 1 {
		t.Fatalf("expected one coalesced resize, got %d events", len(events))
	}
	size, ok := events[0].Msg.(tea.WindowSizeMsg)
	if !ok {
		t.Fatalf("expected tea.WindowSizeMsg, got %T", events[0].Msg)
	}
	if size.Width != 189 || size.Height != 56 {
		t.Fatalf("expected the newest size 189x56, got %dx%d", size.Width, size.Height)
	}
}

func TestLifecycleDisplacesTokenTraffic(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(4)

	for i := 0; i < 4; i++ {
		hub.Publish(KindToken, tokenMsg{N: i})
	}
	hub.Publish(KindLifecycle, "stream-done")

	events := sub.Drain()
	if len(events) != 4 {
		t.Fatalf("expected a full queue of 4, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.Kind != KindLifecycle || last.Msg != "stream-done" {
		t.Fatalf("lifecycle event was not preserved: %#v", events)
	}
	if sub.Dropped() != 1 {
		t.Fatalf("expected one evicted token, got %d drops", sub.Dropped())
	}
}

func TestTokenFloodCannotEvictLifecycle(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(3)

	for i := 0; i < 3; i++ {
		hub.Publish(KindLifecycle, "done")
	}
	for i := 0; i < 100; i++ {
		hub.Publish(KindToken, tokenMsg{N: i})
	}

	events := sub.Drain()
	if got := countEvents(events, KindLifecycle); got != 3 {
		t.Fatalf("expected all 3 lifecycle events to survive, got %d", got)
	}
	if sub.Dropped() != 100 {
		t.Fatalf("expected every token to be dropped, got %d", sub.Dropped())
	}
}

func TestConcurrentPublishersAreAccountedFor(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(64)

	const publishers = 8
	const perPublisher = 500

	var wg sync.WaitGroup
	var published atomic.Uint64
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perPublisher; i++ {
				hub.Publish(KindToken, tokenMsg{N: i})
				published.Add(1)
			}
		}()
	}
	wg.Wait()

	events := sub.Drain()
	if got := uint64(len(events)) + sub.Dropped(); got != publishers*perPublisher {
		t.Fatalf("delivered %d + dropped %d = %d, want %d",
			len(events), sub.Dropped(), got, publishers*perPublisher)
	}
	if published.Load() != publishers*perPublisher {
		t.Fatalf("publisher counter drifted: %d", published.Load())
	}
}

func TestCloseStopsDelivery(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(4)
	hub.Close()

	hub.Publish(KindToken, tokenMsg{N: 1})
	if events := sub.Drain(); len(events) != 0 {
		t.Fatalf("expected no delivery after Close, got %d", len(events))
	}
	if hub.Published() != 0 {
		t.Fatalf("closed hub accepted %d events", hub.Published())
	}
}
