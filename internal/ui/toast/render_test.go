package toast

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/theme"
)

func testPalette() theme.Palette {
	return theme.Resolve("openleet")
}

func sample(kind Kind, title, message string, count int) Toast {
	return Toast{
		ID:        count,
		Kind:      kind,
		Title:     title,
		Message:   message,
		Count:     count,
		CreatedAt: time.Unix(1000, 0),
		UpdatedAt: time.Unix(1000, 0),
	}
}

func TestCardWidthClampsToSpec(t *testing.T) {
	short := []Toast{sample(Info, "Saved", "", 1)}
	if got := CardWidth(short, 160); got != 32 {
		t.Fatalf("short width = %d, want 32", got)
	}
	long := []Toast{sample(Info, "Leaderboard publish failed", strings.Repeat("long detail ", 12), 1)}
	if got := CardWidth(long, 160); got != 48 {
		t.Fatalf("long width = %d, want 48", got)
	}
	if got := CardWidth(long, 100); got > 48 || got < 32 {
		t.Fatalf("100-column width = %d, want 32..48", got)
	}
	if got := CardWidth(long, 80); got != 78 {
		t.Fatalf("narrow width = %d, want 78", got)
	}
}

func TestRenderCardExactWidthAndThreeMessageLines(t *testing.T) {
	pal := testPalette()
	long := sample(Success, "Saved", strings.Repeat("word ", 40), 1)
	rendered := RenderCard(pal, long, 40, true)
	if rendered.Width != 40 {
		t.Fatalf("width = %d, want 40", rendered.Width)
	}
	if rendered.Height != 8 {
		t.Fatalf("height = %d, want 8 (title + 3 message lines + padding/border)", rendered.Height)
	}
	for i, line := range rendered.Lines {
		if got := ansi.StringWidth(line); got != 40 {
			t.Fatalf("line %d width = %d, want 40: %q", i, got, ansi.Strip(line))
		}
	}
	if !strings.Contains(ansi.Strip(rendered.Lines[5]), "…") {
		t.Fatalf("third message line was not truncated: %q", ansi.Strip(rendered.Lines[5]))
	}
}

func TestRenderCardUsesKindGlyphsAndAsciiFallback(t *testing.T) {
	pal := testPalette()
	success := sample(Success, "Done", "", 1)
	unicode := ansi.Strip(Preview(pal, success, 40, true))
	if !strings.Contains(unicode, "✓") {
		t.Fatalf("unicode success glyph missing: %q", unicode)
	}
	ascii := ansi.Strip(Preview(pal, success, 40, false))
	if !strings.Contains(ascii, "+") || strings.Contains(ascii, "✓") {
		t.Fatalf("ascii fallback wrong: %q", ascii)
	}
	errorText := ansi.Strip(Preview(pal, sample(Error, "Failed", "", 1), 40, true))
	if !strings.Contains(errorText, "✕") {
		t.Fatalf("error glyph missing: %q", errorText)
	}
	infoText := ansi.Strip(Preview(pal, sample(Info, "Note", "", 1), 40, true))
	if !strings.Contains(infoText, "i") {
		t.Fatalf("info glyph missing: %q", infoText)
	}
	warnText := ansi.Strip(Preview(pal, sample(Warning, "Careful", "", 1), 40, true))
	if !strings.Contains(warnText, "!") {
		t.Fatalf("warning glyph missing: %q", warnText)
	}
}

func TestRenderCardActionHitCoordinates(t *testing.T) {
	pal := testPalette()
	tst := sample(Info, "Update", "v0.5.0", 1)
	tst.ActionHint = "enter view"
	tst.ActionID = "update"
	rendered := RenderCard(pal, tst, 40, true)
	if !rendered.HasAction {
		t.Fatal("action should be exposed")
	}
	if rendered.ActionX1 != 36 {
		t.Fatalf("action X1 = %d, want 36", rendered.ActionX1)
	}
	if !strings.Contains(ansi.Strip(rendered.Lines[rendered.ActionY]), "enter view") {
		t.Fatalf("action row = %q", ansi.Strip(rendered.Lines[rendered.ActionY]))
	}
}

