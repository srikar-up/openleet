package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/config"
	"openleet/internal/storage"
)

// TestFramesTrackResize drives the app through grow/shrink cycles and asserts
// that every view repaints to exactly the new terminal grid. A short frame
// leaves stale cells on screen, which is what "the TUI does not re-render after
// a resize" looks like.
func TestFramesTrackResize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_responsive_test")
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

	sizes := []struct{ w, h int }{
		{189, 56},
		{120, 40},
		{80, 24},
		{60, 20},
		{50, 16},
		{40, 12},
		{100, 30},
		{189, 56},
	}
	views := []ViewMode{ViewHome, ViewRoadmap, ViewSolve, ViewStats}

	for _, size := range sizes {
		state, _ := app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		app = state.(AppModel)

		for _, view := range views {
			app.currentView = view
			frame := app.View()
			rows := strings.Split(frame, "\n")
			if len(rows) != size.h {
				t.Fatalf("view %v at %dx%d: expected %d rows, got %d", view, size.w, size.h, size.h, len(rows))
			}
			for index, row := range rows {
				if width := lipgloss.Width(row); width != size.w {
					t.Fatalf("view %v at %dx%d: row %d is %d cells, want %d", view, size.w, size.h, index, width, size.w)
				}
			}
		}
	}
}

// TestResizeUpdatesChildModels asserts the size message actually reaches every
// child view, not just the root grid.
func TestResizeUpdatesChildModels(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_resize_child_test")
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

	for _, size := range []struct{ w, h int }{{160, 48}, {72, 22}, {100, 30}} {
		state, _ := app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		app = state.(AppModel)

		if app.width != size.w || app.height != size.h {
			t.Fatalf("root is %dx%d, want %dx%d", app.width, app.height, size.w, size.h)
		}
		if app.homeView.Width() != size.w || app.homeView.Height() != size.h-2 {
			t.Fatalf("home is %dx%d, want %dx%d (chrome owns 2 rows)", app.homeView.Width(), app.homeView.Height(), size.w, size.h-2)
		}
		if app.mapView.Width() != size.w || app.mapView.Height() != size.h-2 {
			t.Fatalf("map is %dx%d, want %dx%d (chrome owns 2 rows)", app.mapView.Width(), app.mapView.Height(), size.w, size.h-2)
		}
		if app.statsView.Width() != size.w || app.statsView.Height() != size.h {
			t.Fatalf("stats is %dx%d, want %dx%d", app.statsView.Width(), app.statsView.Height(), size.w, size.h)
		}
	}
}
