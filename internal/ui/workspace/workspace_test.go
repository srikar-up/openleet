package workspace

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/coach"
	"openleet/internal/model"
	"openleet/internal/runner"
	"openleet/internal/storage"
	"openleet/internal/ui/theme"
)

func makeTestProblem() *model.Problem {
	return &model.Problem{
		ID:             "001",
		Title:          "Two Sum",
		Slug:           "two_sum",
		Difficulty:     model.DifficultyEasy,
		Topics:         []string{"Array", "Hash Table"},
		Companies:      []string{"Google", "Amazon"},
		AcceptanceRate: 49.2,
		Description:    "# Two Sum\n\nGiven an array of integers `nums` and an integer `target`...",
		Templates: map[string]string{
			"python": "class Solution:\n    def twoSum(self, nums, target):\n        pass\n",
			"go":     "package main\n\nfunc twoSum(nums []int, target int) []int {\n    return nil\n}\n",
		},
		TestCases: []model.TestCase{
			{ID: "tc1", Input: map[string]interface{}{"nums": []int{2, 7, 11, 15}, "target": 9}, Expected: []int{0, 1}},
		},
	}
}

func TestWorkspaceLifecycle(t *testing.T) {
	prob := makeTestProblem()
	detector := runner.NewDetector()
	exec := runner.NewExecutor(detector, 5*time.Second)

	ws := New(prob, "python", nil, exec, nil, 120, 36)

	if ws.CurrentFocus() != FocusEditor {
		t.Fatalf("expected initial focus FocusEditor, got %v", ws.CurrentFocus())
	}

	rendered := ws.View()
	if !strings.Contains(rendered, "Two Sum") {
		t.Fatal("expected problem title in rendered workspace")
	}
	if !strings.Contains(rendered, "solution.py") {
		t.Fatal("expected solution.py in editor header")
	}
	if !strings.Contains(rendered, "Test Results") {
		t.Fatal("expected Test Results in console tab bar")
	}

	// Window resize
	m, _ := ws.Update(tea.WindowSizeMsg{Width: 140, Height: 44})
	ws = m
	if ws.width != 140 || ws.height != 44 {
		t.Fatalf("expected dimensions 140x44, got %dx%d", ws.width, ws.height)
	}
}

func TestWorkspaceFocusCycling(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// Initial focus is FocusEditor (1)
	if ws.CurrentFocus() != FocusEditor {
		t.Fatalf("expected FocusEditor, got %v", ws.CurrentFocus())
	}

	// ctrl+w -> FocusConsole (2)
	m, _ := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	ws = m
	if ws.CurrentFocus() != FocusConsole {
		t.Fatalf("expected FocusConsole, got %v", ws.CurrentFocus())
	}

	// ctrl+w -> FocusDescription (0)
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	ws = m
	if ws.CurrentFocus() != FocusDescription {
		t.Fatalf("expected FocusDescription, got %v", ws.CurrentFocus())
	}

	// shift+tab -> FocusConsole (2)
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	ws = m
	if ws.CurrentFocus() != FocusConsole {
		t.Fatalf("expected FocusConsole, got %v", ws.CurrentFocus())
	}
}

func TestWorkspaceConsoleTabs(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// Toggle console tab via ctrl+t
	m, _ := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	ws = m
	view := ws.View()
	if !strings.Contains(view, "Socratic AI Coach") {
		t.Fatal("expected Socratic AI Coach view after ctrl+t tab switch")
	}
}

func TestWorkspaceCoachStreaming(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// Switch to Socratic Coach tab via ctrl+t
	m, _ := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	ws = m

	channels := coach.NewStreamChannels()
	go func() {
		defer close(channels.Tokens)
		for _, tok := range []string{"Have ", "you ", "considered ", "two pointers?"} {
			channels.Tokens <- tok
		}
	}()

	m, cmd := ws.Update(coach.CoachStreamStartedMsg{
		Provider: "cloud",
		Action:   coach.ActionHint,
		Channels: channels,
	})
	ws = m

	// Loop through streamed tokens
	for cmd != nil {
		msg := cmd()
		m, cmd = ws.Update(msg)
		ws = m
		if _, ok := msg.(coach.CoachStreamDoneMsg); ok {
			break
		}
	}

	rendered := ws.View()
	if !strings.Contains(rendered, "two pointers") {
		t.Fatal("expected coach streamed tokens in rendered view")
	}
}

