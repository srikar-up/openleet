package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Sender is the subset of *tea.Program the pump needs. Keeping it an interface
// lets tests drive the exact same loop without a terminal.
type Sender interface {
	Send(tea.Msg)
}

const (
	// DefaultFrameInterval caps how often a token/log burst is turned into a
	// frame. 33ms is 30 FPS: fast enough to read as live typing, slow enough
	// that a 200 token burst costs one layout pass instead of 200.
	DefaultFrameInterval = 33 * time.Millisecond

	// DefaultMaxBatch bounds a single BatchMsg. Without it a stalled pump could
	// accumulate unbounded work between ticks.
	DefaultMaxBatch = 256
)

// PumpOptions configures delivery coalescing.
type PumpOptions struct {
	// FrameInterval is the maximum delay before pending data events are sent.
	// A value <= 0 disables coalescing and delivers every drain immediately.
	FrameInterval time.Duration

	// MaxBatch flushes early once this many events are pending.
	MaxBatch int
}

func (o PumpOptions) withDefaults() PumpOptions {
	if o.MaxBatch <= 0 {
		o.MaxBatch = DefaultMaxBatch
	}
	return o
}

// Pump is the single goroutine that bridges a Subscription to Program.Send.
// One pump per program is enough: fan-in happens in the Hub, and ordering is
// preserved because everything funnels through this loop.
type Pump struct {
	sender  Sender
	sub     *Subscription
	opts    PumpOptions
	started sync.Once
	stopped chan struct{}

	batches   atomic.Uint64
	delivered atomic.Uint64
}

// NewPump constructs a pump. Call Start to run it.
func NewPump(sender Sender, sub *Subscription, opts PumpOptions) *Pump {
	return &Pump{
		sender:  sender,
		sub:     sub,
		opts:    opts.withDefaults(),
		stopped: make(chan struct{}),
	}
}

// Start launches the delivery loop. It is safe to call once; later calls are
// ignored.
func (p *Pump) Start(ctx context.Context) {
	p.started.Do(func() {
		go p.run(ctx)
	})
}

// Wait blocks until the loop has exited.
func (p *Pump) Wait() { <-p.stopped }

// Batches reports how many BatchMsg values were sent.
func (p *Pump) Batches() uint64 { return p.batches.Load() }

// Delivered reports how many individual events reached the program.
func (p *Pump) Delivered() uint64 { return p.delivered.Load() }

func (p *Pump) run(ctx context.Context) {
	defer close(p.stopped)

	interval := p.opts.FrameInterval
	var tick <-chan time.Time
	if interval > 0 {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		tick = ticker.C
	}

	pending := make([]Event, 0, p.opts.MaxBatch)

	flush := func() {
		if len(pending) == 0 {
			return
		}
		// Hand a copy to the program: pending is reused on the next tick.
		events := append([]Event(nil), pending...)
		pending = pending[:0]
		p.batches.Add(1)
		p.delivered.Add(uint64(len(events)))
		p.sender.Send(BatchMsg{Events: events, Dropped: p.sub.Dropped()})
	}

	deliver := func(events []Event) {
		for _, event := range events {
			if event.Kind == KindResize {
				// Geometry is never batched. A resize that waits 33ms behind a
				// token burst is exactly the lag this design exists to remove.
				p.delivered.Add(1)
				p.sender.Send(event.Msg)
				continue
			}
			pending = append(pending, event)
			if len(pending) >= p.opts.MaxBatch {
				flush()
			}
		}
		if interval <= 0 {
			flush()
		}
	}

	for {
		select {
		case <-ctx.Done():
			// Pending data is intentionally dropped on shutdown: the program is
			// gone or going, and Send could block behind a dead renderer.
			return

		case <-p.sub.Notify():
			deliver(p.sub.Drain())

		case <-tick:
			flush()
		}
	}
}
