package toast

import (
	"fmt"
	"time"
)

// Kind selects the semantic colour and default lifetime of a notification.
type Kind int

const (
	Info Kind = iota
	Success
	Warning
	Error
)

// String returns the lowercase kind name used by history rows and tests.
func (k Kind) String() string { return kindName(k) }

// Options carries the optional second line, action, and timing overrides.
type Options struct {
	Message    string
	ActionHint string
	ActionID   string
	Duration   time.Duration
	Sticky     bool
}

// Toast is one notification. Timestamps are assigned by the caller's clock so
// tests never depend on the wall clock.
type Toast struct {
	ID         int
	Kind       Kind
	Title      string
	Message    string
	ActionHint string
	ActionID   string
	Count      int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  time.Time
	Hovered    bool
	Focused    bool

	duration   time.Duration
	persistent bool
	pausedAt   time.Time
}

// Persistent reports whether the toast remains until it is dismissed.
func (t Toast) Persistent() bool { return t.persistent }

// Notifier owns the visible stack, the FIFO queue, and the bounded history.
// It never starts goroutines: the UI tick calls Tick with its current time.
type Notifier struct {
	visible []Toast
	queue   []Toast
	history []Toast
	nextID  int
	narrow  bool

	globalPaused bool
	ticking      bool
}

const (
	maxVisible = 3
	maxHistory = 20
	dedupeFor  = 2 * time.Second
)

// New creates an empty notification service.
func New() *Notifier {
	return &Notifier{}
}

func defaultLifetime(kind Kind) (time.Duration, bool) {
	switch kind {
	case Success:
		return 4 * time.Second, false
	case Warning:
		return 6 * time.Second, false
	case Error:
		return 0, true
	default:
		return 4 * time.Second, false
	}
}

// Push adds a notification, merges an identical one fired within two seconds,
// and promotes queued notifications while the visible stack has room.
func (n *Notifier) Push(kind Kind, title, message string, opts Options, now time.Time) Toast {
	for i := range n.visible {
		if identical(n.visible[i], kind, title, message, opts.ActionID) &&
			now.Sub(n.visible[i].UpdatedAt) <= dedupeFor {
			n.visible[i].Count++
			n.visible[i].UpdatedAt = now
			n.refreshExpiry(&n.visible[i], now)
			n.updateHistory(n.visible[i])
			return n.visible[i]
		}
	}
	for i := range n.queue {
		if identical(n.queue[i], kind, title, message, opts.ActionID) &&
			now.Sub(n.queue[i].UpdatedAt) <= dedupeFor {
			n.queue[i].Count++
			n.queue[i].UpdatedAt = now
			n.updateHistory(n.queue[i])
			return n.queue[i]
		}
	}

	duration, persistent := defaultLifetime(kind)
	if opts.Sticky {
		duration, persistent = 0, true
	} else if opts.Duration > 0 {
		duration, persistent = opts.Duration, false
	}

	n.nextID++
	t := Toast{
		ID:         n.nextID,
		Kind:       kind,
		Title:      title,
		Message:    message,
		ActionHint: opts.ActionHint,
		ActionID:   opts.ActionID,
		Count:      1,
		CreatedAt:  now,
		UpdatedAt:  now,
		duration:   duration,
		persistent: persistent,
	}
	n.history = append(n.history, t)
	if len(n.history) > maxHistory {
		n.history = append([]Toast(nil), n.history[len(n.history)-maxHistory:]...)
	}

	if n.narrow {
		// Narrow terminals show exactly one card, and it must always be the
		// newest notification. Older cards wait in the queue.
		if len(n.visible) > 0 {
			n.queue = append(append([]Toast(nil), n.visible...), n.queue...)
			n.visible = nil
		}
		n.visible = []Toast{n.activate(t, now)}
		return n.visible[0]
	}

	n.queue = append(n.queue, t)
	n.promote(now)
	for _, visible := range n.visible {
		if visible.ID == t.ID {
			return visible
		}
	}
	for _, queued := range n.queue {
		if queued.ID == t.ID {
			return queued
		}
	}
	return t
}

func (n *Notifier) activate(t Toast, now time.Time) Toast {
	if !t.persistent {
		t.ExpiresAt = now.Add(t.duration)
	}
	if n.globalPaused {
		t.pausedAt = now
	}
	return t
}

func identical(t Toast, kind Kind, title, message, actionID string) bool {
	return t.Kind == kind &&
		t.Title == title &&
		t.Message == message &&
		t.ActionID == actionID
}

func (n *Notifier) refreshExpiry(t *Toast, now time.Time) {
	if t.persistent {
		return
	}
	t.ExpiresAt = now.Add(t.duration)
}

func (n *Notifier) updateHistory(t Toast) {
	for i := len(n.history) - 1; i >= 0; i-- {
		if n.history[i].ID == t.ID {
			n.history[i].Count = t.Count
			n.history[i].UpdatedAt = t.UpdatedAt
			return
		}
	}
}

func (n *Notifier) visibleLimit() int {
	if n.narrow {
		return 1
	}
	return maxVisible
}

func (n *Notifier) promote(now time.Time) {
	limit := n.visibleLimit()
	for len(n.visible) < limit && len(n.queue) > 0 {
		t := n.queue[0]
		n.queue = n.queue[1:]
		n.visible = append(n.visible, n.activate(t, now))
	}
}

