// Package eventbus decouples asynchronous producers from the Bubble Tea event
// loop.
//
// The contract is deliberately small:
//
//   - Producers call Hub.Publish from any goroutine. Publish never blocks and
//     never mutates UI state.
//   - A single Pump goroutine drains a Subscription and calls Program.Send.
//     Program.Send is the only bridge into the Elm loop, so every state
//     mutation still happens inside Update.
//   - High-frequency events are coalesced into one BatchMsg per frame, while
//     terminal geometry is delivered immediately because a resize must never
//     wait behind a token burst.
//
// This mirrors the subscription pattern used by OpenCode, but without its two
// known failure modes: Publish here is non-blocking with a bounded per-
// subscriber queue (OpenCode blocks producers on a slow consumer or silently
// drops events), and lifecycle events cannot be evicted by token traffic.
package eventbus

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Kind classifies an event so the transport can apply the right delivery
// policy. The zero value is invalid on purpose: an uncategorised event would
// otherwise be treated as high-frequency and could be dropped.
type Kind uint8

const (
	// KindToken is incremental streamed text (LLM output, coach answers).
	// High frequency and coalescable within a frame.
	KindToken Kind = iota + 1

	// KindLog is background diagnostic output. High frequency and lossy under
	// pressure.
	KindLog

	// KindAgent is an agent/activity state transition. Ordered and coalescable
	// within a frame, but not silently dropped in favour of tokens.
	KindAgent

	// KindLifecycle marks the start or end of a stream. Ordered and never
	// evicted by token traffic.
	KindLifecycle

	// KindResize carries a tea.WindowSizeMsg. Only the newest size matters, so
	// pending resize events collapse into one and bypass the frame batch.
	KindResize
)

// String implements fmt.Stringer for logs and test failures.
func (k Kind) String() string {
	switch k {
	case KindToken:
		return "token"
	case KindLog:
		return "log"
	case KindAgent:
		return "agent"
	case KindLifecycle:
		return "lifecycle"
	case KindResize:
		return "resize"
	default:
		return fmt.Sprintf("kind(%d)", uint8(k))
	}
}

// Event is one unit of work on the bus.
type Event struct {
	Kind Kind
	Seq  uint64
	At   time.Time
	Msg  tea.Msg
}

// BatchMsg is a coalesced delivery. AppModel applies every event in a single
// Update pass, so a burst of N tokens produces one View call instead of N.
type BatchMsg struct {
	Events  []Event
	Dropped uint64
}
