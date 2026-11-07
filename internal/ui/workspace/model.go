package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/coach"
	"openleet/internal/model"
	"openleet/internal/runner"
	"openleet/internal/storage"
	"openleet/internal/ui/uikit"
)

// Model is the root Bubble Tea Elm model for the 3-panel split workspace.
type Model struct {
	problem           *model.Problem
	language          string
	storage           storage.Manager
	storageScope      string
	executor          *runner.Executor
	coachManager      *coach.Manager
	theme             WorkspaceTheme
	width             int
	height            int
	focus             Focus
	description       DescriptionModel
	editor            EditorModel
	console           ConsoleModel
	isRunning         bool
	startedAt         time.Time
	attempts          int
	hintsOpen         bool
	langPickerOpen    bool
	langPickerIndex   int
	langPickerOptions []string
	formatOnSave      bool
	pendingFormatSave bool

	// Live pane geometry. Zero widths mean "use the default percentage".
	// Collapsed panes are hidden but keep their requested width for reopening.
	descWidth         int
	sidebarWidth      int
	descriptionHidden bool
	sidebarHidden     bool
	dragEdge          int
	dragOffset        int
	hoverEdge         int
	lastRailPress     time.Time
	lastRailEdge      int
	lastRailX         int
	lastRailY         int

	synced viewportLayout
}

// viewportLayout is the pane geometry last pushed into the child widgets.
// Comparing the whole shape (not only terminal width/height) is what makes a
// collapse, reopen, or rail drag reflow the viewport immediately.
type viewportLayout struct {
	width             int
	height            int
	description       int
	editor            int
	sidebar           int
	descriptionHidden bool
	sidebarHidden     bool
	hintsOpen         bool
}

// New constructs an initialized Model for the 3-column split workspace.
func New(
	problem *model.Problem,
	language string,
	storageMgr storage.Manager,
	exec *runner.Executor,
	coachMgr *coach.Manager,
	width, height int,
) Model {
	theme := DefaultWorkspaceTheme()
	editorDefaults := DefaultEditorOptions()
	m := Model{
		problem:      problem,
		language:     language,
		storage:      storageMgr,
		executor:     exec,
		coachManager: coachMgr,
		theme:        theme,
		width:        width,
		height:       height,
		focus:        FocusEditor, // Default focus: code editor buffer
		isRunning:    false,
		startedAt:    time.Now(),
		hintsOpen:    true,
		formatOnSave: editorDefaults.FormatOnSave,
	}

	if storageMgr != nil {
		if saved, err := storageMgr.LoadWorkspaceLayout(); err == nil {
			m.applySavedLayout(saved)
		}
	}

	layout := m.layout()

	descHeight := PanelContentHeight(layout.DescriptionHeight) - m.hintsFooterRows()
	if descHeight < 4 {
		descHeight = 4
	}
	desc := NewDescriptionModel(problem, theme, DescriptionContentWidth(layout.DescriptionWidth), descHeight)
	ed := NewEditorModel(problem, language, theme, EditorContentWidth(layout.EditorWidth), PanelContentHeight(layout.EditorHeight))
	con := NewConsoleModel(theme, EditorContentWidth(layout.ConsoleWidth), PanelContentHeight(layout.ConsoleHeight), coachMgr)

	m.description = desc
	m.editor = ed
	m.console = con

	if problem != nil {
		m.console.SetContext(problem, ed.Content(), language)
	}

	m.syncEditorFocus()
	m.synced = m.layoutState()
	return m
}

// LayoutState extracts the current layout configuration for persistence.
func (m Model) LayoutState() storage.WorkspaceLayout {
	return storage.WorkspaceLayout{
		DescriptionWidth:  m.descWidth,
		SidebarWidth:      m.sidebarWidth,
		DescriptionHidden: m.descriptionHidden,
		SidebarHidden:     m.sidebarHidden,
	}
}

func (m *Model) applySavedLayout(saved storage.WorkspaceLayout) {
	if saved.DescriptionWidth >= 0 {
		m.descWidth = saved.DescriptionWidth
	}
	if saved.SidebarWidth >= 0 {
		m.sidebarWidth = saved.SidebarWidth
	}
	m.descriptionHidden = saved.DescriptionHidden
	m.sidebarHidden = saved.SidebarHidden
}

func (m Model) saveLayoutCmd() tea.Cmd {
	if m.storage == nil {
		return nil
	}
	store := m.storage
	state := m.LayoutState()
	return func() tea.Msg {
		_ = store.SaveWorkspaceLayout(state)
		return nil
	}
}

// layout computes the live pane geometry from the terminal size plus the
// user's rail state.
func (m Model) layout() Layout {
	return ComputeLayoutWith(m.width, m.height, LayoutOptions{
		HideDescription:  m.descriptionHidden,
		HideSidebar:      m.sidebarHidden,
		DescriptionWidth: m.descWidth,
		SidebarWidth:     m.sidebarWidth,
	})
}