func TestWorkspaceFailureDiagnosisNudge(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	failedResult := &model.ExecutionResult{
		ProblemID:   prob.ID,
		Language:    "python",
		TotalTests:  2,
		PassedTests: 1,
		Results: []model.TestResult{
			{TestCaseID: "tc1", Passed: true},
			{TestCaseID: "tc2", Passed: false, Expected: []int{1, 2}, Actual: []int{0, 0}},
		},
	}

	m, _ := ws.Update(TestsCompletedMsg{Result: failedResult})
	ws = m

	rendered := ws.View()
	if !strings.Contains(rendered, "Hint") {
		t.Fatal("expected 'Hint' in the console border after test failure")
	}

	ws.console.SwitchToTab(TabHint)
	hintView := ws.View()
	if !strings.Contains(hintView, "Failing case: tc2") {
		t.Fatal("expected the failing case in the Hint tab")
	}
	if !strings.Contains(hintView, "expected") || !strings.Contains(hintView, "got") {
		t.Fatal("expected expected/got details in the Hint tab")
	}
}

func TestWorkspaceRunsCustomFixture(t *testing.T) {
	detector := runner.NewDetector()
	if !detector.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}
	exec := runner.NewExecutor(detector, 5*time.Second)
	prob := makeTestProblem()
	ws := New(prob, "python", nil, exec, nil, 120, 40)
	ws.editor.SetContent("class Solution:\n    def twoSum(self, nums, target):\n        return [0, 1]\n")

	m, cmd := ws.Update(CustomTestRequestedMsg{
		Input:    map[string]interface{}{"nums": []int{2, 7}, "target": 9},
		Expected: []int{0, 1},
	})
	ws = m
	if cmd == nil {
		t.Fatal("expected a custom fixture command")
	}
	msg, ok := cmd().(CustomTestCompletedMsg)
	if !ok {
		t.Fatal("expected CustomTestCompletedMsg")
	}
	ws, _ = ws.Update(msg)
	ws.console.SwitchToTab(TabCustom)

	view := ws.View()
	if !strings.Contains(view, "CUSTOM PASSED") {
		t.Fatalf("expected the custom fixture result in the Custom tab, got:\n%s", view)
	}
}

func TestWorkspaceAutocomplete(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// Cycle focus to Console (FocusEditor -> FocusConsole)
	m, _ := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	ws = m
	if ws.CurrentFocus() != FocusConsole {
		t.Fatalf("expected FocusConsole, got %v", ws.CurrentFocus())
	}

	// Switch to coach tab
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	ws = m

	// Type "/h"
	ws.console.coachInput.SetValue("/h")

	// Press Tab in the coach input autocompletes.
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyTab})
	ws = m

	val := ws.console.coachInput.Value()
	if val != "/hint " {
		t.Fatalf("expected autocomplete to '/hint ', got '%s'", val)
	}
}

