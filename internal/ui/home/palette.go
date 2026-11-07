package home

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/model"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/uikit"
)

// PaletteAction defines what action to take when an item is chosen.
type PaletteAction int

const (
	ActionNone PaletteAction = iota
	ActionRoadmap
	ActionSolve
	ActionOpenProblem
	ActionRandomProblem
	ActionFilterDifficulty
	ActionFilterTopic
	ActionSwitchLanguage
	ActionCycleFocus
	ActionNextTab
	ActionOpenCoach
	ActionRunTests
	ActionSaveCode
	ActionFormatCode
	ActionViewStats
	ActionThemes
	ActionSettings
	ActionResetCode
	ActionDoctor
	ActionHelp
	ActionQuit
)

// PaletteSelectionMsg is dispatched when a problem or footer action is selected.
type PaletteSelectionMsg struct {
	Action   PaletteAction
	Argument string
	Problem  *model.Problem
}

// PaletteItem is a rendered item representing a problem in the search menu.
type PaletteItem struct {
	IsProblem   bool
	Command     string
	Description string
	Problem     *model.Problem
	Action      PaletteAction
	Argument    string
}

// PaletteModel manages problem search results floating above or inside the input card.
type PaletteModel struct {
	problems      []*model.Problem
	filteredItems []PaletteItem
	selectedIndex int
	width         int
	maxVisible    int
	scrollOffset  int
	pal           theme.Palette
}

// NewPaletteModel constructs an initialized PaletteModel for problem search.
func NewPaletteModel(problems []*model.Problem, width int) PaletteModel {
	pm := PaletteModel{
		problems:      problems,
		selectedIndex: 0,
		width:         width,
		maxVisible:    11,
		scrollOffset:  0,
		pal:           theme.Default(),
	}
	pm.Filter("")
	return pm
}

// SetPalette repaints the dropdown with the active theme.
func (m *PaletteModel) SetPalette(p theme.Palette) {
	m.pal = p
}

// SetProblems updates the problems list.
func (m *PaletteModel) SetProblems(problems []*model.Problem) {
	m.problems = problems
}

// SetWidth updates the palette box width.
func (m *PaletteModel) SetWidth(w int) {
	m.width = w
}

// Filter updates the filtered list of items based on the user's query.
func (m *PaletteModel) Filter(query string) {
	trimmed := strings.TrimSpace(query)
	var items []PaletteItem

	if trimmed != "" {
		q := strings.ToLower(trimmed)
		for _, p := range m.problems {
			match := strings.Contains(strings.ToLower(p.Title), q) ||
				strings.Contains(strings.ToLower(p.Slug), q) ||
				strings.Contains(strings.ToLower(string(p.Difficulty)), q)
			if !match {
				for _, top := range p.Topics {
					if strings.Contains(strings.ToLower(top), q) {
						match = true
						break
					}
				}
			}

			if match {
				desc := uikit.PadRight(string(p.Difficulty), 6) + " • " + strings.Join(p.Topics, ", ")
				items = append(items, PaletteItem{
					IsProblem:   true,
					Command:     fmt.Sprintf("#%s %s", p.ID, p.Title),
					Description: desc,
					Problem:     p,
					Action:      ActionOpenProblem,
				})
			}
		}
	}

	m.filteredItems = items
	if m.selectedIndex >= len(m.filteredItems) {
		m.selectedIndex = len(m.filteredItems) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	m.updateScroll()
}

// SelectNext moves selection down.
func (m *PaletteModel) SelectNext() {
	if len(m.filteredItems) == 0 {
		return
	}
	m.selectedIndex = (m.selectedIndex + 1) % len(m.filteredItems)
	m.updateScroll()
}

// SelectPrev moves selection up.
func (m *PaletteModel) SelectPrev() {
	if len(m.filteredItems) == 0 {
		return
	}
	m.selectedIndex = (m.selectedIndex - 1 + len(m.filteredItems)) % len(m.filteredItems)
	m.updateScroll()
}

func (m *PaletteModel) updateScroll() {
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+m.maxVisible {
		m.scrollOffset = m.selectedIndex - m.maxVisible + 1
	}
}

// SelectedItem returns the currently active item, or nil if none.
func (m PaletteModel) SelectedItem() *PaletteItem {
	if len(m.filteredItems) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredItems) {
		return nil
	}
	return &m.filteredItems[m.selectedIndex]
}

// ItemCount returns total filtered items.
func (m PaletteModel) ItemCount() int {
	return len(m.filteredItems)
}

// View renders the floating problem search box.
func (m PaletteModel) View() string {
	return m.containerStyle().Render(m.ViewRows())
}

// ViewRows renders only problem rows.
func (m PaletteModel) ViewRows() string {
	boxW := m.width
	rowW := boxW - 2
	if rowW < 4 {
		rowW = 4
	}

	if len(m.filteredItems) == 0 {
		return lipgloss.NewStyle().
			Width(rowW).
			Padding(0, 1).
			Foreground(m.pal.Muted).
			Render("No matching problems found.")
	}

	visibleCount := m.maxVisible
	if len(m.filteredItems) < visibleCount {
		visibleCount = len(m.filteredItems)
	}

	endIdx := m.scrollOffset + visibleCount
	if endIdx > len(m.filteredItems) {
		endIdx = len(m.filteredItems)
	}

	textW := rowW - 2
	if textW < 2 {
		textW = 2
	}

	hideDesc := textW < 40
	descColWidth := 24
	if textW >= 60 {
		descColWidth = 28
	}
	if textW-descColWidth-2 < 14 {
		descColWidth = textW - 16
		if descColWidth < 10 {
			descColWidth = 10
		}
	}
	titleColWidth := textW - descColWidth - 2
	if titleColWidth < 4 {
		titleColWidth = 4
	}

	var renderedRows []string

	for i := m.scrollOffset; i < endIdx; i++ {
		item := m.filteredItems[i]
		isSelected := (i == m.selectedIndex)

		var rowContent string
		var titlePadded, descPadded string
		if hideDesc {
			rowContent = uikit.Fit(item.Command, textW)
		} else {
			titlePadded = uikit.Fit(item.Command, titleColWidth)
			descPadded = uikit.Fit(item.Description, descColWidth)
			rowContent = titlePadded + "  " + descPadded
		}

		if isSelected {
			highlightStyle := lipgloss.NewStyle().
				Background(m.pal.Accent).
				Foreground(m.pal.Background).
				Bold(true).
				Width(rowW).
				Padding(0, 1)
			renderedRows = append(renderedRows, highlightStyle.Render(rowContent))
		} else {
			titleStyle := lipgloss.NewStyle().
				Foreground(m.pal.Text).
				Bold(true)
			descStyle := lipgloss.NewStyle().
				Foreground(m.pal.Muted)

			rowStyle := lipgloss.NewStyle().
				Background(m.pal.Surface).
				Width(rowW).
				Padding(0, 1)

			var renderedRow string
			if hideDesc {
				renderedRow = titleStyle.Render(rowContent)
			} else {
				renderedRow = titleStyle.Render(titlePadded) + "  " + descStyle.Render(descPadded)
			}
			renderedRows = append(renderedRows, rowStyle.Render(renderedRow))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, renderedRows...)
}

// containerStyle is the shared frame used by View.
func (m PaletteModel) containerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(m.width).
		Background(m.pal.Surface).
		Border(lipgloss.NormalBorder(), true, true, true, true).
		BorderForeground(m.pal.Border)
}
