package home

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/config"
	"openleet/internal/model"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/uikit"
)

// OpenCommandsMsg is dispatched when the user requests the global command palette
// via ctrl+p or the footer shortcut.
type OpenCommandsMsg struct{}

// Model represents the OpenCode-style minimalist landing / home screen.
type Model struct {
	input         textinput.Model
	palette       PaletteModel
	isPaletteOpen bool
	problems      []*model.Problem
	userState     *model.UserState
	config        *config.Config
	activeLang    string
	width         int
	height        int
	workingDir    string
	statusMessage string
	pal           theme.Palette
}

// New constructs an initialized Home model.
func New(cfg *config.Config, problems []*model.Problem, userState *model.UserState, width, height int) Model {
	ti := textinput.New()
	ti.Placeholder = "Search problems, practice, or type / for commands..."
	ti.CharLimit = 80
	ti.Focus()

	cardW := computeCardWidth(width)
	ti.Width = cardW - 6

	wd, err := os.Getwd()
	if err != nil {
		wd = "openleet"
	}

	pm := NewPaletteModel(problems, cardW)

	return Model{
		input:         ti,
		palette:       pm,
		isPaletteOpen: false,
		problems:      problems,
		userState:     userState,
		config:        cfg,
		activeLang:    cfg.DefaultLanguage,
		width:         width,
		height:        height,
		workingDir:    wd,
		statusMessage: "",
		pal:           theme.Default(),
	}
}

func computeCardWidth(termWidth int) int {
	w := int(float64(termWidth) * 0.65)
	if w < 52 {
		w = 52
	}
	if w > 72 {
		w = 72
	}
	if w > termWidth-4 {
		w = termWidth - 4
	}
	if w < 34 {
		w = 34
	}
	return w
}

// metaLine renders the session metadata shown under the prompt.
func (m Model) metaLine(maxWidth int) string {
	totalCount := len(m.problems)

	coachPill := lipgloss.NewStyle().Foreground(m.pal.Accent).Render("Coach")
	sep := lipgloss.NewStyle().Foreground(m.pal.Border).Render(" · ")
	langPill := lipgloss.NewStyle().Foreground(m.pal.Primary).Render(strings.Title(m.activeLang))
	statPill := lipgloss.NewStyle().Foreground(m.pal.Subtext).Render(fmt.Sprintf("%d Problems", totalCount))
	runnerPill := lipgloss.NewStyle().Foreground(m.pal.Success).Render("Local")

	pillsLine := coachPill + sep + langPill + sep + statPill + sep + runnerPill
	if maxWidth > 0 {
		if lipgloss.Width(pillsLine) > maxWidth {
			pillsLine = coachPill + sep + langPill + sep + statPill
		}
		if lipgloss.Width(pillsLine) > maxWidth {
			pillsLine = coachPill + sep + langPill
		}
	}
	return pillsLine
}

// miniStatsLine renders the streak and solved summary under the search box.
// Numbers use the text token, labels and separators use text_dim. It returns
// "" when there is nothing to show.
func (m Model) miniStatsLine(maxWidth int) string {
	streak, solved, total := 0, 0, len(m.problems)
	if m.userState != nil {
		streak = m.userState.CurrentStreak
		solved = len(m.userState.SolvedProblems)
	}
	if streak == 0 && solved == 0 {
		return ""
	}

	number := lipgloss.NewStyle().Foreground(m.pal.Text)
	dim := lipgloss.NewStyle().Foreground(m.pal.TextDim)
	sep := dim.Render(" · ")
	count := fmt.Sprintf("%d/%d", solved, total)

	var full, short string
	if streak > 0 {
		full = number.Render(fmt.Sprintf("%dd", streak)) + dim.Render(" streak") + sep +
			number.Render(count) + dim.Render(" solved")
		short = number.Render(fmt.Sprintf("%dd", streak)) + sep + number.Render(count)
	} else {
		full = number.Render(count) + dim.Render(" solved")
		short = number.Render(count)
	}
	if maxWidth > 0 && lipgloss.Width(full) > maxWidth {
		return short
	}
	return full
}

