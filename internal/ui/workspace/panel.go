package workspace

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

type panelEdges struct {
	Left, Right                                bool
	TopLeft, TopRight, BottomLeft, BottomRight string
}

// renderPanel draws a single-line bordered pane with its title and optional
// metadata embedded in the top border. Edges adjacent to a rail omit the border
// and custom corner glyphs blend seamlessly into rail junctions.
func (m Model) renderPanel(width, height int, title, meta, content string, focused bool, edges panelEdges) string {
	if width < 1 || height < 4 {
		return ""
	}

	borderColor := m.theme.Border
	if focused {
		borderColor = m.theme.Accent
	}
	border := lipgloss.NewStyle().Foreground(borderColor)

	sideCount := 0
	if edges.Left {
		sideCount++
	}
	if edges.Right {
		sideCount++
	}
	contentW := width - sideCount - 2
	if contentW < 1 {
		contentW = 1
	}

	contentH := height - 4
	if contentH < 0 {
		contentH = 0
	}

	lines := strings.Split(content, "\n")
	if len(lines) > contentH {
		lines = lines[:contentH]
	}

	topLeft := edges.TopLeft
	if topLeft == "" {
		if edges.Left {
			topLeft = "╭"
		} else {
			topLeft = "─"
		}
	}
	topRight := edges.TopRight
	if topRight == "" {
		if edges.Right {
			topRight = "╮"
		} else {
			topRight = "─"
		}
	}
	bottomLeft := edges.BottomLeft
	if bottomLeft == "" {
		if edges.Left {
			bottomLeft = "╰"
		} else {
			bottomLeft = "─"
		}
	}
	bottomRight := edges.BottomRight
	if bottomRight == "" {
		if edges.Right {
			bottomRight = "╯"
		} else {
			bottomRight = "─"
		}
	}

	// Top border
	topLeftW := lipgloss.Width(topLeft)
	topRightW := lipgloss.Width(topRight)
	innerW := width - topLeftW - topRightW
	if innerW < 0 {
		innerW = 0
	}

	titleW := lipgloss.Width(title)
	metaW := lipgloss.Width(meta)
	leftW := 0
	if title != "" {
		leftW = 3 + titleW
	}
	rightW := 0
	if meta != "" {
		rightW = 3 + metaW
	}

	if leftW+rightW > innerW {
		meta = ""
		metaW = 0
		rightW = 0
	}
	if title != "" && leftW > innerW {
		maxTitleW := innerW - 3
		if maxTitleW > 0 {
			title = uikit.Truncate(title, maxTitleW, "…")
			titleW = lipgloss.Width(title)
			leftW = 3 + titleW
		} else {
			title = ""
			leftW = 0
		}
	}

	fillW := innerW - leftW - rightW
	if fillW < 0 {
		fillW = 0
	}

	var top strings.Builder
	top.WriteString(border.Render(topLeft))
	if title != "" {
		top.WriteString(border.Render("─ "))
		top.WriteString(title)
		top.WriteString(border.Render(" "))
	}
	if fillW > 0 {
		top.WriteString(border.Render(strings.Repeat("─", fillW)))
	}
	if meta != "" {
		top.WriteString(border.Render(" "))
		top.WriteString(meta)
		top.WriteString(border.Render(" ─"))
	}
	top.WriteString(border.Render(topRight))
	topLine := top.String()

	if w := lipgloss.Width(topLine); w != width {
		if w > width {
			topLine = uikit.Truncate(topLine, width-topRightW, "") + border.Render(topRight)
		} else if gap := width - w; gap > 0 {
			renderedTopRight := border.Render(topRight)
			if strings.HasSuffix(topLine, renderedTopRight) {
				topLine = strings.TrimSuffix(topLine, renderedTopRight) + border.Render(strings.Repeat("─", gap)) + renderedTopRight
			} else {
				topLine = uikit.FitExact(topLine, width)
			}
		}
	}

	// Bottom border: edges.BottomLeft + ─*(width-2) + edges.BottomRight
	bottomLeftW := lipgloss.Width(bottomLeft)
	bottomRightW := lipgloss.Width(bottomRight)
	innerBottomW := maxInt(0, width-bottomLeftW-bottomRightW)
	bottomLine := border.Render(bottomLeft + strings.Repeat("─", innerBottomW) + bottomRight)
	if w := lipgloss.Width(bottomLine); w != width {
		bottomLine = uikit.FitExact(bottomLine, width)
	}

	// Body rows
	leftBorder := ""
	if edges.Left {
		leftBorder = border.Render("│")
	}
	rightBorder := ""
	if edges.Right {
		rightBorder = border.Render("│")
	}

	formatRow := func(contentLine string) string {
		row := leftBorder + " " + uikit.FitExact(contentLine, contentW) + " " + rightBorder
		if lipgloss.Width(row) != width {
			row = uikit.FitExact(row, width)
		}
		return row
	}

	bodyRows := make([]string, 0, height-2)
	bodyRows = append(bodyRows, formatRow(""))
	for i := 0; i < contentH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		bodyRows = append(bodyRows, formatRow(line))
	}
	bodyRows = append(bodyRows, formatRow(""))

	allRows := make([]string, 0, height)
	allRows = append(allRows, topLine)
	allRows = append(allRows, bodyRows...)
	allRows = append(allRows, bottomLine)
	return strings.Join(allRows, "\n")
}
