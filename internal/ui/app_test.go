package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/config"
	"openleet/internal/model"
	"openleet/internal/storage"
	"openleet/internal/ui/home"
	"openleet/internal/ui/toast"
	"openleet/internal/ui/workspace"
)

func TestAppModelLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_ui_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}

	app := NewAppModel(cfg, mgr)

	// 1. Init command
	initCmd := app.Init()
	if initCmd == nil {
		t.Fatal("expected non-nil init command")
	}

	// 2. WindowSizeMsg
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(AppModel)
	if app.width != 120 || app.height != 40 {
		t.Fatalf("expected dimensions 120x40, got %dx%d", app.width, app.height)
	}

	// 3. Load problems message
	problems, err := mgr.ListProblems()
	if err != nil {
		t.Fatal(err)
	}
	m, _ = app.Update(ProblemListLoadedMsg{Problems: problems})
	app = m.(AppModel)
	if len(app.problems) == 0 {
		t.Fatal("expected problems loaded into AppModel")
	}

	// 4. View rendering in Home mode (OpenCode-style landing page)
	renderedHome := app.View()
	if !strings.Contains(renderedHome, "Coach") {
		t.Fatal("expected Coach pill in rendered home view")
	}

	// Transition to Roadmap mode via F1
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyF1})
	app = m.(AppModel)
	if app.currentView != ViewRoadmap {
		t.Fatalf("expected ViewRoadmap after F1, got %v", app.currentView)
	}

	renderedRoadmap := app.View()
	if !strings.Contains(renderedRoadmap, "OPENLEET") {
		t.Fatal("expected OPENLEET banner in rendered roadmap view")
	}
	if !strings.Contains(renderedRoadmap, "Two Sum") {
		t.Fatal("expected 'Two Sum' in rendered roadmap view")
	}

	// 5. Navigation key events in Roadmap mode (KeyRight/KeyLeft along path)
	m, navCmd := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	if navCmd != nil {
		m, _ = m.Update(navCmd())
	}
	app = m.(AppModel)
	if app.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex 1 after right key, got %d", app.selectedIndex)
	}

	m, navCmd = app.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if navCmd != nil {
		m, _ = m.Update(navCmd())
	}
	app = m.(AppModel)
	if app.selectedIndex != 0 {
		t.Fatalf("expected selectedIndex 0 after left key, got %d", app.selectedIndex)
	}

	// 6. Open selected problem via Enter key -> transitions to ViewSolve
	m, enterCmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if enterCmd != nil {
		m, _ = m.Update(enterCmd())
	}
	app = m.(AppModel)
	if app.currentView != ViewSolve {
		t.Fatalf("expected currentView ViewSolve after Enter, got %v", app.currentView)
	}

	// 7. View rendering in Solve mode
	renderedSolve := app.View()
	t.Logf("Rendered Solve View:\n%s\n", renderedSolve)
	if !strings.Contains(renderedSolve, "Two Sum") {
		t.Fatal("expected problem title in rendered solve view")
	}
	if !strings.Contains(renderedSolve, "solution.py") {
		t.Fatal("expected 'solution.py' in rendered solve view")
	}
	if !strings.Contains(renderedSolve, "Test Results") {
		t.Fatal("expected 'Test Results' in rendered solve view")
	}

	// 8. Problem switching in Solve mode via ctrl+pgdown / ctrl+pgup.
	// ctrl+n now opens notification history globally.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlPgDown})
	app = m.(AppModel)
	if app.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex 1 after ctrl+pgdown, got %d", app.selectedIndex)
	}

	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlPgUp})
	app = m.(AppModel)
	if app.selectedIndex != 0 {
		t.Fatalf("expected selectedIndex 0 after ctrl+pgup, got %d", app.selectedIndex)
	}

	// ctrl+b is the workspace beautify action. It must not be consumed as a
	// previous-problem shortcut.
	m, formatCmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	app = m.(AppModel)
	if app.selectedIndex != 0 {
		t.Fatalf("expected ctrl+b to leave the problem selection alone, got %d", app.selectedIndex)
	}
	if formatCmd == nil {
		t.Fatal("expected ctrl+b to dispatch a beautify command")
	}

	// 9. ctrl+p opens the command palette over the current section
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	app = m.(AppModel)
	if !app.quick.paletteOpen {
		t.Fatal("expected the command palette overlay in solve after ctrl+p")
	}
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(AppModel)
	if app.quick.paletteOpen || app.currentView != ViewSolve {
		t.Fatal("expected Esc to return from the command palette to solve")
	}

	// Switch back to Solve for running tests
	m, _ = app.switchToSolveView(0)
	app = m.(AppModel)

	// 9. Run tests command via ctrl+r
	m, runCmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	app = m.(AppModel)
	if runCmd == nil {
		t.Fatal("expected non-nil runCmd from ctrl+r")
	}

	// 10. Dispatch TestsCompletedMsg
	resMsg := runCmd()
	m, _ = app.Update(resMsg)
	app = m.(AppModel)

	renderedWithResult := app.View()
	if !strings.Contains(renderedWithResult, "Test Results") {
		t.Fatal("expected console in rendered solve view with results")
	}

	// 11. Language is a quick popup, not a screen change.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	app = m.(AppModel)
	if !app.quick.paletteOpen {
		t.Fatal("expected the command palette overlay before selecting a command")
	}
	app = choosePaletteAction(t, app, "lang")
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)
	if !app.quick.dialogOpen {
		t.Fatal("expected the language popup")
	}
	app = chooseDialogID(t, app, "go")
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)
	if app.currentView != ViewSolve {
		t.Fatal("language quick action must not change the screen")
	}
	if app.config.DefaultLanguage != "go" {
		t.Fatalf("expected DefaultLanguage 'go', got %s", app.config.DefaultLanguage)
	}

	renderedGoSolve := app.View()
	if !strings.Contains(renderedGoSolve, "solution.go") {
		t.Fatal("expected 'solution.go' in editor header after switching to Go")
	}
	if !strings.Contains(renderedGoSolve, "Go ▾ · [saved]") {
		t.Fatal("expected 'Go ▾ · [saved]' status after switching to Go")
	}
}

