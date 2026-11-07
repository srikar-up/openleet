package workspace

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/storage"
)

// 1. Exact grid at widths 100, 120, 160, 200 and heights 30, 40 with all four open/closed combinations.
func TestRailExactGrid(t *testing.T) {
	prob := makeTestProblem()
	widths := []int{100, 120, 160, 200}
	heights := []int{30, 40}
	combinations := []struct {
		descHidden    bool
		sidebarHidden bool
	}{
		{false, false},
		{true, false},
		{false, true},
		{true, true},
	}

	for _, w := range widths {
		for _, h := range heights {
			for _, combo := range combinations {
				ws := New(prob, "python", nil, nil, nil, w, h)
				ws.descriptionHidden = combo.descHidden
				ws.sidebarHidden = combo.sidebarHidden
				ws.synced = viewportLayout{}

				rendered := ws.View()
				lines := strings.Split(rendered, "\n")
				if len(lines) != h {
					t.Fatalf("w=%d h=%d combo=%+v: got %d lines, want %d", w, h, combo, len(lines), h)
				}
				for rowIdx, line := range lines {
					lineWidth := lipgloss.Width(line)
					if lineWidth != w {
						t.Fatalf("w=%d h=%d combo=%+v row=%d: lineWidth=%d, want %d", w, h, combo, rowIdx, lineWidth, w)
					}
				}

				layout := ws.layout()
				if layout.TotalWidth() != w {
					t.Fatalf("w=%d h=%d combo=%+v: TotalWidth=%d, want %d", w, h, combo, layout.TotalWidth(), w)
				}
			}
		}
	}
}

// 2. Rail is 1 cell: cell between description and editor is │/┬/┴/glyph with no double border.
func TestRailSingleCellSeam(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)
	layout := ws.layout()

	view := ws.View()
	lines := strings.Split(view, "\n")
	leftX := layout.LeftDividerX

	// On the top row, the seam junction is ┬
	topClean := ansi.Strip(lines[0])
	topRunes := []rune(topClean)
	if leftX >= len(topRunes) || (topRunes[leftX] != '┬' && topRunes[leftX] != '│') {
		t.Fatalf("expected top rail junction at column %d, got %c", leftX, topRunes[leftX])
	}

	// On the bottom row, the seam junction is ┴
	bottomClean := ansi.Strip(lines[len(lines)-1])
	bottomRunes := []rune(bottomClean)
	if leftX >= len(bottomRunes) || (bottomRunes[leftX] != '┴' && bottomRunes[leftX] != '│') {
		t.Fatalf("expected bottom rail junction at column %d, got %c", leftX, bottomRunes[leftX])
	}

	// Check a body row: rail cell is │, left adjacent is padding space, right adjacent is padding space.
	bodyClean := ansi.Strip(lines[5])
	bodyRunes := []rune(bodyClean)
	if leftX >= len(bodyRunes) {
		t.Fatalf("body row too short: %d <= %d", len(bodyRunes), leftX)
	}
	seamChar := bodyRunes[leftX]
	if seamChar != '│' {
		t.Fatalf("expected rail character '│' at column %d, got %c", leftX, seamChar)
	}

	// Check that there is no duplicate vertical border adjacent to the rail
	if leftX > 0 && bodyRunes[leftX-1] == '│' {
		t.Fatalf("found duplicate left vertical border at column %d", leftX-1)
	}
	if leftX+1 < len(bodyRunes) && bodyRunes[leftX+1] == '│' {
		t.Fatalf("found duplicate right vertical border at column %d", leftX+1)
	}
}

