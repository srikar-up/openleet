package workspace

import (
	"testing"

	"openleet/internal/storage"
)

func TestSwitchLanguagePreservesOutgoingBuffer(t *testing.T) {
	prob := makeTestProblem()
	mgr, err := storage.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ws := New(prob, "python", mgr, nil, nil, 120, 36)
	ws.editor.SetContent("python_unique_marker\n")
	if err := mgr.SaveCodeBuffer(prob.ID, "go", "go_saved_marker\n"); err != nil {
		t.Fatal(err)
	}

	cmd := ws.SwitchLanguage("go")
	if cmd == nil {
		t.Fatal("expected a language switch command")
	}
	msg, ok := cmd().(LanguageSwitchedMsg)
	if !ok {
		t.Fatalf("expected LanguageSwitchedMsg, got %T", cmd())
	}
	if msg.Language != "go" || !msg.HasBuffer || msg.Content != "go_saved_marker\n" {
		t.Fatalf("unexpected switch message: %+v", msg)
	}
	if ws.Language() != "go" {
		t.Fatalf("expected go, got %s", ws.Language())
	}
	saved, err := mgr.LoadCodeBuffer(prob.ID, "python")
	if err != nil {
		t.Fatal(err)
	}
	if saved != "python_unique_marker\n" {
		t.Fatalf("outgoing buffer was not preserved: %q", saved)
	}
}

func TestResetCodeRestoresTemplateAndClearsBuffer(t *testing.T) {
	prob := makeTestProblem()
	mgr, err := storage.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ws := New(prob, "python", mgr, nil, nil, 120, 36)
	ws.editor.SetContent("broken_marker")
	if err := mgr.SaveCodeBuffer(prob.ID, "python", "broken_marker"); err != nil {
		t.Fatal(err)
	}

	cmd := ws.ResetCode()
	if cmd != nil {
		cmd()
	}
	if got := ws.Code(); got != prob.Templates["python"] {
		t.Fatalf("expected template after reset, got %q", got)
	}
	savedState, err := mgr.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if saved := savedState.GetCodeBuffer(prob.ID, "python"); saved != "" {
		t.Fatalf("expected cleared buffer, got %q", saved)
	}
}

func TestShowCoachAndHintFocusConsole(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	ws.ShowCoach()
	if ws.CurrentFocus() != FocusConsole || ws.console.activeTab != TabSocraticCoach {
		t.Fatal("expected the coach tab to own focus")
	}
	ws.ShowHint()
	if ws.CurrentFocus() != FocusConsole || ws.console.activeTab != TabHint {
		t.Fatal("expected the hint tab to own focus")
	}
}

func TestCursorAndConsoleResultAccessors(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)
	row, col := ws.Cursor()
	if row != 0 || col != 0 {
		t.Fatalf("expected initial cursor 0,0, got %d,%d", row, col)
	}
	if ws.ConsoleResult() != nil {
		t.Fatal("expected no test result before a run")
	}
}
