package workspace

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"openleet/internal/coach"
	uitheme "openleet/internal/ui/theme"
)

func TestConsoleThemeRepaintsCoachAndStylesIntro(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(oldProfile)

	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	m.SwitchTab() // Socratic Coach
	pal := uitheme.Resolve("openleet-light")
	light := FromPalette(pal)
	m.SetTheme(light)

	view := m.View()
	header := lipgloss.NewStyle().Bold(true).Foreground(light.Accent).Render("🧠 Socratic AI Coach")
	if !strings.Contains(view, header) {
		t.Fatal("light-theme accent did not repaint the coach header")
	}
	intro := lipgloss.NewStyle().Foreground(light.Text).
		Render("I will guide your problem-solving process without spoiling the solution.")
	if !strings.Contains(view, intro) {
		t.Fatal("light-theme text colour did not style the coach intro")
	}
}

func TestConsoleAppliesTokenBatches(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)

	m, cmd := m.Update(coach.CoachStreamStartedMsg{
		Provider: "test",
		StreamID: 9,
		Channels: coach.StreamChannels{}, // bus owns delivery
	})
	if cmd != nil {
		t.Fatal("expected no WaitForNextToken command when channels are empty")
	}
	if !m.coachStreaming {
		t.Fatal("expected the console to enter streaming state")
	}

	m, _ = m.Update(coach.CoachTokenBatchMsg{StreamID: 9, Tokens: []string{"Hello", " ", "world"}})
	if got, want := m.currentStream, "Hello world"; got != want {
		t.Fatalf("currentStream = %q, want %q", got, want)
	}

	m, _ = m.Update(coach.CoachStreamDoneMsg{StreamID: 9})
	if m.coachStreaming {
		t.Fatal("expected streaming to finish")
	}
	if len(m.coachHistory) != 1 || m.coachHistory[0].Content != "Hello world" {
		t.Fatalf("unexpected history: %#v", m.coachHistory)
	}
}

func TestConsoleIgnoresStaleStreamIDs(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	m, _ = m.Update(coach.CoachStreamStartedMsg{StreamID: 2, Channels: coach.StreamChannels{}})

	m, _ = m.Update(coach.CoachTokenBatchMsg{StreamID: 1, Tokens: []string{"stale"}})
	if m.currentStream != "" {
		t.Fatalf("stale tokens were appended: %q", m.currentStream)
	}

	m, _ = m.Update(coach.CoachStreamDoneMsg{StreamID: 1})
	if !m.coachStreaming {
		t.Fatal("a stale completion ended the active stream")
	}
	if len(m.coachHistory) != 0 {
		t.Fatalf("stale completion polluted history: %#v", m.coachHistory)
	}
}

func TestConsoleDoesNotReportCancellationAsFailure(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	m, _ = m.Update(coach.CoachStreamStartedMsg{StreamID: 3, Channels: coach.StreamChannels{}})
	m.CancelStream()

	m, _ = m.Update(coach.CoachStreamErrorMsg{StreamID: 3, Err: errTestCancelled})
	if len(m.coachHistory) != 0 {
		t.Fatalf("cancel surfaced as an error turn: %#v", m.coachHistory)
	}
}

func TestWrapToWidthUsesTerminalCells(t *testing.T) {
	text := "日本語のタイトルを折り返します emoji 🙂🙂 and a verylongtokenthatcannotfit"
	for _, width := range []int{6, 12, 20} {
		wrapped := wrapToWidth(text, width)
		for _, line := range strings.Split(wrapped, "\n") {
			if got := ansi.StringWidth(line); got > width {
				t.Fatalf("width %d: line %q is %d cells", width, line, got)
			}
			if !utf8.ValidString(line) {
				t.Fatalf("width %d: invalid UTF-8 line %q", width, line)
			}
		}
	}
}

func TestClickOnCoachTabSwitchesThroughWorkspace(t *testing.T) {
	ws := New(nil, "go", nil, nil, nil, 120, 40)
	layout := ComputeLayout(120, 40)

	var coachStart int
	for _, span := range ws.console.tabSpans() {
		if span.tab == TabSocraticCoach {
			coachStart = span.start
		}
	}

	// The console tabs start after "╭─ " in the pane border.
	x := layout.DescriptionWidth + layout.ColumnGap + 3 + coachStart
	y := layout.EditorHeight

	ws, _ = ws.Update(tea.MouseMsg{
		X:      x,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})

	if ws.console.activeTab != TabSocraticCoach {
		t.Fatalf("expected the coach tab after clicking %d,%d, got %v", x, y, ws.console.activeTab)
	}
}

