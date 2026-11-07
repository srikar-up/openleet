package home

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"openleet/internal/config"
	"openleet/internal/model"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/uikit"
)

func makeTestProblems() []*model.Problem {
	return []*model.Problem{
		{
			ID:         "1",
			Slug:       "two-sum",
			Title:      "Two Sum",
			Difficulty: model.DifficultyEasy,
			Topics:     []string{"array", "hash-table"},
		},
		{
			ID:         "2",
			Slug:       "add-two-numbers",
			Title:      "Add Two Numbers",
			Difficulty: model.DifficultyMedium,
			Topics:     []string{"linked-list", "math"},
		},
	}
}

func TestLogoRendering(t *testing.T) {
	rendered := RenderLogo(
		lipgloss.AdaptiveColor{Light: "#5c5f77", Dark: "#a6adc8"},
		lipgloss.AdaptiveColor{Light: "#fe640b", Dark: "#fab387"},
		theme.Default().Background,
		100,
	)
	if !strings.Contains(rendered, "█") {
		t.Fatal("expected block characters in logo rendering")
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) != 7 {
		t.Fatalf("expected 7 rows in logo (five glyph rows plus block padding), got %d", len(lines))
	}
}

func TestPaletteProblemFilter(t *testing.T) {
	problems := makeTestProblems()
	pm := NewPaletteModel(problems, 60)

	// Initial empty or whitespace query should yield zero items
	pm.Filter("")
	if pm.ItemCount() != 0 {
		t.Fatalf("expected 0 items on empty query, got %d", pm.ItemCount())
	}
	pm.Filter("   ")
	if pm.ItemCount() != 0 {
		t.Fatalf("expected 0 items on whitespace query, got %d", pm.ItemCount())
	}

	// Filter by title ("Two Sum")
	pm.Filter("Two Sum")
	if pm.ItemCount() != 1 {
		t.Fatalf("expected 1 item for 'Two Sum', got %d", pm.ItemCount())
	}
	if sel := pm.SelectedItem(); sel == nil || sel.Problem == nil || sel.Problem.ID != "1" {
		t.Fatalf("expected Problem #1 selected, got %+v", sel)
	}

	// Filter by topic ("math")
	pm.Filter("math")
	if pm.ItemCount() != 1 {
		t.Fatalf("expected 1 item for topic 'math', got %d", pm.ItemCount())
	}
	if sel := pm.SelectedItem(); sel == nil || sel.Problem == nil || sel.Problem.ID != "2" {
		t.Fatalf("expected Problem #2 selected, got %+v", sel)
	}

	// Filter by difficulty ("medium")
	pm.Filter("medium")
	if pm.ItemCount() != 1 {
		t.Fatalf("expected 1 item for difficulty 'medium', got %d", pm.ItemCount())
	}
	if sel := pm.SelectedItem(); sel == nil || sel.Problem == nil || sel.Problem.ID != "2" {
		t.Fatalf("expected Problem #2 selected, got %+v", sel)
	}
}

func TestPaletteProblemSearch(t *testing.T) {
	problems := makeTestProblems()
	pm := NewPaletteModel(problems, 60)

	// Search problem by name
	pm.Filter("two sum")
	if pm.ItemCount() != 1 {
		t.Fatalf("expected 1 problem match for 'two sum', got %d", pm.ItemCount())
	}
	item := pm.SelectedItem()
	if !item.IsProblem || item.Problem == nil || item.Problem.ID != "1" {
		t.Fatalf("expected Problem #1 Two Sum, got %+v", item)
	}
}

func TestHomeModelLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	problems := makeTestProblems()
	userState := model.NewUserState()

	m := New(cfg, problems, userState, 100, 30)

	// Initial view rendering
	rendered := m.View()
	if !strings.Contains(rendered, "Coach") {
		t.Fatal("expected 'Coach' pill in rendered home view")
	}
	if !strings.Contains(rendered, "Python") {
		t.Fatal("expected 'Python' in rendered home view")
	}

	// Typing "/" must NOT open search
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m.IsPaletteOpen() {
		t.Fatal("expected search to remain closed when '/' is typed")
	}

	// ctrl+p must return OpenCommandsMsg
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if cmd == nil {
		t.Fatal("expected cmd on ctrl+p")
	}
	msg := cmd()
	if _, ok := msg.(OpenCommandsMsg); !ok {
		t.Fatalf("expected OpenCommandsMsg, got %#v", msg)
	}
	if m.IsPaletteOpen() {
		t.Fatal("expected search to remain closed on ctrl+p")
	}

	// Typing problem title opens problem search
	for _, r := range "two" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !m.IsPaletteOpen() {
		t.Fatal("expected problem search to open when typing problem title")
	}

	// Navigate problem search
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.palette.selectedIndex != 1 {
		t.Fatalf("expected palette selectedIndex 1, got %d", m.palette.selectedIndex)
	}

	// Escape closes problem search
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsPaletteOpen() {
		t.Fatal("expected search to close on Esc")
	}
}