func TestWorkspaceRailTogglesPanes(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)

	layout := ws.layout()
	if !layout.ShowDescription || !layout.ShowSidebar {
		t.Fatal("expected both panes open at 120 columns")
	}
	if layout.LeftDividerX <= 0 || layout.RightDividerX <= layout.LeftDividerX {
		t.Fatalf("unexpected rail positions: left=%d right=%d", layout.LeftDividerX, layout.RightDividerX)
	}

	arrowY := dividerArrowRow(ws.height)
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.LeftDividerX,
		Y:      arrowY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !ws.descriptionHidden {
		t.Fatal("expected the description rail arrow to collapse the statement")
	}
	collapsed := ws.layout()
	if collapsed.ShowDescription || collapsed.LeftDividerX != 0 {
		t.Fatalf("expected a closed left rail at column 0, got show=%v x=%d", collapsed.ShowDescription, collapsed.LeftDividerX)
	}

	ws.lastRailPress = time.Time{}
	ws, _ = ws.Update(tea.MouseMsg{
		X:      0,
		Y:      arrowY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if ws.descriptionHidden {
		t.Fatal("expected clicking the closed left rail to reopen the statement")
	}

	layout = ws.layout()
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.RightDividerX,
		Y:      arrowY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !ws.sidebarHidden {
		t.Fatal("expected the session rail arrow to collapse the sidebar")
	}
	if ws.layout().ShowSidebar {
		t.Fatal("expected the session pane to be hidden")
	}

	closed := ws.layout()
	ws.lastRailPress = time.Time{}
	ws, _ = ws.Update(tea.MouseMsg{
		X:      closed.RightDividerX,
		Y:      arrowY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if ws.sidebarHidden {
		t.Fatal("expected clicking the closed right rail to reopen the session pane")
	}
}

func TestWorkspaceRailDragResizesPanes(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)
	layout := ws.layout()

	// Drag the statement rail right by eight cells.
	targetX := layout.LeftDividerX + 8
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.LeftDividerX,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	ws, _ = ws.Update(tea.MouseMsg{
		X:      targetX,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	if ws.descWidth != targetX {
		t.Fatalf("description width = %d, want %d", ws.descWidth, targetX)
	}
	if got := ws.layout().DescriptionWidth; got != targetX {
		t.Fatalf("layout description width = %d, want %d", got, targetX)
	}
	ws, _ = ws.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	if ws.dragEdge != 0 {
		t.Fatal("expected the drag to end on mouse release")
	}

	// Drag the session rail left by four cells.
	layout = ws.layout()
	targetX = layout.RightDividerX - 4
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.RightDividerX,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	ws, _ = ws.Update(tea.MouseMsg{
		X:      targetX,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	wantSidebar := ws.width - targetX - DividerWidth
	if ws.sidebarWidth != wantSidebar {
		t.Fatalf("sidebar width = %d, want %d", ws.sidebarWidth, wantSidebar)
	}
	if got := ws.layout().SidebarWidth; got != wantSidebar {
		t.Fatalf("layout sidebar width = %d, want %d", got, wantSidebar)
	}
}

func TestWorkspaceRailsKeepExactGrid(t *testing.T) {
	prob := makeTestProblem()
	for _, width := range []int{100, 119, 120, 160} {
		for _, hidden := range []struct {
			desc    bool
			sidebar bool
		}{
			{false, false},
			{true, false},
			{false, true},
			{true, true},
		} {
			ws := New(prob, "python", nil, nil, nil, width, 40)
			ws.descriptionHidden = hidden.desc
			ws.sidebarHidden = hidden.sidebar
			ws.synced = viewportLayout{}

			lines := strings.Split(ws.View(), "\n")
			if len(lines) != 40 {
				t.Fatalf("%dx40 closed=%v: got %d rows", width, hidden, len(lines))
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got != width {
					t.Fatalf("%dx40 closed=%v row %d: width %d", width, hidden, i, got)
				}
			}
			if width >= SidebarBreakpoint && !strings.Contains(ws.View(), "│") {
				t.Fatalf("%d columns: expected a divider rail", width)
			}
			if width < SidebarBreakpoint && ws.layout().ShowRightDivider {
				t.Fatalf("%d columns: expected no right rail", width)
			}
		}
	}
}

func TestWorkspaceRailKeyboardShortcuts(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)

	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}, Alt: true})
	if !ws.descriptionHidden {
		t.Fatal("expected alt+d to hide the statement")
	}
	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}, Alt: true})
	if ws.descriptionHidden {
		t.Fatal("expected alt+d to restore the statement")
	}

	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	if !ws.sidebarHidden {
		t.Fatal("expected alt+s to hide the session pane")
	}
	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	if ws.sidebarHidden {
		t.Fatal("expected alt+s to restore the session pane")
	}
}

func TestRailStylingAcrossThemes(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)
	ws.SetTheme(FromPalette(theme.Resolve("tron")))

	view := ws.View()
	if !strings.Contains(view, "‹") || !strings.Contains(view, "›") {
		t.Fatal("expected the single-cell rail toggle glyphs to be rendered")
	}
}