// searchPanel composes the problem search list, prompt, and session metadata into
// a single search surface.
func (m Model) searchPanel() string {
	boxW := computeCardWidth(m.width)
	innerW := boxW - 2
	rows := m.palette.ViewRows()

	// The prompt owns the cursor when problem search is open. Hide the landing-page
	// placeholder so the first cell is a clean cursor block rather than a
	// half-drawn "Search" label.
	input := m.input
	input.Placeholder = ""
	prompt := lipgloss.NewStyle().
		Width(innerW).
		Padding(0, 1).
		Background(m.pal.Surface).
		Render(input.View())
	meta := lipgloss.NewStyle().
		Width(innerW).
		Padding(0, 1).
		Background(m.pal.Surface).
		Render(m.metaLine(innerW - 2))

	spacer := lipgloss.NewStyle().
		Width(innerW).
		Background(m.pal.Surface).
		Render("")

	parts := []string{rows, spacer, prompt, spacer, meta}

	return lipgloss.NewStyle().
		Width(boxW).
		Background(m.pal.Surface).
		Border(lipgloss.NormalBorder(), true, true, true, true).
		BorderForeground(m.pal.Border).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// Init triggers initial setup.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// SetProblems updates available problems.
func (m *Model) SetProblems(problems []*model.Problem) {
	m.problems = problems
	m.palette.SetProblems(problems)
}

// SetUserState updates user progress state.
func (m *Model) SetUserState(state *model.UserState) {
	m.userState = state
}

// SetPalette repaints the landing page and its problem search with a theme.
func (m *Model) SetPalette(p theme.Palette) {
	m.pal = p
	m.palette.SetPalette(p)
	m.input.TextStyle = lipgloss.NewStyle().Foreground(p.Text)
	m.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(p.Muted)
	m.input.PromptStyle = lipgloss.NewStyle().Foreground(p.Accent)
	m.input.CompletionStyle = lipgloss.NewStyle().Foreground(p.Muted)
	// Cursor.View applies Reverse itself; an explicit background would leak
	// through uikit.PaintBackground's reset restoration.
	m.input.Cursor.Style = lipgloss.NewStyle().Foreground(p.Text)
}

// Width returns the model's current width.
func (m Model) Width() int {
	return m.width
}

// Height returns the model's current height.
func (m Model) Height() int {
	return m.height
}

// SetDimensions handles window resize.
func (m *Model) SetDimensions(w, h int) {
	m.width = w
	m.height = h
	cardW := computeCardWidth(w)
	m.palette.SetWidth(cardW)
	m.input.Width = cardW - 6
}

// IsPaletteOpen returns whether the problem search dropdown is visible.
func (m Model) IsPaletteOpen() bool {
	return m.isPaletteOpen
}

// SetActiveLanguage updates the language shown in the metadata pills.
func (m *Model) SetActiveLanguage(lang string) {
	m.activeLang = lang
}

// Update processes Bubble Tea events for the Home screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetDimensions(msg.Width, msg.Height)
		return m, nil

	case tea.MouseMsg:
		// 1. Mouse wheel navigation on problem search dropdown
		if msg.Button == tea.MouseButtonWheelUp {
			if m.isPaletteOpen {
				m.palette.SelectPrev()
			}
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			if m.isPaletteOpen {
				m.palette.SelectNext()
			}
			return m, nil
		}

		// 2. Click handling
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			cardW := computeCardWidth(m.width)
			centerX := m.width / 2
			startX := centerX - cardW/2
			endX := startX + cardW

			// Horizontal check
			if msg.X < startX || msg.X > endX {
				if m.isPaletteOpen {
					m.isPaletteOpen = false
				}
				return m, nil
			}

			availableBodyHeight := m.height - 1
			if m.isPaletteOpen {
				panel := m.searchPanel()
				panelH := lipgloss.Height(panel)
				startY := (availableBodyHeight - panelH) / 2
				if startY < 0 {
					startY = 0
				}
				rowsTop := startY + 1 // top border
				visible := m.palette.maxVisible
				if len(m.palette.filteredItems) < visible {
					visible = len(m.palette.filteredItems)
				}
				if msg.Y >= rowsTop && msg.Y < rowsTop+visible {
					clickedRow := msg.Y - rowsTop
					targetIdx := m.palette.scrollOffset + clickedRow
					if targetIdx >= 0 && targetIdx < len(m.palette.filteredItems) {
						m.palette.selectedIndex = targetIdx
						sel := m.palette.SelectedItem()
						if sel != nil && sel.Problem != nil {
							prob := sel.Problem
							m.isPaletteOpen = false
							m.input.Reset()
							return m, func() tea.Msg {
								return PaletteSelectionMsg{
									Action:   ActionOpenProblem,
									Argument: sel.Argument,
									Problem:  prob,
								}
							}
						}
					}
					return m, nil
				}
				if msg.Y >= startY && msg.Y < startY+panelH {
					m.input.Focus()
					return m, nil
				}
				m.isPaletteOpen = false
				return m, nil
			}

			logoH := 7
			cardH := 5
			totalStackH := logoH + 1 + cardH
			startY := (availableBodyHeight - totalStackH) / 2
			if startY < 0 {
				startY = 0
			}
			cardStartY := startY + logoH + 1
			cardEndY := cardStartY + cardH
			if msg.Y >= cardStartY && msg.Y <= cardEndY {
				m.input.Focus()
				val := strings.TrimSpace(m.input.Value())
				if val != "" && !strings.HasPrefix(val, "/") {
					m.isPaletteOpen = true
					m.palette.Filter(val)
				}
				return m, nil
			}
		}
		return m, nil

	case tea.KeyMsg:
		keyStr := msg.String()

		switch keyStr {
		case "ctrl+p":
			return m, func() tea.Msg { return OpenCommandsMsg{} }

		case "esc":
			if m.isPaletteOpen {
				m.isPaletteOpen = false
				return m, nil
			}
			if m.input.Value() != "" {
				m.input.Reset()
				return m, nil
			}

		case "up", "ctrl+k":
			if m.isPaletteOpen {
				m.palette.SelectPrev()
				return m, nil
			}

		case "down", "ctrl+j":
			if m.isPaletteOpen {
				m.palette.SelectNext()
				return m, nil
			}

		case "enter":
			if m.isPaletteOpen {
				sel := m.palette.SelectedItem()
				if sel != nil && sel.Problem != nil {
					prob := sel.Problem
					m.isPaletteOpen = false
					m.input.Reset()
					return m, func() tea.Msg {
						return PaletteSelectionMsg{
							Action:  ActionOpenProblem,
							Problem: prob,
						}
					}
				}
			}
			return m, nil
		}
	}

	var tiCmd tea.Cmd
	prevVal := m.input.Value()
	m.input, tiCmd = m.input.Update(msg)
	if tiCmd != nil {
		cmds = append(cmds, tiCmd)
	}

	newVal := m.input.Value()
	if newVal != prevVal {
		if strings.HasPrefix(newVal, "/") {
			m.input.Reset()
			m.isPaletteOpen = false
			m.palette.Filter("")
			return m, tea.Batch(cmds...)
		} else if newVal != "" {
			m.isPaletteOpen = true
			m.palette.Filter(newVal)
		} else {
			m.isPaletteOpen = false
			m.palette.Filter("")
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the centered OpenCode-style landing page.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Initializing OpenLeet..."
	}

	cardW := computeCardWidth(m.width)
	cardInnerW := cardW - 4
	if cardInnerW < 20 {
		cardInnerW = 20
	}

	// 1. Logo
	logo := RenderLogo(m.pal.Text, m.pal.Accent, m.pal.Background, m.width)

	// 2. Metadata Pills Row (OpenCode clean text style, zero emoji line-wrapping)
	pillsLine := m.metaLine(cardInnerW)

	// 3. Input card content
	promptLine := m.input.View()

	innerCardContent := lipgloss.JoinVertical(
		lipgloss.Left,
		promptLine,
		"",
		pillsLine,
	)

	// Left vertical accent bar, tinted by the active theme.
	cardStyle := lipgloss.NewStyle().
		Width(cardW).
		Background(m.pal.Surface).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.pal.Accent).
		Padding(1, 2)

	inputCard := cardStyle.Render(innerCardContent)

	// 4. Problem search panel (if open). The logo is intentionally hidden so the
	// search panel becomes the hero surface.
	var centerStack string
	if m.isPaletteOpen {
		centerStack = m.searchPanel()
	} else {
		parts := []string{logo, "", inputCard}
		// The mini stats sit one blank row under the search box, centered on
		// the box width rather than the terminal. They are omitted entirely
		// when there is no streak and nothing solved.
		if stats := m.miniStatsLine(cardW); stats != "" {
			parts = append(parts, "", lipgloss.PlaceHorizontal(cardW, lipgloss.Center, stats))
		}
		centerStack = lipgloss.JoinVertical(lipgloss.Center, parts...)
	}

	// 6. Page assembly. The app chrome (breadcrumb bar, footer bar) lives at
	// the AppModel level; home owns only its centered landing card.
	availableBodyHeight := m.height
	if availableBodyHeight < 5 {
		availableBodyHeight = 5
	}

	stackHeight := lipgloss.Height(centerStack)
	var bodyCentered string
	if m.isPaletteOpen {
		bodyCentered = lipgloss.Place(
			m.width,
			availableBodyHeight,
			lipgloss.Center,
			lipgloss.Center,
			centerStack,
		)
	} else if stackHeight >= availableBodyHeight {
		bodyCentered = lipgloss.NewStyle().Width(m.width).Render(centerStack)
	} else {
		bodyCentered = lipgloss.Place(
			m.width,
			availableBodyHeight,
			lipgloss.Center,
			lipgloss.Center,
			centerStack,
		)
	}

	return uikit.PaintBackground(bodyCentered, m.pal.Background)
}