func TestHomeResizeRobustness(t *testing.T) {
	cfg := config.DefaultConfig()
	problems := makeTestProblems()
	userState := model.NewUserState()

	m := New(cfg, problems, userState, 80, 24)

	// Test a series of resize events simulating user resizing terminal window
	testSizes := []struct {
		w, h int
	}{
		{w: 120, h: 40}, // expanded
		{w: 70, h: 22},  // normal
		{w: 50, h: 16},  // compact / narrow
		{w: 40, h: 12},  // ultra-compact
	}

	for _, sz := range testSizes {
		m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})

		rendered := m.View()
		if len(rendered) == 0 {
			t.Fatalf("expected non-empty rendered view for size %dx%d", sz.w, sz.h)
		}

		// Ensure no UTF-8 corruption replacement characters exist in output
		if strings.Contains(rendered, "\ufffd") {
			t.Fatalf("detected UTF-8 replacement character (corrupted runes) in view at size %dx%d", sz.w, sz.h)
		}
	}
}

func TestHomeCardSurfaceDoesNotBleedIntoFramePadding(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	pal := theme.Resolve("openleet")
	m := New(config.DefaultConfig(), makeTestProblems(), model.NewUserState(), 160, 40)
	m.SetPalette(pal)

	var cardLine string
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(ansi.Strip(line), "Coach") {
			cardLine = line
			break
		}
	}
	if cardLine == "" {
		t.Fatal("expected a rendered card line containing the metadata pills")
	}

	// The trailing run after the card's final reset is frame padding. It must
	// use the frame background; the card's surface colour must stop at the
	// card edge or the search bar looks like it runs to the terminal edge.
	const reset = "\x1b[0m"
	end := strings.LastIndex(cardLine, reset)
	if end < 0 {
		t.Fatalf("card line has no final reset: %q", cardLine)
	}
	prefix := cardLine[:end]
	start := strings.LastIndex(prefix, reset)
	if start < 0 {
		t.Fatalf("card line has no card reset: %q", cardLine)
	}
	trailing := prefix[start+len(reset):]

	frameBg := uikit.BackgroundSequence(pal.Background)
	surfaceBg := uikit.BackgroundSequence(pal.Surface)
	if !strings.HasPrefix(trailing, frameBg) {
		t.Fatalf("card padding uses the wrong background, want %q in %q", frameBg, trailing)
	}
	if strings.HasPrefix(trailing, surfaceBg) {
		t.Fatalf("card surface bleeds past its right edge: %q", trailing)
	}
}

func TestHomeRendersNoInternalChrome(t *testing.T) {
	cfg := config.DefaultConfig()
	m := New(cfg, nil, model.NewUserState(), 120, 40)

	// The app-level top bar and bottom bar own the breadcrumb and hints, so
	// the landing page must not duplicate them.
	view := ansi.Strip(m.View())
	if strings.Contains(view, "home>") {
		t.Fatalf("home rendered its own breadcrumb row: %q", view)
	}
	if strings.Contains(view, "F4 settings") || strings.Contains(view, "shift+tab") {
		t.Fatalf("home rendered its own footer hints: %q", view)
	}
}

func TestMiniStatsUnderSearchBox(t *testing.T) {
	cfg := config.DefaultConfig()
	state := model.NewUserState()
	state.MarkSolved("001", "python", "solution")
	problems := makeTestProblems()
	m := New(cfg, problems, state, 160, 40)

	view := ansi.Strip(m.View())
	want := "1d streak · 1/" + itoa(len(problems)) + " solved"
	if !strings.Contains(view, want) {
		t.Fatalf("expected %q under the search box:\n%s", want, view)
	}
	if strings.Contains(view, "Solved)") {
		t.Fatalf("the status line must not repeat the solved count: %s", view)
	}

	rows := strings.Split(view, "\n")
	statsRow := -1
	for i, row := range rows {
		if strings.Contains(row, "streak") {
			statsRow = i
			break
		}
	}
	if statsRow < 1 {
		t.Fatalf("stats row not found: %s", view)
	}
	if strings.TrimSpace(rows[statsRow-1]) != "" {
		t.Fatalf("expected one blank row above the stats, got %q", rows[statsRow-1])
	}
}

func TestMiniStatsHiddenWithoutProgress(t *testing.T) {
	m := New(config.DefaultConfig(), makeTestProblems(), model.NewUserState(), 160, 40)
	view := ansi.Strip(m.View())
	if strings.Contains(view, "streak") || strings.Contains(view, "solved") {
		t.Fatalf("empty progress must not render the mini stats: %s", view)
	}
}

func TestMiniStatsShowsSolvedOnlyWhenStreakIsZero(t *testing.T) {
	state := model.NewUserState()
	state.SolvedProblems = append(state.SolvedProblems, model.SolvedProblem{ProblemID: "001", Language: "python"})
	m := New(config.DefaultConfig(), makeTestProblems(), state, 160, 40)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "1/2 solved") {
		t.Fatalf("expected solved-only stats: %s", view)
	}
	if strings.Contains(view, "0d streak") {
		t.Fatalf("zero streak must be omitted: %s", view)
	}
}

func TestMiniStatsShortensWhenNarrow(t *testing.T) {
	state := model.NewUserState()
	state.CurrentStreak = 12
	state.SolvedProblems = append(state.SolvedProblems, model.SolvedProblem{ProblemID: "001", Language: "python"})
	m := New(config.DefaultConfig(), makeTestProblems(), state, 100, 30)

	full := ansi.Strip(m.miniStatsLine(80))
	if !strings.Contains(full, "12d streak · 1/2 solved") {
		t.Fatalf("full stats = %q", full)
	}
	short := ansi.Strip(m.miniStatsLine(10))
	if short != "12d · 1/2" {
		t.Fatalf("short stats = %q", short)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
