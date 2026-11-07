package eventbus

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// DefaultResizeInterval is the polling cadence used where the terminal cannot
// signal resizes. 33ms matches a frame, so the perceived latency is the time it
// takes the window manager to finish dragging.
const DefaultResizeInterval = 33 * time.Millisecond

// SizeFunc reports the current terminal grid.
type SizeFunc func() (width, height int, ok bool)

// WatchOptions configures the geometry watcher.
type WatchOptions struct {
	// Interval between size probes. Zero uses DefaultResizeInterval.
	Interval time.Duration

	// Size is the probe implementation. Nil uses TerminalSize.
	Size SizeFunc
}

// Watcher polls the terminal size and publishes a tea.WindowSizeMsg whenever it
// changes. Bubble Tea v1 cannot report resizes on Windows at all (there is no
// SIGWINCH equivalent and its Windows input path never synthesises one), so the
// only reliable source is GetConsoleScreenBufferInfo via term.GetSize.
//
// The watcher is event-driven from the UI's perspective: it is just another
// producer on the Hub, not an input-loop hook. That is what makes a resize
// repaint even when no key is pressed.
type Watcher struct {
	hub      *Hub
	interval time.Duration
	size     SizeFunc

	started   sync.Once
	stopped   chan struct{}
	published atomic.Uint64
}

// NewWatcher constructs a geometry watcher. Call Start to run it.
func NewWatcher(hub *Hub, opts WatchOptions) *Watcher {
	interval := opts.Interval
	if interval <= 0 {
		interval = DefaultResizeInterval
	}
	size := opts.Size
	if size == nil {
		size = TerminalSize
	}
	return &Watcher{
		hub:      hub,
		interval: interval,
		size:     size,
		stopped:  make(chan struct{}),
	}
}

// Start launches the probe loop once.
func (w *Watcher) Start(ctx context.Context) {
	w.started.Do(func() {
		go w.run(ctx)
	})
}

// Wait blocks until the loop has exited.
func (w *Watcher) Wait() { <-w.stopped }

// Published reports how many size changes were announced.
func (w *Watcher) Published() uint64 { return w.published.Load() }

func (w *Watcher) run(ctx context.Context) {
	defer close(w.stopped)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	lastWidth, lastHeight := -1, -1
	check := func() {
		width, height, ok := w.size()
		if !ok || width <= 0 || height <= 0 {
			return
		}
		if width == lastWidth && height == lastHeight {
			return
		}
		lastWidth, lastHeight = width, height
		w.published.Add(1)
		w.hub.PublishSize(width, height)
	}

	// Announce the size seen at startup as well: it is the baseline the watcher
	// dedupes against, and on Windows the program may not have received one.
	check()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

// TerminalSize probes stdout, which is the stream Bubble Tea renders to. It
// returns ok=false for pipes, files, and CI logs so the watcher stays silent
// where there is no grid to track.
func TerminalSize() (int, int, bool) {
	fd := os.Stdout.Fd()
	if !isatty.IsTerminal(fd) && !isatty.IsCygwinTerminal(fd) {
		return 0, 0, false
	}
	width, height, err := term.GetSize(int(fd))
	if err != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