// layoutState is both the input for resizing children and the cache key for
// deciding whether a render needs to reflow them.
func (m Model) layoutState() viewportLayout {
	l := m.layout()
	return viewportLayout{
		width:             m.width,
		height:            m.height,
		description:       l.DescriptionWidth,
		editor:            l.EditorWidth,
		sidebar:           l.SidebarWidth,
		descriptionHidden: m.descriptionHidden,
		sidebarHidden:     m.sidebarHidden,
		hintsOpen:         m.hintsOpen,
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func dividerArrowRow(height int) int {
	if height <= 0 {
		return 0
	}
	return height / 2
}

// Init loads the saved code buffer from storage if available.
func (m Model) Init() tea.Cmd {
	return m.loadCodeBufferCmd()
}

func (m Model) loadCodeBufferCmd() tea.Cmd {
	if m.problem == nil || m.storage == nil {
		return nil
	}
	pid := m.problem.ID
	lang := m.language
	scope := m.storageScope
	store := m.storage
	return func() tea.Msg {
		code, err := store.LoadCodeBuffer(pid, lang)
		if err == nil && code != "" {
			return BufferLoadedMsg{ProblemID: pid, Content: code, Language: lang, Scope: scope}
		}
		return nil
	}
}

// SetScope labels asynchronous buffer loads so a stale reply from the previous
// account cannot overwrite the new profile.
func (m *Model) SetScope(scope string) { m.storageScope = scope }

// SetStorage swaps the scoped manager, reloads that profile's layout, resets
// the editor to the problem template, and schedules its saved buffer.
func (m *Model) SetStorage(mgr storage.Manager, scope string) tea.Cmd {
	m.storage = mgr
	m.storageScope = scope
	m.descWidth = 0
	m.sidebarWidth = 0
	m.descriptionHidden = false
	m.sidebarHidden = false
	if mgr != nil {
		if saved, err := mgr.LoadWorkspaceLayout(); err == nil {
			m.applySavedLayout(saved)
		}
	}
	if m.problem != nil {
		m.SetProblem(m.problem)
	} else {
		m.editor.SetContent("")
	}
	m.synced = m.layoutState()
	return m.loadCodeBufferCmd()
}

// LoadCodeBuffer asks the workspace to load the persisted buffer for the
// current problem and language.
func (m *Model) LoadCodeBuffer() tea.Cmd {
	return m.loadCodeBufferCmd()
}

// Update processes events and handles focus routing.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.hoverEdge = 0
		var cmd tea.Cmd
		m, cmd = m.applyViewportSize()
		return m, cmd

	case BufferLoadedMsg:
		if msg.Scope != "" && msg.Scope != m.storageScope {
			return m, nil
		}
		if m.problem == nil || msg.ProblemID != m.problem.ID || normalizeLanguage(msg.Language) != m.language {
			return m, nil
		}
		if msg.Content != "" {
			m.editor.SetContent(msg.Content)
		}
		return m, nil

	case CodeSavedMsg:
		if m.problem == nil || msg.ProblemID != m.problem.ID || normalizeLanguage(msg.Language) != m.language {
			return m, nil
		}
		m.editor.SetSaving(false)
		if msg.Err == nil {
			m.editor.SetModified(false)
			m.console.AppendLog("Saved solution buffer ✓")
		} else {
			m.console.AppendLog("Error saving buffer: " + msg.Err.Error())
		}
		return m, nil

	case CodeFormattedMsg:
		if m.problem == nil || msg.ProblemID != m.problem.ID || normalizeLanguage(msg.Language) != m.language {
			return m, nil
		}
		pendingSave := m.pendingFormatSave
		m.pendingFormatSave = false
		if msg.Err != nil {
			m.console.AppendLog("Format failed: " + msg.Err.Error())
			if pendingSave {
				if cmd := m.saveCodeCmd(); cmd != nil {
					m.editor.SetSaving(true)
					return m, cmd
				}
			}
			return m, nil
		}
		if msg.Code != m.editor.Content() {
			m.editor.SetContent(msg.Code)
			m.editor.SetModified(true)
			m.console.AppendLog("Code beautified ✓")
		} else if !pendingSave {
			m.console.AppendLog("Code already formatted")
		}
		if pendingSave {
			if cmd := m.saveCodeCmd(); cmd != nil {
				m.editor.SetSaving(true)
				return m, cmd
			}
		}
		return m, nil

	case TestsCompletedMsg:
		m.isRunning = false
		m.console.SetRunning(false)
		m.console.SetResult(msg.Result)
		m.console.SetContext(m.problem, m.editor.Content(), m.language)
		if msg.Result != nil && !msg.Result.IsSuccessful() {
			m.console.SetHintAvailable(true)
		}
		return m, nil

	case CustomTestRequestedMsg:
		m.console.SetCustomRunning(true)
		cmd := m.runCustomTestCmd(msg.Input, msg.Expected)
		if cmd == nil {
			m.console.SetCustomError("Runner unavailable for a custom fixture.")
			return m, nil
		}
		return m, cmd

	case CustomTestCompletedMsg:
		m.console.SetCustomResult(msg.Result)
		return m, nil

	case coach.CoachStreamStartedMsg, coach.CoachTokenMsg, coach.CoachTokenBatchMsg, coach.CoachStreamDoneMsg, coach.CoachStreamErrorMsg:
		var cmd tea.Cmd
		m.console, cmd = m.console.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		layout := m.layout()
		centerX := layout.EditorX
		centerEndX := centerX + layout.EditorWidth

		// 0. Rail drag lifecycle. Cell-motion reports arrive while the left
		// button is held; a drag resizes the pane and reflows immediately.
		if m.dragEdge != 0 {
			switch msg.Action {
			case tea.MouseActionMotion:
				cmd := m.resizeDivider(m.dragEdge, msg.X, layout)
				synced, viewCmd := m.applyViewportSize()
				return synced, tea.Batch(cmd, viewCmd)
			case tea.MouseActionRelease:
				m.dragEdge = 0
				m.dragOffset = 0
				return m, m.saveLayoutCmd()
			}
		}
		if msg.Action == tea.MouseActionRelease {
			return m, nil
		}

		// If not dragging and Action == Motion: update rail hover.
		if msg.Action == tea.MouseActionMotion {
			m.hoverEdge = m.dividerAt(msg.X, msg.Y, layout)
			return m, nil
		}

		wasPickerOpen := m.langPickerOpen
		if m.langPickerOpen && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if optIdx, ok := m.languageMenuOptionAt(msg.X, msg.Y, layout); ok {
				m.langPickerIndex = optIdx
				return m.selectLanguagePicker()
			}
			m.langPickerOpen = false
		}

		// 1. Mouse wheel scrolling
		if msg.Button == tea.MouseButtonWheelUp {
			if layout.ShowDescription && msg.X < layout.DescriptionWidth {
				m.description.viewport.LineUp(2)
			} else if msg.X >= centerX && msg.X < centerEndX && msg.Y >= layout.EditorHeight {
				m.console.viewport.LineUp(2)
			}
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			if layout.ShowDescription && msg.X < layout.DescriptionWidth {
				m.description.viewport.LineDown(2)
			} else if msg.X >= centerX && msg.X < centerEndX && msg.Y >= layout.EditorHeight {
				m.console.viewport.LineDown(2)
			}
			return m, nil
		}

		// 2. Click to focus and interact
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			edge := m.dividerAt(msg.X, msg.Y, layout)

			now := time.Now()
			// Track the click position as well as the edge. Collapsing or
			// expanding a pane moves its rail, so the second click of a
			// double-click can land on the pane that just opened. Comparing
			// the original cells keeps the reset gesture deterministic.
			doubleClick := m.lastRailEdge != 0 &&
				!m.lastRailPress.IsZero() &&
				now.Sub(m.lastRailPress) <= 400*time.Millisecond &&
				absInt(msg.X-m.lastRailX) <= 1 &&
				absInt(msg.Y-m.lastRailY) <= 1
			if doubleClick {
				m.resetRail(m.lastRailEdge)
				synced, viewCmd := m.applyViewportSize()
				return synced, tea.Batch(viewCmd, m.saveLayoutCmd())
			}
			if edge != 0 {
				m.lastRailEdge = edge
				m.lastRailPress = now
				m.lastRailX = msg.X
				m.lastRailY = msg.Y
			} else {
				m.lastRailEdge = 0
				m.lastRailPress = time.Time{}
			}

			if edge != 0 {
				m.hoverEdge = edge
				hidden := (edge < 0 && m.descriptionHidden) || (edge > 0 && m.sidebarHidden)
				if hidden || onDividerHandle(msg.Y, m.height) {
					m = m.toggleDivider(edge)
					synced, viewCmd := m.applyViewportSize()
					return synced, tea.Batch(viewCmd, m.saveLayoutCmd())
				}
				m.dragEdge = edge
				m.dragOffset = 0
				return m, nil
			}

			m.hoverEdge = 0

			if layout.ShowDescription && msg.X < layout.DescriptionWidth {
				// The hints footer occupies the last rows of the description
				// content. Clicking them toggles the detail row and reflows
				// the viewport; anything above still focuses the pane.
				contentH := PanelContentHeight(layout.DescriptionHeight)
				footerTop := 2 + contentH - m.hintsFooterRows()
				if msg.Y >= footerTop && msg.Y < footerTop+m.hintsFooterRows() {
					m.hintsOpen = !m.hintsOpen
					synced, viewCmd := m.applyViewportSize()
					return synced, viewCmd
				}
				m.focus = FocusDescription
				m.syncEditorFocus()
				return m, nil
			}

			if msg.X >= centerX && msg.X < centerEndX && msg.Y < layout.EditorHeight {
				m.focus = FocusEditor
				m.syncEditorFocus()
				if msg.Y == 0 {
					if m.formatIconHit(msg.X, msg.Y, layout) {
						m.langPickerOpen = false
						return m, m.FormatCode()
					}
					if !wasPickerOpen {
						m.openLanguagePicker()
					}
					return m, nil
				}
				// Content starts after the top border and one row of padding.
				bodyRow := msg.Y - 2
				if bodyRow >= 0 && bodyRow < PanelContentHeight(layout.EditorHeight) {
					line := m.editor.ScrollRow() + bodyRow
					if line >= m.editor.LineCount() {
						line = m.editor.LineCount() - 1
					}
					if line < 0 {
						line = 0
					}
					codeX := centerX + 1 + m.editor.GutterWidth()
					cell := msg.X - codeX + m.editor.ScrollCol()
					if cell < 0 {
						cell = 0
					}
					m.editor.SetCursor(line, m.editor.ColumnAtCell(line, cell))
				}
				return m, nil
			}

			if msg.X >= centerX && msg.X < centerEndX && msg.Y >= layout.EditorHeight {
				m.focus = FocusConsole
				m.syncEditorFocus()

				// The tab labels live inside the console's top border.
				if msg.Y == layout.EditorHeight {
					relX := msg.X - centerX - 3
					if tab, ok := m.console.TabAt(relX); ok {
						m.console.SwitchToTab(tab)
						return m, nil
					}
				}
				return m, nil
			}
		}
		return m, nil

	case tea.KeyMsg:
		keyStr := msg.String()

		if m.langPickerOpen {
			switch {
			case keyStr == "esc" || keyStr == "ctrl+l" || msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlL:
				m.langPickerOpen = false
				return m, nil
			case keyStr == "up" || keyStr == "k" || msg.Type == tea.KeyUp:
				if m.langPickerIndex > 0 {
					m.langPickerIndex--
				}
				return m, nil
			case keyStr == "down" || keyStr == "j" || msg.Type == tea.KeyDown:
				if m.langPickerIndex < len(m.langPickerOptions)-1 {
					m.langPickerIndex++
				}
				return m, nil
			case keyStr == "enter" || msg.Type == tea.KeyEnter:
				return m.selectLanguagePicker()
			default:
				return m, nil
			}
		}

		// Tab cycles panes outside the code buffer. The editor keeps Tab for
		// indentation, and the coach input keeps it for slash autocomplete.
		if (keyStr == "tab" || msg.Type == tea.KeyTab) &&
			m.focus != FocusEditor &&
			!(m.focus == FocusConsole && (m.console.activeTab == TabSocraticCoach || m.console.activeTab == TabCustom)) {
			m.CycleFocus()
			return m, nil
		}

		switch {
		case keyStr == "ctrl+l" || msg.Type == tea.KeyCtrlL:
			m.openLanguagePicker()
			return m, nil

		case keyStr == "alt+1" || (msg.Alt && len(msg.Runes) == 1 && msg.Runes[0] == '1'):
			m.focus = FocusDescription
			m.syncEditorFocus()
			return m, nil

		case keyStr == "alt+2" || (msg.Alt && len(msg.Runes) == 1 && msg.Runes[0] == '2'):
			m.focus = FocusEditor
			m.syncEditorFocus()
			return m, nil

		case keyStr == "alt+3" || (msg.Alt && len(msg.Runes) == 1 && msg.Runes[0] == '3'):
			m.focus = FocusConsole
			m.syncEditorFocus()
			return m, nil

		case keyStr == "alt+d" || keyStr == "alt+[" || (msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 'd' || msg.Runes[0] == '[')):
			m = m.toggleDescription()
			synced, viewCmd := m.applyViewportSize()
			return synced, tea.Batch(viewCmd, m.saveLayoutCmd())

		case keyStr == "alt+s" || keyStr == "alt+]" || (msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 's' || msg.Runes[0] == ']')):
			m = m.toggleSidebar()
			synced, viewCmd := m.applyViewportSize()
			return synced, tea.Batch(viewCmd, m.saveLayoutCmd())

		case keyStr == "ctrl+w" || msg.Type == tea.KeyCtrlW:
			// Cycle focus forward: Description -> Editor -> Console -> Description
			m.focus = Focus((int(m.focus) + 1) % 3)
			m.syncEditorFocus()
			return m, nil

		case keyStr == "shift+tab" || msg.Type == tea.KeyShiftTab:
			// Cycle focus backwards
			m.focus = Focus((int(m.focus) + 2) % 3)
			m.syncEditorFocus()
			return m, nil

		case keyStr == "ctrl+r" || msg.Type == tea.KeyCtrlR:
			if !m.isRunning {
				m.isRunning = true
				m.attempts++
				m.console.SetRunning(true)
				m.console.CancelStream()
				return m, m.runTestsCmd()
			}
			return m, nil

		case keyStr == "ctrl+s" || msg.Type == tea.KeyCtrlS:
			return m, m.RequestSave()

		case keyStr == "ctrl+b" || msg.Type == tea.KeyCtrlB:
			return m, m.FormatCode()

		case keyStr == "ctrl+t" || msg.Type == tea.KeyCtrlT:
			m.console.SwitchTab()
			return m, nil

		case keyStr == "esc" || msg.Type == tea.KeyEsc:
			// If editor is focused and modified, blur first
			if m.focus == FocusEditor {
				m.focus = FocusConsole
				m.syncEditorFocus()
				return m, nil
			}
			return m, func() tea.Msg { return ExitWorkspaceMsg{} }
		}
	}

	// Route events to the currently focused child component
	var cmd tea.Cmd
	switch m.focus {
	case FocusDescription:
		m.description, cmd = m.description.Update(msg)
	case FocusEditor:
		m.editor, cmd = m.editor.Update(msg)
	case FocusConsole:
		m.console, cmd = m.console.Update(msg)
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) syncEditorFocus() {
	m.editor.SetFocused(m.focus == FocusEditor)
	m.description.SetFocused(m.focus == FocusDescription)
	m.console.SetFocused(m.focus == FocusConsole)
}