func TestHostAnchorsBottomRightAndStacks(t *testing.T) {
	const width, height = 160, 40
	canvas := strings.Join(repeatRows("z", width, height), "\n")
	toasts := []Toast{
		sample(Info, "one", "", 1),
		sample(Success, "two", "", 2),
		sample(Warning, "three", "", 3),
	}
	rendered, hits := Host{Width: width, Height: height, Palette: testPalette(), Unicode: true}.Render(canvas, toasts)
	if rendered == canvas {
		t.Fatal("host did not paint")
	}
	lines := strings.Split(rendered, "\n")
	if got := ansi.StringWidth(lines[0]); got != width {
		t.Fatalf("row 0 width = %d", got)
	}
	if lines[0] != strings.Repeat("z", width) {
		t.Fatal("host modified content outside the cards")
	}

	body := make([]Hit, 0, 3)
	for _, hit := range hits {
		if !hit.Action {
			body = append(body, hit)
		}
	}
	if len(body) != 3 {
		t.Fatalf("body hits = %d, want 3", len(body))
	}
	if body[2].X1 != width-3 {
		t.Fatalf("right edge = %d, want %d", body[2].X1, width-3)
	}
	if body[2].Y1 != height-2 {
		t.Fatalf("bottom row = %d, want %d", body[2].Y1, height-2)
	}
	if body[0].Y1 != body[1].Y0-2 {
		t.Fatalf("expected one blank row between cards: %#v %#v", body[0], body[1])
	}
	if body[1].Y1 != body[2].Y0-2 {
		t.Fatalf("expected one blank row between cards: %#v %#v", body[1], body[2])
	}
}

func TestHostNarrowUsesFullWidthMinusTwo(t *testing.T) {
	const width, height = 80, 30
	canvas := strings.Join(repeatRows("z", width, height), "\n")
	toasts := []Toast{
		sample(Info, "old", "", 1),
		sample(Info, "new", "", 2),
	}
	_, hits := Host{Width: width, Height: height, Palette: testPalette(), Unicode: true}.Render(canvas, toasts)
	body := make([]Hit, 0, 1)
	for _, hit := range hits {
		if !hit.Action {
			body = append(body, hit)
		}
	}
	if len(body) != 1 {
		t.Fatalf("narrow body hits = %d, want 1", len(body))
	}
	if body[0].X0 != 1 || body[0].X1 != width-2 {
		t.Fatalf("narrow card = %#v, want x=1..%d", body[0], width-2)
	}
	if body[0].Y1 != height-2 {
		t.Fatalf("narrow bottom = %d, want %d", body[0].Y1, height-2)
	}
}

func TestHostActionHitInsideBody(t *testing.T) {
	const width, height = 120, 40
	canvas := strings.Join(repeatRows("z", width, height), "\n")
	tst := sample(Info, "Update", "v0.5.0", 1)
	tst.ActionHint = "enter update"
	tst.ActionID = "update"
	_, hits := Host{Width: width, Height: height, Palette: testPalette(), Unicode: true}.Render(canvas, []Toast{tst})
	var body, action *Hit
	for i := range hits {
		if hits[i].Action {
			action = &hits[i]
		} else {
			body = &hits[i]
		}
	}
	if body == nil || action == nil {
		t.Fatalf("missing hits: %#v", hits)
	}
	if action.X0 < body.X0 || action.X1 > body.X1 || action.Y0 < body.Y0 || action.Y0 > body.Y1 {
		t.Fatalf("action hit %#v outside body %#v", action, body)
	}
}

func repeatRows(cell string, width, height int) []string {
	row := strings.Repeat(cell, width)
	out := make([]string, height)
	for i := range out {
		out[i] = row
	}
	return out
}
