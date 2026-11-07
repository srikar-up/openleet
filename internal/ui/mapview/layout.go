package mapview

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

// padRight pads value with spaces until it occupies exactly width cells.
func padRight(value string, width int) string { return uikit.PadRight(value, width) }

// joinLeftRight places left and right on one row of exactly width cells.
func joinLeftRight(width int, left, right string) string {
	return uikit.JoinLeftRight(width, left, right)
}

// fitLine renders value on a single row of exactly width cells.
func fitLine(value string, width int) string { return uikit.FitLine(value, width) }

// fitPanel clips content to width x height without adding padding.
func fitPanel(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = fitLine(line, width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

// fitLines joins the first height lines, each clipped to width.
func fitLines(lines []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	visible := make([]string, 0, height)
	for _, line := range lines {
		if len(visible) >= height {
			break
		}
		visible = append(visible, fitLine(line, width))
	}
	for len(visible) < height {
		visible = append(visible, strings.Repeat(" ", width))
	}
	return strings.Join(visible, "\n")
}

// wrapText word-wraps text to maxWidth cells, preserving blank lines.
func wrapText(text string, maxWidth int) []string {
	if maxWidth < 1 {
		return nil
	}
	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		line := ""
		for _, word := range words {
			if line != "" && lipgloss.Width(line)+1+lipgloss.Width(word) > maxWidth {
				result = append(result, line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// stripMarkdown removes inline markdown markers that would leak into the UI.
func stripMarkdown(value string) string {
	replacer := strings.NewReplacer(
		"**", "",
		"__", "",
		"`", "",
		"*", "",
		"### ", "",
		"## ", "",
		"# ", "",
	)
	return replacer.Replace(value)
}

// paintBackground keeps every cell opaque across nested style resets.
func paintBackground(content string, bg lipgloss.AdaptiveColor) string {
	return uikit.PaintBackground(content, bg)
}

// padFrame sizes a rendered block to exactly width x height cells.
func padFrame(content string, width, height int) string {
	return uikit.PadFrame(content, width, height)
}
