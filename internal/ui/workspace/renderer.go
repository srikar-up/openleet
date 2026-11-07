package workspace

// Layout constants keep the three-column proportions in one place.
const (
	SidebarBreakpoint = 120
	MinTerminalWidth  = 100
	MinTerminalHeight = 30

	DescriptionPercent = 32
	EditorPercent      = 48
	SidebarPercent     = 20
	EditorHeightPct    = 65

	// DividerWidth is the single-cell shared border rail between panes.
	DividerWidth        = 1
	ColumnGap           = 1
	MinDescriptionWidth = 20
	MinEditorWidth      = 44
	MinSidebarWidth     = 20
)

// Layout describes the workspace panes in terminal cells. Widths are panel
// widths, including any outer border and the interior padding.
// X positions are absolute cell offsets inside the workspace canvas.
type Layout struct {
	Width  int
	Height int

	// ColumnGap is retained for callers that derive the centre pane from the
	// left pane. It is the single-cell width of the divider rail.
	ColumnGap        int
	ShowDescription  bool
	ShowSidebar      bool
	ShowRightDivider bool

	DescriptionX  int
	LeftDividerX  int
	EditorX       int
	RightDividerX int
	SidebarX      int

	DescriptionWidth  int
	DescriptionHeight int
	EditorWidth       int
	EditorHeight      int
	ConsoleWidth      int
	ConsoleHeight     int
	SidebarWidth      int
	SidebarHeight     int
}

// TotalWidth returns the combined width of all visible panes and rails.
func (l Layout) TotalWidth() int {
	total := l.DescriptionWidth + 1 + l.EditorWidth
	if l.ShowRightDivider {
		total += 1 + l.SidebarWidth
	}
	return total
}

// LayoutOptions carries the user's live pane preferences into layout math.
// Zero widths mean "use the default percentage"; a hidden pane reports a zero
// width but keeps the requested width for when it is reopened.
type LayoutOptions struct {
	HideDescription  bool
	HideSidebar      bool
	DescriptionWidth int
	SidebarWidth     int
}

// DescriptionContentWidth returns the usable content width inside the description
// pane (outer left border plus two padding cells).
func DescriptionContentWidth(panelWidth int) int {
	return nonNegative(panelWidth - 3)
}

// EditorContentWidth returns the usable content width inside the editor or
// console pane (two padding cells, no side borders).
func EditorContentWidth(panelWidth int) int {
	return nonNegative(panelWidth - 2)
}

// SidebarContentWidth returns the usable content width inside the sidebar
// pane (outer right border plus two padding cells).
func SidebarContentWidth(panelWidth int) int {
	return nonNegative(panelWidth - 3)
}

// PanelContentHeight returns the usable content height inside a bordered panel
// with one row of vertical padding on each side.
func PanelContentHeight(panelHeight int) int {
	return nonNegative(panelHeight - 4)
}

// ComputeLayout divides the supplied terminal area into the target 32/48/20
// grid with the default open panes.
func ComputeLayout(width, height int) Layout {
	return ComputeLayoutWith(width, height, LayoutOptions{})
}

// ComputeLayoutWith divides the terminal into panes, honouring collapsed rails
// and explicit pane widths. The left rail is always present: it is the handle
// that reopens the description after it has been collapsed. The right rail is
// present only at the sidebar breakpoint, so a narrow screen cannot advertise a
// pane it cannot fit.
func ComputeLayoutWith(width, height int, opts LayoutOptions) Layout {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	layout := Layout{
		Width:            width,
		Height:           height,
		ColumnGap:        DividerWidth,
		ShowDescription:  !opts.HideDescription,
		ShowSidebar:      width >= SidebarBreakpoint && !opts.HideSidebar,
		ShowRightDivider: width >= SidebarBreakpoint,
	}

	visibleRails := 1
	if layout.ShowRightDivider {
		visibleRails = 2
	}
	contentWidth := width - visibleRails
	if contentWidth < 20 {
		contentWidth = 20
	}

	sidebarWidth := 0
	if layout.ShowSidebar {
		if opts.SidebarWidth > 0 {
			sidebarWidth = opts.SidebarWidth
		} else {
			sidebarWidth = contentWidth * SidebarPercent / 100
		}
	}

	descriptionWidth := 0
	if layout.ShowDescription {
		if opts.DescriptionWidth > 0 {
			descriptionWidth = opts.DescriptionWidth
		} else {
			percent := DescriptionPercent
			if !layout.ShowRightDivider {
				percent = 38
			}
			descriptionWidth = contentWidth * percent / 100
		}
	}

	maxPane := width / 2
	if layout.ShowSidebar {
		upper := minInt(maxPane, contentWidth-descriptionWidth-MinEditorWidth)
		if upper < MinSidebarWidth {
			upper = MinSidebarWidth
		}
		sidebarWidth = clampInt(sidebarWidth, MinSidebarWidth, upper)
	}
	if layout.ShowDescription {
		upper := minInt(maxPane, contentWidth-sidebarWidth-MinEditorWidth)
		if upper < MinDescriptionWidth {
			upper = MinDescriptionWidth
		}
		descriptionWidth = clampInt(descriptionWidth, MinDescriptionWidth, upper)
	}

	layout.DescriptionWidth = descriptionWidth
	layout.SidebarWidth = sidebarWidth
	layout.EditorWidth = maxInt(contentWidth-descriptionWidth-sidebarWidth, 0)

	layout.DescriptionX = 0
	layout.DescriptionHeight = height
	layout.SidebarHeight = height

	layout.LeftDividerX = 0
	if layout.ShowDescription {
		layout.LeftDividerX = layout.DescriptionWidth
	}
	layout.EditorX = layout.LeftDividerX + 1
	layout.RightDividerX = -1
	if layout.ShowRightDivider {
		layout.RightDividerX = layout.EditorX + layout.EditorWidth
	}
	layout.SidebarX = -1
	if layout.ShowSidebar {
		layout.SidebarX = layout.RightDividerX + 1
	}

	layout.ConsoleWidth = layout.EditorWidth
	layout.EditorHeight = height * EditorHeightPct / 100
	layout.ConsoleHeight = height - layout.EditorHeight
	layout.EditorHeight = maxInt(layout.EditorHeight, 12)
	layout.ConsoleHeight = maxInt(layout.ConsoleHeight, 9)
	if layout.EditorHeight+layout.ConsoleHeight > height {
		layout.ConsoleHeight = maxInt(height-layout.EditorHeight, 4)
		if layout.EditorHeight+layout.ConsoleHeight > height {
			layout.EditorHeight = maxInt(0, height-layout.ConsoleHeight)
		}
	}
	if layout.EditorHeight+layout.ConsoleHeight < height {
		layout.ConsoleHeight = height - layout.EditorHeight
	}

	return layout
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(value, low, high int) int {
	if high < low {
		high = low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// InnerContentWidth returns the content width remaining after padding is removed.
func InnerContentWidth(width, horizontalPadding int) int {
	return nonNegative(width - 2*nonNegative(horizontalPadding))
}

// InnerContentHeight returns the content height remaining after vertical padding
// is removed.
func InnerContentHeight(height, verticalPadding int) int {
	return nonNegative(height - 2*nonNegative(verticalPadding))
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