func (m Model) saveCodeCmd() tea.Cmd {
	if m.problem == nil || m.storage == nil {
		return nil
	}
	pid := m.problem.ID
	lang := m.language
	code := m.editor.Content()
	store := m.storage
	return func() tea.Msg {
		err := store.SaveCodeBuffer(pid, lang, code)
		return CodeSavedMsg{ProblemID: pid, Language: lang, Err: err}
	}
}

func (m Model) runTestsCmd() tea.Cmd {
	if m.problem == nil || m.executor == nil {
		return nil
	}
	p := m.problem
	lang := m.language
	code := m.editor.Content()
	exec := m.executor
	return func() tea.Msg {
		res := exec.Execute(context.Background(), p, lang, code)
		return TestsCompletedMsg{Result: res, Err: nil}
	}
}

func (m Model) runCustomTestCmd(input, expected interface{}) tea.Cmd {
	if m.problem == nil || m.executor == nil {
		return nil
	}
	p := m.problem
	lang := m.language
	code := m.editor.Content()
	exec := m.executor
	return func() tea.Msg {
		res := exec.ExecuteCustomCase(context.Background(), p, lang, code, input, expected)
		return CustomTestCompletedMsg{Result: res}
	}
}

// View composes the 32/48/20 workspace grid. Child widget geometry is derived
// from the live canvas on the value copy, so a resize always repaints against
// the terminal it is about to fill.
func (m Model) View() string {
	m = m.syncViewport()
	if m.width <= 0 || m.height <= 0 {
		return "Initializing workspace..."
	}

	layout := m.layout()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text)

	editorContent := m.editor.View()
	if m.langPickerOpen {
		editorContent = m.editor.ViewLanguageMenu(m.langPickerOptions, m.langPickerIndex)
	}

	descEdges := panelEdges{
		Left:        true,
		Right:       false,
		TopLeft:     "╭",
		TopRight:    "─",
		BottomLeft:  "╰",
		BottomRight: "─",
	}

	centerEdges := panelEdges{
		Left:        false,
		Right:       false,
		TopLeft:     "─",
		BottomLeft:  "─",
		TopRight:    "─",
		BottomRight: "─",
	}
	if !layout.ShowRightDivider {
		centerEdges.TopRight = "╮"
		centerEdges.BottomRight = "╯"
	}

	sidebarEdges := panelEdges{
		Left:        false,
		Right:       true,
		TopLeft:     "─",
		TopRight:    "╮",
		BottomLeft:  "─",
		BottomRight: "╯",
	}

	var descLines []string
	if layout.ShowDescription {
		descContent := m.description.View() + "\n" +
			m.renderHintsFooter(DescriptionContentWidth(layout.DescriptionWidth))
		descPanel := m.renderPanel(
			layout.DescriptionWidth,
			layout.DescriptionHeight,
			titleStyle.Render("Problem"),
			"",
			descContent,
			m.focus == FocusDescription,
			descEdges,
		)
		descLines = strings.Split(descPanel, "\n")
	}

	leftRail := strings.Split(m.renderRail(-1, m.descriptionHidden, layout.Height), "\n")

	editorPanel := m.renderPanel(
		layout.EditorWidth,
		layout.EditorHeight,
		titleStyle.Render(m.editor.FileName()),
		m.editor.Meta(),
		editorContent,
		m.focus == FocusEditor,
		centerEdges,
	)
	consolePanel := m.renderPanel(
		layout.ConsoleWidth,
		layout.ConsoleHeight,
		m.console.TabTitle(),
		"",
		m.console.View(),
		m.focus == FocusConsole,
		centerEdges,
	)
	centerLines := append(strings.Split(editorPanel, "\n"), strings.Split(consolePanel, "\n")...)

	var rightRail []string
	if layout.ShowRightDivider {
		rightRail = strings.Split(m.renderRail(1, m.sidebarHidden, layout.Height), "\n")
	}

	var sidebarLines []string
	if layout.ShowSidebar {
		sidebarContent := m.renderSidebar(
			SidebarContentWidth(layout.SidebarWidth),
			PanelContentHeight(layout.SidebarHeight),
		)
		sidebarPanel := m.renderPanel(
			layout.SidebarWidth,
			layout.SidebarHeight,
			titleStyle.Render("Session"),
			"",
			sidebarContent,
			false,
			sidebarEdges,
		)
		sidebarLines = strings.Split(sidebarPanel, "\n")
	}

	rows := make([]string, 0, layout.Height)
	for y := 0; y < layout.Height; y++ {
		var row strings.Builder
		if layout.ShowDescription && y < len(descLines) {
			row.WriteString(descLines[y])
		}
		if y < len(leftRail) {
			row.WriteString(leftRail[y])
		}
		if y < len(centerLines) {
			row.WriteString(centerLines[y])
		}
		if layout.ShowRightDivider && y < len(rightRail) {
			row.WriteString(rightRail[y])
		}
		if layout.ShowSidebar && y < len(sidebarLines) {
			row.WriteString(sidebarLines[y])
		}
		rows = append(rows, row.String())
	}

	return uikit.PaintBackground(strings.Join(rows, "\n"), m.theme.Background)
}