// 3. Collapsed left/right panes show vertical letters and an expand glyph; reopening restores.
func TestRailCollapsedVerticalLabelsAndRestore(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// Collapse description
	ws.descriptionHidden = true
	ws.synced = viewportLayout{}
	view := ws.View()

	// Check vertical PROBLEM letters
	for _, ch := range "PROBLEM" {
		if !strings.Contains(view, string(ch)) {
			t.Fatalf("expected collapsed left rail to display letter %q", ch)
		}
	}
	// Check expand glyph ›
	if !strings.Contains(view, "›") {
		t.Fatal("expected collapsed left rail to display expand glyph '›'")
	}

	// Reopen by clicking on the rail
	arrowY := dividerArrowRow(ws.height)
	ws, _ = ws.Update(tea.MouseMsg{
		X:      0,
		Y:      arrowY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if ws.descriptionHidden {
		t.Fatal("expected description to be restored after clicking collapsed rail")
	}

	// Collapse sidebar
	ws.sidebarHidden = true
	ws.synced = viewportLayout{}
	view = ws.View()

	for _, ch := range "SESSION" {
		if !strings.Contains(view, string(ch)) {
			t.Fatalf("expected collapsed right rail to display letter %q", ch)
		}
	}
	// Check expand glyph ‹
	if !strings.Contains(view, "‹") {
		t.Fatal("expected collapsed right rail to display expand glyph '‹'")
	}

	// Reopen sidebar
	rightX := ws.layout().RightDividerX
	ws, _ = ws.Update(tea.MouseMsg{
		X:      rightX,
		Y:      arrowY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if ws.sidebarHidden {
		t.Fatal("expected sidebar to be restored after clicking collapsed rail")
	}
}

// 4. Hover: mouse motion over rail highlights accent; moving away clears it. Dragging accents.
func TestRailHoverAndDragAccent(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)
	layout := ws.layout()

	// Motion over left rail
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.LeftDividerX,
		Y:      10,
		Action: tea.MouseActionMotion,
	})
	if ws.hoverEdge != -1 {
		t.Fatalf("expected hoverEdge -1, got %d", ws.hoverEdge)
	}

	// Render rail when hovered. The workspace composes rows from pane blocks,
	// so compare each rail row against the matching frame row rather than
	// looking for one contiguous multi-line block.
	hoveredRail := ws.renderRail(-1, false, ws.height)
	accentRows := strings.Split(ws.View(), "\n")
	for i, railRow := range strings.Split(hoveredRail, "\n") {
		if i >= len(accentRows) || !strings.Contains(accentRows[i], railRow) {
			t.Fatalf("hovered rail row %d is missing from the composed frame", i)
		}
	}

	// Motion away clears hover
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.EditorX + 5,
		Y:      10,
		Action: tea.MouseActionMotion,
	})
	if ws.hoverEdge != 0 {
		t.Fatalf("expected hoverEdge 0 after moving away, got %d", ws.hoverEdge)
	}

	// Dragging also accents
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.LeftDividerX,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if ws.dragEdge != -1 {
		t.Fatalf("expected dragEdge -1, got %d", ws.dragEdge)
	}
	draggedRail := ws.renderRail(-1, false, ws.height)
	draggedRows := strings.Split(ws.View(), "\n")
	for i, railRow := range strings.Split(draggedRail, "\n") {
		if i >= len(draggedRows) || !strings.Contains(draggedRows[i], railRow) {
			t.Fatalf("dragged rail row %d is missing from the composed frame", i)
		}
	}
}

