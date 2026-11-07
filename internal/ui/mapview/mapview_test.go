package mapview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/model"
)

func makeTestProblems() []*model.Problem {
	return []*model.Problem{
		{
			ID:             "001",
			Title:          "Two Sum",
			Slug:           "two_sum",
			Difficulty:     model.DifficultyEasy,
			Topics:         []string{"array", "hash-table"},
			Companies:      []string{"google", "amazon"},
			AcceptanceRate: 50.8,
			Description:    "Given an array of integers `nums` and an integer `target`, return indices of the two numbers.",
			TestCases: []model.TestCase{
				{ID: "tc1", Input: "[2,7,11,15]", Expected: "[0,1]"},
			},
		},
		{
			ID:             "002",
			Title:          "Add Two Numbers",
			Slug:           "add_two_numbers",
			Difficulty:     model.DifficultyMedium,
			Topics:         []string{"linked-list", "math"},
			Companies:      []string{"amazon"},
			AcceptanceRate: 38.5,
			Description:    "You are given two non-empty linked lists representing two non-negative integers.",
		},
		{
			ID:             "003",
			Title:          "Longest Substring",
			Slug:           "longest_substring",
			Difficulty:     model.DifficultyMedium,
			Topics:         []string{"hash-table", "string"},
			Companies:      []string{"google"},
			AcceptanceRate: 33.8,
			Description:    "Given a string s, find the length of the longest substring without repeating characters.",
		},
		{
			ID:             "004",
			Title:          "Median of Two Sorted Arrays",
			Slug:           "median_of_two_sorted_arrays",
			Difficulty:     model.DifficultyHard,
			Topics:         []string{"array", "binary-search"},
			Companies:      []string{"google"},
			AcceptanceRate: 29.1,
			Description:    "Given two sorted arrays nums1 and nums2, return the median of the two combined arrays.",
		},
		{
			ID:             "005",
			Title:          "Longest Palindromic Substring",
			Slug:           "longest_palindromic_substring",
			Difficulty:     model.DifficultyMedium,
			Topics:         []string{"string", "dynamic-programming"},
			Companies:      []string{"amazon"},
			AcceptanceRate: 32.4,
			Description:    "Given a string s, return the longest palindromic substring in s.",
		},
	}
}

// loadSeedProblem reads a real seed fixture from the embedded asset tree so the
// visual checks exercise production copy instead of a stub.
func loadSeedProblem(t *testing.T, dir string) *model.Problem {
	t.Helper()
	root := filepath.Join("..", "..", "..", "assets", "problems", dir)
	data, err := os.ReadFile(filepath.Join(root, "problem.json"))
	if err != nil {
		t.Fatalf("read problem.json: %v", err)
	}
	var problem model.Problem
	if err := json.Unmarshal(data, &problem); err != nil {
		t.Fatalf("decode problem.json: %v", err)
	}
	description, err := os.ReadFile(filepath.Join(root, "description.md"))
	if err != nil {
		t.Fatalf("read description.md: %v", err)
	}
	problem.Description = string(description)
	return &problem
}

func TestMapViewLifecycle(t *testing.T) {
	problems := makeTestProblems()
	userState := model.NewUserState()
	userState.MarkSolved("001", "python", "solution code")

	mv := New(problems, userState, 120, 35)

	if mv.Init() != nil {
		t.Fatal("expected nil init cmd")
	}

	rendered := mv.View()
	for _, want := range []string{"OPENLEET", "Two Sum", "SOLVED"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered view", want)
		}
	}

	mv, _ = mv.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
	if mv.width != 140 || mv.height != 45 {
		t.Fatalf("expected 140x45, got %dx%d", mv.width, mv.height)
	}
}

func TestMapViewNavigation(t *testing.T) {
	mv := New(makeTestProblems(), nil, 100, 30)

	if mv.SelectedIndex() != 0 {
		t.Fatalf("expected initial index 0, got %d", mv.SelectedIndex())
	}

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyRight})
	if mv.SelectedIndex() != 1 {
		t.Fatalf("expected index 1, got %d", mv.SelectedIndex())
	}

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if mv.SelectedIndex() != 0 {
		t.Fatalf("expected index 0, got %d", mv.SelectedIndex())
	}

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if mv.SelectedIndex() != 0 {
		t.Fatalf("expected index 0 after left clamp, got %d", mv.SelectedIndex())
	}

	mv, cmd := mv.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on Enter")
	}
	openMsg, ok := cmd().(OpenProblemMsg)
	if !ok || openMsg.Problem.ID != "001" {
		t.Fatalf("expected OpenProblemMsg for 001, got %v", openMsg)
	}
}

