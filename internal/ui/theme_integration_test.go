package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/config"
	"openleet/internal/storage"
	"openleet/internal/ui/settings"
)

func newThemeTestApp(t *testing.T) (AppModel, *config.Config) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "openleet_theme_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	cfg := config.DefaultConfig()
	mgr, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppModel(cfg, mgr)
	state, _ := app.Update(tea.WindowSizeMsg{Width: 140, Height: 44})
	return state.(AppModel), cfg
}

// TestF4OpensSettingsAppearance pins the replacement for the old theme
// gallery: F4 opens Settings deep-linked to the Appearance section.
func TestF4OpensSettingsAppearance(t *testing.T) {
	app, _ := newThemeTestApp(t)
	state, _ := app.Update(tea.KeyMsg{Type: tea.KeyF4})
	app = state.(AppModel)

	if app.currentView != ViewSettings {
		t.Fatalf("expected ViewSettings after F4, got %v", app.currentView)
	}
	if got := app.settingsView.ActiveSection(); got != settings.SectionAppearance {
		t.Fatalf("F4 opened section %q, want appearance", got)
	}
	view := ansi.Strip(app.View())
	if !strings.Contains(view, "APPEARANCE") || !strings.Contains(view, "THEME PREVIEW") {
		t.Fatalf("expected the Appearance section in F4's view, got:\n%s", view)
	}
}

func TestApplyingThemeRepaintsEveryView(t *testing.T) {
	app, cfg := newThemeTestApp(t)
	state, _ := app.Update(settings.ApplyThemeMsg{Name: "dracula"})
	app = state.(AppModel)

	if app.palette.Name != "dracula" {
		t.Fatalf("expected the dracula palette, got %s", app.palette.Name)
	}
	if cfg.Theme != "dracula" {
		t.Fatalf("expected the choice to persist in config, got %s", cfg.Theme)
	}

	// The home chrome must pick up the new hint, keep its top bar chips, and
	// still render.
	homeView := app.View()
	if !strings.Contains(ansi.Strip(homeView), "ctrl+p commands") {
		t.Fatal("expected the commands hint in the home footer")
	}
	stripped := ansi.Strip(homeView)
	for _, want := range []string{"Settings", "Log in"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("home top bar missing %q:\n%s", want, stripped)
		}
	}

	// The dashboard and roadmap must repaint without dropping their content.
	app.currentView = ViewStats
	if statsView := app.View(); !strings.Contains(statsView, "openleet / stats") {
		t.Fatal("expected the dashboard to render after a theme switch")
	}

	app.currentView = ViewRoadmap
	if roadmap := app.View(); !strings.Contains(roadmap, "OPENLEET") {
		t.Fatal("expected the roadmap to render after a theme switch")
	}
}

func TestUnknownApplyFallsBackToBrand(t *testing.T) {
	app, cfg := newThemeTestApp(t)
	state, _ := app.Update(settings.ApplyThemeMsg{Name: "not-a-theme"})
	app = state.(AppModel)

	if app.palette.Name != "openleet" {
		t.Fatalf("expected fallback to openleet, got %s", app.palette.Name)
	}
	if cfg.Theme != "openleet" {
		t.Fatalf("expected fallback persisted, got %s", cfg.Theme)
	}
}
