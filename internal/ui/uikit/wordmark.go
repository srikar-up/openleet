package uikit

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// wordGlyphs is a 6x5 bold pixel font for the "OPENLEET" wordmark.
var wordGlyphs = map[rune][]string{
	'O': {"011110", "110011", "110011", "110011", "011110"},
	'P': {"111110", "110011", "111110", "110000", "110000"},
	'E': {"111111", "110000", "111110", "110000", "111111"},
	'N': {"110011", "111011", "110111", "110011", "110011"},
	'L': {"110000", "110000", "110000", "110000", "111111"},
	'T': {"111111", "001100", "001100", "001100", "001100"},
}

// wordmarkRows lays out the product name as five plain pixel rows.
func wordmarkRows(word string) []string {
	rows := make([]string, 5)
	first := true
	for _, r := range strings.ToUpper(word) {
		glyph, ok := wordGlyphs[r]
		if !ok {
			continue
		}
		for i := 0; i < 5; i++ {
			if !first {
				rows[i] += " "
			}
			rows[i] += expand(glyph[i], 1)
		}
		first = false
	}
	return rows
}

// counterGlyphs is a 5x5 pixel font for the compact solved counter. It uses
// 2-unit vertical stems to compensate for terminal aspect ratio while staying
// smaller than the wordmark.
var counterGlyphs = map[rune][]string{
	'0': {"01110", "11011", "11011", "11011", "01110"},
	'1': {"00110", "01110", "00110", "00110", "01111"},
	'2': {"11110", "00011", "01110", "11000", "11111"},
	'3': {"11110", "00011", "01110", "00011", "11110"},
	'4': {"11011", "11011", "11111", "00011", "00011"},
	'5': {"11111", "11000", "11110", "00011", "11110"},
	'6': {"01110", "11000", "11110", "11011", "01110"},
	'7': {"11111", "00011", "00110", "01100", "01100"},
	'8': {"01110", "11011", "01110", "11011", "01110"},
	'9': {"01110", "11011", "01111", "00011", "01110"},
	'/': {"00011", "00110", "01100", "11000", "11000"},
}

// RenderPixelText draws a short string in the compact 3x5 counter font.
func RenderPixelText(text string, color lipgloss.AdaptiveColor) string {
	rows := make([]string, 5)
	first := true
	for _, r := range text {
		glyph, ok := counterGlyphs[r]
		if !ok {
			continue
		}
		for i := 0; i < 5; i++ {
			if !first {
				rows[i] += " "
			}
			rows[i] += expand(glyph[i], 1)
		}
		first = false
	}
	style := lipgloss.NewStyle().Bold(true).Foreground(color)
	for i := range rows {
		rows[i] = style.Render(rows[i])
	}
	return strings.Join(rows, "\n")
}

// RenderWordmark draws the product name in its 4x5 pixel font.
func RenderWordmark(word string, color lipgloss.AdaptiveColor) string {
	rows := wordmarkRows(word)
	style := lipgloss.NewStyle().Bold(true).Foreground(color)
	for i := range rows {
		rows[i] = style.Render(rows[i])
	}
	return strings.Join(rows, "\n")
}

// RenderWordmarkTwoTone draws a two-part wordmark, e.g. a neutral product
// prefix followed by an accented suffix. The tone boundary gets a wider gap so
// the split reads even without colour, and survives colour-blind palettes.
func RenderWordmarkTwoTone(first, second string, firstColor, secondColor lipgloss.AdaptiveColor) string {
	left := wordmarkRows(first)
	right := wordmarkRows(second)
	if len(left) == 0 || left[0] == "" {
		return RenderWordmark(second, secondColor)
	}
	if len(right) == 0 || right[0] == "" {
		return RenderWordmark(first, firstColor)
	}

	leftStyle := lipgloss.NewStyle().Bold(true).Foreground(firstColor)
	rightStyle := lipgloss.NewStyle().Bold(true).Foreground(secondColor)
	rows := make([]string, len(left))
	for i := range left {
		rows[i] = leftStyle.Render(left[i]) + "  " + rightStyle.Render(right[i])
	}
	return strings.Join(rows, "\n")
}

// RenderWordmarkBlock renders the first word in firstColor and the second word
// as dark glyphs on a solid accent-coloured block. The block is drawn with
// foreground-coloured block cells rather than a background style, so it renders
// correctly even in terminals that drop SGR background colours. One row of
// padding is added above and below the letters, and the first word is centred
// vertically against the taller block.
func RenderWordmarkBlock(first, second string, firstColor, blockBackground, blockForeground lipgloss.AdaptiveColor) string {
	left := wordmarkRows(first)
	right := wordmarkRows(second)
	if len(left) == 0 || left[0] == "" {
		return RenderWordmark(second, blockBackground)
	}
	if len(right) == 0 || right[0] == "" {
		return RenderWordmark(first, firstColor)
	}

	leftStyle := lipgloss.NewStyle().Bold(true).Foreground(firstColor)
	blockStyle := lipgloss.NewStyle().Bold(true).Foreground(blockBackground)
	letterStyle := lipgloss.NewStyle().Bold(true).Foreground(blockForeground)

	cell := blockStyle.Render("█")
	topCell := blockStyle.Render("▄")    // half-height top border
	bottomCell := blockStyle.Render("▀") // half-height bottom border

	width := len([]rune(right[0])) + 2 // 1 cell of padding on each side

	blockRows := make([]string, 0, len(right)+2)
	blockRows = append(blockRows, strings.Repeat(topCell, width))
	for _, row := range right {
		var b strings.Builder
		b.WriteString(cell) // 1 cell left padding
		for _, r := range row {
			if r == '█' {
				b.WriteString(letterStyle.Render("█"))
			} else {
				b.WriteString(cell)
			}
		}
		b.WriteString(cell) // 1 cell right padding
		blockRows = append(blockRows, b.String())
	}
	blockRows = append(blockRows, strings.Repeat(bottomCell, width))

	leftRows := make([]string, len(left))
	for i := range left {
		leftRows[i] = leftStyle.Render(left[i])
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		strings.Join(leftRows, "\n"),
		" ", // 1-cell gap between OPEN and the orange block
		strings.Join(blockRows, "\n"),
	)
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
