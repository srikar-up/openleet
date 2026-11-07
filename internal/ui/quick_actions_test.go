package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"openleet/internal/model"
	"openleet/internal/ui/commands"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/toast"
	"openleet/internal/ui/uikit"
)

func quickActionProblem() *model.Problem {
	return &model.Problem{
		ID:         "001",
		Title:      "Two Sum",
		Slug:       "two-sum",
		Difficulty: model.DifficultyEasy,
		Topics:     []string{"array", "hash-table"},
		Templates: map[string]string{
			"python": "class Solution:\n    def twoSum(self, nums, target):\n        pass\n",
			"go":     "package main\n\nfunc twoSum(nums []int, target int) []int {\n\treturn nil\n}\n",
		},
		TestCases: []model.TestCase{{ID: "tc1", Input: "[]", Expected: "[]"}},
	}
}

func TestEveryCommandHasHandler(t *testing.T) {
	for _, ctx := range commands.Contexts() {
		for _, cmd := range commands.CommandsFor(ctx) {
			if quickHandlers[cmd.Action] == nil {
				t.Fatalf("%s: %s has no handler", ctx, cmd.Name)
			}
		}
	}
	for _, cmd := range commands.All() {
		if quickHandlers[cmd.Action] == nil {
			t.Fatalf("%s has no handler", cmd.Name)
		}
	}
}

func TestOnlySettingsAndRoadmapNavigate(t *testing.T) {
	for _, cmd := range commands.All() {
		app := newRoutingTestApp(t)
		app.currentView = ViewSolve
		app.selectedIndex = 0
		app.problems = []*model.Problem{quickActionProblem()}
		app.workspace.SetProblem(app.problems[0])
		next, _ := app.executeCommand(cmd, "")
		switch cmd.Action {
		case commands.ActionSettings:
			if next.currentView != ViewSettings {
				t.Fatalf("/settings should open settings, got %v", next.currentView)
			}
			continue
		case commands.ActionRoadmap:
			if next.currentView != ViewRoadmap {
				t.Fatalf("/roadmap should open the roadmap, got %v", next.currentView)
			}
			continue
		}
		if next.currentView != ViewSolve {
			t.Fatalf("%s changed the screen to %v", cmd.Name, next.currentView)
		}
	}
}

func TestThemeQuickActionPreservesSolveStateAndSyncsSettings(t *testing.T) {
	app, _ := newPersistentRoutingTestApp(t)
	app.problems = []*model.Problem{quickActionProblem()}
	app.selectedIndex = 0
	app, _ = app.switchToSolveView(0)
	app.currentView = ViewSolve

	// Move the editor cursor away from the origin so the assertion is real.
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	app = m.(AppModel)
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = m.(AppModel)
	beforeCode := app.workspace.Code()
	beforeRow, beforeCol := app.workspace.Cursor()
	beforeResult := app.workspace.ConsoleResult()
	if beforeCode == "" {
		t.Fatal("expected a non-empty solve buffer")
	}

	app, _ = app.handleThemes("")
	if !app.quick.dialogOpen || app.currentView != ViewSolve {
		t.Fatal("expected the theme popup over solve")
	}
	app = chooseDialogID(t, app, "gruvbox")
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)

	if !app.quick.dialogOpen {
		// Applying closes the popup; this branch is the expected path.
	} else {
		t.Fatal("expected the theme popup to close after applying")
	}
	if app.currentView != ViewSolve {
		t.Fatalf("theme apply changed the screen to %v", app.currentView)
	}
	if got := app.workspace.Code(); got != beforeCode {
		t.Fatal("theme apply changed the code buffer")
	}
	row, col := app.workspace.Cursor()
	if row != beforeRow || col != beforeCol {
		t.Fatalf("theme apply moved the cursor from %d,%d to %d,%d", beforeRow, beforeCol, row, col)
	}
	if app.workspace.ConsoleResult() != beforeResult {
		t.Fatal("theme apply changed the test results")
	}
	if app.palette.Name != "gruvbox" || app.config.Theme != "gruvbox" {
		t.Fatalf("theme did not apply: palette=%s config=%s", app.palette.Name, app.config.Theme)
	}
	if got := app.config.Settings["appearance.theme"]; got != "gruvbox" {
		t.Fatalf("appearance.theme = %q, want gruvbox", got)
	}

	app = app.openSettings("appearance")
	if got := app.settingsView.ActiveTheme(); got != "gruvbox" {
		t.Fatalf("Settings > Appearance shows %q, want gruvbox", got)
	}
}

func TestConnectQuickFlowWritesOneSourceOfTruth(t *testing.T) {
	app, _ := newPersistentRoutingTestApp(t)
	app.currentView = ViewSolve

	app, _ = app.handleConnect("")
	if !app.quick.dialogOpen || app.quick.stage != connectStageProvider {
		t.Fatal("expected the provider step")
	}
	app = chooseDialogID(t, app, "anthropic")
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)
	if app.quick.stage != connectStageKey {
		t.Fatalf("expected key step, got %d", app.quick.stage)
	}

	for _, r := range "sk-ant-test" {
		m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = m.(AppModel)
	}
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)
	if app.quick.stage != connectStageModel {
		t.Fatalf("expected model step, got %d", app.quick.stage)
	}
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)

	if app.quick.dialogOpen {
		t.Fatal("expected the connect flow to close")
	}
	if app.currentView != ViewSolve {
		t.Fatalf("connect changed the screen to %v", app.currentView)
	}
	if got := app.config.Settings["ai.coach"]; got != "anthropic" {
		t.Fatalf("ai.coach = %q, want anthropic", got)
	}
	if app.settingsView.SecretMasked("anthropic") == "" {
		t.Fatal("API key was not saved to the shared vault")
	}
	if got := app.config.Settings["ai.provider.anthropic.model"]; got == "" {
		t.Fatal("model setting was not written")
	}
	if !hasVisibleToast(app, toast.Success, "Provider connected") {
		t.Fatalf("missing connect toast: %#v", visibleToasts(app))
	}
}

