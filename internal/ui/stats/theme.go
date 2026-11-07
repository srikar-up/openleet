package stats

import (
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/theme"
)

// palette is the subset of theme colours the dashboard draws with.
type palette struct {
	Background lipgloss.AdaptiveColor
	Text       lipgloss.AdaptiveColor
	Muted      lipgloss.AdaptiveColor
	Faint      lipgloss.AdaptiveColor
	Accent     lipgloss.AdaptiveColor

	Easy   lipgloss.AdaptiveColor
	Medium lipgloss.AdaptiveColor
	Hard   lipgloss.AdaptiveColor

	HeatLow  lipgloss.AdaptiveColor
	HeatMid  lipgloss.AdaptiveColor
	HeatHigh lipgloss.AdaptiveColor
}

// defaultPalette returns the brand colours used when no theme is applied.
func defaultPalette() palette {
	return palette{
		Background: lipgloss.AdaptiveColor{Light: "#0d0a0b", Dark: "#0d0a0b"},
		Text:       lipgloss.AdaptiveColor{Light: "#f2f2f2", Dark: "#f2f2f2"},
		Muted:      lipgloss.AdaptiveColor{Light: "#8a8a8a", Dark: "#8a8a8a"},
		Faint:      lipgloss.AdaptiveColor{Light: "#3a3a3a", Dark: "#3a3a3a"},
		Accent:     lipgloss.AdaptiveColor{Light: "#ff7a1a", Dark: "#ff7a1a"},
		Easy:       lipgloss.AdaptiveColor{Light: "#cfcfcf", Dark: "#cfcfcf"},
		Medium:     lipgloss.AdaptiveColor{Light: "#b5f9ad", Dark: "#b5f9ad"},
		Hard:       lipgloss.AdaptiveColor{Light: "#ffb27a", Dark: "#ffb27a"},
		HeatLow:    lipgloss.AdaptiveColor{Light: "#7a4a20", Dark: "#7a4a20"},
		HeatMid:    lipgloss.AdaptiveColor{Light: "#c96a1e", Dark: "#c96a1e"},
		HeatHigh:   lipgloss.AdaptiveColor{Light: "#ff7a1a", Dark: "#ff7a1a"},
	}
}

// paletteFromTheme maps a shared theme onto the dashboard colours.
func paletteFromTheme(p theme.Palette) palette {
	return palette{
		Background: p.Background,
		Text:       p.Text,
		Muted:      p.Muted,
		Faint:      p.Faint,
		Accent:     p.Accent,
		Easy:       p.Easy,
		Medium:     p.Medium,
		Hard:       p.Hard,
		HeatLow:    p.HeatLow,
		HeatMid:    p.HeatMid,
		HeatHigh:   p.HeatHigh,
	}
}