func TestMapViewSlashCommands(t *testing.T) {
	problems := makeTestProblems()
	mv := New(problems, nil, 100, 30)

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !mv.IsCommandMode() {
		t.Fatal("expected command mode after /")
	}

	res := ParseAndExecuteCommand("/company google", problems, "all")
	if res.ActiveFilter != "company:google" {
		t.Fatalf("expected filter company:google, got %s", res.ActiveFilter)
	}
	if got := len(FilterProblems(problems, res.ActiveFilter)); got != 3 {
		t.Fatalf("expected 3 google problems, got %d", got)
	}

	resSolve := ParseAndExecuteCommand("/solve 003", problems, "all")
	if !resSolve.OpenWorkspace || resSolve.TargetProblem == nil || resSolve.TargetProblem.ID != "003" {
		t.Fatalf("expected solve 003 to target problem 003, got %+v", resSolve)
	}

	resClear := ParseAndExecuteCommand("/clear", problems, "company:google")
	if resClear.ActiveFilter != "all" {
		t.Fatalf("expected filter 'all' after /clear, got %s", resClear.ActiveFilter)
	}
}

func TestMapViewTopicCycling(t *testing.T) {
	mv := New(makeTestProblems(), nil, 100, 30)

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.EqualFold(mv.activeFilter, "CP") {
		t.Fatalf("expected topic CP after Tab, got %s", mv.activeFilter)
	}

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if !strings.EqualFold(mv.activeFilter, "DSA") {
		t.Fatalf("expected topic DSA after Shift+Tab, got %s", mv.activeFilter)
	}

	mv, _ = mv.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !strings.EqualFold(mv.activeFilter, "DSA") {
		t.Fatalf("expected topic DSA after Esc, got %s", mv.activeFilter)
	}
}

func TestMapViewRenderOutput(t *testing.T) {
	problems := makeTestProblems()
	userState := model.NewUserState()
	userState.MarkSolved("001", "python", "solution")

	mv := New(problems, userState, 120, 40)
	output := mv.View()

	checks := []string{
		"OPENLEET/DSA",
		"DSA",       // rail item
		"LINK LIST", // rail item
		"roadmap",   // canvas badge
		"# 1. Two Sum",
		"✓", // solved tick tag above the card
		"Acceptance: 50.8%",
		"Topics:",
		"- array",
		"■ easy",
		"■ medium",
		"■ hard",
		"scroll ↓",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in rendered roadmap", want)
		}
	}
	if !strings.Contains(output, "1/5 solved") {
		t.Fatal("expected live solved counter in header")
	}
	if !strings.Contains(output, "SOLVED") {
		t.Fatal("expected solved call-to-action in inspector")
	}

	// Every rendered row must fit the terminal width exactly.
	for i, line := range strings.Split(output, "\n") {
		if got := ansi.StringWidth(line); got > 120 {
			t.Fatalf("row %d exceeds width: %d cells", i, got)
		}
	}

	// Narrow terminals collapse the inspector but keep the roadmap.
	mvNarrow := New(problems, userState, 84, 24)
	narrow := mvNarrow.View()
	if strings.Contains(narrow, "Acceptance:") {
		t.Fatal("expected inspector hidden below 96 columns")
	}
	if !strings.Contains(narrow, "OPENLEET") {
		t.Fatal("expected roadmap header on a narrow terminal")
	}
}

func TestMapViewInspectorShowsSeedCopy(t *testing.T) {
	problem := loadSeedProblem(t, "001_two_sum")
	mv := New([]*model.Problem{problem}, model.NewUserState(), 130, 44)
	output := mv.View()

	for _, want := range []string{
		"# 1. Two Sum",
		"** EASY",
		"Acceptance: 50.8%",
		"- hash-table",
		"- amazon",
		"Example 1:",
		"Input: nums = [2,7,11,15], target = 9",
		"Constraints:",
		"- 2 <= nums.length <= 10^4",
		"▶ SOLVE LEVEL (Enter / Click)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in inspector output", want)
		}
	}
}