func TestConsoleSwitchTab(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	m.SwitchTab()
	if m.activeTab != TabSocraticCoach {
		t.Fatalf("expected the coach tab, got %v", m.activeTab)
	}
	m.SwitchTab()
	if m.activeTab != TabHint {
		t.Fatalf("expected the hint tab, got %v", m.activeTab)
	}
	m.SwitchTab()
	if m.activeTab != TabCustom {
		t.Fatalf("expected the custom tab, got %v", m.activeTab)
	}
	m.SwitchTab()
	if m.activeTab != TabTestResults {
		t.Fatalf("expected the results tab, got %v", m.activeTab)
	}
}

func TestTabAtMatchesRenderedSpans(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	spans := m.tabSpans()
	if len(spans) != 4 {
		t.Fatalf("expected 4 tab spans, got %d", len(spans))
	}
	if spans[0].start != 0 {
		t.Fatalf("results start = %d, want 0", spans[0].start)
	}
	if spans[0].end != lipgloss.Width("Test Results") {
		t.Fatalf("results end = %d", spans[0].end)
	}
	if spans[1].start != spans[0].end+lipgloss.Width(m.tabGap()) {
		t.Fatalf("coach start = %d, want %d", spans[1].start, spans[0].end+lipgloss.Width(m.tabGap()))
	}
	for _, span := range spans {
		if span.end <= span.start {
			t.Fatalf("invalid span %d..%d", span.start, span.end)
		}
	}

	if tab, ok := m.TabAt(1); !ok || tab != TabTestResults {
		t.Fatalf("cell 1 should hit Test Results, got %v %v", tab, ok)
	}
	if _, ok := m.TabAt(spans[0].end); ok {
		t.Fatal("the gap between the tabs must not be clickable")
	}
	if tab, ok := m.TabAt(spans[1].start); !ok || tab != TabSocraticCoach {
		t.Fatalf("first coach cell should hit, got %v %v", tab, ok)
	}
	if tab, ok := m.TabAt(spans[1].end - 1); !ok || tab != TabSocraticCoach {
		t.Fatalf("last coach cell should hit, got %v %v", tab, ok)
	}
	if tab, ok := m.TabAt(spans[2].start); !ok || tab != TabHint {
		t.Fatalf("hint tab should hit, got %v %v", tab, ok)
	}
	if tab, ok := m.TabAt(spans[3].start); !ok || tab != TabCustom {
		t.Fatalf("custom tab should hit, got %v %v", tab, ok)
	}
	if _, ok := m.TabAt(spans[3].end); ok {
		t.Fatal("a cell past the tab bar should not hit")
	}
}

func TestCustomTabDispatchesFixture(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	m.SwitchToTab(TabCustom)
	m.customInput.SetValue(`{"nums":[1,2,3,1]}`)
	m.customExpected.SetValue(`true`)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a custom fixture command")
	}
	msg, ok := cmd().(CustomTestRequestedMsg)
	if !ok {
		t.Fatalf("expected CustomTestRequestedMsg, got %T", cmd())
	}
	input, ok := msg.Input.(map[string]interface{})
	if !ok {
		t.Fatalf("expected an object input, got %T", msg.Input)
	}
	if input["nums"] == nil {
		t.Fatalf("expected the nums argument, got %#v", input)
	}
	if msg.Expected != true {
		t.Fatalf("expected true, got %#v", msg.Expected)
	}
	if !m.customRunning {
		t.Fatal("expected the custom run to be marked running")
	}
}

func TestCustomTabRejectsInvalidJSON(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	m.SwitchToTab(TabCustom)
	m.customInput.SetValue(`{not json}`)
	m.customExpected.SetValue(`true`)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected invalid JSON to be rejected before dispatch")
	}
	if m.customErr == "" {
		t.Fatal("expected a JSON error in the custom tab")
	}
}

func TestTabTitleMatchesSpans(t *testing.T) {
	m := NewConsoleModel(DefaultWorkspaceTheme(), 120, 30, nil)
	spans := m.tabSpans()
	title := m.TabTitle()
	if got, want := lipgloss.Width(title), spans[len(spans)-1].end; got != want {
		t.Fatalf("title width = %d, want %d", got, want)
	}
}

var errTestCancelled = testError("context canceled")

type testError string

func (e testError) Error() string { return string(e) }
