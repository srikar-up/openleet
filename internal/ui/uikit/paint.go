// Package uikit holds rendering primitives shared by the full-screen views.
package uikit

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// PadRight pads value with spaces until it occupies exactly width cells. Width
// is measured in cells, not bytes, so styled and multi-byte text pads correctly.
func PadRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	gap := width - ansi.StringWidth(value)
	if gap <= 0 {
		return value
	}
	return value + strings.Repeat(" ", gap)
}

// FitLine renders value on a single row of exactly width cells.
func FitLine(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(value)
}

// JoinLeftRight places left and right on one row of exactly width cells.
func JoinLeftRight(width int, left, right string) string {
	if width <= 0 {
		return ""
	}
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth > width {
		right = lipgloss.NewStyle().MaxWidth(width).Render(right)
		rightWidth = lipgloss.Width(right)
		if leftWidth+rightWidth > width {
			left = lipgloss.NewStyle().MaxWidth(width - rightWidth).Render(left)
			leftWidth = lipgloss.Width(left)
		}
	}
	gap := width - leftWidth - rightWidth
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

// BackgroundSequence extracts the raw ANSI background prefix Lip Gloss uses for
// the current terminal profile (truecolor or 256-color).
func BackgroundSequence(bg lipgloss.AdaptiveColor) string {
	rendered := lipgloss.NewStyle().Background(bg).Render("x")
	if i := strings.IndexByte(rendered, 'x'); i > 0 {
		return rendered[:i]
	}
	return ""
}

// PaintBackground makes every cell opaque. Lip Gloss resets the SGR state at
// the end of each nested style, so joined lines otherwise leave their padding
// transparent. The painter remembers the last background seen on a line and
// restores it after every reset, which keeps nested panels intact.
//
// The last reset on a line closes the outermost nested style. Cells after it
// are frame padding, so they use the frame background rather than the nested
// one. Without that boundary a card's surface colour bleeds across the rest of
// the row and the card appears to extend to the terminal edge.
func PaintBackground(content string, bg lipgloss.AdaptiveColor) string {
	frameBg := BackgroundSequence(bg)
	if frameBg == "" {
		return content
	}
	const reset = "\x1b[0m"
	lines := strings.Split(content, "\n")
	for li, line := range lines {
		var b strings.Builder
		b.WriteString(frameBg)
		lastBg := frameBg
		lastReset := strings.LastIndex(line, reset)
		if alt := strings.LastIndex(line, "\x1b[m"); alt > lastReset {
			lastReset = alt
		}
		for i := 0; i < len(line); {
			if line[i] != 0x1b {
				b.WriteByte(line[i])
				i++
				continue
			}
			j := i + 1
			if j < len(line) && line[j] == '[' {
				j++
				for j < len(line) && !(line[j] >= 0x40 && line[j] <= 0x7e) {
					j++
				}
				if j < len(line) {
					j++
				}
			} else if j < len(line) {
				j++
			}
			seq := line[i:j]
			if seqBg := extractBackground(seq); seqBg != "" {
				lastBg = seqBg
			}
			if seq == reset || seq == "\x1b[m" {
				b.WriteString(reset)
				if i >= lastReset {
					b.WriteString(frameBg)
				} else {
					b.WriteString(lastBg)
				}
			} else {
				b.WriteString(seq)
			}
			i = j
		}
		b.WriteString(reset)
		lines[li] = b.String()
	}
	return strings.Join(lines, "\n")
}

// extractBackground pulls the background portion out of an SGR sequence.
func extractBackground(seq string) string {
	if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
		return ""
	}
	parts := strings.Split(seq[2:len(seq)-1], ";")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "48" || i+1 >= len(parts) {
			continue
		}
		switch parts[i+1] {
		case "5":
			if i+2 < len(parts) {
				return "\x1b[48;5;" + parts[i+2] + "m"
			}
		case "2":
			if i+4 < len(parts) {
				return "\x1b[48;2;" + parts[i+2] + ";" + parts[i+3] + ";" + parts[i+4] + "m"
			}
		}
	}
	return ""
}

// PadFrame sizes a rendered block to exactly width x height cells.
func PadFrame(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = FitExact(lines[i], width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}
