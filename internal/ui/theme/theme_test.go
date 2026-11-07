package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func isEmpty(c lipgloss.AdaptiveColor) bool {
	return c.Light == "" && c.Dark == ""
}

func TestRegistryHasUniqueNames(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range Names() {
		if seen[name] {
			t.Fatalf("duplicate theme name %q", name)
		}
		seen[name] = true
	}
	if len(seen) < 8 {
		t.Fatalf("expected a useful catalogue, got %d themes", len(seen))
	}
	if !seen["openleet"] {
		t.Fatal("expected the openleet brand theme to be registered")
	}
}

func TestEveryPaletteIsComplete(t *testing.T) {
	for _, p := range All() {
		colors := map[string]lipgloss.AdaptiveColor{
			"background":     p.Background,
			"surface":        p.Surface,
			"surface_raised": p.SurfaceRaised,
			"card":           p.Card,
			"highlight":      p.CardHighlight,
			"border":         p.Border,
			"border_focus":   p.BorderFocus,
			"text":           p.Text,
			"text_dim":       p.TextDim,
			"subtext":        p.Subtext,
			"muted":          p.Muted,
			"faint":          p.Faint,
			"inactive":       p.Inactive,
			"stripe":         p.Stripe,
			"accent":         p.Accent,
			"accent_fg":      p.AccentFG,
			"primary":        p.Primary,
			"secondary":      p.Secondary,
			"success":        p.Success,
			"warning":        p.Warning,
			"danger":         p.Danger,
			"pass":           p.Pass,
			"fail":           p.Fail,
			"warn":           p.Warn,
			"info":           p.Info,
			"easy":           p.Easy,
			"medium":         p.Medium,
			"hard":           p.Hard,
			"heat-low":       p.HeatLow,
			"heat-mid":       p.HeatMid,
			"heat-high":      p.HeatHigh,
		}
		for field, color := range colors {
			if isEmpty(color) {
				t.Fatalf("%s: %s is empty", p.Name, field)
			}
			if !strings.HasPrefix(color.Dark, "#") || len(color.Dark) != 7 {
				t.Fatalf("%s: %s has invalid dark value %q", p.Name, field, color.Dark)
			}
			if color.Light != color.Dark {
				t.Fatalf("%s: %s should be pinned across modes", p.Name, field)
			}
		}
		if p.Label == "" || p.Description == "" {
			t.Fatalf("%s: missing label or description", p.Name)
		}
	}
}

func TestAliasesResolve(t *testing.T) {
	cases := map[string]string{
		"catppuccin": "catppuccin-mocha",
		"one-dark":   "onedark",
		"tokyo":      "tokyonight",
		"default":    "openleet",
		"open-code":  "opencode",
		"light":      "openleet-light",
		"latte":      "catppuccin-latte",
	}
	for alias, want := range cases {
		got, ok := Get(alias)
		if !ok {
			t.Fatalf("alias %q did not resolve", alias)
		}
		if got.Name != want {
			t.Fatalf("alias %q resolved to %s, want %s", alias, got.Name, want)
		}
	}
	if _, ok := Get("not-a-theme"); ok {
		t.Fatal("expected unknown theme to fail")
	}
	if got := Resolve("not-a-theme"); got.Name != "openleet" {
		t.Fatalf("expected fallback to openleet, got %s", got.Name)
	}
}

func TestLightThemesAreRegistered(t *testing.T) {
	ol := Resolve("openleet-light")
	if ol.Name != "openleet-light" {
		t.Fatalf("expected openleet-light to exist, got %s", ol.Name)
	}
	cl := Resolve("catppuccin-latte")
	if cl.Name != "catppuccin-latte" {
		t.Fatalf("expected catppuccin-latte to exist, got %s", cl.Name)
	}

	if !ol.Light {
		t.Fatal("expected openleet-light.Light to be true")
	}
	if !cl.Light {
		t.Fatal("expected catppuccin-latte.Light to be true")
	}

	if ol.Background == ol.Text {
		t.Fatal("expected openleet-light background and text values to differ")
	}
	if cl.Background == cl.Text {
		t.Fatal("expected catppuccin-latte background and text values to differ")
	}

	if Default().Light {
		t.Fatal("expected Default().Light to be false")
	}

	got, ok := Get("light")
	if !ok || got.Name != "openleet-light" {
		t.Fatalf("expected Get(\"light\") to return openleet-light, got %v (ok=%v)", got.Name, ok)
	}
}

