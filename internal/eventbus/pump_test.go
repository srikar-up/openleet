package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type captureSender struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (s *captureSender) Send(msg tea.Msg) {
	s.mu.Lock()
	s.msgs = append(s.msgs, msg)
	s.mu.Unlock()
}

func (s *captureSender) snapshot() []tea.Msg {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]tea.Msg, len(s.msgs))
	copy(out, s.msgs)
	return out
}

func countTokens(msgs []tea.Msg) int {
	count := 0
	for _, msg := range msgs {
		if batch, ok := msg.(BatchMsg); ok {
			count += countEvents(batch.Events, KindToken)
		}
	}
	return count
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPumpCoalescesTokenBurst(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(0)
	sink := &captureSender{}
	pump := NewPump(sink, sub, PumpOptions{FrameInterval: 20 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	pump.Start(ctx)
	t.Cleanup(cancel)

	for i := 0; i < 400; i++ {
		hub.Publish(KindToken, tokenMsg{N: i})
	}

	waitFor(t, "all tokens to reach the program", func() bool {
		return countTokens(sink.snapshot()) == 400
	})

	if got := pump.Batches(); got >= 100 {
		t.Fatalf("expected coalescing, got %d batches for 400 tokens", got)
	}

	// Order is preserved across batch boundaries.
	seen := 0
	for _, msg := range sink.snapshot() {
		batch, ok := msg.(BatchMsg)
		if !ok {
			continue
		}
		for _, event := range batch.Events {
			token, ok := event.Msg.(tokenMsg)
			if !ok {
				continue
			}
			if token.N != seen {
				t.Fatalf("token order broke: got %d, want %d", token.N, seen)
			}
			seen++
		}
	}
}

func TestResizeBypassesTheBatchWindow(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(0)
	sink := &captureSender{}
	// A deliberately huge window: only resize can arrive before the tick.
	pump := NewPump(sink, sub, PumpOptions{FrameInterval: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pump.Start(ctx)

	hub.Publish(KindToken, tokenMsg{N: 1})
	hub.PublishSize(120, 40)

	waitFor(t, "the resize to be delivered", func() bool {
		for _, msg := range sink.snapshot() {
			if size, ok := msg.(tea.WindowSizeMsg); ok && size.Width == 120 && size.Height == 40 {
				return true
			}
		}
		return false
	})

	if countTokens(sink.snapshot()) != 0 {
		t.Fatal("a pending token batch was delivered before the frame window")
	}
}

func TestPumpFlushesAtMaxBatch(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(0)
	sink := &captureSender{}
	pump := NewPump(sink, sub, PumpOptions{
		FrameInterval: time.Hour,
		MaxBatch:      4,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pump.Start(ctx)

	for i := 0; i < 10; i++ {
		hub.Publish(KindToken, tokenMsg{N: i})
	}

	waitFor(t, "two full batches", func() bool {
		return countTokens(sink.snapshot()) >= 8
	})
	if got := pump.Batches(); got < 2 {
		t.Fatalf("expected at least 2 batches, got %d", got)
	}
}

func TestSlowSenderDoesNotBlockProducers(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(16)
	release := make(chan struct{})
	sender := &blockingSender{release: release}

	pump := NewPump(sender, sub, PumpOptions{FrameInterval: 5 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pump.Start(ctx)

	// The pump is stuck inside Send. Producers must still complete.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 5000; i++ {
			hub.Publish(KindToken, tokenMsg{N: i})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publishing blocked behind a stalled Program.Send")
	}

	close(release)
	cancel()
	pump.Wait()
}

type blockingSender struct {
	release chan struct{}
}

func (s *blockingSender) Send(tea.Msg) { <-s.release }

func TestPumpStopsOnCancel(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(0)
	pump := NewPump(&captureSender{}, sub, PumpOptions{FrameInterval: 5 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	pump.Start(ctx)
	cancel()

	waited := make(chan struct{})
	go func() {
		pump.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not stop after context cancellation")
	}
}
