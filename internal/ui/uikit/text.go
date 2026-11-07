package uikit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Truncate shortens value to at most width terminal cells. Width is measured in
// grapheme clusters (ansi.StringWidth), never bytes, so CJK titles, emoji
// badges, and accented paths are cut on a character boundary and cannot leave a
// broken UTF-8 rune on screen.
func Truncate(value string, width int, tail string) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	if ansi.StringWidth(tail) >= width {
		return ansi.Truncate(tail, width, "")
	}
	return ansi.Truncate(value, width, tail)
}

// Fit renders value on exactly width cells: truncate with an ellipsis when it is
// too wide, pad with spaces when it is too narrow. Truncation alone is not
// enough: cutting in front of a double-width grapheme can leave a cell unused.
func Fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) > width {
		value = ansi.Truncate(value, width, "…")
	}
	if gap := width - ansi.StringWidth(value); gap > 0 {
		return value + strings.Repeat(" ", gap)
	}
	return value
}

// FitExact is Fit without the ellipsis. It is used where a hard clip reads
// better than a marker, such as a clipped background fill.
func FitExact(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = ansi.Truncate(value, width, "")
	if gap := width - ansi.StringWidth(value); gap > 0 {
		return value + strings.Repeat(" ", gap)
	}
	return value
}