// renderRail paints the single-cell divider rail between panes.
// edge is -1 for the description rail and +1 for the session rail.
func (m Model) renderRail(edge int, hidden bool, height int) string {
	if height <= 0 {
		return ""
	}

	isActive := (m.hoverEdge == edge || m.dragEdge == edge)

	lineColor := m.theme.Border
	if isActive {
		lineColor = m.theme.Accent
	}
	lineStyle := lipgloss.NewStyle().
		Foreground(lineColor).
		Background(m.theme.Background)

	topChar := "┬"
	bottomChar := "┴"
	if edge < 0 {
		if hidden {
			topChar = "╭"
			bottomChar = "╰"
		}
	} else {
		if hidden {
			topChar = "╮"
			bottomChar = "╯"
		}
	}

	arrowRow := dividerArrowRow(height)

	glyph := "‹"
	if edge < 0 {
		if hidden {
			glyph = "›"
		} else {
			glyph = "‹"
		}
	} else {
		if hidden {
			glyph = "‹"
		} else {
			glyph = "›"
		}
	}

	glyphColor := m.theme.Muted
	glyphBold := false
	if isActive || (edge < 0 && m.focus == FocusDescription) {
		glyphColor = m.theme.Accent
		glyphBold = true
	}
	glyphStyle := lipgloss.NewStyle().
		Foreground(glyphColor).
		Background(m.theme.Background).
		Bold(glyphBold)

	label := ""
	if hidden {
		if edge < 0 {
			label = "PROBLEM"
		} else {
			label = "SESSION"
		}
	}
	letterColor := m.theme.Muted
	if isActive {
		letterColor = m.theme.Accent
	}
	letterStyle := lipgloss.NewStyle().
		Foreground(letterColor).
		Background(m.theme.Background)

	lines := make([]string, height)
	for i := 0; i < height; i++ {
		switch {
		case i == 0:
			lines[i] = lineStyle.Render(topChar)
		case i == height-1:
			lines[i] = lineStyle.Render(bottomChar)
		case i == arrowRow:
			lines[i] = glyphStyle.Render(glyph)
		default:
			letterPlaced := false
			if label != "" && i >= arrowRow+2 && i < height-1 {
				letterIdx := i - (arrowRow + 2)
				if letterIdx < len(label) {
					lines[i] = letterStyle.Render(string(label[letterIdx]))
					letterPlaced = true
				}
			}
			if !letterPlaced {
				lines[i] = lineStyle.Render("│")
			}
		}
	}
	return strings.Join(lines, "\n")
}

