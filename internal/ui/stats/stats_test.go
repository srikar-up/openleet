package stats

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"openleet/internal/model"
	"openleet/internal/ui/uikit"
)

func sampleData() ([]*model.Problem, *model.UserState) {
	problems := []*model.Problem{
		{ID: "001", Title: "Two Sum", Difficulty: model.DifficultyEasy, Topics: []string{"array", "hash-table"}},
		{ID: "002", Title: "Valid Parentheses", Difficulty: model.DifficultyEasy, Topics: []string{"string", "stack"}},
		{ID: "003", Title: "Trapping Rain Water", Difficulty: model.DifficultyHard, Topics: []string{"array", "two-pointers"}},
		{ID: "004", Title: "Coin Change", Difficulty: model.DifficultyMedium, Topics: []string{"dp"}},
	}
	state := model.NewUserState()
	now := time.Now()
	state.SolvedProblems = []model.SolvedProblem{
		{ProblemID: "001", SolvedAt: now.AddDate(0, 0, -2)},
		{ProblemID: "002", SolvedAt: now.AddDate(0, 0, -1)},
		{ProblemID: "004", SolvedAt: now},
	}
	state.CurrentStreak = 3
	state.Sessions = 7
	return problems, state
}

func TestSummarize(t *testing.T) {
	problems, state := sampleData()
	summary := summarize(problems, state)

	if summary.Total != 4 || summary.Solved != 3 {
		t.Fatalf("expected 4 total / 3 solved, got %d/%d", summary.Total, summary.Solved)
	}
	if summary.EasySolved != 2 || summary.EasyTotal != 2 {
		t.Fatalf("easy stats wrong: %d/%d", summary.EasySolved, summary.EasyTotal)
	}
	if summary.MediumSolved != 1 || summary.MediumTotal != 1 {
		t.Fatalf("medium stats wrong: %d/%d", summary.MediumSolved, summary.MediumTotal)
	}
	if summary.HardSolved != 0 || summary.HardTotal != 1 {
		t.Fatalf("hard stats wrong: %d/%d", summary.HardSolved, summary.HardTotal)
	}
	if summary.ActiveDays != 3 || summary.BestStreak != 3 {
		t.Fatalf("expected 3 active days and streak 3, got %d and %d", summary.ActiveDays, summary.BestStreak)
	}
	if summary.Sessions != 7 {
		t.Fatalf("expected 7 sessions, got %d", summary.Sessions)
	}
	if len(summary.Topics) == 0 || summary.Topics[0].Name != "array" || summary.Topics[0].Count != 1 {
		t.Fatalf("expected array (alphabetical tie-break) to lead the topic list, got %+v", summary.Topics)
	}
}

func TestRenderContainsDashboardSections(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	problems, state := sampleData()
	mv := New(problems, state, 120, 40)
	view := mv.View()

	for _, want := range []string{
		"openleet / stats",
		"SOLVED",
		"█",
		"best streak",
		"active days",
		"sessions",
		"less",
		"more",
		"easy",
		"medium",
		"hard",
		"2/2",
		"1/1",
		"0/1",
		"array 1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in stats view", want)
		}
	}

	rows := strings.Split(view, "\n")
	if len(rows) > 40 {
		t.Fatalf("expected at most 40 rows, got %d", len(rows))
	}
}

func TestRenderNarrowTerminalFits(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	problems, state := sampleData()
	mv := New(problems, state, 60, 24)
	view := mv.View()
	for i, row := range strings.Split(view, "\n") {
		if ansi.StringWidth(row) > 60 {
			t.Fatalf("row %d exceeds 60 cells: %q", i, row)
		}
	}
}

func TestEmptyState(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	mv := New(nil, model.NewUserState(), 120, 40)
	view := mv.View()
	if !strings.Contains(view, "0/0") {
		t.Fatal("expected a zero counter for an empty catalogue")
	}
	if !strings.Contains(view, "solve a level to populate topic stats") {
		t.Fatal("expected the topic empty state")
	}
}

func TestPixelCounterIsCompact(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	hero := uikit.RenderPixelText("16/16", defaultPalette().Text)
	rows := strings.Split(hero, "\n")
	if len(rows) != 5 {
		t.Fatalf("expected 5 counter rows, got %d", len(rows))
	}
	if width := lipgloss.Width(rows[0]); width != 29 {
		t.Fatalf("expected a 29-cell counter, got %d", width)
	}
}

func TestWordmarkTwoToneKeepsRowsAligned(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	mark := RenderWordmarkTwoTone("OPEN", "LEET", defaultPalette().Text, defaultPalette().Accent)
	rows := strings.Split(mark, "\n")
	if len(rows) != 5 {
		t.Fatalf("expected 5 wordmark rows, got %d", len(rows))
	}
	// OPEN and LEET are 27 cells each in the bold 6x5 font, plus a two-cell gap.
	for i, row := range rows {
		if width := lipgloss.Width(row); width != 56 {
			t.Fatalf("row %d is %d cells wide, want 56", i, width)
		}
	}
}

func TestBigCounterScalesBothAxes(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	hero := RenderBigCounter("3/16", defaultPalette().Text, 4, 2)
	rows := strings.Split(hero, "\n")
	if len(rows) != 14 {
		t.Fatalf("expected 14 hero rows at 2x vertical scale, got %d", len(rows))
	}
	if pixels := CounterPixelWidth("3/16"); pixels != 23 {
		t.Fatalf("expected 23 glyph pixels for 3/16, got %d", pixels)
	}
	for i, row := range rows {
		if width := lipgloss.Width(row); width != 92 {
			t.Fatalf("row %d is %d cells wide, want 92", i, width)
		}
	}
}

func TestLeaderboardLineSignedIn(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	problems, state := sampleData()
	mv := New(problems, state, 120, 40)
	mv.SetLeaderboard([]RankEntry{
		{UID: "a", Name: "ada", Score: 420, Solved: 10},
		{UID: "b", Name: "lin", Score: 390, Solved: 9},
		{UID: "c", Name: "kai", Score: 385, Solved: 9},
		{UID: "d", Name: "ivy", Score: 381, Solved: 8},
		{UID: "me", Name: "neo", Score: 380, Solved: 8},
	}, "me", true, false, "")

	view := mv.View()
	for _, want := range []string{"LEADERBOARD", "#1 ada 420", "#2 lin 390", "#3 kai 385", "you #5"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in leaderboard line", want)
		}
	}
}

func TestLeaderboardLineSignedOut(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	mv := New(nil, model.NewUserState(), 120, 40)
	mv.SetLeaderboard(nil, "", false, false, "")

	if view := mv.View(); !strings.Contains(view, "sign in from settings to rank") {
		t.Fatal("expected the signed-out leaderboard hint")
	}
}

func TestLeaderboardLineError(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	mv := New(nil, model.NewUserState(), 120, 40)
	mv.SetLeaderboard(nil, "me", true, false, "network down")

	if view := mv.View(); !strings.Contains(view, "LEADERBOARD  unavailable") {
		t.Fatal("expected the leaderboard unavailable state")
	}
}