func TestWorkspaceCtrlBBeautifiesCode(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)
	ws.editor.SetContent("        def solve():\n            return 1\n")

	m, cmd := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	ws = m
	if cmd == nil {
		t.Fatal("expected ctrl+b to dispatch a beautify command")
	}
	msg, ok := cmd().(CodeFormattedMsg)
	if !ok {
		t.Fatalf("expected CodeFormattedMsg, got %T", msg)
	}
	ws, _ = ws.Update(msg)
	if !strings.HasPrefix(ws.Code(), "def solve():") {
		t.Fatalf("expected ctrl+b to dedent the buffer, got:\n%s", ws.Code())
	}
	if !strings.Contains(ws.Code(), "\n    return 1\n") {
		t.Fatalf("expected four-space block indentation, got:\n%s", ws.Code())
	}
}

func TestWorkspaceLayoutMath(t *testing.T) {
	// Expanded layout (>= 100 cols): right sidebar is enabled
	layout := ComputeLayout(120, 40)
	if !layout.ShowSidebar {
		t.Fatal("expected ShowSidebar true at 120 columns")
	}
	if layout.SidebarWidth != 23 {
		t.Fatalf("expected the 20%% sidebar to be 23 cells, got %d", layout.SidebarWidth)
	}
	if layout.DescriptionWidth <= 0 || layout.EditorWidth <= 0 || layout.ConsoleWidth <= 0 || layout.ConsoleHeight <= 0 {
		t.Fatal("expected positive dimensions for all panes")
	}
	if layout.ConsoleWidth != layout.EditorWidth {
		t.Fatalf("expected editor and console widths to match, got %d/%d", layout.EditorWidth, layout.ConsoleWidth)
	}

	// Compact layout (< 120 cols): right sidebar is hidden
	compactLayout := ComputeLayout(110, 34)
	if compactLayout.ShowSidebar {
		t.Fatal("expected ShowSidebar false below the 120-column breakpoint")
	}
}

func TestInnerContentDimensions(t *testing.T) {
	if width := InnerContentWidth(20, 1); width != 18 {
		t.Fatalf("expected content width 18 after padding, got %d", width)
	}
	if height := InnerContentHeight(12, 1); height != 10 {
		t.Fatalf("expected content height 10 after padding, got %d", height)
	}
	if width := InnerContentWidth(1, 1); width != 0 {
		t.Fatalf("expected undersized content width to clamp to 0, got %d", width)
	}
}

func TestWorkspaceLanguagePicker(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	if ws.Language() != "python" {
		t.Fatalf("expected initial language python, got %s", ws.Language())
	}
	if !strings.Contains(ws.View(), "solution.py") {
		t.Fatal("expected solution.py in initial view")
	}

	// ctrl+l opens language picker
	m, _ := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	ws = m
	view := ws.View()
	if !strings.Contains(view, "Select language") {
		t.Fatal("expected 'Select language' menu in view after ctrl+l")
	}

	// down + enter switches to "go"
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyDown})
	ws = m

	m, cmd := ws.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ws = m

	if ws.Language() != "go" {
		t.Fatalf("expected language switched to go, got %s", ws.Language())
	}
	if !strings.Contains(ws.View(), "solution.go") {
		t.Fatal("expected view to contain 'solution.go' after switching to go")
	}
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd emitting LanguageChangedMsg")
	}
	msg := cmd()
	langMsg, ok := msg.(LanguageChangedMsg)
	if !ok {
		batch, isBatch := msg.(tea.BatchMsg)
		if !isBatch {
			t.Fatalf("expected LanguageChangedMsg, got %T", msg)
		}
		for _, batchCmd := range batch {
			if batchCmd == nil {
				continue
			}
			if candidate, isLang := batchCmd().(LanguageChangedMsg); isLang {
				langMsg, ok = candidate, true
				break
			}
		}
	}
	if !ok {
		t.Fatal("expected LanguageChangedMsg in the command batch")
	}
	if langMsg.Language != "go" {
		t.Fatalf("expected LanguageChangedMsg.Language == 'go', got %s", langMsg.Language)
	}
}