func TestStatsPopupShowsSevenDays(t *testing.T) {
	app := newRoutingTestApp(t)
	app.problems = []*model.Problem{quickActionProblem()}
	app.userState = model.NewUserState()
	app.userState.MarkSolved("001", "python", "code")

	app, _ = app.handleStats("")
	if !app.quick.dialogOpen {
		t.Fatal("expected the stats popup")
	}
	panel := ansi.Strip(app.quick.dialog.Panel())
	for offset := 0; offset < 7; offset++ {
		day := time.Now().AddDate(0, 0, -offset).Format("Mon 02 Jan")
		if !strings.Contains(panel, day) {
			t.Fatalf("missing %s from the last 7 days: %q", day, panel)
		}
	}
}

func TestPaletteTypedSettingsDeepLink(t *testing.T) {
	app := newRoutingTestApp(t)
	app = app.openCommandPalette()
	for _, r := range "/settings ai" {
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = m.(AppModel)
	}
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = m.(AppModel)

	if app.quick.paletteOpen {
		t.Fatal("expected the palette to close after a typed command")
	}
	if app.currentView != ViewSettings {
		t.Fatalf("expected /settings ai to open settings, got %v", app.currentView)
	}
	if got := app.settingsView.ActiveSection(); got != settings.SectionAI {
		t.Fatalf("active settings section = %q, want %q", got, settings.SectionAI)
	}
}

func TestParseTypedCommandOnlyMatchesContextCommands(t *testing.T) {
	cmd, argument, ok := parseTypedCommand(commands.ContextHome, "/settings ai")
	if !ok || cmd.Action != commands.ActionSettings || argument != "ai" {
		t.Fatalf("unexpected parse: %+v %q %v", cmd, argument, ok)
	}
	if _, _, ok := parseTypedCommand(commands.ContextHome, "/run"); ok {
		t.Fatal("/run is not available on Home and must not parse")
	}
	if _, _, ok := parseTypedCommand(commands.ContextHome, "settings ai"); ok {
		t.Fatal("queries without a leading slash must keep filtering the list")
	}
}

func TestLightThemeOverlayPaintsDarkScrim(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	pal := theme.Resolve("openleet-light")
	width, height := 48, 8
	row := strings.Repeat("w", width)
	bg := strings.Repeat(row+"\n", height)

	out := overlayPanel(bg, "PANEL", width, height, pal)
	probe := lipgloss.NewStyle().Foreground(pal.Muted).Background(pal.Text).Render("x")
	prefix := probe[:strings.IndexByte(probe, 'x')]
	if prefix == "" {
		t.Fatal("could not resolve the scrim style prefix")
	}
	if !strings.Contains(out, prefix) {
		t.Fatal("light-theme backdrop was not painted with the dark scrim")
	}

	lightSeq := uikit.BackgroundSequence(pal.Background)
	lightRGB := lightSeq
	if i := strings.Index(lightSeq, "48;2;"); i >= 0 {
		lightRGB = strings.TrimSuffix(lightSeq[i+5:], "m")
	}
	if lightRGB != "" && strings.Contains(out, lightRGB) {
		t.Fatal("light-theme backdrop used the light page background instead of the scrim")
	}
}

func TestRoadmapQuickActionNavigates(t *testing.T) {
	app := newRoutingTestApp(t)
	app.problems = []*model.Problem{quickActionProblem()}
	app.selectedIndex = 0
	app, _ = app.switchToSolveView(0)
	app.currentView = ViewSolve
	before := app.workspace.Code()

	next, _ := app.executeCommand(commands.Command{
		Name:   "/roadmap",
		Action: commands.ActionRoadmap,
	}, "")
	if next.currentView != ViewRoadmap {
		t.Fatalf("/roadmap should open the roadmap, got %v", next.currentView)
	}
	if next.workspace.Code() != before {
		t.Fatal("/roadmap changed the solve buffer")
	}
}

func TestRoadmapInlineCommandBarOwnsKeys(t *testing.T) {
	app := newRoutingTestApp(t)
	app.currentView = ViewRoadmap

	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = m.(AppModel)
	if !app.mapView.IsCommandMode() {
		t.Fatal("expected / to open the roadmap command bar")
	}
	if app.quick.paletteOpen {
		t.Fatal("the global palette must not open when the roadmap bar is active")
	}

	m, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	app = m.(AppModel)
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q quit the app while the roadmap command bar was open")
		}
	}
	if app.currentView != ViewRoadmap || !app.mapView.IsCommandMode() {
		t.Fatal("typing q must stay inside the roadmap command bar")
	}

	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(AppModel)
	if app.mapView.IsCommandMode() {
		t.Fatal("expected Esc to close the roadmap command bar")
	}
	if app.currentView != ViewRoadmap {
		t.Fatalf("Esc left the roadmap: %v", app.currentView)
	}
}
