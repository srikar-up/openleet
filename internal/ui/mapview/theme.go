package mapview

import (
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/theme"
)

// MapTheme holds the Figma roadmap palette and its reusable Lip Gloss styles.
// Both adaptive variants use the mockup value so the design remains consistent
// regardless of the terminal's light/dark background detection.
type MapTheme struct {
	Easy          lipgloss.AdaptiveColor
	Medium        lipgloss.AdaptiveColor
	Hard          lipgloss.AdaptiveColor
	Accent        lipgloss.AdaptiveColor
	Primary       lipgloss.AdaptiveColor
	Secondary     lipgloss.AdaptiveColor
	Surface       lipgloss.AdaptiveColor
	Card          lipgloss.AdaptiveColor
	CardHighlight lipgloss.AdaptiveColor
	Border        lipgloss.AdaptiveColor
	Muted         lipgloss.AdaptiveColor
	Background    lipgloss.AdaptiveColor
	Text          lipgloss.AdaptiveColor
	Subtext       lipgloss.AdaptiveColor
	Inactive      lipgloss.AdaptiveColor
	Stripe        lipgloss.AdaptiveColor

	TitleStyle       lipgloss.Style
	HeaderStyle      lipgloss.Style
	BreadcrumbStyle  lipgloss.Style
	BreadcrumbAccent lipgloss.Style
	ProgressFill     lipgloss.Style
	ProgressEmpty    lipgloss.Style
	FilterStyle      lipgloss.Style
	ActiveFilter     lipgloss.Style
	EasyBadge        lipgloss.Style
	MediumBadge      lipgloss.Style
	HardBadge        lipgloss.Style
	NodeStyle        lipgloss.Style
	ActiveNode       lipgloss.Style
	SolvedNode       lipgloss.Style
	LockedNode       lipgloss.Style
	InspectorStyle   lipgloss.Style
	InspectorHeading lipgloss.Style
	CommandStyle     lipgloss.Style
	PromptStyle      lipgloss.Style
	TopicStyle       lipgloss.Style
	ActiveTopic      lipgloss.Style
	RailStyle        lipgloss.Style
	DividerStyle     lipgloss.Style
	CanvasStyle      lipgloss.Style
	RoadmapBadge     lipgloss.Style
	CardStyle        lipgloss.Style
	CardBorderStyle  lipgloss.Style
	FooterStyle      lipgloss.Style
	SolveButton      lipgloss.Style
}

func adaptive(hex string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: hex, Dark: hex}
}

// DefaultMapTheme returns the exact palette used by the Figma roadmap.
func DefaultMapTheme() MapTheme {
	return newMapTheme(MapTheme{
		Easy:          adaptive("#cfcfcf"),
		Medium:        adaptive("#b5f9ad"),
		Hard:          adaptive("#ffb27a"),
		Accent:        adaptive("#ff7a1a"),
		Primary:       adaptive("#8fa8f5"),
		Secondary:     adaptive("#8fa8f5"),
		Surface:       adaptive("#131313"),
		Card:          adaptive("#1f1f1f"),
		CardHighlight: adaptive("#2b2b2b"),
		Border:        adaptive("#3a3a3a"),
		Muted:         adaptive("#8a8a8a"),
		Background:    adaptive("#0d0a0b"),
		Text:          adaptive("#f2f2f2"),
		Subtext:       adaptive("#8a8a8a"),
		Inactive:      adaptive("#6e6e6e"),
		Stripe:        adaptive("#d9d9d9"),
	})
}

// FromPalette derives the roadmap palette from a shared application theme.
func FromPalette(p theme.Palette) MapTheme {
	return newMapTheme(MapTheme{
		Easy:          p.Easy,
		Medium:        p.Medium,
		Hard:          p.Hard,
		Accent:        p.Accent,
		Primary:       p.Primary,
		Secondary:     p.Secondary,
		Surface:       p.Surface,
		Card:          p.Card,
		CardHighlight: p.CardHighlight,
		Border:        p.Border,
		Muted:         p.Muted,
		Background:    p.Background,
		Text:          p.Text,
		Subtext:       p.Subtext,
		Inactive:      p.Inactive,
		Stripe:        p.Stripe,
	})
}

// newMapTheme fills in the derived tones and rebuilds every composed style.
func newMapTheme(t MapTheme) MapTheme {
	if t.CardHighlight.Light == "" && t.CardHighlight.Dark == "" {
		t.CardHighlight = t.Card
	}
	if t.Stripe.Light == "" && t.Stripe.Dark == "" {
		t.Stripe = t.Border
	}

	t.TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.HeaderStyle = lipgloss.NewStyle().Foreground(t.Muted)
	t.BreadcrumbStyle = lipgloss.NewStyle().Foreground(t.Text)
	t.BreadcrumbAccent = lipgloss.NewStyle().Foreground(t.Accent)
	t.ProgressFill = lipgloss.NewStyle().Foreground(t.Medium)
	t.ProgressEmpty = lipgloss.NewStyle().Foreground(t.Stripe)
	t.FilterStyle = lipgloss.NewStyle().Foreground(t.Inactive)
	t.ActiveFilter = lipgloss.NewStyle().Bold(true).Foreground(t.Text)
	t.EasyBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Easy)
	t.MediumBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Medium)
	t.HardBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Hard)
	t.NodeStyle = lipgloss.NewStyle().Foreground(t.Text)
	t.ActiveNode = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.SolvedNode = lipgloss.NewStyle().Foreground(t.Medium)
	t.LockedNode = lipgloss.NewStyle().Foreground(t.Muted)
	t.InspectorStyle = lipgloss.NewStyle().Foreground(t.Text)
	t.InspectorHeading = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.CommandStyle = lipgloss.NewStyle().Foreground(t.Subtext)
	t.PromptStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.TopicStyle = lipgloss.NewStyle().Foreground(t.Inactive).Background(t.Card)
	t.ActiveTopic = lipgloss.NewStyle().Bold(true).Foreground(t.Text).Background(t.CardHighlight)
	t.RailStyle = lipgloss.NewStyle().Foreground(t.Accent)
	t.DividerStyle = lipgloss.NewStyle().Foreground(t.Accent)
	t.CanvasStyle = lipgloss.NewStyle().Background(t.Surface)
	t.RoadmapBadge = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(t.Accent).Border(lipgloss.NormalBorder()).BorderForeground(t.Accent).Padding(0, 1)
	t.CardStyle = lipgloss.NewStyle().Background(t.Card)
	t.CardBorderStyle = lipgloss.NewStyle().Foreground(t.Border).Background(t.Card)
	t.FooterStyle = lipgloss.NewStyle().Foreground(t.Muted)
	t.SolveButton = lipgloss.NewStyle().Bold(true).Foreground(t.Background).Background(t.Medium).Padding(0, 1)

	return t
}
