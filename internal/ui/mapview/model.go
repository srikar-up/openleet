package mapview

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/model"
)

// Model represents the Bubble Tea component state for the Figma-style Level Roadmap.
type Model struct {
	problems      []*model.Problem
	filtered      []*model.Problem
	userState     *model.UserState
	renderer      Renderer
	width         int
	height        int
	selectedIndex int
	activeFilter  string
	filters       []string
	commandInput  textinput.Model
	isCommandMode bool
	statusMsg     string
}

// New constructs an initialized Level Map model.
func New(problems []*model.Problem, userState *model.UserState, width, height int) Model {
	ti := textinput.New()
	ti.Placeholder = "/command (e.g. /solve 1, /topic dp, /company google)"
	ti.CharLimit = 64

	defaultFilters := []string{"DSA", "CP", "DP", "ARRAY", "LINK LIST", "GRAPH"}

	m := Model{
		problems:      problems,
		filtered:      problems,
		userState:     userState,
		renderer:      NewRenderer(),
		width:         width,
		height:        height,
		selectedIndex: 0,
		activeFilter:  "DSA",
		filters:       defaultFilters,
		commandInput:  ti,
		isCommandMode: false,
		statusMsg:     "Tab/Shift+Tab: topic · ←/→: problem · Enter: solve · /: command",
	}

	return m
}

// Width returns the model's current width.
func (m Model) Width() int {
	return m.width
}

// Height returns the model's current height.
func (m Model) Height() int {
	return m.height
}

// SetTheme replaces the roadmap palette used by the renderer.
func (m *Model) SetTheme(theme MapTheme) {
	m.renderer.Theme = theme
}

// Init initializes the map view component.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update processes Bubble Tea messages for the map view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		if m.isCommandMode {
			return m.handleCommandKey(msg)
		}
		return m.handleNormalKey(msg)
	}

	if m.isCommandMode {
		var cmd tea.Cmd
		m.commandInput, cmd = m.commandInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleCommandKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		res := ParseAndExecuteCommand(m.commandInput.Value(), m.problems, m.activeFilter)
		m.isCommandMode = false
		m.commandInput.Blur()
		m.commandInput.Reset()
		m.statusMsg = res.StatusMsg

		if res.ActiveFilter != m.activeFilter {
			m.activeFilter = res.ActiveFilter
			m.filtered = FilterProblems(m.problems, m.activeFilter)
			m.selectedIndex = 0
		}

		if res.TargetProblem != nil {
			for i, p := range m.filtered {
				if p.ID == res.TargetProblem.ID {
					m.selectedIndex = i
					break
				}
			}
			if res.OpenWorkspace {
				target := res.TargetProblem
				return m, func() tea.Msg {
					return OpenProblemMsg{Problem: target}
				}
			}
		}

		return m, nil

	case "esc":
		m.isCommandMode = false
		m.commandInput.Blur()
		m.commandInput.Reset()
		return m, nil
	}

	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	return m, cmd
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	keyStr := msg.String()
	total := len(m.filtered)

	switch keyStr {
	case "h", "left":
		if total > 0 && m.selectedIndex > 0 {
			m.selectedIndex--
			return m, m.notifySelected()
		}

	case "l", "right":
		if total > 0 && m.selectedIndex < total-1 {
			m.selectedIndex++
			return m, m.notifySelected()
		}

	case "k", "up":
		if total > 0 {
			cols := m.computeColumns()
			if m.selectedIndex-cols >= 0 {
				m.selectedIndex -= cols
			} else {
				m.selectedIndex = 0
			}
			return m, m.notifySelected()
		}

	case "j", "down":
		if total > 0 {
			cols := m.computeColumns()
			if m.selectedIndex+cols < total {
				m.selectedIndex += cols
			} else {
				m.selectedIndex = total - 1
			}
			return m, m.notifySelected()
		}

	case "enter":
		if total > 0 && m.selectedIndex < total {
			target := m.filtered[m.selectedIndex]
			return m, func() tea.Msg {
				return OpenProblemMsg{Problem: target}
			}
		}

	case "tab":
		m.cycleFilter(1)
		return m, m.notifySelected()

	case "shift+tab":
		m.cycleFilter(-1)
		return m, m.notifySelected()

	case "/", ":":
		m.isCommandMode = true
		m.commandInput.Focus()
		m.commandInput.SetValue("/")
		return m, textinput.Blink

	case "esc":
		if !strings.EqualFold(m.activeFilter, "DSA") {
			m.activeFilter = "DSA"
			m.filtered = FilterProblems(m.problems, "DSA")
			m.selectedIndex = 0
			m.statusMsg = "Reset to DSA roadmap."
		}
	}

	return m, nil
}

