package toast

import (
	"fmt"
	"testing"
	"time"
)

func at(seconds int) time.Time {
	return time.Unix(1000, 0).Add(time.Duration(seconds) * time.Second)
}

func TestQueueCapsVisibleAtThreeAndPromotes(t *testing.T) {
	n := New()
	for i := 1; i <= 5; i++ {
		n.Push(Info, fmt.Sprintf("message %d", i), "", Options{}, at(i))
	}
	visible := n.Visible()
	if len(visible) != 3 {
		t.Fatalf("visible = %d, want 3", len(visible))
	}
	if visible[0].ID != 1 {
		t.Fatalf("oldest visible = %#v", visible[0])
	}
	if visible[2].ID != 3 {
		t.Fatalf("newest visible ID = %d, want 3", visible[2].ID)
	}
	if !n.HasActive() {
		t.Fatal("queue should keep the notifier active")
	}

	n.Tick(at(7))
	visible = n.Visible()
	if len(visible) != 2 || visible[0].ID != 4 || visible[1].ID != 5 {
		t.Fatalf("after expiry visible = %#v, want IDs 4,5", visible)
	}
	if n.Tick(at(20)) {
		t.Fatal("notifier should be idle after every toast expired")
	}
}

func TestNewestIsBottomOfVisibleSlice(t *testing.T) {
	n := New()
	n.Push(Info, "one", "", Options{}, at(1))
	n.Push(Success, "two", "", Options{}, at(2))
	visible := n.Visible()
	if visible[0].Title != "one" || visible[1].Title != "two" {
		t.Fatalf("visible order = %#v", visible)
	}
}

func TestDedupeWithinTwoSeconds(t *testing.T) {
	n := New()
	first := n.Push(Success, "Saved", "buffer.go", Options{ActionHint: "enter view", ActionID: "open"}, at(1))
	second := n.Push(Success, "Saved", "buffer.go", Options{ActionHint: "enter view", ActionID: "open"}, at(2))
	if first.ID != second.ID {
		t.Fatalf("deduped ID = %d, want %d", second.ID, first.ID)
	}
	visible := n.Visible()
	if len(visible) != 1 {
		t.Fatalf("visible = %d, want 1", len(visible))
	}
	if visible[0].Count != 2 {
		t.Fatalf("count = %d, want 2", visible[0].Count)
	}
	history := n.History()
	if len(history) != 1 || history[0].Count != 2 {
		t.Fatalf("history = %#v, want one entry with count 2", history)
	}

	third := n.Push(Success, "Saved", "buffer.go", Options{ActionHint: "enter view", ActionID: "open"}, at(5))
	if third.Count != 1 || third.ID == first.ID {
		t.Fatalf("outside 2s should create a new toast: %#v", third)
	}
}

func TestDefaultDurations(t *testing.T) {
	info := New().Push(Info, "info", "", Options{}, at(0))
	if got := info.ExpiresAt.Sub(at(0)); got != 4*time.Second {
		t.Fatalf("info lifetime = %v", got)
	}
	success := New().Push(Success, "success", "", Options{}, at(0))
	if got := success.ExpiresAt.Sub(at(0)); got != 4*time.Second {
		t.Fatalf("success lifetime = %v", got)
	}
	warning := New().Push(Warning, "warning", "", Options{}, at(0))
	if got := warning.ExpiresAt.Sub(at(0)); got != 6*time.Second {
		t.Fatalf("warning lifetime = %v", got)
	}
	errorToast := New().Push(Error, "error", "", Options{}, at(0))
	if !errorToast.Persistent() || !errorToast.ExpiresAt.IsZero() {
		t.Fatalf("error should be persistent: %#v", errorToast)
	}
	update := New().Push(Info, "update", "", Options{Duration: 8 * time.Second}, at(0))
	if got := update.ExpiresAt.Sub(at(0)); got != 8*time.Second {
		t.Fatalf("override lifetime = %v", got)
	}
}