// Visible returns the painted stack in oldest-to-newest order.
func (n *Notifier) Visible() []Toast {
	out := make([]Toast, len(n.visible))
	copy(out, n.visible)
	return out
}

// History returns the last 20 notifications, newest first.
func (n *Notifier) History() []Toast {
	out := make([]Toast, len(n.history))
	for i, t := range n.history {
		out[len(n.history)-1-i] = t
	}
	return out
}

// HasActive reports whether anything is visible or waiting.
func (n *Notifier) HasActive() bool { return len(n.visible) > 0 || len(n.queue) > 0 }

// StartTick registers the single UI tick loop. It returns false when a loop is
// already running, so every producer can call it without spawning duplicates.
func (n *Notifier) StartTick() bool {
	if n.ticking {
		return false
	}
	n.ticking = true
	return true
}

// FinishTick releases the tick loop once nothing is left to expire.
func (n *Notifier) FinishTick() { n.ticking = false }

// Tick expires visible notifications and fills free slots from the queue. It
// reports whether the tick loop should keep running.
func (n *Notifier) Tick(now time.Time) bool {
	kept := n.visible[:0]
	for _, t := range n.visible {
		if t.persistent || t.ExpiresAt.IsZero() || !t.pausedAt.IsZero() || now.Before(t.ExpiresAt) {
			kept = append(kept, t)
		}
	}
	n.visible = kept
	n.promote(now)
	return n.HasActive()
}

// DismissNewest removes the newest visible toast so the next queued item can
// be promoted.
func (n *Notifier) DismissNewest(now time.Time) bool {
	if len(n.visible) == 0 {
		return false
	}
	n.visible = n.visible[:len(n.visible)-1]
	n.promote(now)
	return true
}

// Dismiss removes one toast by ID, visible or queued.
func (n *Notifier) Dismiss(id int, now time.Time) bool {
	for i, t := range n.visible {
		if t.ID == id {
			n.visible = append(n.visible[:i], n.visible[i+1:]...)
			n.promote(now)
			return true
		}
	}
	for i, t := range n.queue {
		if t.ID == id {
			n.queue = append(n.queue[:i], n.queue[i+1:]...)
			return true
		}
	}
	return false
}

// SetHovered pauses or resumes a visible toast's expiry timer.
func (n *Notifier) SetHovered(id int, hovered bool, now time.Time) {
	for i := range n.visible {
		if n.visible[i].ID != id {
			continue
		}
		n.visible[i].Hovered = hovered
		if hovered {
			n.pause(&n.visible[i], now)
		} else {
			n.resume(&n.visible[i], now)
		}
	}
}

// ClearHover clears every hover state.
func (n *Notifier) ClearHover(now time.Time) {
	for i := range n.visible {
		if n.visible[i].Hovered {
			n.visible[i].Hovered = false
			n.resume(&n.visible[i], now)
		}
	}
}

// SetFocused pauses or resumes a visible toast's expiry timer.
func (n *Notifier) SetFocused(id int, focused bool, now time.Time) {
	for i := range n.visible {
		if n.visible[i].ID != id {
			continue
		}
		n.visible[i].Focused = focused
		if focused {
			n.pause(&n.visible[i], now)
		} else {
			n.resume(&n.visible[i], now)
		}
	}
}

func (n *Notifier) pause(t *Toast, now time.Time) {
	if t.pausedAt.IsZero() {
		t.pausedAt = now
	}
}

func (n *Notifier) resume(t *Toast, now time.Time) {
	if t.pausedAt.IsZero() {
		return
	}
	if !t.ExpiresAt.IsZero() {
		t.ExpiresAt = t.ExpiresAt.Add(now.Sub(t.pausedAt))
	}
	t.pausedAt = time.Time{}
}

// SetNarrow changes the visible capacity between three and one.
func (n *Notifier) SetNarrow(narrow bool, now time.Time) {
	if n.narrow == narrow {
		return
	}
	n.narrow = narrow
	if narrow && len(n.visible) > 1 {
		keep := n.visible[len(n.visible)-1]
		older := append([]Toast(nil), n.visible[:len(n.visible)-1]...)
		n.visible = []Toast{keep}
		n.queue = append(older, n.queue...)
	}
	n.promote(now)
}

// SetGlobalPaused pauses every visible toast, for example while a modal is
// open and the host is hidden. New toasts start paused while this is true.
func (n *Notifier) SetGlobalPaused(paused bool, now time.Time) {
	if n.globalPaused == paused {
		return
	}
	n.globalPaused = paused
	for i := range n.visible {
		if paused {
			n.pause(&n.visible[i], now)
		} else {
			n.resume(&n.visible[i], now)
		}
	}
}

// Clear drops every pending and historical notification.
func (n *Notifier) Clear() {
	n.visible = nil
	n.queue = nil
	n.history = nil
	n.globalPaused = false
	n.ticking = false
}

// String is used in debug output and tests.
func (t Toast) String() string {
	return fmt.Sprintf("#%d %s %q x%d", t.ID, kindName(t.Kind), t.Title, t.Count)
}

func kindName(kind Kind) string {
	switch kind {
	case Success:
		return "success"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "info"
	}
}
