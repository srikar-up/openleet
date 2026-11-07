package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/coach"
	"openleet/internal/eventbus"
	"openleet/internal/model"
)

func TestBatchAppliesEveryEventInOnePass(t *testing.T) {
	app, _ := newThemeTestApp(t)

	batch := eventbus.BatchMsg{Events: []eventbus.Event{
		{Kind: eventbus.KindResize, Msg: tea.WindowSizeMsg{Width: 120, Height: 40}},
		{Kind: eventbus.KindLifecycle, Msg: tea.KeyMsg{Type: tea.KeyF4}},
	}}

	state, _ := app.Update(batch)
	app = state.(AppModel)

	if app.width != 120 || app.height != 40 {
		t.Fatalf("resize event not applied: %dx%d", app.width, app.height)
	}
	if app.currentView != ViewSettings {
		t.Fatalf("key event not applied: view is %v", app.currentView)
	}
}

func TestEmptyBatchIsANoOp(t *testing.T) {
	app, _ := newThemeTestApp(t)
	state, cmd := app.Update(eventbus.BatchMsg{})
	app = state.(AppModel)
	if cmd != nil {
		t.Fatal("expected no command from an empty batch")
	}
	if app.currentView != ViewHome {
		t.Fatalf("view changed to %v", app.currentView)
	}
}

func TestCoachBatchReachesTheWorkspaceThroughTheRoot(t *testing.T) {
	app, _ := newThemeTestApp(t)
	problem := &model.Problem{
		ID:         "001",
		Title:      "Two Sum",
		Slug:       "two_sum",
		Difficulty: model.DifficultyEasy,
		Topics:     []string{"array"},
	}
	app.problems = []*model.Problem{problem}
	app.workspace.SetProblem(problem)
	app.currentView = ViewSolve

	// Switch the console to the coach tab through the normal key path.
	state, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	app = state.(AppModel)

	// StartedMsg with no channels means the bus owns delivery; the console must
	// not schedule WaitForNextToken.
	state, cmd := app.Update(coach.CoachStreamStartedMsg{
		Provider: "test",
		StreamID: 7,
		Channels: coach.StreamChannels{},
	})
	app = state.(AppModel)
	if cmd != nil {
		t.Fatal("expected no token-reading command when the bus owns the stream")
	}

	state, _ = app.Update(coach.CoachTokenBatchMsg{StreamID: 7, Tokens: []string{"streamed", "-", "token"}})
	app = state.(AppModel)

	if view := app.View(); !strings.Contains(view, "streamed-token") {
		t.Fatalf("expected the streamed token in the view, got:\n%s", view)
	}

	// A stale completion from another stream must not close this one.
	state, _ = app.Update(coach.CoachStreamDoneMsg{StreamID: 8})
	app = state.(AppModel)
	if view := app.View(); !strings.Contains(view, "streamed-token") {
		t.Fatal("a stale stream ID ended the active stream")
	}
}