func TestHoverPausesAndResumesTimer(t *testing.T) {
	n := New()
	first := n.Push(Info, "hover", "", Options{}, at(0))
	n.Tick(at(3))
	if len(n.Visible()) != 1 {
		t.Fatal("toast expired too early")
	}
	n.SetHovered(first.ID, true, at(3))
	n.Tick(at(30))
	if len(n.Visible()) != 1 {
		t.Fatal("hovered toast expired")
	}
	n.SetHovered(first.ID, false, at(30))
	n.Tick(at(30))
	if len(n.Visible()) != 1 {
		t.Fatal("toast should still be visible right after unhover")
	}
	// Expiry moved from t=4s to t=31s, so t=32s removes it.
	n.Tick(at(32))
	if len(n.Visible()) != 0 {
		t.Fatal("toast did not expire after the pause ended")
	}
}

func TestGlobalPauseHoldsNewToasts(t *testing.T) {
	n := New()
	n.SetGlobalPaused(true, at(0))
	n.Push(Info, "paused", "", Options{}, at(0))
	n.Tick(at(60))
	if len(n.Visible()) != 1 {
		t.Fatal("globally paused toast expired")
	}
	n.SetGlobalPaused(false, at(60))
	n.Tick(at(63))
	if len(n.Visible()) != 1 {
		t.Fatal("toast expired immediately after global pause lift")
	}
	n.Tick(at(65))
	if len(n.Visible()) != 0 {
		t.Fatal("toast should expire once the pause is lifted")
	}
}

func TestNarrowShowsOneToast(t *testing.T) {
	n := New()
	n.SetNarrow(true, at(0))
	n.Push(Info, "one", "", Options{}, at(0))
	n.Push(Info, "two", "", Options{}, at(0))
	n.Push(Info, "three", "", Options{}, at(0))
	if got := len(n.Visible()); got != 1 {
		t.Fatalf("narrow visible = %d, want 1", got)
	}
	if got := n.Visible()[0].Title; got != "three" {
		t.Fatalf("narrow kept %q, want newest", got)
	}
}

func TestHistoryKeepsLastTwentyNewestFirst(t *testing.T) {
	n := New()
	for i := 1; i <= 25; i++ {
		n.Push(Info, "entry", "", Options{ActionID: string(rune('a' + i))}, at(i))
	}
	history := n.History()
	if len(history) != 20 {
		t.Fatalf("history = %d, want 20", len(history))
	}
	if history[0].CreatedAt != at(25) || history[19].CreatedAt != at(6) {
		t.Fatalf("history order = %v .. %v", history[0].CreatedAt, history[19].CreatedAt)
	}
	n.DismissNewest(at(30))
	if len(n.History()) != 20 {
		t.Fatal("dismiss removed a history entry")
	}
}

func TestDismissNewestPromotesQueue(t *testing.T) {
	n := New()
	for i := 1; i <= 4; i++ {
		n.Push(Info, "entry", "", Options{ActionID: string(rune('a' + i))}, at(i))
	}
	if !n.DismissNewest(at(5)) {
		t.Fatal("dismiss reported no removal")
	}
	visible := n.Visible()
	if len(visible) != 3 || visible[2].ID != 4 {
		t.Fatalf("visible after dismiss = %#v, want 4 promoted", visible)
	}
	if !n.Dismiss(visible[0].ID, at(5)) {
		t.Fatal("dismiss by ID failed")
	}
	if len(n.Visible()) != 2 {
		t.Fatal("dismiss by ID did not remove the card")
	}
}

func TestIdenticalToastsHaveDistinctIDs(t *testing.T) {
	n := New()
	a := n.Push(Info, "same", "", Options{}, at(0))
	b := n.Push(Info, "same", "", Options{}, at(10))
	if a.ID == b.ID {
		t.Fatal("toasts outside the dedupe window must have distinct IDs")
	}
}