func TestRenderFigmaPreview(t *testing.T) {
	problem := loadSeedProblem(t, "001_two_sum")
	userState := model.NewUserState()
	userState.MarkSolved("001", "python", "solution")

	problems := append([]*model.Problem{problem}, makeTestProblems()[1:]...)
	mv := New(problems, userState, 150, 44)
	t.Logf("\n%s", mv.View())
}

func TestMapViewMouseInteractions(t *testing.T) {
	problems := makeTestProblems()
	mv := New(problems, nil, 100, 30)

	// 1. Click the CP topic button on the left rail.
	mv, _ = mv.Update(tea.MouseMsg{
		X:      5,
		Y:      9,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !strings.EqualFold(mv.activeFilter, "CP") {
		t.Fatalf("expected topic CP after rail click, got %q", mv.activeFilter)
	}

	// 2. Click DSA again (the first button starts at row 4).
	mv, _ = mv.Update(tea.MouseMsg{
		X:      5,
		Y:      5,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if !strings.EqualFold(mv.activeFilter, "DSA") {
		t.Fatalf("expected topic DSA after rail click, got %q", mv.activeFilter)
	}

	// 3. Click the inspector to open the selected problem.
	_, cmd := mv.Update(tea.MouseMsg{
		X:      88,
		Y:      20,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatal("expected a command after clicking the inspector")
	}
	openMsg, ok := cmd().(OpenProblemMsg)
	if !ok || openMsg.Problem.ID != "001" {
		t.Fatalf("expected OpenProblemMsg for 001, got %v", openMsg)
	}

	// 4. Wheel scrolling walks the level list.
	mv, _ = mv.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if mv.SelectedIndex() != 1 {
		t.Fatalf("expected wheel-down to select index 1, got %d", mv.SelectedIndex())
	}
	mv, _ = mv.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	if mv.SelectedIndex() != 0 {
		t.Fatalf("expected wheel-up to return to index 0, got %d", mv.SelectedIndex())
	}
}

func TestMapViewCanvasClickSelectsCard(t *testing.T) {
	problems := makeTestProblems()
	mv := New(problems, nil, 120, 40)

	layout := computeBodyLayout(mv.Width())
	canvasHeight := mv.Height() - 4
	cells := computeCardCells(len(mv.filtered), mv.selectedIndex, layout.CanvasWidth, canvasHeight)
	if len(cells) < 2 {
		t.Fatalf("expected at least two cards, got %d", len(cells))
	}

	target := cells[1]
	x := layout.CanvasX + target.X + target.W/2
	y := 3 + target.Y + target.H/2

	mv, cmd := mv.Update(tea.MouseMsg{
		X:      x,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatal("expected a selection command after clicking a card")
	}
	if mv.SelectedIndex() != target.Index {
		t.Fatalf("expected card %d selected, got %d", target.Index, mv.SelectedIndex())
	}
}

func TestFilterByRoadmapCategory(t *testing.T) {
	problems := []*model.Problem{
		{ID: "001", Title: "DSA level", Category: "dsa"},
		{ID: "002", Title: "Graph level", Category: "graph"},
		{ID: "003", Title: "Legacy level"},
	}

	if got := len(FilterProblems(problems, "GRAPH")); got != 1 {
		t.Fatalf("expected 1 graph problem, got %d", got)
	}
	if got := len(FilterProblems(problems, "LINK LIST")); got != 0 {
		t.Fatalf("expected 0 link-list problems, got %d", got)
	}
	// Problems without a category belong to the DSA roadmap.
	if got := len(FilterProblems(problems, "DSA")); got != 2 {
		t.Fatalf("expected 2 DSA problems, got %d", got)
	}
	if got := len(FilterProblems(problems, "all")); got != 3 {
		t.Fatalf("expected all 3 problems, got %d", got)
	}
}

func makeScrollProblems() []*model.Problem {
	problems := makeTestProblems()
	return append(problems, &model.Problem{
		ID:             "006",
		Title:          "Trapping Rain Water",
		Slug:           "trapping_rain_water",
		Difficulty:     model.DifficultyHard,
		Topics:         []string{"array", "two-pointers", "stack"},
		Companies:      []string{"amazon"},
		AcceptanceRate: 59.8,
		Description:    "Given n non-negative integers representing an elevation map.",
	})
}

func TestWindingGridScrollsToSelectedCard(t *testing.T) {
	problems := makeScrollProblems()

	top := New(problems, nil, 80, 24)
	top.selectedIndex = 0
	topView := strings.ToLower(top.View())
	if !strings.Contains(topView, "two sum") {
		t.Fatal("expected the first card on screen at scroll 0")
	}
	if strings.Contains(topView, "trapping rain water") {
		t.Fatal("the last card should be below the fold at scroll 0")
	}

	bottom := New(problems, nil, 80, 24)
	bottom.selectedIndex = len(problems) - 1
	bottomView := strings.ToLower(bottom.View())
	if !strings.Contains(bottomView, "trapping rain water") {
		t.Fatal("selecting the last card did not scroll the canvas")
	}
	if strings.Contains(bottomView, "two sum") {
		t.Fatal("the canvas did not scroll away from the first card")
	}
}

func TestCardCellsFollowScroll(t *testing.T) {
	problems := makeScrollProblems()
	mv := New(problems, nil, 80, 24)
	mv.selectedIndex = len(problems) - 1

	layout := computeBodyLayout(80)
	canvasHeight := mv.Height() - 4
	canvas := computeCanvasLayout(canvasHeight)
	cells := computeCardCells(len(problems), mv.selectedIndex, layout.CanvasWidth, canvasHeight)

	var target *cardCell
	for i := range cells {
		if cells[i].Index == len(problems)-1 {
			target = &cells[i]
			break
		}
	}
	if target == nil {
		t.Fatal("the selected card has no visible hit box")
	}
	if target.Y < canvas.GridTop || target.Y+target.H > canvas.GridTop+canvas.GridHeight {
		t.Fatalf("card box %d..%d escapes the grid %d..%d",
			target.Y, target.Y+target.H, canvas.GridTop, canvas.GridTop+canvas.GridHeight)
	}
}

func TestCanvasClickUsesScrollAdjustedCells(t *testing.T) {
	problems := makeScrollProblems()
	mv := New(problems, nil, 80, 24)
	mv.selectedIndex = len(problems) - 1

	layout := computeBodyLayout(80)
	canvasHeight := mv.Height() - 4
	cells := computeCardCells(len(problems), mv.selectedIndex, layout.CanvasWidth, canvasHeight)

	var target *cardCell
	for i := range cells {
		if cells[i].Index == len(problems)-1 {
			target = &cells[i]
			break
		}
	}
	if target == nil {
		t.Fatal("the scrolled card has no hit box")
	}

	_, cmd := mv.Update(tea.MouseMsg{
		X:      layout.CanvasX + target.X + target.W/2,
		Y:      3 + target.Y + 1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatal("expected a command from the scrolled card")
	}
	msg, ok := cmd().(OpenProblemMsg)
	if !ok {
		t.Fatalf("expected OpenProblemMsg, got %T", msg)
	}
	if msg.Problem == nil || msg.Problem.ID != problems[len(problems)-1].ID {
		t.Fatalf("opened the wrong problem: %#v", msg.Problem)
	}
}

func TestSetFilterDifficultyAndTopic(t *testing.T) {
	problems := makeTestProblems()
	mv := New(problems, model.NewUserState(), 160, 40)

	mv.SetFilter("easy")
	if got := len(mv.filtered); got != 1 || mv.filtered[0].ID != "001" {
		t.Fatalf("expected the single easy problem, got %d", got)
	}
	if mv.ActiveFilter() != "easy" {
		t.Fatalf("expected easy filter, got %q", mv.ActiveFilter())
	}

	mv.SetFilter("string")
	if got := len(mv.filtered); got != 2 {
		t.Fatalf("expected two string problems, got %d", got)
	}
	if !mv.SelectProblem("003") {
		t.Fatal("expected to select a visible problem")
	}
}
