package toast

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/theme"
)

// Rendered is one card ready to be spliced into a frame. Action coordinates are
// relative to the card's top-left corner.
type Rendered struct {
	Lines     []string
	Width     int
	Height    int
	HasAction bool
	ActionX0  int
	ActionX1  int
	ActionY   int
}

// Hit is an inclusive absolute cell rectangle on the terminal grid.
type Hit struct {
	ID     int
	X0     int
	Y0     int
	X1     int
	Y1     int
	Action bool
}

// Host renders the notification stack over a frame.
type Host struct {
	Width   int
	Height  int
	Palette theme.Palette
	Unicode bool
}

// CardWidth returns the shared stack width: 32..48 cells normally, and the
// full canvas minus two cells on narrow terminals.
func CardWidth(toasts []Toast, screenWidth int) int {
	if screenWidth < 100 {
		return max(1, screenWidth-2)
	}
	widest := 0
	for _, t := range toasts {
		title := t.Title
		if t.Count > 1 {
			title += " x" + itoa(t.Count)
		}
		widest = max(widest, ansi.StringWidth(title)+3)
		widest = max(widest, ansi.StringWidth(t.Message))
		widest = max(widest, ansi.StringWidth(t.ActionHint))
	}
	width := max(32, widest+6)
	if width > 48 {
		width = 48
	}
	if limit := screenWidth - 2; width > limit {
		width = limit
	}
	if width < 12 {
		width = max(1, screenWidth-2)
	}
	return width
}

func itoa(value int) string {
	if value <= 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func kindColor(pal theme.Palette, kind Kind) lipgloss.TerminalColor {
	switch kind {
	case Success:
		return pal.Pass
	case Warning:
		return pal.Warn
	case Error:
		return pal.Fail
	default:
		return pal.Accent
	}
}

func glyph(kind Kind, unicode bool) string {
	switch kind {
	case Success:
		if unicode {
			return "✓"
		}
		return "+"
	case Warning:
		return "!"
	case Error:
		if unicode {
			return "✕"
		}
		return "x"
	default:
		return "i"
	}
}

func fitCell(content string, width int, background lipgloss.TerminalColor, ellipsis bool) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(content) > width {
		tail := ""
		if ellipsis && width > 1 {
			tail = "…"
		}
		content = ansi.Truncate(content, width, tail)
	}
	if gap := width - ansi.StringWidth(content); gap > 0 {
		content += lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", gap))
	}
	return content
}

// RenderCard paints one notification card with a single-line border. The left
// border cell carries the kind colour; the rest of the border uses the dim
// border token.
func RenderCard(pal theme.Palette, t Toast, width int, unicode bool) Rendered {
	if width < 7 {
		width = 7
	}
	contentW := width - 6
	background := pal.SurfaceRaised
	border := lipgloss.NewStyle().Foreground(pal.Border).Background(background)
	bar := lipgloss.NewStyle().Foreground(kindColor(pal, t.Kind)).Background(background)
	dim := lipgloss.NewStyle().Foreground(pal.TextDim).Background(background)
	text := lipgloss.NewStyle().Bold(true).Foreground(pal.Text).Background(background)

	top := border.Render("┌" + strings.Repeat("─", width-2) + "┐")
	bottom := border.Render("└" + strings.Repeat("─", width-2) + "┘")
	blank := bar.Render("│") +
		lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", width-2)) +
		border.Render("│")
	row := func(content string) string {
		return bar.Render("│") +
			lipgloss.NewStyle().Background(background).Render("  ") +
			fitCell(content, contentW, background, false) +
			lipgloss.NewStyle().Background(background).Render("  ") +
			border.Render("│")
	}

	title := ansi.Truncate(t.Title, contentW, "…")
	suffix := ""
	if t.Count > 1 {
		suffix = " x" + itoa(t.Count)
	}
	iconW := 2
	titleMax := contentW - iconW - ansi.StringWidth(suffix)
	if titleMax < 1 {
		suffix = ""
		titleMax = contentW - iconW
	}
	if titleMax < 1 {
		titleMax = contentW
	}
	title = ansi.Truncate(title, titleMax, "…")
	titleLine := lipgloss.NewStyle().Foreground(kindColor(pal, t.Kind)).Background(background).
		Render(glyph(t.Kind, unicode)+" ") + text.Render(title)
	if suffix != "" {
		titleLine += dim.Render(suffix)
	}

	lines := []string{top, blank, row(titleLine)}
	if message := strings.TrimSpace(t.Message); message != "" {
		message = strings.ReplaceAll(message, "\n", " ")
		wrapped := ansi.Wordwrap(message, contentW, " -/,.")
		messageLines := strings.Split(wrapped, "\n")
		if len(messageLines) > 3 {
			messageLines = messageLines[:3]
			last := messageLines[2]
			if contentW > 1 {
				last = ansi.Truncate(last, contentW-1, "") + "…"
			} else {
				last = "…"
			}
			messageLines[2] = last
		}
		for _, line := range messageLines {
			lines = append(lines, row(dim.Render(line)))
		}
	}

	actionY := -1
	actionX0, actionX1 := 0, 0
	if t.ActionHint != "" {
		hint := ansi.Truncate(t.ActionHint, contentW, "…")
		pad := contentW - ansi.StringWidth(hint)
		actionLine := lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", max(0, pad))) +
			dim.Render(hint)
		actionY = len(lines)
		actionX0 = 3 + max(0, pad)
		actionX1 = 3 + contentW - 1
		lines = append(lines, row(actionLine))
	}

	lines = append(lines, blank, bottom)
	return Rendered{
		Lines:     lines,
		Width:     width,
		Height:    len(lines),
		HasAction: actionY >= 0 && t.ActionID != "",
		ActionX0:  actionX0,
		ActionX1:  actionX1,
		ActionY:   actionY,
	}
}

