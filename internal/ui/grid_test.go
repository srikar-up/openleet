package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func TestFitToGridHandlesWideRunes(t *testing.T) {
	frame := fitToGrid("日本語のタイトル\n🙂🙂🙂🙂\nshort", 7, 5)
	rows := strings.Split(frame, "\n")
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	for i, row := range rows {
		if width := ansi.StringWidth(row); width != 7 {
			t.Fatalf("row %d is %d cells, want 7", i, width)
		}
		if !utf8.ValidString(row) {
			t.Fatalf("row %d is not valid UTF-8", i)
		}
		if strings.ContainsRune(row, utf8.RuneError) {
			t.Fatalf("row %d contains a broken rune", i)
		}
	}
}

func TestFitToGridDropsExtraRowsAndPadsShortOnes(t *testing.T) {
	frame := fitToGrid("a\nb\nc", 4, 2)
	rows := strings.Split(frame, "\n")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0] != "a   " || rows[1] != "b   " {
		t.Fatalf("unexpected padding: %q", rows)
	}
}

func TestFitToGridIsIdempotent(t *testing.T) {
	once := fitToGrid("openleet\nroadmap", 12, 3)
	twice := fitToGrid(once, 12, 3)
	if once != twice {
		t.Fatalf("second pass changed the frame:\n%q\n%q", once, twice)
	}
}