// 5. Drag: below 20 cells snap-collapses; above 20 clamps to <= width/2; sidebar symmetric.
func TestRailDragSnapAndClamp(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)
	layout := ws.layout()

	// Start dragging left rail
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.LeftDividerX,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})

	// Drag below 20 cells -> snaps closed
	ws, _ = ws.Update(tea.MouseMsg{
		X:      15,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	if !ws.descriptionHidden {
		t.Fatal("expected description to snap-collapse when dragged below 20 cells")
	}
	if ws.dragEdge != 0 {
		t.Fatal("expected drag to end after snap-collapse")
	}

	// Restore and drag past width / 2 -> clamps to width / 2
	ws.descriptionHidden = false
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.LeftDividerX,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	ws, _ = ws.Update(tea.MouseMsg{
		X:      ws.width - 10,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	if ws.descWidth > ws.width/2 {
		t.Fatalf("expected descWidth clamped to <= %d, got %d", ws.width/2, ws.descWidth)
	}
	ws, _ = ws.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})

	// Sidebar symmetric: start dragging right rail
	layout = ws.layout()
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.RightDividerX,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	// Drag right so remaining sidebar width < 20
	ws, _ = ws.Update(tea.MouseMsg{
		X:      ws.width - 5,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	if !ws.sidebarHidden {
		t.Fatal("expected sidebar to snap-collapse when dragged below 20 cells")
	}
	if ws.dragEdge != 0 {
		t.Fatal("expected sidebar drag to end after snap-collapse")
	}

	// Restore and drag left towards 0 -> clamps to width / 2
	ws.sidebarHidden = false
	layout = ws.layout()
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.RightDividerX,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	ws, _ = ws.Update(tea.MouseMsg{
		X:      5,
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	if ws.sidebarWidth > ws.width/2 {
		t.Fatalf("expected sidebarWidth clamped to <= %d, got %d", ws.width/2, ws.sidebarWidth)
	}
}

// 6. Double-click resets to default percentage and unhides.
func TestRailDoubleClickReset(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)
	layout := ws.layout()

	// Modify description width and collapse it
	ws.descWidth = 45
	ws.descriptionHidden = true

	// First click
	ws, _ = ws.Update(tea.MouseMsg{
		X:      0,
		Y:      5,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	// Second click within 400ms on same edge
	ws, _ = ws.Update(tea.MouseMsg{
		X:      0,
		Y:      5,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})

	if ws.descriptionHidden {
		t.Fatal("expected double-click to unhide description")
	}
	if ws.descWidth != 0 {
		t.Fatalf("expected descWidth reset to 0 (default percentage), got %d", ws.descWidth)
	}

	// Repeat for sidebar
	ws.sidebarWidth = 40
	ws.sidebarHidden = true
	layout = ws.layout()

	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.RightDividerX,
		Y:      5,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	ws, _ = ws.Update(tea.MouseMsg{
		X:      layout.RightDividerX,
		Y:      5,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})

	if ws.sidebarHidden {
		t.Fatal("expected double-click to unhide sidebar")
	}
	if ws.sidebarWidth != 0 {
		t.Fatalf("expected sidebarWidth reset to 0 (default percentage), got %d", ws.sidebarWidth)
	}
}

// 7. alt+[ and alt+] toggle the panes.
func TestRailBracketShortcuts(t *testing.T) {
	prob := makeTestProblem()
	ws := New(prob, "python", nil, nil, nil, 120, 36)

	// alt+[ toggles description
	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}, Alt: true})
	if !ws.descriptionHidden {
		t.Fatal("expected alt+[ to collapse description")
	}
	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}, Alt: true})
	if ws.descriptionHidden {
		t.Fatal("expected alt+[ to restore description")
	}

	// alt+] toggles sidebar
	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}, Alt: true})
	if !ws.sidebarHidden {
		t.Fatal("expected alt+] to collapse sidebar")
	}
	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}, Alt: true})
	if ws.sidebarHidden {
		t.Fatal("expected alt+] to restore sidebar")
	}
}

// 8. Persistence across workspace instances using real storage.Manager.
func TestRailLayoutPersistence(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}

	prob := makeTestProblem()
	ws := New(prob, "python", store, nil, nil, 120, 36)

	// Set customized widths and toggle a pane
	ws.descWidth = 35
	ws.sidebarWidth = 25
	ws.sidebarHidden = true

	// Dispatch save command
	saveCmd := ws.saveLayoutCmd()
	if saveCmd == nil {
		t.Fatal("expected non-nil saveLayoutCmd")
	}
	saveCmd()

	// Verify on disk directly via storage manager
	saved, err := store.LoadWorkspaceLayout()
	if err != nil {
		t.Fatalf("failed to load saved layout: %v", err)
	}
	if saved.DescriptionWidth != 35 || saved.SidebarWidth != 25 || !saved.SidebarHidden {
		t.Fatalf("saved layout mismatch: %+v", saved)
	}

	// Construct a brand new workspace with the same storage
	fresh := New(prob, "python", store, nil, nil, 120, 36)
	if fresh.descWidth != 35 {
		t.Fatalf("expected descWidth 35, got %d", fresh.descWidth)
	}
	if fresh.sidebarWidth != 25 {
		t.Fatalf("expected sidebarWidth 25, got %d", fresh.sidebarWidth)
	}
	if !fresh.sidebarHidden {
		t.Fatal("expected sidebarHidden true in new workspace")
	}
	if fresh.descriptionHidden {
		t.Fatal("expected descriptionHidden false in new workspace")
	}
}

func TestStorageLayoutDirectRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}

	// Missing file should return zero value and nil error
	empty, err := store.LoadWorkspaceLayout()
	if err != nil {
		t.Fatalf("expected nil error on missing file, got: %v", err)
	}
	if empty != (storage.WorkspaceLayout{}) {
		t.Fatalf("expected zero value layout, got: %+v", empty)
	}

	// Save and round-trip
	target := storage.WorkspaceLayout{
		DescriptionWidth:  42,
		SidebarWidth:      22,
		DescriptionHidden: true,
		SidebarHidden:     false,
	}
	if err := store.SaveWorkspaceLayout(target); err != nil {
		t.Fatalf("failed to save workspace layout: %v", err)
	}

	reloaded, err := store.LoadWorkspaceLayout()
	if err != nil {
		t.Fatalf("failed to reload workspace layout: %v", err)
	}
	if reloaded != target {
		t.Fatalf("expected %+v, got %+v", target, reloaded)
	}
}
