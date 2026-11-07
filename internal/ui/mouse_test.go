package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/settings"
)

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{
		X:      x,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
}

func TestMouseBreadcrumbNavigates(t *testing.T) {
	app, _ := newThemeTestApp(t)

	// The breadcrumb lives in the shared top bar; its segments map from the
	// topBarHits instead of a per-view header row.
	app.currentView = ViewRoadmap
	var homeCrumb, roadmapCrumb topBarHit
	foundHome, foundRoadmap := false, false
	for _, hit := range app.topBarHits() {
		if hit.target != topBarTargetBreadcrumb {
			continue
		}
		switch hit.view {
		case ViewHome:
			homeCrumb, foundHome = hit, true
		case ViewRoadmap:
			roadmapCrumb, foundRoadmap = hit, true
		}
	}
	if !foundHome || !foundRoadmap {
		t.Fatalf("missing breadcrumb hits: home=%v roadmap=%v", foundHome, foundRoadmap)
	}

	state, _ := app.Update(click(homeCrumb.start, 0))
	app = state.(AppModel)
	if app.currentView != ViewHome {
		t.Fatalf("expected ViewHome after clicking the home crumb, got %v", app.currentView)
	}

	// The current crumb stays on the roadmap.
	app.currentView = ViewRoadmap
	state, _ = app.Update(click(roadmapCrumb.start, 0))
	app = state.(AppModel)
	if app.currentView != ViewRoadmap {
		t.Fatalf("expected ViewRoadmap after clicking the roadmap crumb, got %v", app.currentView)
	}
}

func TestMouseStatsHeaderReturnsHome(t *testing.T) {
	app, _ := newThemeTestApp(t)
	app.currentView = ViewStats

	state, _ := app.Update(click(2, 0))
	app = state.(AppModel)
	if app.currentView != ViewHome {
		t.Fatalf("expected ViewHome after clicking the stats header, got %v", app.currentView)
	}
}

func TestF4OpensSettingsAndHomeTopBarChips(t *testing.T) {
	app, _ := newThemeTestApp(t)

	// F4 now routes to Settings > Appearance, which replaced the gallery.
	state, _ := app.Update(tea.KeyMsg{Type: tea.KeyF4})
	app = state.(AppModel)
	if app.currentView != ViewSettings {
		t.Fatalf("expected ViewSettings after F4, got %v", app.currentView)
	}
	if got := app.settingsView.ActiveSection(); got != settings.SectionAppearance {
		t.Fatalf("F4 opened section %q, want appearance", got)
	}

	// Back on the landing page, the top bar must carry both chips side by
	// side with the signed-out Guest label. Row 0 belongs to the top bar,
	// so the chip click must still open the account menu.
	app.currentView = ViewHome
	top := ansi.Strip(app.renderTopBar())
	if !strings.Contains(top, "Log in") {
		t.Fatalf("home top bar missing %q: %q", "Log in", top)
	}
	var account topBarHit
	found := false
	for _, hit := range app.topBarHits() {
		if hit.target == topBarTargetAccount {
			account, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("home account chip hit target missing")
	}
	m, _ := app.Update(tea.MouseMsg{
		X:      account.start,
		Y:      0,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	next := m.(AppModel)
	if !next.accountMenuOpen {
		t.Fatal("clicking the home account chip did not open the dropdown")
	}
}