// onDividerHandle reports whether a row is the centre collapse/expand glyph row.
func onDividerHandle(y, height int) bool {
	return y == dividerArrowRow(height)
}

// dividerAt maps a cell to a rail edge: -1 description, +1 session, 0 none.
func (m Model) dividerAt(x, y int, layout Layout) int {
	if y < 0 || y >= m.height {
		return 0
	}
	if x == layout.LeftDividerX {
		return -1
	}
	if layout.ShowRightDivider && x == layout.RightDividerX {
		return 1
	}
	return 0
}

// toggleDivider flips the pane attached to a rail.
func (m Model) toggleDivider(edge int) Model {
	if edge < 0 {
		return m.toggleDescription()
	}
	return m.toggleSidebar()
}

// toggleDescription hides or restores the problem statement pane. Keyboard
// focus returns to the editor while the pane is closed.
func (m Model) toggleDescription() Model {
	m.descriptionHidden = !m.descriptionHidden
	if m.descriptionHidden && m.focus == FocusDescription {
		m.focus = FocusEditor
		m.syncEditorFocus()
	}
	return m
}

// toggleSidebar hides or restores the session pane. The rail does not exist
// below the breakpoint, so the shortcut is a no-op there.
func (m Model) toggleSidebar() Model {
	if m.width < SidebarBreakpoint {
		return m
	}
	m.sidebarHidden = !m.sidebarHidden
	return m
}

