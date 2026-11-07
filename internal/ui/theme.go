package ui

import (
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/theme"
)

// Theme contains colors and styled components for OpenLeet's own chrome
// (top bar, footer, badges). View packages derive their palettes from the same
// shared theme so a switch repaints everything.
type Theme struct {
	Primary       lipgloss.AdaptiveColor
	Secondary     lipgloss.AdaptiveColor
	Accent        lipgloss.AdaptiveColor
	AccentFG      lipgloss.AdaptiveColor
	Success       lipgloss.AdaptiveColor
	Warning       lipgloss.AdaptiveColor
	Danger        lipgloss.AdaptiveColor
	Pass          lipgloss.AdaptiveColor
	Fail          lipgloss.AdaptiveColor
	Warn          lipgloss.AdaptiveColor
	Info          lipgloss.AdaptiveColor
	Muted         lipgloss.AdaptiveColor
	Background    lipgloss.AdaptiveColor
	Surface       lipgloss.AdaptiveColor
	SurfaceRaised lipgloss.AdaptiveColor
	Border        lipgloss.AdaptiveColor
	BorderFocus   lipgloss.AdaptiveColor
	Text          lipgloss.AdaptiveColor
	TextDim       lipgloss.AdaptiveColor
	Subtext       lipgloss.AdaptiveColor

	// Pre-composed styles
	TitleStyle   lipgloss.Style
	StatusBadge  lipgloss.Style
	EasyBadge    lipgloss.Style
	MediumBadge  lipgloss.Style
	HardBadge    lipgloss.Style
	CardActive   lipgloss.Style
	CardInactive lipgloss.Style
	TopBar       lipgloss.Style
	FooterBar    lipgloss.Style
	Divider      lipgloss.Style
}

// DefaultTheme returns the OpenLeet brand palette.
func DefaultTheme() Theme {
	return themeFromPalette(theme.Default())
}

// themeFromPalette builds the application chrome palette from a shared theme.
func themeFromPalette(p theme.Palette) Theme {
	t := Theme{
		Primary:       p.Primary,
		Secondary:     p.Secondary,
		Accent:        p.Accent,
		AccentFG:      p.AccentFG,
		Success:       p.Success,
		Warning:       p.Warning,
		Danger:        p.Danger,
		Pass:          p.Pass,
		Fail:          p.Fail,
		Warn:          p.Warn,
		Info:          p.Info,
		Muted:         p.Muted,
		Background:    p.Background,
		Surface:       p.Surface,
		SurfaceRaised: p.SurfaceRaised,
		Border:        p.Border,
		BorderFocus:   p.BorderFocus,
		Text:          p.Text,
		TextDim:       p.TextDim,
		Subtext:       p.Subtext,
	}

	t.TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary)

	t.StatusBadge = lipgloss.NewStyle().
		Foreground(t.Subtext).
		Padding(0, 1)

	t.EasyBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Success)

	t.MediumBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Warning)

	t.HardBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Danger)

	t.CardActive = lipgloss.NewStyle().
		Background(t.Surface).
		Padding(0, 1)

	t.CardInactive = lipgloss.NewStyle().
		Foreground(t.Subtext).
		Padding(0, 1)

	t.TopBar = lipgloss.NewStyle().
		Background(t.Background).
		Foreground(t.Subtext).
		Padding(0, 1)

	t.FooterBar = lipgloss.NewStyle().
		Background(t.Background).
		Foreground(t.Subtext).
		Padding(0, 1)

	t.Divider = lipgloss.NewStyle().
		Foreground(t.Border)

	return t
}
