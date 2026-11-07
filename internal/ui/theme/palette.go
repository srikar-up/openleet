// Package theme defines the shared colour palette and the registry of
// selectable themes. Every view derives its styles from a Palette so switching
// themes repaints the whole application at once.
package theme

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette is the single source of truth for one theme. Backgrounds are always
// explicit so a theme looks the same on light and dark terminals.
type Palette struct {
	Name        string
	Label       string
	Description string
	// Light marks palettes whose background is deliberate light paper so
	// consumers can choose light-safe rendering (Glamour/Markdown).
	Light bool

	Background    lipgloss.AdaptiveColor
	Surface       lipgloss.AdaptiveColor
	SurfaceRaised lipgloss.AdaptiveColor
	Card          lipgloss.AdaptiveColor
	CardHighlight lipgloss.AdaptiveColor
	Border        lipgloss.AdaptiveColor
	BorderFocus   lipgloss.AdaptiveColor
	Text          lipgloss.AdaptiveColor
	TextDim       lipgloss.AdaptiveColor
	Subtext       lipgloss.AdaptiveColor
	Muted         lipgloss.AdaptiveColor
	Faint         lipgloss.AdaptiveColor
	Inactive      lipgloss.AdaptiveColor
	Stripe        lipgloss.AdaptiveColor

	Accent    lipgloss.AdaptiveColor
	AccentFG  lipgloss.AdaptiveColor
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor

	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Danger  lipgloss.AdaptiveColor

	// Kind colours are guarded against the raised surface so badges and
	// toasts remain readable in every theme.
	Pass lipgloss.AdaptiveColor
	Fail lipgloss.AdaptiveColor
	Warn lipgloss.AdaptiveColor
	Info lipgloss.AdaptiveColor

	Easy   lipgloss.AdaptiveColor
	Medium lipgloss.AdaptiveColor
	Hard   lipgloss.AdaptiveColor

	HeatLow  lipgloss.AdaptiveColor
	HeatMid  lipgloss.AdaptiveColor
	HeatHigh lipgloss.AdaptiveColor
}

// Swatches lists the colours shown in the theme picker, most representative
// first.
func (p Palette) Swatches() []lipgloss.AdaptiveColor {
	return []lipgloss.AdaptiveColor{p.Accent, p.Primary, p.Secondary, p.Success, p.Danger}
}

// ApplyAccent returns a copy of p with Accent and HeatHigh replaced by value.
// Empty, "theme", non-#RRGGBB preset names, and invalid hex leave the palette
// unchanged.
func ApplyAccent(p Palette, value string) Palette {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "theme" {
		return p
	}
	if !strings.HasPrefix(trimmed, "#") {
		return p
	}
	if _, _, _, ok := parseHex(trimmed); !ok {
		return p
	}
	p.Accent = hex(trimmed)
	p.HeatHigh = hex(trimmed)
	return Normalize(p)
}

// hex pins a colour to the same value in both terminal modes. Themes define
// their own background, so they stay legible regardless of the host palette.
func hex(value string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: value, Dark: value}
}

// spec is the compact authoring form for a theme. Tones that are not listed
// are derived from the base colours so every theme stays internally coherent.
type spec struct {
	name        string
	label       string
	description string
	light       bool

	background string
	surface    string
	text       string
	subtext    string
	muted      string

	accent    string
	primary   string
	secondary string

	success string
	warning string
	danger  string

	easy   string
	medium string
	hard   string

	border string
}

// build expands a spec into a full Palette, mixing secondary tones from the
// base colours so borders, cards, and heat levels always harmonise.
func build(s spec) Palette {
	p := Palette{
		Name:        s.name,
		Label:       s.label,
		Description: s.description,
		Light:       s.light,

		Background: hex(s.background),
		Surface:    hex(s.surface),
		Text:       hex(s.text),
		Subtext:    hex(s.subtext),
		Muted:      hex(s.muted),

		Accent:    hex(s.accent),
		Primary:   hex(s.primary),
		Secondary: hex(s.secondary),

		Success: hex(s.success),
		Warning: hex(s.warning),
		Danger:  hex(s.danger),

		Easy:   hex(s.easy),
		Medium: hex(s.medium),
		Hard:   hex(s.hard),
	}

	p.Card = hex(mix(s.surface, s.text, 0.05))
	p.CardHighlight = hex(mix(s.surface, s.text, 0.10))
	border := s.border
	if border == "" {
		border = mix(s.surface, s.text, 0.16)
	}
	p.Border = hex(border)
	p.Faint = hex(mix(s.background, s.text, 0.22))
	p.Inactive = hex(mix(s.background, s.text, 0.45))
	p.Stripe = hex(mix(s.background, s.text, 0.35))

	p.HeatHigh = hex(s.accent)
	p.HeatMid = hex(mix(s.background, s.accent, 0.62))
	p.HeatLow = hex(mix(s.background, s.accent, 0.32))

	return Normalize(p)
}