func TestWorkspacePaneFocusState(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// initial editor focused
	if ws.CurrentFocus() != FocusEditor {
		t.Fatalf("expected initial focus FocusEditor, got %v", ws.CurrentFocus())
	}
	if !ws.editor.IsFocused() {
		t.Fatal("expected editor to be focused initially")
	}
	if ws.description.IsFocused() {
		t.Fatal("expected description to not be focused initially")
	}
	if ws.console.IsFocused() {
		t.Fatal("expected console to not be focused initially")
	}

	// ctrl+w -> console
	m, _ := ws.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	ws = m
	if ws.CurrentFocus() != FocusConsole {
		t.Fatalf("expected FocusConsole after ctrl+w, got %v", ws.CurrentFocus())
	}
	if ws.editor.IsFocused() {
		t.Fatal("expected editor not focused")
	}
	if ws.description.IsFocused() {
		t.Fatal("expected description not focused")
	}
	if !ws.console.IsFocused() {
		t.Fatal("expected console focused")
	}

	// ctrl+w -> description
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	ws = m
	if ws.CurrentFocus() != FocusDescription {
		t.Fatalf("expected FocusDescription after ctrl+w, got %v", ws.CurrentFocus())
	}
	if ws.editor.IsFocused() {
		t.Fatal("expected editor not focused")
	}
	if !ws.description.IsFocused() {
		t.Fatal("expected description focused")
	}
	if ws.console.IsFocused() {
		t.Fatal("expected console not focused")
	}

	// alt+2 -> editor
	m, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	ws = m
	if ws.CurrentFocus() != FocusEditor {
		t.Fatalf("expected FocusEditor after alt+2, got %v", ws.CurrentFocus())
	}
	if !ws.editor.IsFocused() {
		t.Fatal("expected editor focused after alt+2")
	}
	if ws.description.IsFocused() {
		t.Fatal("expected description not focused")
	}
	if ws.console.IsFocused() {
		t.Fatal("expected console not focused")
	}
}

func TestSaveCompletesAndReloadsBuffer(t *testing.T) {
	store, err := storage.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prob := makeTestProblem()
	code := "class Solution:\n    def twoSum(self, nums, target):\n        return []  # persisted\n"

	ws := New(prob, "python", store, nil, nil, 120, 40)
	ws.editor.SetContent(code)

	cmd := ws.SaveCode()
	if cmd == nil {
		t.Fatal("expected a save command")
	}
	if !ws.editor.isSaving {
		t.Fatal("expected the editor to enter saving mode")
	}
	msg, ok := cmd().(CodeSavedMsg)
	if !ok {
		t.Fatal("expected CodeSavedMsg")
	}
	ws, _ = ws.Update(msg)
	if ws.editor.isSaving {
		t.Fatal("expected saving mode to clear after CodeSavedMsg")
	}
	if ws.editor.isModified {
		t.Fatal("expected the saved buffer to be unmodified")
	}

	reopened := New(prob, "python", store, nil, nil, 120, 40)
	loadCmd := reopened.Init()
	if loadCmd == nil {
		t.Fatal("expected a buffer load command")
	}
	loaded, ok := loadCmd().(BufferLoadedMsg)
	if !ok {
		t.Fatal("expected BufferLoadedMsg")
	}
	if loaded.ProblemID != prob.ID {
		t.Fatalf("expected problem %s, got %s", prob.ID, loaded.ProblemID)
	}
	reopened, _ = reopened.Update(loaded)
	if !strings.Contains(reopened.Code(), "# persisted") {
		t.Fatalf("expected the persisted buffer, got:\n%s", reopened.Code())
	}
}

func TestFormatIconHitTarget(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)
	layout := ComputeLayout(ws.width, ws.height)
	iconX := layout.DescriptionWidth + layout.ColumnGap + layout.EditorWidth - 3 - lipgloss.Width(formatIcon)

	if !ws.formatIconHit(iconX, 0, layout) {
		t.Fatal("expected the beautify icon to be clickable")
	}
	if ws.formatIconHit(iconX-3, 0, layout) {
		t.Fatal("expected clicks left of the icon to miss")
	}
	if ws.formatIconHit(iconX, 1, layout) {
		t.Fatal("expected clicks below the header to miss")
	}

	_, cmd := ws.Update(tea.MouseMsg{
		X:      iconX,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if cmd == nil {
		t.Fatal("expected clicking the icon to dispatch a format command")
	}
}

func TestFormattedCodeReplacesBufferAsModified(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)
	formatted := "class Solution:\n    def twoSum(self, nums, target):\n        return []\n"

	ws, _ = ws.Update(CodeFormattedMsg{
		ProblemID: prob.ID,
		Language:  "python",
		Code:      formatted,
	})
	if ws.Code() != formatted {
		t.Fatalf("expected formatted buffer, got:\n%s", ws.Code())
	}
	if !ws.editor.isModified {
		t.Fatal("expected a beautified buffer to be unsaved")
	}
}

