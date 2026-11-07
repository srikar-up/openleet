package home

import (
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

// RenderLogo returns the styled pixel wordmark with dual-tone shading.
// If terminal width is too narrow (< 45 cells), returns a compact clean text logo.
func RenderLogo(subtextColor, accentColor, backgroundColor lipgloss.AdaptiveColor, termWidth int) string {
	if termWidth < 60 {
		open := lipgloss.NewStyle().Bold(true).Foreground(subtextColor).Render("OPEN")
		leet := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("LEET")
		return open + " " + leet
	}

	return uikit.RenderWordmarkBlock("OPEN", "LEET", subtextColor, accentColor, backgroundColor)
}
