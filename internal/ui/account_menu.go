package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/settings"
	"openleet/internal/ui/toast"
	"openleet/internal/ui/uikit"
)

type accountMenuItem struct {
	Label  string
	Action string
}

type accountMenuHit struct {
	Y      int
	Start  int
	End    int
	Index  int
	Action string
}

func (m AppModel) accountMenuItems() []accountMenuItem {
	if m.account == nil {
		return []accountMenuItem{
			{Label: "Log in", Action: "login"},
			{Label: "Continue offline", Action: "offline"},
		}
	}
	return []accountMenuItem{
		{Label: "Sync now", Action: "sync"},
		{Label: "Account settings", Action: "settings"},
		{Label: "Log out", Action: "logout"},
	}
}

func (m AppModel) accountMenuWidth() int {
	width := 34
	if m.width > 0 && width > m.width-4 {
		width = m.width - 4
	}
	if width < 24 {
		width = 24
	}
	return width
}

func (m AppModel) accountMenuPanel() (string, []accountMenuHit) {
	pal := m.palette
	width := m.accountMenuWidth()
	contentW := width - 4
	if contentW < 10 {
		contentW = 10
	}
	background := pal.SurfaceRaised
	border := lipgloss.NewStyle().Foreground(pal.Border).Background(background)
	text := lipgloss.NewStyle().Foreground(pal.Text).Background(background)
	dim := lipgloss.NewStyle().Foreground(pal.TextDim).Background(background)
	selectedBackground := pal.Surface
	selected := lipgloss.NewStyle().Foreground(pal.Accent).Background(selectedBackground).Bold(true)

	fit := func(content string, bg lipgloss.TerminalColor) string {
		if ansi.StringWidth(content) > contentW {
			content = ansi.Truncate(content, contentW, "…")
		}
		if gap := contentW - ansi.StringWidth(content); gap > 0 {
			content += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
		}
		return content
	}
	row := func(content string, bg lipgloss.TerminalColor) string {
		return border.Render("│") +
			lipgloss.NewStyle().Background(bg).Render(" ") +
			fit(content, bg) +
			lipgloss.NewStyle().Background(bg).Render(" ") +
			border.Render("│")
	}

	lines := []string{border.Render("┌" + strings.Repeat("─", width-2) + "┐")}
	hits := make([]accountMenuHit, 0, len(m.accountMenuItems()))
	y := 1

	if m.account != nil {
		email := strings.TrimSpace(m.accountChipEmail())
		if email == "" {
			email = "Signed in"
		}
		lines = append(lines, row(text.Render(" "+uikit.Truncate(email, contentW-2, "…")), background))
		plan := strings.TrimSpace(m.account.Plan)
		if plan == "" {
			plan = "Free"
		}
		lines = append(lines, row(dim.Render(" Plan  "+plan), background))
		y += 2
		lines = append(lines, border.Render("├"+strings.Repeat("─", width-2)+"┤"))
		y++
	} else {
		lines = append(lines, row(dim.Render(" Not signed in"), background))
		y++
		lines = append(lines, border.Render("├"+strings.Repeat("─", width-2)+"┤"))
		y++
	}

	for index, item := range m.accountMenuItems() {
		active := index == m.accountMenuIndex
		bg := background
		style := text
		prefix := "  "
		if active {
			bg = selectedBackground
			style = selected
			prefix = "› "
		}
		content := style.Render(prefix + item.Label)
		lines = append(lines, row(content, bg))
		hits = append(hits, accountMenuHit{
			Y:      y + index,
			Start:  0,
			End:    width - 1,
			Index:  index,
			Action: item.Action,
		})
	}
	lines = append(lines, border.Render("└"+strings.Repeat("─", width-2)+"┘"))
	return strings.Join(lines, "\n"), hits
}

// accountMenuOrigin returns the top-left cell of the panel, anchored under the
// right edge of the account chip.
func (m AppModel) accountMenuOrigin(width int) (int, int) {
	x := m.width - width - 2
	for _, hit := range m.topBarHits() {
		if hit.target == topBarTargetAccount {
			x = hit.end - width + 1
			break
		}
	}
	if x < 0 {
		x = 0
	}
	if x+width > m.width {
		x = m.width - width
	}
	if x < 0 {
		x = 0
	}
	return x, 1
}

