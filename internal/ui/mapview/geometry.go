package mapview

// bodyLayout holds the horizontal split of the roadmap body in terminal cells.
// The renderer and the mouse hit tester share this single source of truth.
type bodyLayout struct {
	RailWidth      int
	ScrollWidth    int
	CanvasX        int
	CanvasWidth    int
	ShowInspector  bool
	DividerWidth   int
	InspectorX     int
	InspectorWidth int
}

func computeBodyLayout(width int) bodyLayout {
	// The mockup rail is ~12% of the slide width.
	layout := bodyLayout{RailWidth: width * 12 / 100, ScrollWidth: 1, DividerWidth: 1}
	if layout.RailWidth < 14 {
		layout.RailWidth = 14
	}
	if layout.RailWidth > 30 {
		layout.RailWidth = 30
	}

	layout.ShowInspector = width >= 96
	if layout.ShowInspector {
		layout.InspectorWidth = width * 34 / 100
		if layout.InspectorWidth < 30 {
			layout.InspectorWidth = 30
		}
		if maxInspector := width / 2; layout.InspectorWidth > maxInspector {
			layout.InspectorWidth = maxInspector
		}
	} else {
		layout.DividerWidth = 0
	}

	layout.CanvasX = layout.RailWidth + layout.ScrollWidth
	layout.CanvasWidth = width - layout.CanvasX - layout.DividerWidth - layout.InspectorWidth
	if layout.CanvasWidth < 1 {
		layout.CanvasWidth = 1
	}
	layout.InspectorX = layout.CanvasX + layout.CanvasWidth + layout.DividerWidth
	return layout
}

// canvasLayout holds the vertical slices of the roadmap canvas.
type canvasLayout struct {
	BadgeHeight  int
	GridHeight   int
	GridTop      int // rows between the view top and the grid origin
	LegendHeight int
}

// computeCanvasLayout mirrors renderCanvas: badge box, one spacer row, the
// winding grid and an optional right-aligned legend pinned to the bottom.
func computeCanvasLayout(viewHeight int) canvasLayout {
	layout := canvasLayout{BadgeHeight: 3, LegendHeight: 5}
	layout.GridTop = layout.BadgeHeight + 1
	layout.GridHeight = viewHeight - layout.GridTop
	if layout.GridHeight < layout.LegendHeight+6 {
		layout.LegendHeight = 0
	} else {
		layout.GridHeight -= layout.LegendHeight
	}
	if layout.GridHeight < 0 {
		layout.GridHeight = 0
	}
	return layout
}

// gridLayout describes how level cards are placed on the canvas.
type gridLayout struct {
	Columns    int
	ColumnX    []int // left cell of each column, relative to the view
	ColumnW    int
	HeaderRows int
}

// computeGridLayout returns the winding grid metrics for a canvas of the given
// inner width and height. Positions are relative to the canvas origin.
func computeGridLayout(width, height int) gridLayout {
	layout := gridLayout{Columns: 3, HeaderRows: 1}
	if width < 46 {
		layout.Columns = 2
	}
	if width < 30 {
		layout.Columns = 1
	}

	inner := width - canvasIndent
	if inner < cardWidth*layout.Columns {
		inner = width
	}
	layout.ColumnW = (inner - columnGap*(layout.Columns-1)) / layout.Columns
	if layout.ColumnW < cardWidth {
		layout.ColumnW = cardWidth
	}

	layout.ColumnX = make([]int, layout.Columns)
	for col := 0; col < layout.Columns; col++ {
		layout.ColumnX[col] = canvasIndent + col*(layout.ColumnW+columnGap)
	}
	return layout
}

// cardBlockHeight is the vertical distance between card tops on the winding
// path. The mockup spaces card tops about two card-heights apart.
const cardBlockHeight = 14

// pageBounds returns the slice of the card list rendered as one winding page.
func pageBounds(problemCount, selected int) (start, count int) {
	if problemCount <= 0 {
		return 0, 0
	}
	selected = clampIndex(selected, problemCount)
	start = (selected / snakeGroupSize) * snakeGroupSize
	end := start + snakeGroupSize
	if end > problemCount {
		end = problemCount
	}
	return start, end - start
}

// cardGridY is the top row of a card inside the unscrolled grid.
func cardGridY(index, pageStart, columns int) int {
	col := snakeColumn(index-pageStart, columns)
	slot := 0
	for i := pageStart; i < index; i++ {
		if snakeColumn(i-pageStart, columns) == col {
			slot++
		}
	}
	// Every card reserves one row above it for the solved tick tag.
	return staggerOffset(col) + slot*cardBlockHeight + 1
}

// gridContentHeight is the tallest column on the page, including the trailing
// gap after its last card.
func gridContentHeight(cards, columns int) int {
	height := 0
	for col := 0; col < columns; col++ {
		count := 0
		for i := 0; i < cards; i++ {
			if snakeColumn(i, columns) == col {
				count++
			}
		}
		if h := staggerOffset(col) + count*cardBlockHeight; h > height {
			height = h
		}
	}
	return height
}

// gridScrollOffset returns the vertical window that keeps the selected card
// visible. It is derived from the selection on every render, so a resize or a
// jump to another page can never leave the grid stranded at an old offset.
func gridScrollOffset(problemCount, selected, columns, viewportHeight int) int {
	if problemCount <= 0 || viewportHeight <= 0 {
		return 0
	}
	start, count := pageBounds(problemCount, selected)
	content := gridContentHeight(count, columns)
	if content <= viewportHeight {
		return 0
	}

	selected = clampIndex(selected, problemCount)
	top := cardGridY(selected, start, columns)
	bottom := top + cardHeight + 1 // card frame plus its label row

	const margin = 1
	scroll := 0
	if bottom+margin > viewportHeight {
		scroll = bottom + margin - viewportHeight
	}
	if top-margin < scroll {
		scroll = top - margin
	}

	maxScroll := content - viewportHeight
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	return scroll
}

// cardCell is one clickable level card rectangle in view coordinates.
type cardCell struct {
	Index int
	X     int
	Y     int
	W     int
	H     int
}

// computeCardCells returns every visible card rectangle for the current page,
// clipped to the grid viewport and shifted by the same scroll offset the
// renderer uses.
func computeCardCells(problemCount, selected, canvasWidth, viewHeight int) []cardCell {
	if problemCount <= 0 {
		return nil
	}
	canvas := computeCanvasLayout(viewHeight)
	grid := computeGridLayout(canvasWidth, canvas.GridHeight)
	start, count := pageBounds(problemCount, selected)
	if count == 0 || canvas.GridHeight <= 0 {
		return nil
	}
	scroll := gridScrollOffset(problemCount, selected, grid.Columns, canvas.GridHeight)

	gridTop := canvas.GridTop
	gridBottom := gridTop + canvas.GridHeight

	cells := make([]cardCell, 0, count)
	for index := start; index < start+count; index++ {
		col := snakeColumn(index-start, grid.Columns)
		top := gridTop + cardGridY(index, start, grid.Columns) - scroll
		bottom := top + cardBlockHeight - 1
		if bottom <= gridTop || top >= gridBottom {
			continue
		}
		if top < gridTop {
			top = gridTop
		}
		if bottom > gridBottom {
			bottom = gridBottom
		}
		cardX := grid.ColumnX[col] + (grid.ColumnW-cardWidth)/2
		cells = append(cells, cardCell{
			Index: index,
			X:     cardX,
			Y:     top,
			W:     cardWidth,
			H:     bottom - top,
		})
	}
	return cells
}
