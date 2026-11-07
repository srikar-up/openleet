package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"openleet/internal/model"
	"openleet/internal/ui/commands"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/toast"
)

func withTrueColor(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

func testTopBarApp(pal theme.Palette, width int, account *accountSession) AppModel {
	state := model.NewUserState()
	return AppModel{
		width:       width,
		height:      40,
		palette:     pal,
		theme:       themeFromPalette(pal),
		currentView: ViewSolve,
		account:     account,
		userState:   state,
		notifier:    toast.New(),
	}
}

func TestThemeAuditEveryThemePasses(t *testing.T) {
	rows := runThemeAudit()
	if len(rows) != len(theme.All()) {
		t.Fatalf("audited %d themes, want %d", len(rows), len(theme.All()))
	}
	for _, row := range rows {
		if !row.pass() {
			t.Fatalf("%s failed: %v", row.Name, row.Failures)
		}
	}
}

func TestThemeComponentsUseSemanticTokens(t *testing.T) {
	withTrueColor(t)

	assertStyle := func(name string, rendered string, want string) {
		t.Helper()
		if !strings.Contains(rendered, want) {
			t.Fatalf("%s is missing semantic style %q: %q", name, want, rendered)
		}
	}

	for _, pal := range theme.All() {
		app := AppModel{
			width:       160,
			height:      40,
			palette:     pal,
			theme:       themeFromPalette(pal),
			currentView: ViewHome,
		}
		bar := app.renderTopBar()
		// Rest state: dim labels, no icon and no border glyphs.
		assertStyle("top bar rest "+pal.Name, bar,
			lipgloss.NewStyle().Foreground(pal.TextDim).Render(" Settings "))
		assertStyle("signed-out label "+pal.Name, bar,
			lipgloss.NewStyle().Foreground(pal.Accent).Render(" Log in "))

		// Hover: text on a raised, borderless pill.
		app.topBarHover = topBarTargetSettings
		hover := app.renderTopBar()
		assertStyle("top bar hover "+pal.Name, hover,
			lipgloss.NewStyle().Foreground(pal.Text).Background(pal.SurfaceRaised).Render(" Settings "))

		kinds := map[toast.Kind]lipgloss.TerminalColor{
			toast.Info:    pal.Accent,
			toast.Success: pal.Pass,
			toast.Warning: pal.Warn,
			toast.Error:   pal.Fail,
		}
		glyphs := map[toast.Kind]string{
			toast.Info:    "i ",
			toast.Success: "✓ ",
			toast.Warning: "! ",
			toast.Error:   "✕ ",
		}
		for kind, color := range kinds {
			card := toast.RenderCard(pal, toast.Toast{
				Kind:    kind,
				Title:   "Audit message",
				Message: "Semantic tokens",
				Count:   1,
			}, 40, true)
			rendered := strings.Join(card.Lines, "\n")
			assertStyle(fmt.Sprintf("toast %s %s", kind, pal.Name), rendered,
				lipgloss.NewStyle().Bold(true).Foreground(pal.Text).Background(pal.SurfaceRaised).Render("Audit message"))
			assertStyle(fmt.Sprintf("toast kind %s %s", kind, pal.Name), rendered,
				lipgloss.NewStyle().Foreground(color).Background(pal.SurfaceRaised).Render(glyphs[kind]))
			assertStyle(fmt.Sprintf("toast message %s %s", kind, pal.Name), rendered,
				lipgloss.NewStyle().Foreground(pal.TextDim).Background(pal.SurfaceRaised).Render("Semantic tokens"))
		}
	}
}

func TestTopBarSettingsAndAccountLayout(t *testing.T) {
	pal := theme.Resolve("openleet")
	account := &accountSession{Email: "user@example.com", DisplayName: "neo"}

	wide := ansi.Strip(testTopBarApp(pal, 160, account).renderTopBar())
	if !strings.Contains(wide, "Settings") {
		t.Fatalf("wide top bar is missing Settings: %q", wide)
	}
	if !strings.Contains(wide, "Settings   user@example.com") {
		t.Fatalf("expected a 3-cell gap and the full email: %q", wide)
	}
	if strings.Contains(wide, "\n") {
		t.Fatalf("top bar must be exactly one row: %q", wide)
	}
	for _, banned := range []string{"[", "]", "⚙", "●", "○", "│", "╭", "╮", "┌", "┐", "*"} {
		if strings.Contains(wide, banned) {
			t.Fatalf("top bar contains forbidden %q: %q", banned, wide)
		}
	}

	// 120+ keeps the full address, 100-119 the local part, under 100 the
	// initial. Signed out always reads "Log in".
	at120 := ansi.Strip(testTopBarApp(pal, 120, account).renderTopBar())
	if !strings.Contains(at120, "user@example.com") {
		t.Fatalf("120-column top bar should keep the email: %q", at120)
	}
	at110 := ansi.Strip(testTopBarApp(pal, 110, account).renderTopBar())
	if !strings.Contains(at110, "Settings   user") || strings.Contains(at110, "@example.com") {
		t.Fatalf("110-column top bar should show the local part: %q", at110)
	}
	at100 := ansi.Strip(testTopBarApp(pal, 100, account).renderTopBar())
	if !strings.Contains(at100, "Settings   user") || strings.Contains(at100, "@example.com") {
		t.Fatalf("100-column top bar should show the local part: %q", at100)
	}
	at90 := ansi.Strip(testTopBarApp(pal, 90, account).renderTopBar())
	if !strings.Contains(at90, "Settings   U") || strings.Contains(at90, "example") {
		t.Fatalf("sub-100-column top bar should show the initial: %q", at90)
	}
	signedOut := ansi.Strip(testTopBarApp(pal, 100, nil).renderTopBar())
	if !strings.Contains(signedOut, "Settings   Log in") {
		t.Fatalf("signed-out top bar should read Log in: %q", signedOut)
	}
}

func TestTopBarHitMapAndActivation(t *testing.T) {
	pal := theme.Resolve("openleet")
	app := testTopBarApp(pal, 160, &accountSession{Email: "user@example.com"})
	hits := app.topBarHits()
	var settingsHit, accountHit *topBarHit
	for i := range hits {
		switch hits[i].target {
		case topBarTargetSettings:
			settingsHit = &hits[i]
		case topBarTargetAccount:
			accountHit = &hits[i]
		}
	}
	if settingsHit == nil || accountHit == nil {
		t.Fatalf("missing top-bar hit targets: %#v", hits)
	}
	if gap := accountHit.start - settingsHit.end - 1; gap != 1 {
		t.Fatalf("expected one gap cell between the padded items (3 visual cells), got %d", gap)
	}

	m, _ := app.Update(tea.MouseMsg{
		X:      settingsHit.start,
		Y:      0,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if got := m.(AppModel).currentView; got != ViewSettings {
		t.Fatalf("Settings click opened view %v", got)
	}

	app = testTopBarApp(pal, 160, &accountSession{Email: "user@example.com"})
	m, _ = app.Update(tea.MouseMsg{
		X:      accountHit.start,
		Y:      0,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if next := m.(AppModel); !next.accountMenuOpen {
		t.Fatal("account label click did not open the dropdown")
	}

	app = testTopBarApp(pal, 160, nil)
	app.topBarFocus = topBarTargetSettings
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.(AppModel).currentView; got != ViewSettings {
		t.Fatalf("Enter on focused Settings opened view %v", got)
	}
}

func TestTopBarTabOrderAndTooltip(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.topBarFocus = topBarTargetBreadcrumb

	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := m.(AppModel).topBarFocus; got != topBarTargetSettings {
		t.Fatalf("tab focus = %v, want Settings", got)
	}
	m, _ = m.(AppModel).Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := m.(AppModel).topBarFocus; got != topBarTargetAccount {
		t.Fatalf("second tab focus = %v, want account", got)
	}
	m, _ = m.(AppModel).Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if got := m.(AppModel).topBarFocus; got != topBarTargetSettings {
		t.Fatalf("shift+tab focus = %v, want Settings", got)
	}

	app.topBarHover = topBarTargetSettings
	if got := app.renderTopBar(); !strings.Contains(ansi.Strip(got), "Settings") {
		t.Fatalf("top bar hint = %q", ansi.Strip(got))
	}
	if got := app.topBarTooltip(); !strings.Contains(got, "Settings (F4)") {
		t.Fatalf("tooltip = %q", got)
	}
}

// TestTopBarHitsCoverTheRenderedGlyphs is the regression guard for the top
// right corner: every pointer target must cover exactly the cells that paint
// its label. An off-by-N origin makes the UI look right while clicks open the
// wrong surface.
func TestTopBarHitsCoverTheRenderedGlyphs(t *testing.T) {
	pal := theme.Resolve("openleet")
	for _, width := range []int{100, 120, 160, 170} {
		app := testTopBarApp(pal, width, &accountSession{Email: "alex@example.com", DisplayName: "Alex"})
		bar := ansi.Strip(app.renderTopBar())
		if got := ansi.StringWidth(bar); got != width {
			t.Fatalf("top bar at %d is %d cells", width, got)
		}
		wantAccount := "alex@example.com"
		switch {
		case width < 100:
			wantAccount = "A"
		case width < 120:
			wantAccount = "alex"
		}

		var settings, account bool
		for _, hit := range app.topBarHits() {
			window := ansi.Cut(bar, hit.start, hit.end+1)
			switch hit.target {
			case topBarTargetSettings:
				settings = true
				if !strings.Contains(window, "Settings") || strings.Contains(window, "Account") {
					t.Fatalf("settings hit at %d covers %q", width, window)
				}
			case topBarTargetAccount:
				account = true
				if !strings.Contains(window, wantAccount) {
					t.Fatalf("account hit at %d covers %q, want %q", width, window, wantAccount)
				}
			case topBarTargetBreadcrumb:
				if window == "" {
					t.Fatalf("empty breadcrumb hit at %d", width)
				}
				if !strings.Contains(window, "home") && !strings.Contains(window, "roadmap") && !strings.Contains(window, "solve") && !strings.Contains(window, "settings") {
					t.Fatalf("breadcrumb hit at %d covers %q", width, window)
				}
			}
		}
		if !settings || !account {
			t.Fatalf("top bar at %d is missing label hits: settings=%v account=%v", width, settings, account)
		}
	}
}

// TestTopBarClicksOpenTheLabelUnderThePointer clicks the middle cell of each
// rendered label and asserts the matching surface opens.
func TestTopBarClicksOpenTheLabelUnderThePointer(t *testing.T) {
	pal := theme.Resolve("openleet")
	app := testTopBarApp(pal, 160, nil)
	bar := ansi.Strip(app.renderTopBar())

	find := func(target topBarTarget) topBarHit {
		t.Helper()
		for _, hit := range app.topBarHits() {
			if hit.target == target {
				return hit
			}
		}
		t.Fatalf("missing hit for target %v", target)
		return topBarHit{}
	}
	click := func(x int) AppModel {
		t.Helper()
		m, _ := app.Update(tea.MouseMsg{
			X:      x,
			Y:      0,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		})
		return m.(AppModel)
	}

	settingsHit := find(topBarTargetSettings)
	settingsWindow := ansi.Cut(bar, settingsHit.start, settingsHit.end+1)
	if !strings.Contains(settingsWindow, "Settings") {
		t.Fatalf("settings hit covers %q", settingsWindow)
	}
	if got := click((settingsHit.start + settingsHit.end) / 2); got.currentView != ViewSettings {
		t.Fatalf("clicking Settings opened view %v", got.currentView)
	}

	accountHit := find(topBarTargetAccount)
	accountWindow := ansi.Cut(bar, accountHit.start, accountHit.end+1)
	if !strings.Contains(accountWindow, "Log in") {
		t.Fatalf("account hit covers %q", accountWindow)
	}
	if got := click((accountHit.start + accountHit.end) / 2); !got.accountMenuOpen {
		t.Fatalf("clicking the account label did not open the dropdown (view %v)", got.currentView)
	}
	if strings.Contains(accountWindow, "Settings") {
		t.Fatalf("account hit leaks into Settings: %q", accountWindow)
	}

	// The hover tooltip is a transient overlay on the row below the header; the
	// header itself stays one row.
	app.topBarHover = topBarTargetAccount
	base := app.renderTopBar() + "\n" + strings.Repeat(" ", 160) + "\n" + strings.Repeat(" ", 160)
	out := app.overlayTopBarTooltip(base)
	rows := strings.Split(out, "\n")
	if strings.Contains(ansi.Strip(rows[0]), "Account") {
		t.Fatalf("tooltip leaked into the header row: %q", ansi.Strip(rows[0]))
	}
	if !strings.Contains(ansi.Strip(rows[1]), "Account") {
		t.Fatalf("account tooltip missing from the overlay row: %q", ansi.Strip(rows[1]))
	}
}

// TestTopBarKeyboardFocusWorksOnEveryChromeView proves F6 and Tab can focus the
// Settings and account labels on Home, Roadmap, Solve, and Settings, and that
// focus shows the same tooltips as hover.
func TestTopBarKeyboardFocusWorksOnEveryChromeView(t *testing.T) {
	pal := theme.Resolve("openleet")
	for _, view := range []ViewMode{ViewHome, ViewRoadmap, ViewSolve, ViewSettings} {
		app := testTopBarApp(pal, 160, nil)
		app.currentView = view

		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyF6})
		got := m.(AppModel)
		if got.topBarFocus != topBarTargetBreadcrumb {
			t.Fatalf("%v: F6 focus = %v, want breadcrumb", view, got.topBarFocus)
		}
		m, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
		got = m.(AppModel)
		if got.topBarFocus != topBarTargetSettings {
			t.Fatalf("%v: tab focus = %v, want Settings", view, got.topBarFocus)
		}
		if tip := got.topBarTooltip(); tip != "Settings (F4)" {
			t.Fatalf("%v: focus tooltip = %q", view, tip)
		}
		m, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
		got = m.(AppModel)
		if got.topBarFocus != topBarTargetAccount {
			t.Fatalf("%v: second tab focus = %v, want account", view, got.topBarFocus)
		}
		if tip := got.topBarTooltip(); tip != "Account" {
			t.Fatalf("%v: account tooltip = %q", view, tip)
		}
		m, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if !m.(AppModel).accountMenuOpen {
			t.Fatalf("%v: Enter on the focused account label did not open the dropdown", view)
		}
	}
}

func TestThemeSwitchRepaintsVisibleToast(t *testing.T) {
	withTrueColor(t)
	app := testTopBarApp(theme.Resolve("openleet"), 120, nil)
	app.notify(toast.Success, "visible", "", toast.Options{})

	app.applyPaletteColors(theme.Resolve("openleet-light"))
	view := app.View()
	wantSurface := lipgloss.NewStyle().Foreground(app.palette.Border).Background(app.palette.SurfaceRaised).
		Render("┌" + strings.Repeat("─", 30) + "┐")
	if !strings.Contains(view, wantSurface) {
		t.Fatal("theme switch did not repaint the visible toast surface")
	}
	wantText := lipgloss.NewStyle().Bold(true).Foreground(app.palette.Text).Background(app.palette.SurfaceRaised).Render("visible")
	if !strings.Contains(view, wantText) {
		t.Fatal("theme switch did not repaint the visible toast text")
	}
}

func TestThemeAuditCommandOpensTable(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	next, _ := app.executeCommand(commands.Command{
		Name:   "/theme-audit",
		Action: commands.ActionThemeAudit,
	}, "")
	if !next.quick.dialogOpen {
		t.Fatal("/theme-audit did not open the audit table")
	}
	if next.quick.action != commands.ActionThemeAudit {
		t.Fatalf("dialog action = %q", next.quick.action)
	}
	if !hasVisibleToast(next, toast.Success, "Theme audit") {
		t.Fatalf("audit toast missing: %#v", visibleToasts(next))
	}
	panel := ansi.Strip(next.quick.dialog.Panel())
	if !strings.Contains(panel, "THEME AUDIT") || !strings.Contains(panel, "OpenLeet") {
		t.Fatalf("audit panel is missing rows: %q", panel)
	}
}

func TestSettingsWithoutDuplicates(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	if app.currentView == ViewSettings {
		t.Fatal("test app should start outside Settings")
	}
	app = app.openSettings("")
	if got := app.settingsView.ActiveSection(); got != settings.SectionGeneral {
		t.Fatalf("Settings opened section %q", got)
	}
}