// Normalize fills every semantic token and applies WCAG contrast guards. It is
// safe to call on user-authored palettes: missing fields are derived from the
// background, surface, text, and accent instead of crashing or rendering
// invisible text.
func Normalize(p Palette) Palette {
	background := adaptiveValue(p.Background)
	if background == "" {
		background = "#000000"
		p.Background = hex(background)
	}
	text := adaptiveValue(p.Text)
	if text == "" {
		text = "#ffffff"
		p.Text = hex(text)
	}
	surface := adaptiveValue(p.Surface)
	if surface == "" {
		surface = mix(background, text, 0.06)
		p.Surface = hex(surface)
	}

	surfaceRaised := adaptiveValue(p.SurfaceRaised)
	if surfaceRaised == "" {
		surfaceRaised = mix(surface, text, 0.12)
	}
	surfaceRaised = EnsureContrast(surfaceRaised, background, 1.15)
	p.SurfaceRaised = guardedColor(p.SurfaceRaised, surfaceRaised, p.Name, "surface_raised")

	text = EnsureContrast(text, background, 4.5)
	p.Text = guardedColor(p.Text, text, p.Name, "text")

	textDim := adaptiveValue(p.TextDim)
	if textDim == "" {
		textDim = adaptiveValue(p.Subtext)
	}
	if textDim == "" {
		textDim = mix(text, background, 0.35)
	}
	textDim = EnsureContrast(textDim, surfaceRaised, 4.5)
	p.TextDim = guardedColor(p.TextDim, textDim, p.Name, "text_dim")

	border := adaptiveValue(p.Border)
	if border == "" {
		border = mix(surfaceRaised, text, 0.20)
	}
	border = EnsureContrast(border, surfaceRaised, 3.0)
	border = EnsureContrast(border, background, 3.0)
	p.Border = guardedColor(p.Border, border, p.Name, "border")

	borderFocus := adaptiveValue(p.BorderFocus)
	if borderFocus == "" {
		borderFocus = adaptiveValue(p.Accent)
	}
	if borderFocus == "" {
		borderFocus = border
	}
	borderFocus = EnsureContrast(borderFocus, surfaceRaised, 3.0)
	p.BorderFocus = guardedColor(p.BorderFocus, borderFocus, p.Name, "border_focus")

	accent := adaptiveValue(p.Accent)
	if accent == "" {
		accent = text
		p.Accent = hex(accent)
	}
	accentFG := adaptiveValue(p.AccentFG)
	if accentFG == "" {
		accentFG = BestTextOn(accent, background, text)
	}
	accentFG = EnsureContrast(accentFG, accent, 4.5)
	p.AccentFG = guardedColor(p.AccentFG, accentFG, p.Name, "accent_fg")

	kinds := []struct {
		name  string
		value lipgloss.AdaptiveColor
	}{
		{"pass", firstColor(p.Pass, p.Success, p.Primary)},
		{"fail", firstColor(p.Fail, p.Danger, p.Accent)},
		{"warn", firstColor(p.Warn, p.Warning, p.Accent)},
		{"info", firstColor(p.Info, p.Secondary, p.Primary)},
	}
	guarded := make([]string, 0, len(kinds))
	for i, kind := range kinds {
		value := adaptiveValue(kind.value)
		if value == "" {
			value = text
		}
		value = EnsureContrast(value, surfaceRaised, 3.0)
		others := append([]string{text}, guarded...)
		value = distinctColor(value, others, surfaceRaised)
		guarded = append(guarded, value)
		kinds[i].value = hex(value)
	}
	p.Pass = guardedColor(p.Pass, guarded[0], p.Name, "pass")
	p.Fail = guardedColor(p.Fail, guarded[1], p.Name, "fail")
	p.Warn = guardedColor(p.Warn, guarded[2], p.Name, "warn")
	p.Info = guardedColor(p.Info, guarded[3], p.Name, "info")

	return p
}

func adaptiveValue(color lipgloss.AdaptiveColor) string {
	if strings.TrimSpace(color.Dark) != "" {
		return color.Dark
	}
	return color.Light
}

func firstColor(colors ...lipgloss.AdaptiveColor) lipgloss.AdaptiveColor {
	for _, color := range colors {
		if adaptiveValue(color) != "" {
			return color
		}
	}
	return lipgloss.AdaptiveColor{}
}

func guardedColor(original lipgloss.AdaptiveColor, value, name, field string) lipgloss.AdaptiveColor {
	old := adaptiveValue(original)
	if old != "" && !strings.EqualFold(old, value) {
		debugWarn("theme %s: adjusted %s from %s to %s for contrast", name, field, old, value)
	}
	return hex(value)
}

// distinctColor nudges a kind colour until it is visibly different from every
// other kind colour and from body text.
func distinctColor(value string, others []string, background string) string {
	if len(others) == 0 {
		return EnsureContrast(value, background, 3.0)
	}
	target := "#ffffff"
	if lum, ok := relativeLuminance(others[0]); ok && lum > 0.45 {
		target = "#000000"
	}
	for i := 0; i < 80; i++ {
		value = EnsureContrast(value, background, 3.0)
		distinct := true
		for _, other := range others {
			if strings.EqualFold(value, other) || ColorDistance(value, other) < 50 {
				distinct = false
				break
			}
		}
		if distinct {
			return value
		}
		value = mix(value, target, 0.05)
	}
	return EnsureContrast(value, background, 3.0)
}

// mix linearly interpolates from one hex colour to another. t=0 returns from,
// t=1 returns to.
func mix(from, to string, t float64) string {
	fr, fg, fb, ok := parseHex(from)
	if !ok {
		return from
	}
	tr, tg, tb, ok := parseHex(to)
	if !ok {
		return from
	}
	blend := func(a, b int) int {
		value := float64(a) + (float64(b-a) * t)
		if value < 0 {
			return 0
		}
		if value > 255 {
			return 255
		}
		return int(value + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", blend(fr, tr), blend(fg, tg), blend(fb, tb))
}

// parseHex converts "#rrggbb" into its components.
func parseHex(value string) (int, int, int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(trimmed) != 6 {
		return 0, 0, 0, false
	}
	r, errR := strconv.ParseInt(trimmed[0:2], 16, 32)
	g, errG := strconv.ParseInt(trimmed[2:4], 16, 32)
	b, errB := strconv.ParseInt(trimmed[4:6], 16, 32)
	if errR != nil || errG != nil || errB != nil {
		return 0, 0, 0, false
	}
	return int(r), int(g), int(b), true
}
