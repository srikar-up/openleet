package mapview

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 3x5 pixel-art font matching the exact Figma mockup cards
var pixelDigits3x5 = [10][5]string{
	// 0
	{"111", "101", "101", "101", "111"},
	// 1
	{"010", "110", "010", "010", "111"},
	// 2
	{"111", "001", "111", "100", "111"},
	// 3
	{"111", "001", "111", "001", "111"},
	// 4
	{"101", "101", "111", "001", "001"},
	// 5
	{"111", "100", "111", "001", "111"},
	// 6
	{"111", "100", "111", "101", "111"},
	// 7
	{"111", "001", "010", "010", "010"},
	// 8
	{"111", "101", "111", "101", "111"},
	// 9
	{"111", "101", "111", "001", "111"},
}

// PixelArtDigit returns the 5-row bitmap for a decimal digit (0-9).
func PixelArtDigit(digit int) []string {
	if digit < 0 || digit > 9 {
		digit = 0
	}
	rows := pixelDigits3x5[digit]
	return append([]string(nil), rows[:]...)
}

// RenderPixelDigit renders a 3x5 digit in the supplied color, centered in width.
func RenderPixelDigit(digit, width int, color lipgloss.AdaptiveColor) string {
	if width < 3 {
		width = 3
	}
	style := lipgloss.NewStyle().Bold(true).Foreground(color)
	rows := PixelArtDigit(digit)
	lines := make([]string, len(rows))
	for i, row := range rows {
		pixelRow := strings.NewReplacer("0", " ", "1", "█").Replace(row)
		pad := (width - lipgloss.Width(pixelRow)) / 2
		if pad < 0 {
			pad = 0
		}
		lines[i] = strings.Repeat(" ", pad) + style.Render(pixelRow)
	}
	return strings.Join(lines, "\n")
}
