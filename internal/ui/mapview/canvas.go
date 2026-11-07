package mapview

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/model"
	"openleet/internal/ui/uikit"
)

const (
	cardWidth      = 11
	cardHeight     = 7 // top border + five glyph rows + bottom border
	columnGap      = 2
	canvasIndent   = 2
	snakeGroupSize = 6
)

// snakeColumn maps an index inside a six-card group onto the winding columns of
// the Figma roadmap: cards 1,4,6 sit left, cards 2,5 middle, card 3 right.
func snakeColumn(indexInGroup, columns int) int {
	if columns == 1 {
		return 0
	}
	if columns == 2 {
		return indexInGroup % 2
	}
	switch indexInGroup % snakeGroupSize {
	case 0, 3, 5:
		return 0
	case 1, 4:
		return 1
	default:
		return 2
	}
}

// staggerOffset is the number of blank rows a column starts with. The offsets
// reproduce the descending zig-zag of the mockup without hardcoding pixels.
func staggerOffset(column int) int {
	switch column {
	case 1:
		return 2
	case 2:
		return 11
	default:
		return 0
	}
}

// renderStaggeredGrid draws the level map as a winding path of cards.
func (r Renderer) renderStaggeredGrid(data ViewData, width, height int) string {
	if height <= 0 || width <= 0 {
		return ""
	}
	if len(data.Problems) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(r.Theme.Muted).
			Render("No levels in this topic yet")
	}

	grid := computeGridLayout(width, height)
	columns := grid.Columns

	// Render only the six-card page that contains the current selection, then
	// window it vertically so the selected card is always on screen.
	start, count := pageBounds(len(data.Problems), data.Selected)

	buckets := make([][]int, columns)
	for i := start; i < start+count; i++ {
		col := snakeColumn(i-start, columns)
		buckets[col] = append(buckets[col], i)
	}

	rendered := make([]string, 0, columns*2)
	rendered = append(rendered, r.blankLine(canvasIndent))
	for col := 0; col < columns; col++ {
		rendered = append(rendered, lipgloss.NewStyle().
			Width(grid.ColumnW).
			Background(r.Theme.Surface).
			Render(r.renderColumn(data, buckets[col], col, grid.ColumnW)))
		if col < columns-1 {
			rendered = append(rendered, r.blankLine(columnGap))
		}
	}

	gridView := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	gridView = sliceCanvasRows(gridView, gridScrollOffset(len(data.Problems), data.Selected, columns, height), height)
	return r.Theme.CanvasStyle.
		Width(width).
		Height(height).
		MaxWidth(width).
		MaxHeight(height).
		Render(gridView)
}