// TestWindowSizeMsgPropagationSequence sends a sequence of resize events through the
// root AppModel and asserts that child models update their internal dimensions.
func TestWindowSizeMsgPropagationSequence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_resize_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewAppModel(cfg, mgr)
	_ = app.Init()

	sequence := []struct {
		w, h int
	}{
		{w: 120, h: 40},
		{w: 80, h: 25},
		{w: 140, h: 45},
	}

	for _, sz := range sequence {
		m, _ := app.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		app = m.(AppModel)

		if app.width != sz.w || app.height != sz.h {
			t.Fatalf("expected root AppModel dimensions %dx%d, got %dx%d", sz.w, sz.h, app.width, app.height)
		}

		// Verify child models received the resize event
		if app.homeView.Width() != sz.w {
			t.Fatalf("expected homeView width %d, got %d", sz.w, app.homeView.Width())
		}
		if app.mapView.Width() != sz.w {
			t.Fatalf("expected mapView width %d, got %d", sz.w, app.mapView.Width())
		}
		if app.workspace.Width() != sz.w {
			t.Fatalf("expected workspace width %d, got %d", sz.w, app.workspace.Width())
		}
	}
}

// TestRenderClampingAtSizes asserts that rendered output lines <= height and line width <= width.
func TestRenderClampingAtSizes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_clamp_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewAppModel(cfg, mgr)
	_ = app.Init()

	sizes := []struct {
		w, h int
	}{
		{w: 120, h: 40}, // Expanded
		{w: 70, h: 22},  // Normal
		{w: 50, h: 16},  // Compact (< 60x20: too small guard)
		{w: 40, h: 12},  // Ultra-compact
	}

	views := []ViewMode{ViewHome, ViewRoadmap, ViewSolve}

	for _, sz := range sizes {
		m, _ := app.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		app = m.(AppModel)

		for _, view := range views {
			app.currentView = view
			rendered := app.View()

			lines := strings.Split(rendered, "\n")
			if len(lines) > sz.h {
				t.Fatalf("view %v at %dx%d exceeded height: %d lines", view, sz.w, sz.h, len(lines))
			}

			for lineIdx, line := range lines {
				lineW := lipgloss.Width(line)
				if lineW > sz.w {
					t.Fatalf("view %v at %dx%d line %d exceeded width: %d > %d ('%s')",
						view, sz.w, sz.h, lineIdx, lineW, sz.w, line)
				}
			}
		}
	}
}

