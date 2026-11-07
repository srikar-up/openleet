package uikit

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestTruncateUsesGraphemeCells(t *testing.T) {
	// Eight CJK cells: a byte slice would cut this mid-rune.
	value := "日本語のタイトル"

	for _, width := range []int{1, 2, 3, 5, 7} {
		got := Truncate(value, width, "…")
		if ansi.StringWidth(got) > width {
			t.Fatalf("width %d: %q occupies %d cells", width, got, ansi.StringWidth(got))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("width %d: invalid UTF-8 %q", width, got)
		}
		if strings.ContainsRune(got, utf8.RuneError) {
			t.Fatalf("width %d: broken rune in %q", width, got)
		}
	}
}

func TestTruncateKeepsShortValues(t *testing.T) {
	if got := Truncate("two sum", 20, "…"); got != "two sum" {
		t.Fatalf("expected the value to be untouched, got %q", got)
	}
}

func TestFitIsExactlyWidthCells(t *testing.T) {
	cases := []struct {
		value string
		width int
	}{
		{"two sum", 12},
		{"日本語", 8},
		{"🙂🙂🙂", 7},
		{"a", 1},
		{"", 5},
	}
	for _, tc := range cases {
		got := Fit(tc.value, tc.width)
		if width := ansi.StringWidth(got); width != tc.width {
			t.Fatalf("Fit(%q, %d) = %q (%d cells), want %d", tc.value, tc.width, got, width, tc.width)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("Fit(%q, %d) produced invalid UTF-8", tc.value, tc.width)
		}
	}
}

func TestFitPreservesANSIStyles(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8800")).Render("日本語")
	got := Fit(styled, 4)
	if width := ansi.StringWidth(got); width != 4 {
		t.Fatalf("expected 4 cells, got %d (%q)", width, got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatal("expected the ANSI style to survive truncation")
	}
}

func TestPadFrameNeverOverflows(t *testing.T) {
	frame := PadFrame("日本語のタイトル\n🙂🙂\nok", 6, 4)
	rows := strings.Split(frame, "\n")
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	for i, row := range rows {
		if width := ansi.StringWidth(row); width != 6 {
			t.Fatalf("row %d is %d cells, want 6", i, width)
		}
	}
}
