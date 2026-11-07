package uikit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const paintReset = "\x1b[0m"

// paintedTrailingRun returns the styled cells after the second-to-last reset,
// which is the frame padding that follows the outermost nested block.
func paintedTrailingRun(t *testing.T, painted string) string {
	t.Helper()
	end := strings.LastIndex(painted, paintReset)
	if end < 0 {
		t.Fatalf("painted line has no final reset: %q", painted)
	}
	prefix := painted[:end]
	start := strings.LastIndex(prefix, paintReset)
	if start < 0 {
		t.Fatalf("painted line has no nested reset: %q", painted)
	}
	return prefix[start+len(paintReset):]
}

func TestPaintBackgroundEndsNestedSurfaceAtLineEnd(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	frame := lipgloss.AdaptiveColor{Light: "#101010", Dark: "#101010"}
	surface := lipgloss.AdaptiveColor{Light: "#222222", Dark: "#222222"}
	frameBg := BackgroundSequence(frame)
	surfaceBg := BackgroundSequence(surface)

	card := lipgloss.NewStyle().Background(surface).Width(12).Render("card")
	painted := PaintBackground(card+strings.Repeat(" ", 8), frame)

	trailing := paintedTrailingRun(t, painted)
	if !strings.HasPrefix(trailing, frameBg) {
		t.Fatalf("frame padding used the card background, want %q in %q", frameBg, trailing)
	}
	if strings.HasPrefix(trailing, surfaceBg) {
		t.Fatalf("card background leaked past its final reset: %q", painted)
	}
}

func TestPaintBackgroundKeepsNestedSurfaceAcrossInnerResets(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	frame := lipgloss.AdaptiveColor{Light: "#101010", Dark: "#101010"}
	surface := lipgloss.AdaptiveColor{Light: "#222222", Dark: "#222222"}
	frameBg := BackgroundSequence(frame)
	surfaceBg := BackgroundSequence(surface)

	inner := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8800")).Render("text")
	card := lipgloss.NewStyle().Background(surface).Width(16).Render(inner + "  ")
	painted := PaintBackground(card+strings.Repeat(" ", 4), frame)

	if !strings.Contains(painted, paintReset+surfaceBg) {
		t.Fatalf("nested surface was not restored after the inner reset: %q", painted)
	}
	trailing := paintedTrailingRun(t, painted)
	if !strings.HasPrefix(trailing, frameBg) {
		t.Fatalf("frame padding used the card background, want %q in %q", frameBg, trailing)
	}
}