// TestSnapshotVisualAt120x40 verifies visual structure of Home, Roadmap, and Solve at 120x40.
func TestSnapshotVisualAt120x40(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_snapshot_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewAppModel(cfg, mgr)
	_ = app.Init()
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(AppModel)
	probs, _ := mgr.ListProblems()
	m, _ = app.Update(ProblemListLoadedMsg{Problems: probs})
	app = m.(AppModel)

	// 1. Home Snapshot
	app.currentView = ViewHome
	homeView := app.View()
	if !strings.Contains(homeView, "█") {
		t.Fatal("expected pixel-block wordmark in Home view")
	}
	if !strings.Contains(homeView, "ctrl+p commands") {
		t.Fatal("expected ctrl+p commands in Home footer")
	}

	// 2. Roadmap Snapshot
	app.currentView = ViewRoadmap
	roadmapView := app.View()
	if !strings.Contains(roadmapView, "OPENLEET") {
		t.Fatal("expected ROADMAP title in Roadmap view")
	}
	if !strings.Contains(roadmapView, "Acceptance:") {
		t.Fatal("expected problem details in 120x40 Roadmap view")
	}

	// 2b. Stats dashboard via F3
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyF3})
	app = m.(AppModel)
	if app.currentView != ViewStats {
		t.Fatalf("expected ViewStats after F3, got %v", app.currentView)
	}
	statsView := app.View()
	for _, want := range []string{"openleet / stats", "SOLVED", "best streak", "easy", "medium", "hard"} {
		if !strings.Contains(statsView, want) {
			t.Fatalf("expected %q in stats view", want)
		}
	}

	// 2c. The /stats palette command opens the same dashboard.
	app.currentView = ViewHome
	m, _ = app.Update(home.PaletteSelectionMsg{Action: home.ActionViewStats})
	app = m.(AppModel)
	if app.currentView != ViewStats {
		t.Fatalf("expected ViewStats after /stats, got %v", app.currentView)
	}

	// 3. Solve Snapshot
	app.currentView = ViewSolve
	solveView := app.View()
	if !strings.Contains(solveView, "solution.py") {
		t.Fatal("expected solution.py in Solve view")
	}
	if !strings.Contains(solveView, "Session") {
		t.Fatal("expected right sidebar Session in 120x40 Solve view")
	}
	if !strings.Contains(solveView, "home›roadmap›solve") {
		t.Fatal("expected the solve breadcrumb in the top bar")
	}
	if !strings.Contains(solveView, "(Two Sum)") {
		t.Fatal("expected the problem name in the solve breadcrumb")
	}
}

func TestCommandPaletteOverlayFromRoadmapAndSolve(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_palette_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewAppModel(cfg, mgr)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(AppModel)

	app.currentView = ViewRoadmap
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	app = m.(AppModel)
	if !app.quick.paletteOpen {
		t.Fatal("expected the command palette overlay from the roadmap")
	}
	if view := app.View(); !strings.Contains(view, "COMMANDS") || !strings.Contains(view, "/filter") {
		t.Fatal("expected the context-aware command list in the overlay")
	}
	if strings.Contains(app.View(), "/roadmap") {
		t.Fatal("navigation-only /roadmap must not be in the palette")
	}

	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(AppModel)
	if app.quick.paletteOpen || app.currentView != ViewRoadmap {
		t.Fatal("expected Esc to return to the roadmap")
	}

	app.currentView = ViewSolve
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	app = m.(AppModel)
	if !app.quick.paletteOpen {
		t.Fatal("expected the command palette overlay from solve")
	}
	app = choosePaletteAction(t, app, "themes")
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)
	if !app.quick.dialogOpen {
		t.Fatal("expected the theme popup")
	}
	if app.currentView != ViewSolve {
		t.Fatal("opening the theme popup must not leave solve")
	}
}

// choosePaletteAction moves the command palette cursor to an action ID.
func choosePaletteAction(t *testing.T, app AppModel, action string) AppModel {
	t.Helper()
	for i := 0; i < 64; i++ {
		if app.quick.palette.SelectedID() == action {
			return app
		}
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
		app = m.(AppModel)
	}
	t.Fatalf("command %q not found in the palette", action)
	return app
}

