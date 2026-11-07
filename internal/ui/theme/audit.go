package theme

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// AuditResult reports the semantic-token health of one theme.
type AuditResult struct {
	Name     string
	Label    string
	Failures []string
}

// Pass reports whether every audit check succeeded.
func (r AuditResult) Pass() bool {
	return len(r.Failures) == 0
}

// Audit checks the semantic tokens and contrast pairs used by toasts, the
// account chip, and the Settings button.
func Audit(p Palette) []string {
	var failures []string
	add := func(format string, args ...any) {
		failures = append(failures, fmt.Sprintf(format, args...))
	}
	require := func(name string, color lipgloss.AdaptiveColor) string {
		value := adaptiveValue(color)
		if strings.TrimSpace(value) == "" {
			add("missing token %s", name)
		}
		return value
	}

	background := require("background", p.Background)
	surfaceRaised := require("surface_raised", p.SurfaceRaised)
	text := require("text", p.Text)
	textDim := require("text_dim", p.TextDim)
	border := require("border", p.Border)
	borderFocus := require("border_focus", p.BorderFocus)
	accent := require("accent", p.Accent)
	accentFG := require("accent_fg", p.AccentFG)
	pass := require("pass", p.Pass)
	fail := require("fail", p.Fail)
	warn := require("warn", p.Warn)
	info := require("info", p.Info)

	check := func(name, fg, bg string, min float64) {
		if fg == "" || bg == "" {
			return
		}
		if ratio := ContrastRatio(fg, bg); ratio < min {
			add("%s contrast %.2f:1 < %.1f:1", name, ratio, min)
		}
	}

	check("text/background", text, background, 4.5)
	check("text_dim/surface_raised", textDim, surfaceRaised, 4.5)
	check("border/surface_raised", border, surfaceRaised, 3.0)
	check("border/background", border, background, 3.0)
	check("border_focus/surface_raised", borderFocus, surfaceRaised, 3.0)
	check("accent_fg/accent", accentFG, accent, 4.5)
	check("surface_raised/background", surfaceRaised, background, 1.15)

	kinds := []struct {
		name  string
		value string
	}{
		{"pass", pass},
		{"fail", fail},
		{"warn", warn},
		{"info", info},
	}
	for _, kind := range kinds {
		check(kind.name+"/surface_raised", kind.value, surfaceRaised, 3.0)
		if kind.value == text {
			add("%s equals text", kind.name)
		} else if ColorDistance(kind.value, text) < 50 {
			add("%s is too close to text", kind.name)
		}
	}
	for i := 0; i < len(kinds); i++ {
		for j := i + 1; j < len(kinds); j++ {
			if kinds[i].value == "" || kinds[j].value == "" {
				continue
			}
			if kinds[i].value == kinds[j].value {
				add("%s equals %s", kinds[i].name, kinds[j].name)
			} else if ColorDistance(kinds[i].value, kinds[j].value) < 50 {
				add("%s is too close to %s", kinds[i].name, kinds[j].name)
			}
		}
	}

	return failures
}

// AuditAll audits every registered theme in authoring order.
func AuditAll() []AuditResult {
	results := make([]AuditResult, 0, len(order))
	for _, name := range order {
		palette := registry[name]
		results = append(results, AuditResult{
			Name:     palette.Name,
			Label:    palette.Label,
			Failures: Audit(palette),
		})
	}
	return results
}

func debugWarn(format string, args ...any) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("OPENLEET_DEBUG")))
	if raw != "1" && raw != "true" && raw != "yes" {
		return
	}
	log.Printf("theme audit: "+format, args...)
}