// resetRail unhides the rail, sets explicit width to 0 (default percentage),
// and resets double-click tracking.
func (m *Model) resetRail(edge int) {
	if edge < 0 {
		m.descriptionHidden = false
		m.descWidth = 0
	} else if edge > 0 {
		m.sidebarHidden = false
		m.sidebarWidth = 0
	}
	m.lastRailEdge = 0
	m.lastRailPress = time.Time{}
}

// resizeDivider converts an absolute pointer column into the pane width the
// user is dragging. The centre pane always keeps MinEditorWidth cells.
func (m *Model) resizeDivider(edge, x int, layout Layout) tea.Cmd {
	maxPane := m.width / 2
	railStart := x - m.dragOffset
	if edge < 0 {
		if railStart < 20 {
			m.descriptionHidden = true
			m.dragEdge = 0
			m.dragOffset = 0
			return m.saveLayoutCmd()
		}
		m.descriptionHidden = false
		m.descWidth = clampInt(railStart, 20, maxPane)
		return nil
	}
	if edge > 0 {
		sidebarWidth := m.width - (railStart + 1)
		if sidebarWidth < 20 {
			m.sidebarHidden = true
			m.dragEdge = 0
			m.dragOffset = 0
			return m.saveLayoutCmd()
		}
		m.sidebarHidden = false
		m.sidebarWidth = clampInt(sidebarWidth, 20, maxPane)
		return nil
	}
	return nil
}

// hintsFooterRows is the number of description content rows reserved for the
// hints-used footer: the marker row plus the detail row while it is open.
func (m Model) hintsFooterRows() int {
	if m.hintsOpen {
		return 2
	}
	return 1
}

// renderHintsFooter paints the collapse-aware hints counter pinned under the
// problem statement. Every row is exactly width cells so the footer and the
// reduced viewport together fill the description panel.
func (m Model) renderHintsFooter(width int) string {
	marker := "▸"
	if m.hintsOpen {
		marker = "▾"
	}
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	used := m.console.HintsUsed()
	header := fmt.Sprintf("%s HINTS USED  %d", marker, used)
	if !m.hintsOpen {
		return uikit.Fit(muted.Render(header), width)
	}
	detail := "No hints yet · ask with /hint"
	switch used {
	case 0:
		detail = "No hints yet · ask with /hint"
	case 1:
		detail = "1 hint requested in Coach"
	default:
		detail = fmt.Sprintf("%d hints requested in Coach", used)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		uikit.Fit(muted.Render(header), width),
		uikit.Fit(muted.Render(detail), width),
	)
}

