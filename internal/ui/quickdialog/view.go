package quickdialog

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

// panelContentWidth returns the usable content width for the fixed dialog
// frame. The total panel is content+6 (2 border + 4 horizontal padding).
func panelContentWidth(viewport int) int {
	total := viewport - 4
	if total > 76 {
		total = 76
	}
	if total < 20 {
		total = viewport
	}
	content := total - 6
	if content < 8 {
		content = 8
	}
	return content
}

func (m Model) muted(value string) string {
	return lipgloss.NewStyle().Foreground(m.pal.Muted).Render(value)
}

func (m Model) text(value string) string {
	return lipgloss.NewStyle().Foreground(m.pal.Text).Render(value)
}

func (m Model) danger(value string) string {
	return lipgloss.NewStyle().Foreground(m.pal.Danger).Render(value)
}

// Panel renders the dialog body. The caller centers it over the active screen.
func (m Model) Panel() string {
	contentW := panelContentWidth(m.width)
	rows := make([]string, 0, 24)

	title := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Text).Render(m.opts.Title)
	rows = append(rows, uikit.JoinLeftRight(contentW, title, m.muted("esc")))
	rows = append(rows, strings.Repeat(" ", contentW))

	switch m.mode {
	case ModeList:
		rows = append(rows, m.renderList(contentW)...)
	case ModeInput:
		rows = append(rows, m.renderInput(contentW)...)
	case ModeConfirm:
		rows = append(rows, m.renderConfirm(contentW)...)
	}

	style := lipgloss.NewStyle().
		Width(contentW+4).
		Background(m.pal.Surface).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.pal.Border).
		Padding(1, 2)
	panel := style.Render(strings.Join(rows, "\n"))
	return uikit.PaintBackground(panel, m.pal.Surface)
}

func (m Model) renderList(contentW int) []string {
	rows := make([]string, 0, m.visibleRows()+8)
	if m.opts.Search {
		search := m.search
		search.Width = max(6, contentW-2)
		line := m.muted("/ ") + search.View()
		rows = append(rows, uikit.FitExact(line, contentW))
		rows = append(rows, strings.Repeat(" ", contentW))
	}

	if len(m.rows) == 0 {
		rows = append(rows, uikit.FitExact(m.muted("No matches"), contentW))
	} else {
		start := m.offset
		if start < 0 {
			start = 0
		}
		end := start + m.visibleRows()
		if end > len(m.rows) {
			end = len(m.rows)
		}
		for i := start; i < end; i++ {
			row := m.rows[i]
			if row.header {
				heading := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Subtext).Render(strings.ToUpper(row.label))
				rows = append(rows, uikit.FitExact(heading, contentW))
				continue
			}
			rows = append(rows, m.renderItem(row.item, i == m.selected && !m.opts.ReadOnly, contentW))
		}
	}

	rows = append(rows, strings.Repeat(" ", contentW))
	hint := m.opts.Hint
	if hint == "" {
		if m.opts.ReadOnly {
			hint = "↑/↓ scroll  esc close"
		} else {
			hint = "↑/↓ move  enter apply  esc cancel"
		}
	}
	rows = append(rows, uikit.FitExact(m.muted(hint), contentW))
	return rows
}

func (m Model) renderItem(item Item, selected bool, contentW int) string {
	marker := "  "
	if item.Active {
		marker = "✓ "
	}
	left := marker + item.Label
	value := item.Value
	if maxValue := contentW - lipgloss.Width(left) - 1; maxValue >= 0 && lipgloss.Width(value) > maxValue {
		value = uikit.Truncate(value, maxValue, "…")
	}
	if selected {
		line := uikit.JoinLeftRight(contentW, left, value)
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(m.pal.Background).
			Background(m.pal.Accent).
			Render(uikit.FitExact(line, contentW))
	}
	leftStyled := lipgloss.NewStyle().Foreground(m.pal.Text).Render(left)
	rightStyled := m.muted(value)
	return uikit.FitExact(uikit.JoinLeftRight(contentW, leftStyled, rightStyled), contentW)
}

func (m Model) renderInput(contentW int) []string {
	rows := make([]string, 0, 8)
	rows = append(rows, uikit.FitExact(m.muted(m.inputLabel), contentW))
	input := m.input
	input.Width = max(6, contentW-2)
	rows = append(rows, uikit.FitExact(input.View(), contentW))
	rows = append(rows, strings.Repeat(" ", contentW))
	if m.errText != "" {
		rows = append(rows, uikit.FitExact(m.danger(m.errText), contentW))
	}
	hint := m.opts.Hint
	if hint == "" {
		hint = "enter save  esc back"
	}
	rows = append(rows, uikit.FitExact(m.muted(hint), contentW))
	return rows
}

func (m Model) renderConfirm(contentW int) []string {
	rows := make([]string, 0, 8)
	rows = append(rows, uikit.FitExact(m.text(m.question), contentW))
	rows = append(rows, strings.Repeat(" ", contentW))

	yes := "  [ Yes ]  "
	no := "  [ No ]  "
	yesStyle := lipgloss.NewStyle().Background(m.pal.Surface).Foreground(m.pal.Text)
	noStyle := lipgloss.NewStyle().Background(m.pal.Surface).Foreground(m.pal.Text)
	if m.confirmYes {
		yesStyle = lipgloss.NewStyle().Bold(true).Background(m.pal.Accent).Foreground(m.pal.Background)
	} else {
		noStyle = lipgloss.NewStyle().Bold(true).Background(m.pal.Accent).Foreground(m.pal.Background)
	}
	buttons := yesStyle.Render(yes) + "  " + noStyle.Render(no)
	rows = append(rows, uikit.FitExact(buttons, contentW))
	rows = append(rows, strings.Repeat(" ", contentW))
	hint := m.opts.Hint
	if hint == "" {
		hint = "enter confirm  esc cancel"
	}
	rows = append(rows, uikit.FitExact(m.muted(hint), contentW))
	return rows
}