func TestStripDuplicateDescriptionHeading(t *testing.T) {
	raw := "# 16. Course Schedule\n\nGiven an array of `courses`, return true."
	got := stripDuplicateDescriptionHeading(raw, "Course Schedule")
	if strings.Contains(got, "Course Schedule") {
		t.Fatalf("expected the duplicate heading to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "Given an array of") {
		t.Fatalf("expected the body to survive, got:\n%s", got)
	}

	preserved := stripDuplicateDescriptionHeading("# Different Heading\n\nBody line.", "Course Schedule")
	if !strings.Contains(preserved, "Different Heading") {
		t.Fatal("expected a different first heading to be preserved")
	}
	if !strings.Contains(preserved, "Body line.") {
		t.Fatal("expected the body after a different heading to be preserved")
	}
}

func TestDescriptionDropsDuplicateHeadingKeepsCustomTitle(t *testing.T) {
	prob := &model.Problem{
		ID:          "16",
		Title:       "Course Schedule",
		Slug:        "course_schedule",
		Difficulty:  model.DifficultyMedium,
		Description: "# 16. Course Schedule\n\nThere are a total of `numCourses` courses.",
	}
	dm := NewDescriptionModel(prob, DefaultWorkspaceTheme(), 40, 12)
	view := dm.View()
	if !strings.Contains(view, "#16  Course Schedule") {
		t.Fatal("expected the custom #ID  Title header to stay")
	}
	if strings.Contains(view, "16. Course Schedule") {
		t.Fatal("expected the duplicated markdown heading to be stripped")
	}
	if !strings.Contains(view, "numCourses") {
		t.Fatal("expected the markdown body to render")
	}
}

func TestHintsFooterTogglesAndCounts(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)

	view := ws.View()
	if !strings.Contains(view, "▾ HINTS USED") {
		t.Fatal("expected the open hints footer marker")
	}
	if !strings.Contains(view, "No hints yet · ask with /hint") {
		t.Fatal("expected the empty hints detail row")
	}

	// Request a hint through the coach console and check the footer reflects it.
	ws.console.SwitchToTab(TabSocraticCoach)
	ws.console.SetFocused(true)
	ws.console.coachInput.SetValue("/hint")
	con, _ := ws.console.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ws.console = con
	if got := ws.console.HintsUsed(); got != 1 {
		t.Fatalf("expected HintsUsed 1, got %d", got)
	}
	view = ws.View()
	if !strings.Contains(view, "1 hint requested in Coach") {
		t.Fatal("expected the footer to report one hint used")
	}

	// Clicking the footer row collapses it to a single marker row.
	contentH := PanelContentHeight(ws.layout().DescriptionHeight)
	footerTop := 2 + contentH - ws.hintsFooterRows()
	ws, _ = ws.Update(tea.MouseMsg{
		X:      4,
		Y:      footerTop,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if ws.hintsOpen {
		t.Fatal("expected the footer click to collapse the hints footer")
	}
	view = ws.View()
	if !strings.Contains(view, "▸ HINTS USED  1") {
		t.Fatal("expected the closed marker row with the count")
	}
	if strings.Contains(view, "1 hint requested in Coach") {
		t.Fatal("expected the detail row hidden while closed")
	}

	// The description viewport height follows the footer state.
	wantDesc := PanelContentHeight(ws.layout().DescriptionHeight) - ws.hintsFooterRows()
	if got := ws.description.height; got != wantDesc {
		t.Fatalf("expected reduced description height %d, got %d", wantDesc, got)
	}

	// Clicking the (moved) footer row reopens the detail line.
	footerTop = 2 + contentH - ws.hintsFooterRows()
	ws, _ = ws.Update(tea.MouseMsg{
		X:      4,
		Y:      footerTop,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !ws.hintsOpen {
		t.Fatal("expected the second footer click to reopen the hints footer")
	}

	// Reset clears the counter.
	ws.console.Reset()
	if got := ws.console.HintsUsed(); got != 0 {
		t.Fatalf("expected HintsUsed 0 after reset, got %d", got)
	}
}

func TestRenderSidebarGridAndKeys(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 40)

	out := ws.renderSidebar(140, 40)
	for _, want := range []string{
		"Attempts   0",
		"RUN",
		"NAVIGATE",
		"ctrl+r",
		"run tests",
		"ctrl+b",
		"beautify",
		"tab",
		"alt+d",
		"alt+s",
		"alt+[",
		"alt+]",
		"ctrl+p",
		"drag rail",
		"resize",
		"dbl-click",
		"reset",
		"LAST RUN",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in the session sidebar, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Attempts0") {
		t.Fatal("expected a space between the Attempts label and the value")
	}
	if strings.Contains(out, "close/drag") {
		t.Fatal("expected the old rail hint to be gone")
	}

	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != 140 {
			t.Fatalf("row %d width %d, want 140 (%q)", i, got, line)
		}
	}
}

func TestRequestSaveHonorsFormatOnSave(t *testing.T) {
	store, err := storage.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prob := makeTestProblem()
	ws := New(prob, "python", store, nil, nil, 120, 40)

	opts := DefaultEditorOptions()
	opts.FormatOnSave = false
	ws.SetEditorOptions(opts)
	ws.editor.SetContent("class Solution:\n    def twoSum(self, nums, target):\n        return []\n")

	direct := ws.RequestSave()
	if direct == nil {
		t.Fatal("expected a direct save command with format-on-save off")
	}
	if ws.pendingFormatSave {
		t.Fatal("format-on-save off should not queue a format pass")
	}
	if msg := direct(); msg == nil {
		t.Fatal("direct save returned no message")
	} else if _, ok := msg.(CodeSavedMsg); !ok {
		t.Fatalf("format-on-save off: expected CodeSavedMsg, got %T", msg)
	}

	opts.FormatOnSave = true
	ws.SetEditorOptions(opts)
	formatCmd := ws.RequestSave()
	if formatCmd == nil {
		t.Fatal("expected a format command with format-on-save on")
	}
	if !ws.pendingFormatSave {
		t.Fatal("expected a pending save after the format pass")
	}
	msg := formatCmd()
	if _, ok := msg.(CodeFormattedMsg); !ok {
		t.Fatalf("expected CodeFormattedMsg, got %T", msg)
	}

	ws, saveCmd := ws.Update(msg)
	if ws.pendingFormatSave {
		t.Fatal("expected pending save to clear after the format result")
	}
	if saveCmd == nil {
		t.Fatal("expected the save command to follow the format result")
	}
	if saved := saveCmd(); saved == nil {
		t.Fatal("save command returned no message")
	} else if _, ok := saved.(CodeSavedMsg); !ok {
		t.Fatalf("expected CodeSavedMsg after formatting, got %T", saved)
	}
}

func TestLightWorkspaceUsesPaletteMarkdown(t *testing.T) {
	wt := FromPalette(theme.Resolve("openleet-light"))
	if !wt.Light {
		t.Fatal("expected wt.Light to be true")
	}

	cfg := markdownStyleConfig(wt)
	if cfg.Document.Color == nil || *cfg.Document.Color != "#211c17" {
		t.Fatalf("expected Document.Color #211c17, got %v", cfg.Document.Color)
	}
	if cfg.Code.BackgroundColor == nil || *cfg.Code.BackgroundColor != "#efe7da" {
		t.Fatalf("expected Code.BackgroundColor #efe7da, got %v", cfg.Code.BackgroundColor)
	}
	if cfg.H1.BackgroundColor != nil {
		t.Fatalf("expected H1.BackgroundColor to be nil, got %v", *cfg.H1.BackgroundColor)
	}
	if cfg.CodeBlock.Chroma == nil {
		t.Fatal("expected CodeBlock.Chroma to be non-nil")
	}
	if cfg.CodeBlock.Chroma.Background.BackgroundColor == nil || *cfg.CodeBlock.Chroma.Background.BackgroundColor != "#efe7da" {
		t.Fatalf("expected CodeBlock.Chroma.Background.BackgroundColor #efe7da, got %v", cfg.CodeBlock.Chroma.Background.BackgroundColor)
	}
}
