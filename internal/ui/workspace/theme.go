package workspace

import (
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/theme"
)

// Fixed rail colours. The grab rails keep the OpenLeet brand orange even when
// the rest of the workspace adopts a theme with a blue, pink, or pastel accent.
const (
	grabRailLight = "#e85d04"
	grabRailDark  = "#ff7a1a"
	grabRailInk   = "#140a04"
)

// WorkspaceTheme contains the adaptive palette and reusable styles for the
// problem-solving workspace.
type WorkspaceTheme struct {
	Primary    lipgloss.AdaptiveColor
	Secondary  lipgloss.AdaptiveColor
	Accent     lipgloss.AdaptiveColor
	Success    lipgloss.AdaptiveColor
	Warning    lipgloss.AdaptiveColor
	Danger     lipgloss.AdaptiveColor
	Muted      lipgloss.AdaptiveColor
	Background lipgloss.AdaptiveColor
	Surface    lipgloss.AdaptiveColor
	Border     lipgloss.AdaptiveColor
	Text       lipgloss.AdaptiveColor
	Subtext    lipgloss.AdaptiveColor
	Light      bool

	// GrabRail is the fixed brand orange used by the pane dividers. It is
	// intentionally not derived from Accent: the rails must stay orange under
	// every theme so the resize/close affordance never changes identity.
	GrabRail    lipgloss.AdaptiveColor
	GrabRailInk lipgloss.AdaptiveColor

	SyntaxKeyword     lipgloss.AdaptiveColor
	SyntaxString      lipgloss.AdaptiveColor
	SyntaxNumber      lipgloss.AdaptiveColor
	SyntaxComment     lipgloss.AdaptiveColor
	SyntaxFunction    lipgloss.AdaptiveColor
	SyntaxType        lipgloss.AdaptiveColor
	SyntaxBuiltin     lipgloss.AdaptiveColor
	SyntaxOperator    lipgloss.AdaptiveColor
	SyntaxPunctuation lipgloss.AdaptiveColor
	SyntaxError       lipgloss.AdaptiveColor

	PanelBorder lipgloss.Border
	FocusBorder lipgloss.Border

	HeaderStyle       lipgloss.Style
	TitleStyle        lipgloss.Style
	DescriptionHeader lipgloss.Style
	EditorHeader      lipgloss.Style
	ConsoleHeader     lipgloss.Style
	TabStyle          lipgloss.Style
	ActiveTabStyle    lipgloss.Style
	PanelStyle        lipgloss.Style
	FocusedPanelStyle lipgloss.Style
	LineNumberStyle   lipgloss.Style
	ActiveLineStyle   lipgloss.Style
	SuccessBadge      lipgloss.Style
	FailureBadge      lipgloss.Style
	RunningBadge      lipgloss.Style
	PendingBadge      lipgloss.Style
}

// DefaultWorkspaceTheme returns the Catppuccin Mocha-inspired adaptive theme
// used by workspace panels, tabs, headers, and runner status indicators.
func DefaultWorkspaceTheme() WorkspaceTheme {
	return newWorkspaceTheme(WorkspaceTheme{
		Primary:     lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"},
		Secondary:   lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"},
		Accent:      lipgloss.AdaptiveColor{Light: "#fe640b", Dark: "#fab387"},
		Success:     lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"},
		Warning:     lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"},
		Danger:      lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"},
		Muted:       lipgloss.AdaptiveColor{Light: "#7c7f93", Dark: "#6c7086"},
		Background:  lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#000000"},
		Surface:     lipgloss.AdaptiveColor{Light: "#f0f2f6", Dark: "#121218"},
		Border:      lipgloss.AdaptiveColor{Light: "#dcdfe7", Dark: "#22222e"},
		Text:        lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"},
		Subtext:     lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#7f849c"},
		Light:       false,
		PanelBorder: lipgloss.HiddenBorder(),
		FocusBorder: lipgloss.HiddenBorder(),
	})
}

// FromPalette derives the workspace palette from a shared application theme.
func FromPalette(p theme.Palette) WorkspaceTheme {
	return newWorkspaceTheme(WorkspaceTheme{
		Primary:    p.Primary,
		Secondary:  p.Secondary,
		Accent:     p.Accent,
		Success:    p.Success,
		Warning:    p.Warning,
		Danger:     p.Danger,
		Muted:      p.Muted,
		Background: p.Background,
		Surface:    p.Surface,
		Border:     p.Border,
		Text:       p.Text,
		Subtext:    p.Subtext,
		Light:      p.Light,

		PanelBorder: lipgloss.HiddenBorder(),
		FocusBorder: lipgloss.HiddenBorder(),
	})
}

// newWorkspaceTheme rebuilds every composed style for a palette.
func newWorkspaceTheme(t WorkspaceTheme) WorkspaceTheme {
	t.GrabRail = lipgloss.AdaptiveColor{Light: grabRailLight, Dark: grabRailDark}
	t.GrabRailInk = lipgloss.AdaptiveColor{Light: grabRailInk, Dark: grabRailInk}
	t.SyntaxKeyword = t.Primary
	t.SyntaxString = t.Success
	t.SyntaxNumber = t.Warning
	t.SyntaxComment = t.Muted
	t.SyntaxFunction = t.Secondary
	t.SyntaxType = t.Primary
	t.SyntaxBuiltin = t.Secondary
	t.SyntaxOperator = t.Subtext
	t.SyntaxPunctuation = t.Subtext
	t.SyntaxError = t.Danger

	t.HeaderStyle = lipgloss.NewStyle().Foreground(t.Subtext)
	t.TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.DescriptionHeader = lipgloss.NewStyle().Bold(true).Foreground(t.Secondary)
	t.EditorHeader = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.ConsoleHeader = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.TabStyle = lipgloss.NewStyle().Foreground(t.Subtext)
	t.ActiveTabStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Text)
	t.PanelStyle = lipgloss.NewStyle().Foreground(t.Text).Padding(0, 1)
	t.FocusedPanelStyle = lipgloss.NewStyle().Foreground(t.Text).Padding(0, 1)
	t.LineNumberStyle = lipgloss.NewStyle().Foreground(t.Muted).Align(lipgloss.Right)
	t.ActiveLineStyle = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface)
	t.SuccessBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Success)
	t.FailureBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Danger)
	t.RunningBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.PendingBadge = lipgloss.NewStyle().Foreground(t.Subtext)

	return t
}
