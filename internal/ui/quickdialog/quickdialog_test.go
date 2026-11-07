package quickdialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/theme"
)

func testPalette() theme.Palette { return theme.Resolve("openleet") }

func TestActiveItemIsPreselected(t *testing.T) {
	m := NewList(Options{Title: "THEMES", Search: true}, []Group{{
		Title: "PALETTES",
		Items: []Item{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta", Active: true},
		},
	}}, testPalette(), 120, 40)

	if got := m.SelectedID(); got != "b" {
		t.Fatalf("expected the active item, got %q", got)
	}
}

func TestFilterNarrowsRowsAndDropsEmptyGroups(t *testing.T) {
	m := NewList(Options{Title: "LANGUAGES", Search: true}, []Group{
		{Title: "POPULAR", Items: []Item{{ID: "python", Label: "Python"}, {ID: "go", Label: "Go"}}},
		{Title: "OTHER", Items: []Item{{ID: "rust", Label: "Rust"}}},
	}, testPalette(), 120, 40)

	for _, r := range "rust" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if got := m.SelectedID(); got != "rust" {
		t.Fatalf("expected rust after filtering, got %q", got)
	}
	for _, row := range m.rows {
		if row.header && strings.EqualFold(row.label, "POPULAR") {
			t.Fatal("empty group heading should be removed")
		}
	}
}

func TestNavigationSkipsGroupHeadingsAndWraps(t *testing.T) {
	m := NewList(Options{Title: "MODELS"}, []Group{
		{Title: "A", Items: []Item{{ID: "one", Label: "One"}}},
		{Title: "B", Items: []Item{{ID: "two", Label: "Two"}}},
	}, testPalette(), 120, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.SelectedID(); got != "two" {
		t.Fatalf("expected two, got %q", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.SelectedID(); got != "one" {
		t.Fatalf("expected wrap to one, got %q", got)
	}
}

func TestInputValueIsReturnedButMaskedInPanel(t *testing.T) {
	m := NewInput("API KEY", "Key", "hunter2", true, testPalette(), 120, 40)
	if got := m.InputValue(); got != "hunter2" {
		t.Fatalf("unexpected input value %q", got)
	}
	panel := ansi.Strip(m.Panel())
	if strings.Contains(panel, "hunter2") {
		t.Fatalf("masked input leaked into the panel: %q", panel)
	}
	if !strings.Contains(panel, "•") {
		t.Fatal("expected mask characters in the panel")
	}
}

func TestConfirmTogglesYesAndNo(t *testing.T) {
	m := NewConfirm("RESET", "Reset the buffer?", testPalette(), 120, 40)
	if !m.Confirmed() {
		t.Fatal("expected Yes by default")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.Confirmed() {
		t.Fatal("expected No after n")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.Confirmed() {
		t.Fatal("expected Yes after y")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Confirmed() {
		t.Fatal("expected No after right arrow")
	}
}

func TestPanelFitsViewportAndRowsShareWidth(t *testing.T) {
	for _, size := range []struct{ w, h int }{{100, 30}, {160, 50}} {
		m := NewList(Options{Title: "COMMANDS", Search: true}, []Group{{
			Title: "QUICK ACTIONS",
			Items: []Item{
				{ID: "themes", Label: "/themes", Value: "Switch colour theme"},
				{ID: "lang", Label: "/lang", Value: "Switch language"},
			},
		}}, testPalette(), size.w, size.h)

		panel := ansi.Strip(m.Panel())
		lines := strings.Split(panel, "\n")
		width := ansi.StringWidth(lines[0])
		if width > size.w {
			t.Fatalf("%dx%d: panel width %d exceeds viewport", size.w, size.h, width)
		}
		for i, line := range lines {
			if got := ansi.StringWidth(line); got != width {
				t.Fatalf("%dx%d: row %d is %d cells, want %d (%q)", size.w, size.h, i, got, width, line)
			}
		}
		if !strings.Contains(panel, "COMMANDS") || !strings.Contains(panel, "esc") {
			t.Fatalf("missing title or esc hint: %q", panel)
		}
	}
}

func TestReadOnlySearchStillFilters(t *testing.T) {
	m := NewList(Options{Title: "KEYS", Search: true, ReadOnly: true}, []Group{{
		Title: "KEYBOARD",
		Items: []Item{{Label: "Run tests", Value: "ctrl+r"}, {Label: "Save", Value: "ctrl+s"}},
	}}, testPalette(), 120, 40)
	for _, r := range "save" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if _, ok := m.SelectedItem(); !ok {
		t.Fatal("expected a filtered read-only row")
	}
}

func TestPanelDoesNotWrapTitleOrCommandRows(t *testing.T) {
	m := NewList(Options{Title: "COMMANDS", Search: true}, []Group{{
		Title: "QUICK ACTIONS",
		Items: []Item{{
			ID:    "pick_problem",
			Label: "/solve",
			Value: "Choose a problem (F2 to solve)",
		}},
	}}, testPalette(), 160, 50)

	panel := ansi.Strip(m.Panel())
	titleLine := ""
	commandLine := ""
	for _, line := range strings.Split(panel, "\n") {
		if strings.Contains(line, "COMMANDS") {
			titleLine = line
		}
		if strings.Contains(line, "/solve") {
			commandLine = line
		}
	}
	if !strings.Contains(titleLine, "esc") {
		t.Fatalf("title and esc hint wrapped onto separate rows: %q", panel)
	}
	if !strings.Contains(commandLine, "Choose a problem") {
		t.Fatalf("command description wrapped onto another row: %q", panel)
	}
}

func TestNarrowPanelTruncatesValuesWithEllipsis(t *testing.T) {
	groups := []Group{{
		Title: "PALETTES",
		Items: []Item{{
			ID:     "openleet-light",
			Label:  "OpenLeet Light",
			Value:  "Brand orange on warm paper - a deliberately long description",
			Active: true,
		}},
	}}
	m := NewList(Options{Title: "THEMES", Search: true}, groups, testPalette(), 60, 30)

	panel := ansi.Strip(m.Panel())
	lines := strings.Split(panel, "\n")
	width := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Fatalf("row %d is %d cells, want %d (%q)", i, got, width, line)
		}
	}
	if width > 60 {
		t.Fatalf("panel width %d exceeds the 60-cell viewport", width)
	}
	if !strings.Contains(panel, "\u2026") {
		t.Fatalf("expected an ellipsis on the truncated value: %q", panel)
	}
}