// chooseDialogID moves a quick-dialog cursor to an item ID.
func chooseDialogID(t *testing.T, app AppModel, id string) AppModel {
	t.Helper()
	for i := 0; i < 64; i++ {
		if app.quick.dialog.SelectedID() == id {
			return app
		}
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
		app = m.(AppModel)
	}
	t.Fatalf("item %q not found in the dialog", id)
	return app
}

func TestSolveFooterShowsProgressAndRunHints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_footer_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewAppModel(cfg, mgr)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 220, Height: 50})
	app = m.(AppModel)
	probs, _ := mgr.ListProblems()
	m, _ = app.Update(ProblemListLoadedMsg{Problems: probs})
	app = m.(AppModel)
	app, _ = app.switchToSolveView(0)

	view := app.View()
	for _, want := range []string{"ctrl+p commands", "ctrl+r run", "ctrl+s save"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in the solve footer", want)
		}
	}
	// The solved progress moved to the top bar stats; the footer stays
	// path + hints only.
	if strings.Contains(ansi.Strip(view), "░") {
		t.Fatal("the solve footer must not render a progress bar anymore")
	}
	if strings.Contains(view, "CONTEXT & SESSION") {
		t.Fatal("expected metadata to be shown once, not in the session sidebar")
	}

	// Navigation-only commands such as /focus are gone from the palette.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	app = m.(AppModel)
	if strings.Contains(app.View(), "/focus") {
		t.Fatal("navigation-only /focus must not be in the palette")
	}
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(AppModel)
}

func TestSavedBufferMessagesReachWorkspace(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_save_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewAppModel(cfg, mgr)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 220, Height: 50})
	app = m.(AppModel)
	probs, _ := mgr.ListProblems()
	m, _ = app.Update(ProblemListLoadedMsg{Problems: probs})
	app = m.(AppModel)
	app, _ = app.switchToSolveView(0)

	// A save must leave [saving...] even though the command completes after the
	// keypress that started it.
	saveCmd := app.workspace.SaveCode()
	if saveCmd == nil {
		t.Fatal("expected a save command")
	}
	m, _ = app.Update(saveCmd())
	app = m.(AppModel)
	view := app.View()
	if strings.Contains(view, "[saving...]") {
		t.Fatal("expected the editor to leave saving mode")
	}
	if !strings.Contains(view, "[saved]") {
		t.Fatal("expected the editor to show [saved]")
	}

	// Reopening the buffer loads the persisted code through the root update.
	p := app.workspace.Problem()
	persisted := "class Solution:\n    def twoSum(self, nums, target):\n        return []  # unique_marker\n"
	if err := app.storage.SaveCodeBuffer(p.ID, "python", persisted); err != nil {
		t.Fatal(err)
	}
	loadCmd := app.workspace.Init()
	if loadCmd == nil {
		t.Fatal("expected a buffer load command")
	}
	m, _ = app.Update(loadCmd())
	app = m.(AppModel)
	if !strings.Contains(app.View(), "unique_marker") {
		t.Fatal("expected the persisted buffer to reach the editor")
	}

	// A beautify result must also reach the workspace through the root.
	beautified := "class Solution:\n    def twoSum(self, nums, target):\n        return []  # beautified_marker\n"
	m, _ = app.Update(workspace.CodeFormattedMsg{
		ProblemID: p.ID,
		Language:  "python",
		Code:      beautified,
	})
	app = m.(AppModel)
	if !strings.Contains(app.View(), "beautified_marker") {
		t.Fatal("expected the beautified buffer to reach the editor")
	}
}

func TestWorkspaceLanguageMessagePersistsConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_lang_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppModel(cfg, mgr)
	m, _ := app.Update(workspace.LanguageChangedMsg{Language: "go"})
	app = m.(AppModel)
	if app.config.DefaultLanguage != "go" {
		t.Fatalf("expected go, got %s", app.config.DefaultLanguage)
	}
}