// Preview returns one card as a standalone string.
func Preview(pal theme.Palette, t Toast, width int, unicode bool) string {
	return strings.Join(RenderCard(pal, t, width, unicode).Lines, "\n")
}

// Hits returns the card and action rectangles without painting a frame.
func (h Host) Hits(toasts []Toast) []Hit {
	_, hits := h.Render("", toasts)
	return hits
}

// Render splices the stack into the canvas. Newest notifications sit at the
// bottom; older cards stack upward with one blank row between.
func (h Host) Render(canvas string, toasts []Toast) (string, []Hit) {
	if len(toasts) == 0 || h.Width <= 0 || h.Height <= 0 {
		return canvas, nil
	}

	limit := maxVisible
	if h.Width < 100 {
		limit = 1
	}
	if len(toasts) > limit {
		toasts = toasts[len(toasts)-limit:]
	}

	cardW := CardWidth(toasts, h.Width)
	if h.Width < 100 {
		cardW = h.Width - 2
	}
	if cardW < 7 || cardW > h.Width {
		return canvas, nil
	}

	x := h.Width - cardW - 2
	if h.Width < 100 {
		x = 1
	}
	if x < 0 {
		x = 0
	}

	lines := strings.Split(canvas, "\n")
	for len(lines) < h.Height {
		lines = append(lines, strings.Repeat(" ", h.Width))
	}
	if len(lines) > h.Height {
		lines = lines[:h.Height]
	}

	type placed struct {
		rendered Rendered
		x        int
		y        int
		id       int
	}
	stack := make([]placed, 0, len(toasts))
	bottom := h.Height - 2
	for i := len(toasts) - 1; i >= 0; i-- {
		rendered := RenderCard(h.Palette, toasts[i], cardW, h.Unicode)
		top := bottom - rendered.Height + 1
		if top < 0 {
			break
		}
		stack = append(stack, placed{rendered: rendered, x: x, y: top, id: toasts[i].ID})
		bottom = top - 2
	}

	for _, card := range stack {
		for rowIndex, line := range card.rendered.Lines {
			row := card.y + rowIndex
			if row < 0 || row >= len(lines) {
				continue
			}
			base := lines[row]
			left := ansi.Cut(base, 0, card.x)
			if gap := card.x - ansi.StringWidth(left); gap > 0 {
				left += strings.Repeat(" ", gap)
			}
			right := ansi.Cut(base, card.x+cardW, h.Width)
			if gap := h.Width - card.x - cardW - ansi.StringWidth(right); gap > 0 {
				right += strings.Repeat(" ", gap)
			}
			lines[row] = left + line + right
		}
	}

	hits := make([]Hit, 0, len(stack)*2)
	for i := len(stack) - 1; i >= 0; i-- {
		card := stack[i]
		hits = append(hits, Hit{
			ID: card.id,
			X0: card.x,
			Y0: card.y,
			X1: card.x + cardW - 1,
			Y1: card.y + card.rendered.Height - 1,
		})
		if card.rendered.HasAction {
			hits = append(hits, Hit{
				ID:     card.id,
				X0:     card.x + card.rendered.ActionX0,
				Y0:     card.y + card.rendered.ActionY,
				X1:     card.x + card.rendered.ActionX1,
				Y1:     card.y + card.rendered.ActionY,
				Action: true,
			})
		}
	}

	return strings.Join(lines, "\n"), hits
}