func TestHeatRampIsOrdered(t *testing.T) {
	for _, p := range All() {
		if p.HeatLow.Dark == p.HeatMid.Dark || p.HeatMid.Dark == p.HeatHigh.Dark {
			t.Fatalf("%s: heat ramp is not graded", p.Name)
		}
		if p.HeatHigh.Dark != p.Accent.Dark {
			t.Fatalf("%s: heat-high should match the accent", p.Name)
		}
	}
}

func TestSemanticAuditPassesForEveryTheme(t *testing.T) {
	results := AuditAll()
	if len(results) != len(Names()) {
		t.Fatalf("audited %d themes, want %d", len(results), len(Names()))
	}
	for _, result := range results {
		if !result.Pass() {
			t.Fatalf("%s failed theme audit: %v", result.Name, result.Failures)
		}
	}
}

func TestAuditDetectsMissingAndLowContrastTokens(t *testing.T) {
	bad := Palette{
		Name:       "broken",
		Background: hex("#101010"),
		Surface:    hex("#151515"),
		Text:       hex("#181818"),
		Border:     hex("#111111"),
		Accent:     hex("#101010"),
		Pass:       hex("#202020"),
		Fail:       hex("#202020"),
		Warn:       hex("#202020"),
		Info:       hex("#202020"),
	}
	bad = Normalize(bad)
	// Normalize fixes tokens, so force the broken pairs back to prove the
	// auditor itself catches them.
	bad.Text = hex("#181818")
	bad.TextDim = hex("")
	failures := Audit(bad)
	if len(failures) == 0 {
		t.Fatal("expected the auditor to reject missing and low-contrast tokens")
	}
}

func TestNormalizeFillsMissingSemanticTokens(t *testing.T) {
	raw := Palette{
		Name:       "custom",
		Background: hex("#101418"),
		Surface:    hex("#161c22"),
		Text:       hex("#e8edf2"),
		Subtext:    hex("#566474"),
		Accent:     hex("#ff8800"),
		Success:    hex("#44cc66"),
		Warning:    hex("#e0a000"),
		Danger:     hex("#ff4444"),
		Secondary:  hex("#6688ff"),
	}
	got := Normalize(raw)
	if got.SurfaceRaised.Dark == "" || got.TextDim.Dark == "" || got.BorderFocus.Dark == "" ||
		got.AccentFG.Dark == "" || got.Pass.Dark == "" || got.Fail.Dark == "" ||
		got.Warn.Dark == "" || got.Info.Dark == "" {
		t.Fatalf("normalized palette has missing tokens: %#v", got)
	}
	if failures := Audit(got); len(failures) > 0 {
		t.Fatalf("normalized custom palette failed audit: %v", failures)
	}
}

func TestMixInterpolates(t *testing.T) {
	if got := mix("#000000", "#ffffff", 0); got != "#000000" {
		t.Fatalf("expected start colour, got %s", got)
	}
	if got := mix("#000000", "#ffffff", 1); got != "#ffffff" {
		t.Fatalf("expected end colour, got %s", got)
	}
	if got := mix("#000000", "#ffffff", 0.5); got != "#808080" {
		t.Fatalf("expected midpoint, got %s", got)
	}
	if got := mix("bogus", "#ffffff", 0.5); got != "bogus" {
		t.Fatalf("expected invalid input to pass through, got %s", got)
	}
}

func TestApplyAccentValidAndInvalidValues(t *testing.T) {
	base := Default()

	for _, value := range []string{"", "theme", "  ", "not-a-hex", "#12", "#12345g", "123456"} {
		got := ApplyAccent(base, value)
		if got.Accent != base.Accent || got.HeatHigh != base.HeatHigh {
			t.Fatalf("ApplyAccent(%q) mutated the palette", value)
		}
	}

	got := ApplyAccent(base, "#1234ab")
	if got.Accent.Dark != "#1234ab" {
		t.Fatalf("accent = %q, want #1234ab", got.Accent.Dark)
	}
	if got.HeatHigh.Dark != "#1234ab" {
		t.Fatalf("heat-high = %q, want #1234ab", got.HeatHigh.Dark)
	}
	// The remaining palette must be untouched.
	if got.Background != base.Background || got.HeatLow != base.HeatLow || got.Primary != base.Primary {
		t.Fatal("ApplyAccent changed unrelated palette entries")
	}
	// A spaced hex value is accepted.
	if spaced := ApplyAccent(base, "  #a1b2c3  "); spaced.Accent.Dark != "#a1b2c3" {
		t.Fatalf("spaced accent = %q, want #a1b2c3", spaced.Accent.Dark)
	}
}