func (m AppModel) accountMenuHits() []accountMenuHit {
	panel, hits := m.accountMenuPanel()
	_ = panel
	x, y := m.accountMenuOrigin(m.accountMenuWidth())
	out := make([]accountMenuHit, len(hits))
	for i, hit := range hits {
		hit.Start += x
		hit.End += x
		hit.Y += y
		out[i] = hit
	}
	return out
}

// overlayAccountMenu splices the dropdown over the current frame.
func (m AppModel) overlayAccountMenu(base string) string {
	panel, hits := m.accountMenuPanel()
	if len(hits) == 0 && panel == "" {
		return base
	}
	width := m.accountMenuWidth()
	x, y := m.accountMenuOrigin(width)
	lines := strings.Split(base, "\n")
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	panelLines := strings.Split(panel, "\n")
	for i, panelLine := range panelLines {
		row := y + i
		if row < 0 || row >= len(lines) || row >= m.height {
			continue
		}
		left := ansi.Cut(lines[row], 0, x)
		if gap := x - ansi.StringWidth(left); gap > 0 {
			left += strings.Repeat(" ", gap)
		}
		right := ansi.Cut(lines[row], x+width, m.width)
		if gap := m.width - x - width - ansi.StringWidth(right); gap > 0 {
			right += strings.Repeat(" ", gap)
		}
		lines[row] = left + panelLine + right
	}
	return strings.Join(lines, "\n")
}

func (m AppModel) openAccountMenu() AppModel {
	m.accountMenuOpen = true
	m.accountMenuIndex = 0
	m.topBarHover = topBarTargetAccount
	m.topBarFocus = topBarTargetAccount
	m.topBarPressed = topBarTargetNone
	if m.notifier != nil {
		m.notifier.SetGlobalPaused(true, time.Now())
	}
	return m
}

func (m AppModel) closeAccountMenu() AppModel {
	m.accountMenuOpen = false
	m.accountMenuIndex = 0
	if m.notifier != nil {
		m.notifier.SetGlobalPaused(false, time.Now())
	}
	return m
}

func (m AppModel) accountMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.accountMenuItems()
	if len(items) == 0 {
		return m.closeAccountMenu(), nil
	}
	switch msg.String() {
	case "esc":
		return m.closeAccountMenu(), nil
	case "up", "k", "shift+tab":
		m.accountMenuIndex = (m.accountMenuIndex - 1 + len(items)) % len(items)
		return m, nil
	case "down", "j", "tab":
		m.accountMenuIndex = (m.accountMenuIndex + 1) % len(items)
		return m, nil
	case "enter", " ":
		return m.activateAccountMenu()
	}
	return m, nil
}

func (m AppModel) activateAccountMenu() (AppModel, tea.Cmd) {
	items := m.accountMenuItems()
	if len(items) == 0 {
		return m.closeAccountMenu(), nil
	}
	if m.accountMenuIndex < 0 || m.accountMenuIndex >= len(items) {
		m.accountMenuIndex = 0
	}
	action := items[m.accountMenuIndex].Action
	m = m.closeAccountMenu()
	switch action {
	case "login", "settings":
		return m.openSettings("account"), nil
	case "offline":
		return m, m.notify(toast.Info, "Continuing offline", "Your local profile is still active.", toast.Options{})
	case "sync":
		return m, func() tea.Msg { return settings.ActionMsg{Action: "account_sync"} }
	case "logout":
		return m, func() tea.Msg { return settings.ActionMsg{Action: "account_logout"} }
	default:
		return m, nil
	}
}

// handleAccountMenuMouse consumes clicks while the dropdown owns the pointer.
func (m AppModel) handleAccountMenuMouse(msg tea.MouseMsg) (bool, AppModel, tea.Cmd) {
	if !m.accountMenuOpen {
		return false, m, nil
	}
	if msg.Action == tea.MouseActionMotion {
		for _, hit := range m.accountMenuHits() {
			if msg.Y == hit.Y && msg.X >= hit.Start && msg.X <= hit.End {
				m.accountMenuIndex = hit.Index
				break
			}
		}
		return true, m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return true, m, nil
	}
	for _, hit := range m.accountMenuHits() {
		if msg.Y == hit.Y && msg.X >= hit.Start && msg.X <= hit.End {
			m.accountMenuIndex = hit.Index
			next, cmd := m.activateAccountMenu()
			return true, next, cmd
		}
	}
	// A click outside closes the menu and is consumed so it cannot trigger a
	// screen action behind the popup.
	return true, m.closeAccountMenu(), nil
}
