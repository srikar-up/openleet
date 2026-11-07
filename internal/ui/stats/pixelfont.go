package stats

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

// bigGlyphs is a 5x7 pixel font for the hero counter.
var bigGlyphs = map[rune][]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11111", "00001", "00001", "01111", "00001", "00001", "11111"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "11111", "00001", "00001", "10001", "01110"},
	'6': {"00110", "01000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00010", "01100"},
	'/': {"00011", "00110", "00110", "01100", "11000", "11000", "10000"},
	'%': {"11001", "11010", "00100", "00100", "01000", "01011", "10011"},
}

// RenderWordmark draws the product name in its 4x5 pixel font.
func RenderWordmark(word string, color lipgloss.AdaptiveColor) string {
	return uikit.RenderWordmark(word, color)
}

// RenderWordmarkTwoTone draws a two-part wordmark, e.g. a neutral product
// prefix followed by an accented suffix. The tone boundary gets a wider gap so
// the split reads even without colour, and survives colour-blind palettes.
func RenderWordmarkTwoTone(first, second string, firstColor, secondColor lipgloss.AdaptiveColor) string {
	return uikit.RenderWordmarkTwoTone(first, second, firstColor, secondColor)
}

// RenderBigCounter draws the hero number in a 5x7 pixel font. Each glyph pixel
// spans horizontalScale cells by verticalScale rows; scaling both axes keeps
// the pixels square, because terminal cells are twice as tall as they are wide.
func RenderBigCounter(text string, color lipgloss.AdaptiveColor, horizontalScale, verticalScale int) string {
	if horizontalScale < 1 {
		horizontalScale = 1
	}
	if verticalScale < 1 {
		verticalScale = 1
	}
	rows := make([]string, 7*verticalScale)
	style := lipgloss.NewStyle().Bold(true).Foreground(color)

	for row := 0; row < 7; row++ {
		var b strings.Builder
		first := true
		for _, r := range text {
			glyph, ok := bigGlyphs[r]
			if !ok {
				continue
			}
			if !first {
				b.WriteString(strings.Repeat(" ", horizontalScale))
			}
			b.WriteString(expand(glyph[row], horizontalScale))
			first = false
		}
		line := style.Render(b.String())
		for v := 0; v < verticalScale; v++ {
			rows[row*verticalScale+v] = line
		}
	}
	return strings.Join(rows, "\n")
}

// CounterPixelWidth reports how many glyph pixels wide text renders, counting
// the one-pixel gap between characters. It returns 0 for unknown input.
func CounterPixelWidth(text string) int {
	width := 0
	for _, r := range text {
		glyph, ok := bigGlyphs[r]
		if !ok || len(glyph) == 0 {
			continue
		}
		if width > 0 {
			width++
		}
		width += len([]rune(glyph[0]))
	}
	return width
}

// expand converts a bitmap row to block glyphs, scaling the horizontal axis.
func expand(row string, scale int) string {
	var b strings.Builder
	for _, r := range row {
		switch r {
		case '1':
			b.WriteString(strings.Repeat("█", scale))
		default:
			b.WriteString(strings.Repeat(" ", scale))
		}
	}
	return b.String()
}