func (m Model) notifySelected() tea.Cmd {
	if len(m.filtered) == 0 || m.selectedIndex >= len(m.filtered) {
		return nil
	}
	p := m.filtered[m.selectedIndex]
	idx := m.selectedIndex
	return func() tea.Msg {
		return ProblemSelectedMsg{Problem: p, Index: idx}
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	// The map view owns the full screen: three header rows sit above the body.
	const headerRows = 3

	if msg.Button == tea.MouseButtonWheelUp {
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		return m, m.notifySelected()
	}
	if msg.Button == tea.MouseButtonWheelDown {
		if m.selectedIndex < len(m.filtered)-1 {
			m.selectedIndex++
		}
		return m, m.notifySelected()
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	layout := computeBodyLayout(m.width)

	// 1. Topic rail: one blank row, then four rows per button.
	if msg.X < layout.RailWidth && msg.Y >= headerRows {
		line := msg.Y - headerRows - 1
		if line >= 0 {
			index := line / 4
			if index >= 0 && index < len(m.filters) {
				m.activeFilter = m.filters[index]
				m.filtered = FilterProblems(m.problems, m.activeFilter)
				m.selectedIndex = 0
				return m, m.notifySelected()
			}
		}
		return m, nil
	}

	// 2. Inspector: clicking the details or the solve button opens the level.
	if layout.ShowInspector && msg.X >= layout.InspectorX {
		return m.openSelected()
	}

	// 3. Canvas: hit-test the winding level cards.
	if msg.X >= layout.CanvasX && msg.X < layout.CanvasX+layout.CanvasWidth {
		footerRows := 1 // the hint bar pinned to the bottom
		if m.isCommandMode {
			footerRows++ // the command input row
		}
		canvasHeight := m.height - headerRows - footerRows
		if canvasHeight < 1 {
			canvasHeight = 1
		}
		for _, cell := range computeCardCells(len(m.filtered), m.selectedIndex, layout.CanvasWidth, canvasHeight) {
			x0 := layout.CanvasX + cell.X
			y0 := headerRows + cell.Y
			if msg.X >= x0 && msg.X < x0+cell.W && msg.Y >= y0 && msg.Y < y0+cell.H {
				if cell.Index == m.selectedIndex {
					return m.openSelected()
				}
				m.selectedIndex = cell.Index
				return m, m.notifySelected()
			}
		}
	}

	return m, nil
}

// openSelected emits the message that opens the focused level.
func (m Model) openSelected() (Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.filtered) {
		return m, nil
	}
	target := m.filtered[m.selectedIndex]
	return m, func() tea.Msg {
		return OpenProblemMsg{Problem: target}
	}
}

// View renders the map view using its Lip Gloss renderer.
func (m Model) View() string {
	cmdText := ""
	if m.isCommandMode {
		cmdText = m.commandInput.Value()
	}

	data := ViewData{
		Width:        m.width,
		Height:       m.height,
		Title:        "OPENLEET/DSA",
		Problems:     m.filtered,
		UserState:    m.userState,
		Selected:     m.selectedIndex,
		ActiveFilter: m.activeFilter,
		Filters:      m.filters,
		Command:      cmdText,
		Status:       m.statusMsg,
	}

	return m.renderer.Render(data)
}

func (m *Model) cycleFilter(dir int) {
	if len(m.filters) == 0 {
		return
	}
	currentIdx := 0
	for i, f := range m.filters {
		if strings.EqualFold(f, m.activeFilter) {
			currentIdx = i
			break
		}
	}
	nextIdx := (currentIdx + dir + len(m.filters)) % len(m.filters)
	m.activeFilter = m.filters[nextIdx]
	m.filtered = FilterProblems(m.problems, m.activeFilter)
	m.selectedIndex = 0
	m.statusMsg = "Topic: " + m.activeFilter
}

func (m Model) computeColumns() int {
	layout := computeBodyLayout(m.width)
	canvasHeight := m.height - 3
	if canvasHeight < 0 {
		canvasHeight = 0
	}
	return computeGridLayout(layout.CanvasWidth, canvasHeight).Columns
}

// SetProblems updates the problems list and recomputes the filtered view.
func (m *Model) SetProblems(problems []*model.Problem) {
	m.problems = problems
	m.filtered = FilterProblems(problems, m.activeFilter)
	if m.selectedIndex >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.selectedIndex = len(m.filtered) - 1
		} else {
			m.selectedIndex = 0
		}
	}
}

// SetUserState updates the user progression state.
func (m *Model) SetUserState(state *model.UserState) {
	m.userState = state
}

// SetFilter applies a difficulty, topic, category, or company filter without
// changing screens. The roadmap recomputes its visible problem window.
func (m *Model) SetFilter(filter string) {
	m.activeFilter = strings.TrimSpace(filter)
	if m.activeFilter == "" {
		m.activeFilter = "all"
	}
	m.filtered = FilterProblems(m.problems, m.activeFilter)
	m.selectedIndex = 0
	m.statusMsg = "Filter: " + m.activeFilter
}

// ActiveFilter returns the filter currently applied to the roadmap list.
func (m Model) ActiveFilter() string {
	return m.activeFilter
}

// SelectProblem moves the roadmap cursor to a problem ID when it is visible.
func (m *Model) SelectProblem(id string) bool {
	for i, p := range m.filtered {
		if p.ID == id {
			m.selectedIndex = i
			return true
		}
	}
	return false
}

// SetSize updates the dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SelectedProblem returns the currently focused problem, if any.
func (m Model) SelectedProblem() *model.Problem {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filtered) {
		return m.filtered[m.selectedIndex]
	}
	return nil
}

// SelectedIndex returns the index of the selected problem.
func (m Model) SelectedIndex() int {
	return m.selectedIndex
}

// IsCommandMode returns true if the command prompt is active.
func (m Model) IsCommandMode() bool {
	return m.isCommandMode
}