func (m Model) renderSidebar(width, height int) string {
	section := func(label string) string {
		return lipgloss.NewStyle().Bold(true).Foreground(m.theme.Muted).
			Render(strings.ToUpper(label))
	}
	label := func(value string) string {
		return lipgloss.NewStyle().Foreground(m.theme.Muted).Render(value)
	}
	value := func(text string) string {
		return lipgloss.NewStyle().Foreground(m.theme.Text).Render(text)
	}
	// A fixed 10-cell label column plus exactly one space keeps the value
	// column aligned no matter how long the key names grow.
	row := func(name, text string) string {
		return uikit.FitExact(label(name), 10) + " " +
			value(uikit.Truncate(text, width-11, "…"))
	}
	line := func(text string) string {
		return uikit.FitExact(text, width)
	}

	var lines []string
	lines = append(lines,
		section("Session"),
		row("Mode", "Solve"),
		row("Lang", languageLabel(m.language)),
		row("Runner", "Local"),
	)

	elapsed := "--:--"
	if !m.startedAt.IsZero() {
		elapsed = time.Since(m.startedAt).Round(time.Second).String()
	}
	attempts := fmt.Sprintf("%d", m.attempts)
	lines = append(lines, row("Elapsed", elapsed))
	lines = append(lines, row("Attempts", attempts))

	if m.console.result != nil {
		res := m.console.result
		lines = append(lines, "", section("Last run"))
		summaryStyle := m.theme.SuccessBadge
		if !res.IsSuccessful() {
			summaryStyle = m.theme.FailureBadge
		}
		lines = append(lines, summaryStyle.Render(fmt.Sprintf("%d/%d passed", res.PassedTests, res.TotalTests)))
		for _, tr := range res.Results {
			marker := "✔"
			style := m.theme.SuccessBadge
			if !tr.Passed {
				marker = "✗"
				style = m.theme.FailureBadge
			}
			left := style.Render(marker) + " " + value(tr.TestCaseID)
			right := label(tr.Duration.Round(time.Microsecond).String())
			lines = append(lines, uikit.JoinLeftRight(width, left, right))
		}
	} else {
		lines = append(lines, "", section("Last run"), label("No runs yet"))
	}

	lines = append(lines, "", section("Keys"))
	lines = append(lines, section("RUN"))
	lines = append(lines,
		row("ctrl+r", "run tests"),
		row("ctrl+s", "save"),
		row("ctrl+b", "beautify"),
	)
	lines = append(lines, section("NAVIGATE"))
	lines = append(lines,
		row("tab", "focus"),
		row("alt+d", "statement"),
		row("alt+s", "session"),
		row("alt+[", "left"),
		row("alt+]", "right"),
		row("ctrl+p", "commands"),
		row("drag rail", "resize"),
		row("dbl-click", "reset"),
	)

	for i, lineText := range lines {
		lines[i] = line(lineText)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// FooterSession is the compact session summary used when the right sidebar is
// hidden below the 120-column breakpoint.
func (m Model) FooterSession() string {
	return fmt.Sprintf("solve · %s · local", languageLabel(m.language))
}

// SetProblem updates the active problem across all panes.
func (m *Model) SetProblem(p *model.Problem) {
	m.console.CancelStream()
	m.problem = p
	m.langPickerOpen = false
	norm := normalizeLanguage(m.language)
	m.editor.SetLanguage(norm)
	m.console.SetLanguage(norm)
	m.description.SetProblem(p)
	tmpl := ""
	if p != nil && p.Templates != nil {
		if t, ok := p.Templates[norm]; ok {
			tmpl = t
		} else if t, ok := p.Templates[m.language]; ok {
			tmpl = t
		}
		m.editor.SetContent(tmpl)
	}
	m.console.Reset()
	if p != nil {
		m.console.SetContext(p, tmpl, norm)
	}
	m.startedAt = time.Now()
	m.attempts = 0
	m.focus = FocusEditor
	m.syncEditorFocus()
}

// Width returns the workspace's current width.
func (m Model) Width() int {
	return m.width
}

// Height returns the workspace's current height.
func (m Model) Height() int {
	return m.height
}

// applyViewportSize propagates the canvas into the pane widgets and records
// which canvas they now reflect. A hidden description is not resized, because
// opening it again changes the layout signature and triggers this path again.
func (m *Model) applyViewportSize() (Model, tea.Cmd) {
	layout := m.layout()
	var cmds []tea.Cmd

	var cmd tea.Cmd
	if layout.ShowDescription {
		// The hints footer owns part of the content height; the viewport gets
		// the remainder so the two together exactly fill the panel.
		descHeight := PanelContentHeight(layout.DescriptionHeight) - m.hintsFooterRows()
		if descHeight < 4 {
			descHeight = 4
		}
		m.description, cmd = m.description.Update(tea.WindowSizeMsg{
			Width:  DescriptionContentWidth(layout.DescriptionWidth),
			Height: descHeight,
		})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	m.editor, cmd = m.editor.Update(tea.WindowSizeMsg{
		Width:  EditorContentWidth(layout.EditorWidth),
		Height: PanelContentHeight(layout.EditorHeight),
	})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.console, cmd = m.console.Update(tea.WindowSizeMsg{
		Width:  EditorContentWidth(layout.ConsoleWidth),
		Height: PanelContentHeight(layout.ConsoleHeight),
	})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.synced = m.layoutState()
	return *m, tea.Batch(cmds...)
}

// syncViewport is the render-time fallback for the same propagation. It is a
// no-op when Update already handled the resize, which is the common path.
func (m Model) syncViewport() Model {
	if m.synced == m.layoutState() {
		return m
	}
	synced, _ := m.applyViewportSize()
	return synced
}

// SetDimensions updates the workspace bounds.
func (m *Model) SetDimensions(w, h int) {
	if m.width == w && m.height == h && m.synced == m.layoutState() {
		return
	}
	m.width = w
	m.height = h
	m.hoverEdge = 0
	*m, _ = m.applyViewportSize()
}

// Problem returns the active problem.
func (m Model) Problem() *model.Problem {
	return m.problem
}

// Code returns the current content of the editor buffer.
func (m Model) Code() string {
	return m.editor.Content()
}

// Focus returns the currently focused pane.
func (m Model) CurrentFocus() Focus {
	return m.focus
}

// SetLanguage changes the active programming language.
func (m *Model) SetLanguage(lang string) {
	norm := normalizeLanguage(lang)
	m.language = norm
	m.langPickerOpen = false
	m.editor.SetLanguage(norm)
	m.console.SetLanguage(norm)
	if m.problem != nil {
		tmpl := ""
		if m.problem.Templates != nil {
			if t, ok := m.problem.Templates[norm]; ok {
				tmpl = t
			} else if t, ok := m.problem.Templates[lang]; ok {
				tmpl = t
			}
		}
		m.editor.SetContent(tmpl)
		m.console.SetContext(m.problem, tmpl, norm)
	}
}

// SwitchLanguage changes language without losing the outgoing buffer. It saves
// the current language's code, installs the target template, and reports the
// target language's saved buffer (if any) back through the root model.
func (m *Model) SwitchLanguage(lang string) tea.Cmd {
	target := normalizeLanguage(lang)
	if target == "" || target == m.language {
		return nil
	}
	oldLanguage := m.language
	oldCode := m.editor.Content()
	problemID := ""
	if m.problem != nil {
		problemID = m.problem.ID
	}
	store := m.storage
	m.SetLanguage(target)
	m.focus = FocusEditor
	m.syncEditorFocus()

	return func() tea.Msg {
		msg := LanguageSwitchedMsg{Language: target, ProblemID: problemID}
		if store == nil || problemID == "" {
			return msg
		}
		if oldCode != "" {
			_ = store.SaveCodeBuffer(problemID, oldLanguage, oldCode)
		}
		code, err := store.LoadCodeBuffer(problemID, target)
		if err == nil && code != "" {
			msg.Content = code
			msg.HasBuffer = true
		}
		return msg
	}
}

// ResetCode restores the current language template and clears the persisted
// working buffer for that language.
func (m *Model) ResetCode() tea.Cmd {
	if m.problem == nil {
		return nil
	}
	tmpl := ""
	if m.problem.Templates != nil {
		if value, ok := m.problem.Templates[m.language]; ok {
			tmpl = value
		}
	}
	m.editor.SetContent(tmpl)
	m.console.SetContext(m.problem, tmpl, m.language)
	m.focus = FocusEditor
	m.syncEditorFocus()
	store := m.storage
	problemID := m.problem.ID
	language := m.language
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		_ = store.SaveCodeBuffer(problemID, language, "")
		return nil
	}
}

// ShowCoach focuses the console on the Socratic Coach tab.
func (m *Model) ShowCoach() {
	m.focus = FocusConsole
	m.console.SwitchToTab(TabSocraticCoach)
	m.syncEditorFocus()
}

// ShowHint focuses the console on the local Hint tab.
func (m *Model) ShowHint() {
	m.focus = FocusConsole
	m.console.SwitchToTab(TabHint)
	m.syncEditorFocus()
}

// Languages returns the selectable language list for the active problem.
func (m Model) Languages() []string {
	return m.languageOptions()
}

// Cursor returns the editor cursor position as zero-based row and column.
func (m Model) Cursor() (int, int) {
	return m.editor.Line(), m.editor.Column()
}

// ConsoleResult returns the latest test result held by the console.
func (m Model) ConsoleResult() *model.ExecutionResult {
	return m.console.Result()
}

// SetEditorOptions applies the Settings editor preferences to the live editor.
func (m *Model) SetEditorOptions(opts EditorOptions) {
	m.editor.SetOptions(opts)
	m.formatOnSave = opts.FormatOnSave
}

// SetTheme repaints the workspace and every panel with a new palette.
func (m *Model) SetTheme(t WorkspaceTheme) {
	m.theme = t
	m.description.SetTheme(t)
	m.editor.SetTheme(t)
	m.console.SetTheme(t)
}

// RunTests triggers test execution programmatically.
func (m *Model) RunTests() tea.Cmd {
	m.isRunning = true
	m.attempts++
	m.console.SetRunning(true)
	m.console.CancelStream()
	return m.runTestsCmd()
}

// CycleFocus moves keyboard focus forward: Description -> Editor -> Console.
func (m *Model) CycleFocus() {
	m.focus = Focus((int(m.focus) + 1) % 3)
	m.syncEditorFocus()
}

// CycleTab advances the console tab.
func (m *Model) CycleTab() {
	m.console.SwitchTab()
}

// RequestSave is the user-initiated save path. When format-on-save is enabled
// for Python or Go, it dispatches the formatter first and remembers that a
// save is pending; CodeFormattedMsg applies the formatted buffer and then
// writes it. Languages without a formatter save immediately.
func (m *Model) RequestSave() tea.Cmd {
	if m.formatOnSave && (m.language == "python" || m.language == "go") {
		if cmd := m.FormatCode(); cmd != nil {
			m.pendingFormatSave = true
			return cmd
		}
	}
	return m.SaveCode()
}

// SaveCode marks the editor as saving and dispatches the write command. The
// saving flag is only set when a command actually exists, otherwise a nil
// problem or storage would leave the header stuck on [saving...].
func (m *Model) SaveCode() tea.Cmd {
	cmd := m.saveCodeCmd()
	if cmd == nil {
		return nil
	}
	m.editor.SetSaving(true)
	return cmd
}

// FormatCode dispatches a language-aware beautify pass for the current buffer.
func (m *Model) FormatCode() tea.Cmd {
	if m.problem == nil {
		return nil
	}
	pid := m.problem.ID
	lang := m.language
	code := m.editor.Content()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		formatted, err := runner.FormatCode(ctx, lang, code)
		return CodeFormattedMsg{ProblemID: pid, Language: lang, Code: formatted, Err: err}
	}
}

// Language returns the currently selected programming language.
func (m Model) Language() string {
	return m.language
}

func (m Model) languageOptions() []string {
	seen := make(map[string]bool)
	var raw []string
	raw = append(raw, "python", "go")
	if m.problem != nil && m.problem.Templates != nil {
		for k := range m.problem.Templates {
			raw = append(raw, k)
		}
	}
	if m.language != "" {
		raw = append(raw, m.language)
	}

	for _, l := range raw {
		norm := normalizeLanguage(l)
		if norm != "" {
			seen[norm] = true
		}
	}

	preferred := []string{"python", "go", "cpp", "rust", "javascript", "typescript"}
	var result []string
	used := make(map[string]bool)
	for _, p := range preferred {
		if seen[p] {
			result = append(result, p)
			used[p] = true
		}
	}

	var leftovers []string
	for k := range seen {
		if !used[k] {
			leftovers = append(leftovers, k)
		}
	}
	sort.Strings(leftovers)
	result = append(result, leftovers...)
	return result
}

func (m *Model) openLanguagePicker() {
	m.focus = FocusEditor
	m.syncEditorFocus()
	m.langPickerOptions = m.languageOptions()
	m.langPickerIndex = 0
	normLang := normalizeLanguage(m.language)
	for i, opt := range m.langPickerOptions {
		if opt == normLang {
			m.langPickerIndex = i
			break
		}
	}
	m.langPickerOpen = true
}

func (m *Model) selectLanguagePicker() (Model, tea.Cmd) {
	m.langPickerOpen = false
	var lang string
	if m.langPickerIndex >= 0 && m.langPickerIndex < len(m.langPickerOptions) {
		lang = m.langPickerOptions[m.langPickerIndex]
	} else if len(m.langPickerOptions) > 0 {
		lang = m.langPickerOptions[0]
	} else {
		lang = m.language
	}
	m.SetLanguage(lang)
	m.focus = FocusEditor
	m.syncEditorFocus()
	return *m, tea.Batch(
		func() tea.Msg { return LanguageChangedMsg{Language: lang} },
		m.loadCodeBufferCmd(),
	)
}

// formatIconHit maps a click on the editor's top border to the beautify icon
// embedded in the border metadata.
func (m Model) formatIconHit(x, y int, layout Layout) bool {
	if y != 0 {
		return false
	}
	iconW := lipgloss.Width(formatIcon)
	if iconW <= 0 || layout.EditorWidth < 44 {
		return false
	}
	// The meta ends at width-4; the closing " ─╮" occupies the last three cells.
	startX := layout.EditorX + layout.EditorWidth - 3 - iconW
	return x >= startX && x < startX+iconW
}

func (m Model) languageMenuOptionAt(x, y int, layout Layout) (int, bool) {
	centerX := layout.EditorX
	contentW := EditorContentWidth(layout.EditorWidth)
	startX := centerX + 1
	if x < startX || x >= startX+contentW {
		return 0, false
	}
	// Content begins below the top border and its one-row padding.
	firstOptionY := 2 + languageMenuHeaderRows
	idx := y - firstOptionY
	if idx >= 0 && idx < len(m.langPickerOptions) {
		return idx, true
	}
	return 0, false
}
