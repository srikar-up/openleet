package eventbus

import (
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DefaultQueueCapacity is the number of non-resize events buffered per
// subscriber. At ~100 tokens per second this is several seconds of slack; a
// consumer that far behind is already the problem, and dropping old tokens is
// preferable to unbounded memory growth.
const DefaultQueueCapacity = 512

// Hub is a thread-safe fan-out broker. The zero value is not usable; call
// NewHub.
type Hub struct {
	mu     sync.RWMutex
	subs   map[*Subscription]struct{}
	seq    atomic.Uint64
	total  atomic.Uint64
	closed atomic.Bool
}

// NewHub constructs an empty broker.
func NewHub() *Hub {
	return &Hub{subs: make(map[*Subscription]struct{})}
}

// Subscribe registers a bounded subscriber queue. A capacity of zero uses
// DefaultQueueCapacity.
func (h *Hub) Subscribe(capacity int) *Subscription {
	if capacity <= 0 {
		capacity = DefaultQueueCapacity
	}
	sub := &Subscription{
		capacity: capacity,
		notify:   make(chan struct{}, 1),
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed.Load() {
		sub.closed = true
		return sub
	}
	h.subs[sub] = struct{}{}
	return sub
}

// Unsubscribe removes a subscriber and discards anything still queued.
func (h *Hub) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}
	h.mu.Lock()
	delete(h.subs, sub)
	h.mu.Unlock()
	sub.close()
}

// Publish enqueues an event for every subscriber. It is safe to call from any
// goroutine and never blocks: when a subscriber's queue is full the oldest
// lossy event is evicted instead.
func (h *Hub) Publish(kind Kind, msg tea.Msg) {
	if msg == nil || h.closed.Load() {
		return
	}
	event := Event{
		Kind: kind,
		Seq:  h.seq.Add(1),
		At:   time.Now(),
		Msg:  msg,
	}
	h.total.Add(1)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subs {
		sub.enqueue(event)
	}
}

// PublishSize publishes terminal geometry. The message type is tea.WindowSizeMsg
// because Bubble Tea's renderer only resets its cached width on that exact type.
func (h *Hub) PublishSize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	h.Publish(KindResize, tea.WindowSizeMsg{Width: width, Height: height})
}

// Published reports how many events were accepted by the broker.
func (h *Hub) Published() uint64 { return h.total.Load() }

// Close unregisters every subscriber. Publish becomes a no-op afterwards.
func (h *Hub) Close() {
	if !h.closed.CompareAndSwap(false, true) {
		return
	}
	h.mu.Lock()
	subs := make([]*Subscription, 0, len(h.subs))
	for sub := range h.subs {
		subs = append(subs, sub)
	}
	h.subs = make(map[*Subscription]struct{})
	h.mu.Unlock()
	for _, sub := range subs {
		sub.close()
	}
}

// Subscription is a lossy, bounded view of the hub. It is intentionally not a
// channel: the pump drains it synchronously, which keeps ordering under the
// consumer's control and avoids an intermediate goroutine per subscriber.
type Subscription struct {
	capacity int

	mu      sync.Mutex
	queue   []Event
	resize  *Event
	dropped uint64
	closed  bool

	// notify holds at most one pending wakeup. Coalescing wakeups is what makes
	// a token flood cost O(1) wakeups instead of O(n).
	notify chan struct{}
}

// Notify is signalled whenever events are available. It is edge-triggered with
// a one-slot buffer, so a consumer that drains after the signal still observes
// a later publish.
func (s *Subscription) Notify() <-chan struct{} { return s.notify }

// Drain returns everything currently queued, newest terminal geometry first.
func (s *Subscription) Drain() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || (s.resize == nil && len(s.queue) == 0) {
		return nil
	}

	events := make([]Event, 0, len(s.queue)+1)
	if s.resize != nil {
		events = append(events, *s.resize)
		s.resize = nil
	}
	events = append(events, s.queue...)
	for i := range s.queue {
		s.queue[i] = Event{} // release message references
	}
	s.queue = s.queue[:0]
	return events
}

// Dropped reports how many events were evicted for this subscriber.
func (s *Subscription) Dropped() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

func (s *Subscription) enqueue(event Event) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}

	// Terminal geometry collapses to the latest value and never queues.
	if event.Kind == KindResize {
		s.resize = &event
		s.mu.Unlock()
		s.wake()
		return
	}

	if len(s.queue) >= s.capacity && !s.makeRoomLocked(event.Kind) {
		// The queue is entirely lifecycle/agent traffic and this event is
		// lossy: drop the newcomer instead of a state transition.
		s.dropped++
		s.mu.Unlock()
		return
	}
	s.queue = append(s.queue, event)
	s.mu.Unlock()
	s.wake()
}

// makeRoomLocked frees one slot for an incoming event and reports whether the
// event may be enqueued.
func (s *Subscription) makeRoomLocked(incoming Kind) bool {
	if idx := s.firstLossyLocked(); idx >= 0 {
		s.queue = append(s.queue[:idx], s.queue[idx+1:]...)
		s.dropped++
		return true
	}
	// Only lifecycle/agent events remain. Tokens and logs are the least
	// valuable thing in the queue, so discard the incoming one.
	if incoming == KindToken || incoming == KindLog {
		return false
	}
	s.queue = s.queue[1:]
	s.dropped++
	return true
}

func (s *Subscription) firstLossyLocked() int {
	for i, event := range s.queue {
		if event.Kind == KindToken || event.Kind == KindLog {
			return i
		}
	}
	return -1
}

func (s *Subscription) wake() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *Subscription) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.queue = nil
	s.resize = nil
	s.mu.Unlock()
}
