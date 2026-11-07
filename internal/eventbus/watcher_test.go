package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWatcherPublishesOnlyChanges(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(8)

	var width, height atomic.Int64
	width.Store(80)
	height.Store(24)

	watcher := NewWatcher(hub, WatchOptions{
		Interval: 2 * time.Millisecond,
		Size: func() (int, int, bool) {
			return int(width.Load()), int(height.Load()), true
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)

	waitFor(t, "the initial size", func() bool { return watcher.Published() == 1 })

	// Several probes at the same size must not repaint the world.
	time.Sleep(25 * time.Millisecond)
	if got := watcher.Published(); got != 1 {
		t.Fatalf("expected 1 published size while stable, got %d", got)
	}

	width.Store(120)
	height.Store(40)
	waitFor(t, "the resize", func() bool { return watcher.Published() == 2 })

	cancel()
	watcher.Wait()

	events := sub.Drain()
	if len(events) != 1 {
		t.Fatalf("expected resize events to coalesce, got %d", len(events))
	}
	size, ok := events[0].Msg.(tea.WindowSizeMsg)
	if !ok {
		t.Fatalf("expected tea.WindowSizeMsg, got %T", events[0].Msg)
	}
	if size.Width != 120 || size.Height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", size.Width, size.Height)
	}
}

func TestWatcherIgnoresUnavailableSize(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(4)

	watcher := NewWatcher(hub, WatchOptions{
		Interval: 2 * time.Millisecond,
		Size:     func() (int, int, bool) { return 0, 0, false },
	})

	ctx, cancel := context.WithCancel(context.Background())
	watcher.Start(ctx)
	time.Sleep(15 * time.Millisecond)
	cancel()
	watcher.Wait()

	if watcher.Published() != 0 {
		t.Fatalf("expected no publishes for a piped terminal, got %d", watcher.Published())
	}
	if events := sub.Drain(); len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}
}
