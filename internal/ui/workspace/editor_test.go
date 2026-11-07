package workspace

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorHighlightSpans(t *testing.T) {
	prob := makeTestProblem()
	theme := DefaultWorkspaceTheme()
	ed := NewEditorModel(prob, "python", theme, 80, 20)

	lines := ed.highlightLines()
	if len(lines) == 0 {
		t.Fatal("expected highlighted lines, got empty")
	}

	// First line must have at least one span with keyword token type containing "class"
	hasFirstLineClass := false
	for _, span := range lines[0] {
		if (span.Type.InCategory(chroma.Keyword) || span.Type == chroma.Keyword) && strings.Contains(span.Text, "class") {
			hasFirstLineClass = true
			break
		}
	}
	if !hasFirstLineClass {
		t.Fatalf("expected first line to have keyword span with 'class', got %+v", lines[0])
	}

	// def line must have a keyword span containing "def"
	hasDef := false
	for _, l := range lines {
		for _, span := range l {
			if (span.Type.InCategory(chroma.Keyword) || span.Type == chroma.Keyword) && strings.Contains(span.Text, "def") {
				hasDef = true
				break
			}
		}
		if hasDef {
			break
		}
	}
	if !hasDef {
		t.Fatal("expected def line to have keyword span with 'def'")
	}
}

func TestEditorEditingAndUndo(t *testing.T) {
	theme := DefaultWorkspaceTheme()
	ed := NewEditorModel(nil, "python", theme, 80, 20)
	ed.SetContent("abc")
	ed.SetCursor(0, 3)

	var cmd tea.Cmd
	ed, cmd = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	_ = cmd
	if ed.Content() != "abcd" {
		t.Fatalf("expected 'abcd', got %q", ed.Content())
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if ed.Content() != "abc" {
		t.Fatalf("expected 'abc', got %q", ed.Content())
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if ed.Content() != "abcd" {
		t.Fatalf("expected 'abcd' after undo, got %q", ed.Content())
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	if ed.Content() != "abc" {
		t.Fatalf("expected 'abc' after redo, got %q", ed.Content())
	}
}

func TestEditorCursorClamp(t *testing.T) {
	theme := DefaultWorkspaceTheme()
	ed := NewEditorModel(nil, "python", theme, 80, 20)
	ed.SetContent("line1\nline2")
	ed.SetCursor(99, 99)

	if ed.Line() != 1 {
		t.Fatalf("expected Line() to be 1, got %d", ed.Line())
	}
	if ed.Column() != len([]rune("line2")) {
		t.Fatalf("expected Column() to be %d, got %d", len([]rune("line2")), ed.Column())
	}

	view := ed.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}

	innerW := 80
	viewLines := strings.Split(view, "\n")
	for i, vl := range viewLines {
		w := ansi.StringWidth(vl)
		if w > innerW {
			t.Fatalf("view line %d exceeds inner width %d: width=%d (%q)", i, innerW, w, vl)
		}
	}
}

func TestEditorTabWidthHonorsOption(t *testing.T) {
	ed := NewEditorModel(nil, "python", DefaultWorkspaceTheme(), 80, 20)
	opts := DefaultEditorOptions()
	opts.TabWidth = 2
	ed.SetOptions(opts)
	ed.SetContent("")
	ed.SetCursor(0, 0)

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := ed.Content(); got != "  " {
		t.Fatalf("tab width 2: content = %q, want two spaces", got)
	}
	if ed.Column() != 2 {
		t.Fatalf("tab width 2: column = %d, want 2", ed.Column())
	}

	ed.SetContent("a\tb")
	if got := ed.Content(); got != "a b" {
		t.Fatalf("tab expansion with width 2 = %q, want %q", got, "a b")
	}
}

func TestEditorLineNumbersCanBeDisabled(t *testing.T) {
	ed := NewEditorModel(nil, "python", DefaultWorkspaceTheme(), 40, 10)
	ed.SetContent("abc")

	withNumbers := strings.Split(ansi.Strip(ed.View()), "\n")[0]
	if strings.HasPrefix(withNumbers, "abc") {
		t.Fatalf("expected a line-number gutter when enabled, got %q", withNumbers)
	}

	opts := DefaultEditorOptions()
	opts.ShowLineNumbers = false
	ed.SetOptions(opts)
	if ed.GutterWidth() != 0 {
		t.Fatalf("GutterWidth() = %d, want 0", ed.GutterWidth())
	}
	without := strings.Split(ansi.Strip(ed.View()), "\n")[0]
	if !strings.HasPrefix(without, "abc") {
		t.Fatalf("line numbers off: expected code at cell 0, got %q", without)
	}
}

func TestEditorAutoIndentCanBeDisabled(t *testing.T) {
	ed := NewEditorModel(nil, "python", DefaultWorkspaceTheme(), 80, 20)
	opts := DefaultEditorOptions()
	opts.AutoIndent = false
	ed.SetOptions(opts)
	ed.SetContent("    x")
	ed.SetCursor(0, len([]rune("    x")))

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := ed.Content(); got != "    x\n" {
		t.Fatalf("auto-indent off: content = %q, want %q", got, "    x\n")
	}
	if ed.Column() != 0 {
		t.Fatalf("auto-indent off: column = %d, want 0", ed.Column())
	}
}

func TestEditorBracketPairingOption(t *testing.T) {
	ed := NewEditorModel(nil, "python", DefaultWorkspaceTheme(), 80, 20)
	ed.SetContent("")
	ed.SetCursor(0, 0)

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("(")})
	if got := ed.Content(); got != "()" {
		t.Fatalf("bracket pairing on: content = %q, want %q", got, "()")
	}
	if ed.Column() != 1 {
		t.Fatalf("bracket pairing on: column = %d, want 1", ed.Column())
	}

	opts := DefaultEditorOptions()
	opts.BracketPairing = false
	ed.SetOptions(opts)
	ed.SetContent("")
	ed.SetCursor(0, 0)

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("(")})
	if got := ed.Content(); got != "(" {
		t.Fatalf("bracket pairing off: content = %q, want %q", got, "(")
	}
}