// sliceCanvasRows windows a rendered grid to the visible row range.
func sliceCanvasRows(content string, top, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if top < 0 {
		top = 0
	}
	if top >= len(lines) {
		return ""
	}
	lines = lines[top:]
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// renderColumn stacks the cards of one column on the mockup's winding path.
func (r Renderer) renderColumn(data ViewData, indices []int, column, width int) string {
	lines := make([]string, 0, len(indices)*cardBlockHeight+4)
	for i := 0; i < staggerOffset(column); i++ {
		lines = append(lines, r.blankLine(width))
	}
	for _, index := range indices {
		problem := data.Problems[index]
		if problem == nil {
			continue
		}
		// Reserve one row above every card for the solved tick. Keeping the row
		// even when unsolved means card positions and click boxes never shift
		// when a problem is solved.
		lines = append(lines, r.renderSolvedTag(data, problem, width))
		card := lipgloss.NewStyle().
			Width(width).
			Background(r.Theme.Surface).
			Align(lipgloss.Center).
			Render(r.renderLevelCard(data, index, problem, cardWidth))
		lines = append(lines, card, r.renderLevelLabel(problem, width, index == data.Selected))
		// Space the next card on this path column.
		for i := 0; i < cardBlockHeight-cardHeight-2; i++ {
			lines = append(lines, r.blankLine(width))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// blankLine returns a canvas-colored filler row.
func (r Renderer) blankLine(width int) string {
	if width <= 0 {
		return ""
	}
	return r.Theme.CanvasStyle.Width(width).Render("")
}

// renderLevelLabel prints the lowercase problem name under its card on a
// single row so grid geometry stays deterministic for hit testing.
func (r Renderer) renderLevelLabel(problem *model.Problem, width int, selected bool) string {
	label := truncateLabel(strings.ToLower(problem.Title), width)
	fg := r.Theme.Text
	if selected {
		fg = r.Theme.Accent
	}
	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Background(r.Theme.Surface).
		Align(lipgloss.Center).
		Foreground(fg).
		Render(label)
}

// truncateLabel keeps the label on one row, ending it with an ellipsis. Widths
// are grapheme cells, so a wide-character title cannot overflow the card.
func truncateLabel(label string, width int) string {
	return uikit.Truncate(label, width, "…")
}

// renderSolvedTag prints the tick badge on the reserved row above a solved
// card. Unsolved cards keep an empty row so the grid geometry stays stable.
func (r Renderer) renderSolvedTag(data ViewData, problem *model.Problem, width int) string {
	if data.UserState == nil || !data.UserState.HasSolved(problem.ID) {
		return r.blankLine(width)
	}
	tag := lipgloss.NewStyle().
		Bold(true).
		Foreground(r.Theme.Medium).
		Background(r.Theme.Surface).
		Render("✓")
	return lipgloss.NewStyle().
		Width(width).
		Background(r.Theme.Surface).
		Align(lipgloss.Center).
		Render(tag)
}

// renderLevelCard draws one bordered level card with its pixel-art number.
func (r Renderer) renderLevelCard(data ViewData, index int, problem *model.Problem, width int) string {
	if width < 9 {
		width = 9
	}
	inner := width - 2
	accentCells := inner * 6 / 10
	if accentCells < 1 {
		accentCells = 1
	}
	if accentCells > inner {
		accentCells = inner
	}

	// Difficulty owns the card colour. Solving adds a tick tag above the card
	// instead of recolouring an easy/hard card green.
	accentColor := difficultyColor(r.Theme, problem.Difficulty)
	borderColor := difficultyColor(r.Theme, problem.Difficulty)

	border := lipgloss.NewStyle().Foreground(borderColor).Background(r.Theme.Card)
	accent := lipgloss.NewStyle().Foreground(accentColor).Background(r.Theme.Card)
	cardBg := r.Theme.CardStyle

	top := border.Render("┌") +
		accent.Render(strings.Repeat("━", accentCells)) +
		border.Render(strings.Repeat("─", inner-accentCells)) +
		border.Render("┐")

	bottom := border.Render("└") +
		border.Render(strings.Repeat("─", inner-accentCells)) +
		accent.Render(strings.Repeat("━", accentCells)) +
		border.Render("┘")

	digitStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		Background(r.Theme.Card)

	rows := make([]string, 0, cardHeight)
	rows = append(rows, top)
	for _, patternRow := range PixelArtDigit((index % 9) + 1) {
		art := strings.NewReplacer("0", " ", "1", "█").Replace(patternRow)
		left := (inner - lipgloss.Width(art)) / 2
		if left < 0 {
			left = 0
		}
		right := inner - left - lipgloss.Width(art)
		if right < 0 {
			right = 0
		}
		rows = append(rows, border.Render("│")+
			cardBg.Render(strings.Repeat(" ", left))+
			digitStyle.Render(art)+
			cardBg.Render(strings.Repeat(" ", right))+
			border.Render("│"))
	}
	rows = append(rows, bottom)

	return lipgloss.NewStyle().
		Width(width).
		Height(cardHeight).
		MaxWidth(width).
		MaxHeight(cardHeight).
		Render(strings.Join(rows, "\n"))
}

// renderLegend draws the difficulty key plus the scroll hint, pinned right.
func (r Renderer) renderLegend(width, scroll, contentHeight, viewportHeight int) string {
	hint := "scroll ↓"
	switch {
	case scroll <= 0:
		hint = "scroll ↓"
	case scroll+viewportHeight >= contentHeight:
		hint = "scroll ↑"
	default:
		hint = "scroll ↑↓"
	}

	items := []string{
		lipgloss.NewStyle().Foreground(r.Theme.Easy).Render("■") + lipgloss.NewStyle().Foreground(r.Theme.Subtext).Render(" easy"),
		lipgloss.NewStyle().Foreground(r.Theme.Medium).Render("■") + lipgloss.NewStyle().Foreground(r.Theme.Subtext).Render(" medium"),
		lipgloss.NewStyle().Foreground(r.Theme.Hard).Render("■") + lipgloss.NewStyle().Foreground(r.Theme.Subtext).Render(" hard"),
		"",
		r.Theme.HeaderStyle.Render(hint),
	}
	lines := make([]string, len(items))
	for i, item := range items {
		gap := width - lipgloss.Width(item) - 2
		if gap < 0 {
			gap = 0
		}
		lines[i] = lipgloss.NewStyle().Background(r.Theme.Surface).Render(strings.Repeat(" ", gap)) + item
	}
	return lipgloss.NewStyle().
		Width(width).
		Background(r.Theme.Surface).
		Render(strings.Join(lines, "\n"))
}

func difficultyColor(theme MapTheme, difficulty model.Difficulty) lipgloss.AdaptiveColor {
	switch difficulty {
	case model.DifficultyEasy:
		return theme.Easy
	case model.DifficultyHard:
		return theme.Hard
	case model.DifficultyMedium:
		return theme.Medium
	default:
		return theme.Easy
	}
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}
