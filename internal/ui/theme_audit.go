package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/theme"
	"openleet/internal/ui/toast"
)

// themeAuditRow is one theme's pass/fail result.
type themeAuditRow struct {
	Name     string
	Label    string
	Failures []string
}

func (r themeAuditRow) pass() bool {
	return len(r.Failures) == 0
}

// runThemeAudit checks the semantic palette and the rendered toast/chip/button
// components at both supported audit widths.
func runThemeAudit() []themeAuditRow {
	rows := make([]themeAuditRow, 0, len(theme.All()))
	for _, pal := range theme.All() {
		failures := theme.Audit(pal)
		for _, width := range []int{100, 160} {
			failures = append(failures, auditThemeComponents(pal, width)...)
		}
		rows = append(rows, themeAuditRow{
			Name:     pal.Name,
			Label:    pal.Label,
			Failures: failures,
		})
	}
	return rows
}

func auditThemeComponents(pal theme.Palette, width int) []string {
	var failures []string
	add := func(format string, args ...any) {
		failures = append(failures, fmt.Sprintf(format, args...))
	}
	checkRendered := func(name, rendered string) {
		if strings.Contains(rendered, "\n") {
			add("%s at %d has a newline", name, width)
		}
		if got := lipgloss.Width(rendered); got > width {
			add("%s at %d is clipped (%d cells)", name, width, got)
		}
	}
	checkCard := func(name string, card toast.Rendered) {
		if card.Width > width {
			add("%s at %d is clipped (%d cells)", name, width, card.Width)
		}
		for index, line := range card.Lines {
			if got := lipgloss.Width(line); got != card.Width {
				add("%s at %d line %d is %d cells, want %d", name, width, index, got, card.Width)
			}
		}
	}

	// The top bar is plain text: two labels, no icons, no border fragments and
	// no stats. Audit both account states at every width that matters.
	for _, signedIn := range []bool{false, true} {
		app := AppModel{
			width:       width,
			height:      40,
			palette:     pal,
			theme:       themeFromPalette(pal),
			currentView: ViewSolve,
		}
		if signedIn {
			app.account = &accountSession{UID: "uid-audit", Email: "user@example.com", DisplayName: "neo"}
		}
		bar := app.renderTopBar()
		checkRendered("top bar", bar)
		if got := ansi.StringWidth(bar); got != width {
			add("top bar at %d has width %d", width, got)
		}
		plain := ansi.Strip(bar)

		wantAccount := "Log in"
		if signedIn {
			switch {
			case width >= 120:
				wantAccount = "user@example.com"
			case width >= 100:
				wantAccount = "user"
			default:
				wantAccount = "U"
			}
		}
		if !strings.Contains(plain, "Settings") {
			add("top bar at %d is missing the Settings label", width)
		}
		if !strings.Contains(plain, wantAccount) {
			add("top bar at %d is missing account label %q", width, wantAccount)
		}
		for _, banned := range []string{"[", "]", "|", "\\", "│", "╭", "╮", "┌", "┐", "*", "●", "○", "⚙", "Account", "streak", "solved"} {
			if strings.Contains(plain, banned) {
				add("top bar at %d contains forbidden %q", width, banned)
			}
		}

		var settingsHit, accountHit topBarHit
		for _, hit := range app.topBarHits() {
			switch hit.target {
			case topBarTargetSettings:
				settingsHit = hit
			case topBarTargetAccount:
				accountHit = hit
			}
		}
		if got := ansi.Cut(plain, settingsHit.start, settingsHit.end+1); !strings.Contains(got, "Settings") {
			add("settings hit at %d covers %q", width, got)
		}
		if got := ansi.Cut(plain, accountHit.start, accountHit.end+1); !strings.Contains(got, wantAccount) {
			add("account hit at %d covers %q", width, got)
		}
	}

	for _, kind := range []toast.Kind{toast.Info, toast.Success, toast.Warning, toast.Error} {
		card := toast.RenderCard(pal, toast.Toast{
			Kind:    kind,
			Title:   "Theme audit message",
			Message: "Semantic tokens at both widths",
			Count:   1,
		}, 40, true)
		checkCard("toast card", card)
	}
	return failures
}