// TestSolveBarsStayExactWithLongTitles drives both bars through a range of
// widths with a deliberately long problem title in the breadcrumb.
func TestSolveBarsStayExactWithLongTitles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_bars_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppModel(cfg, mgr)

	longTitle := "Longest Substring Without Repeating Characters And An Even Longer Problem Title Here"
	app.problems = []*model.Problem{
		{
			ID:         "001",
			Title:      longTitle,
			Slug:       "longest_substring",
			Difficulty: model.DifficultyMedium,
		},
	}
	app.selectedIndex = 0
	app, _ = app.switchToSolveView(0)

	for _, width := range []int{100, 120, 160, 200} {
		m, _ := app.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		app = m.(AppModel)

		top := app.renderTopBar()
		footer := app.renderFooterBar()
		for name, bar := range map[string]string{"top": top, "footer": footer} {
			if got := lipgloss.Width(bar); got != width {
				t.Fatalf("%s bar at %d: width %d, want %d", name, width, got, width)
			}
			if !strings.HasSuffix(ansi.Strip(bar), " ") {
				t.Fatalf("%s bar at %d: expected trailing space", name, width)
			}
		}

		// The top bar is two plain labels: Settings and the account. It never
		// carries progress stats; those live under the Home search box.
		strippedTop := ansi.Strip(top)
		if !strings.Contains(strippedTop, "Settings") {
			t.Fatalf("top bar at %d: expected the Settings label, got %q", width, strippedTop)
		}
		if !strings.Contains(strippedTop, "Log in") {
			t.Fatalf("top bar at %d: expected the signed-out account label, got %q", width, strippedTop)
		}
		if strings.Contains(strippedTop, "streak") || strings.Contains(strippedTop, "solved") {
			t.Fatalf("top bar at %d: progress stats must not be in the header, got %q", width, strippedTop)
		}
		for _, banned := range []string{"[", "]", "│", "╭", "╮", "⚙", "●", "○", "*"} {
			if strings.Contains(strippedTop, banned) {
				t.Fatalf("top bar at %d: forbidden glyph %q in %q", width, banned, strippedTop)
			}
		}
		if width <= 120 {
			if !strings.Contains(strippedTop, "…") {
				t.Fatalf("top bar at %d: expected an ellipsis-truncated breadcrumb, got %q", width, strippedTop)
			}
		} else if !strings.Contains(strippedTop, longTitle) {
			t.Fatalf("top bar at %d: expected the full title, got %q", width, strippedTop)
		}

		// The bottom bar carries exactly three per-screen hints and never the
		// Settings entry, which lives in the top bar chips now.
		strippedFooter := ansi.Strip(footer)
		for _, want := range []string{"ctrl+r run", "ctrl+s save", "ctrl+p commands"} {
			if !strings.Contains(strippedFooter, want) {
				t.Fatalf("footer at %d: missing hint %q, got %q", width, want, strippedFooter)
			}
		}
		if strings.Contains(strippedFooter, "F4") || strings.Contains(strippedFooter, "settings") {
			t.Fatalf("footer at %d: must not advertise settings, got %q", width, strippedFooter)
		}
		if width >= 120 && !strings.Contains(strippedFooter, "ctrl+s save") {
			t.Fatalf("footer at %d: expected ctrl+s save intact", width)
		}
	}
}

func TestQuitConfirmationArmsAndCancels(t *testing.T) {
	mgr, err := storage.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Settings["general.confirm_quit"] = "on"
	app := NewAppModel(cfg, mgr)

	m, cmd := app.requestQuit()
	app = m.(AppModel)
	if !app.quitArmed {
		t.Fatal("first quit request should arm the confirmation")
	}
	if cmd == nil {
		t.Fatal("first quit request should schedule the notification tick")
	}
	if !hasVisibleToast(app, toast.Warning, "Quit OpenLeet?") {
		t.Fatalf("quit prompt missing: %#v", visibleToasts(app))
	}

	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	app = m.(AppModel)
	if app.quitArmed {
		t.Fatal("any other key should cancel the quit confirmation")
	}
	if hasVisibleToast(app, toast.Warning, "Quit OpenLeet?") {
		t.Fatal("cancel did not dismiss the quit prompt")
	}

	m, _ = app.requestQuit()
	app = m.(AppModel)
	if !app.quitArmed {
		t.Fatal("expected the prompt to be armed again")
	}
	_, quitCmd := app.requestQuit()
	if quitCmd == nil {
		t.Fatal("second quit request should exit")
	}
	if msg := quitCmd(); msg == nil {
		t.Fatal("quit command returned no message")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestQuitWithoutConfirmationExitsImmediately(t *testing.T) {
	mgr, err := storage.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppModel(config.DefaultConfig(), mgr)
	_, cmd := app.requestQuit()
	if cmd == nil {
		t.Fatal("quit should exit immediately when confirmation is off")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("quit command returned no message")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}
