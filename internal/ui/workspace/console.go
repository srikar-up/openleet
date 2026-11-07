package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/coach"
	"openleet/internal/model"
	"openleet/internal/ui/uikit"
)

// CoachTurn records a turn in the Socratic dialog.
type CoachTurn struct {
	Role    string // "user", "coach", "system"
	Content string
	Action  coach.Action
}

// ConsoleModel manages the bottom-right tabbed test runner and Socratic coach console.
type ConsoleModel struct {
	activeTab ConsoleTab
	result    *model.ExecutionResult
	isRunning bool
	logs      []string
	viewport  viewport.Model
	theme     WorkspaceTheme
	width     int
	height    int
	focused   bool

	// Socratic Coach fields
	coachManager   *coach.Manager
	coachInput     textinput.Model
	coachHistory   []CoachTurn
	coachStreaming bool
	currentStream  string
	activeProvider string
	activeStreamID uint64
	hintAvailable  bool
	hintsUsed      int
	showDetails    bool
	slashPrefixes  []string
	problem        *model.Problem
	userCode       string
	language       string

	// Custom fixture editor.
	customInput    textinput.Model
	customExpected textinput.Model
	customField    int
	customRunning  bool
	customErr      string
	customResult   *model.ExecutionResult
}

// NewConsoleModel constructs the console component with Coach integration.
func NewConsoleModel(theme WorkspaceTheme, width, height int, coachMgr *coach.Manager) ConsoleModel {
	innerW := width
	innerH := height
	if innerW < 10 {
		innerW = 10
	}
	if innerH < 2 {
		innerH = 2
	}

	vp := viewport.New(innerW, innerH)

	ti := textinput.New()
	ti.Placeholder = "Ask coach... (/hint, /explain, /complexity, /diagnose)"
	ti.CharLimit = 250
	ti.Width = innerW - 4

	ci := textinput.New()
	ci.Prompt = ""
	ci.Placeholder = `{"nums":[2,7,11,15],"target":9}`
	ci.CharLimit = 500
	ci.Width = customInputWidth(innerW)
	ce := textinput.New()
	ce.Prompt = ""
	ce.Placeholder = `[0,1]`
	ce.CharLimit = 500
	ce.Width = customInputWidth(innerW)

	cm := ConsoleModel{
		activeTab:      TabTestResults,
		logs:           make([]string, 0),
		viewport:       vp,
		theme:          theme,
		width:          width,
		height:         height,
		coachManager:   coachMgr,
		coachInput:     ti,
		coachHistory:   make([]CoachTurn, 0),
		coachStreaming: false,
		hintAvailable:  false,
		slashPrefixes:  []string{"/hint", "/complexity", "/explain", "/diagnose"},
		customInput:    ci,
		customExpected: ce,
	}
	cm.applyInputStyles()
	cm.rebuildContent()
	return cm
}

// SetTheme applies a new workspace palette and repaints the cached console
// content. Without the rebuild, the viewport keeps ANSI colours from the
// previous theme until the user switches tabs.
func (m *ConsoleModel) SetTheme(t WorkspaceTheme) {
	m.theme = t
	m.applyInputStyles()
	m.rebuildContent()
}

// applyInputStyles keeps text inputs on palette colours instead of the
// terminal's default foreground/placeholder colours. The coach intro is
// unstyled terminal text, so this is required for readable light mode.
func (m *ConsoleModel) applyInputStyles() {
	text := lipgloss.NewStyle().Foreground(m.theme.Text)
	placeholder := lipgloss.NewStyle().Foreground(m.theme.Subtext)
	accent := lipgloss.NewStyle().Foreground(m.theme.Accent)

	for _, input := range []*textinput.Model{
		&m.coachInput,
		&m.customInput,
		&m.customExpected,
	} {
		input.TextStyle = text
		input.PlaceholderStyle = placeholder
		input.PromptStyle = accent
		input.Cursor.Style = accent
		input.Cursor.TextStyle = text
	}
}

// Update processes console events, coach messages, and resizes.
func (m ConsoleModel) Update(msg tea.Msg) (ConsoleModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		innerW := msg.Width
		if innerW < 10 {
			innerW = 10
		}
		m.viewport.Width = innerW
		m.viewport.Height = m.viewportHeight()
		m.coachInput.Width = innerW - 2
		m.customInput.Width = customInputWidth(innerW)
		m.customExpected.Width = customInputWidth(innerW)
		m.rebuildContent()

	case ConsoleTabChangedMsg:
		m.activeTab = msg.Tab
		if m.activeTab == TabSocraticCoach && m.focused {
			m.coachInput.Focus()
		} else {
			m.coachInput.Blur()
		}
		m.syncCustomFocus()
		m.viewport.Height = m.viewportHeight()
		m.rebuildContent()

	case coach.CoachStreamStartedMsg:
		m.coachStreaming = true
		m.activeProvider = msg.Provider
		m.activeStreamID = msg.StreamID
		m.currentStream = ""
		m.rebuildContent()
		if msg.Channels.Tokens == nil {
			// The event bus pump owns delivery; scheduling another reader here
			// would race the pump for tokens.
			return m, nil
		}
		return m, coach.WaitForNextToken(msg.Channels)

	case coach.CoachTokenMsg:
		m.currentStream += msg.Token
		m.rebuildContent()
		m.viewport.GotoBottom()
		if msg.Channels.Tokens == nil {
			return m, nil
		}
		return m, coach.WaitForNextToken(msg.Channels)

	case coach.CoachTokenBatchMsg:
		if len(msg.Tokens) == 0 {
			return m, nil
		}
		if msg.StreamID != 0 && m.activeStreamID != 0 && msg.StreamID != m.activeStreamID {
			return m, nil
		}
		m.currentStream += strings.Join(msg.Tokens, "")
		m.rebuildContent()
		m.viewport.GotoBottom()
		return m, nil

	case coach.CoachStreamDoneMsg:
		if !m.coachStreaming {
			return m, nil
		}
		if msg.StreamID != 0 && m.activeStreamID != 0 && msg.StreamID != m.activeStreamID {
			return m, nil
		}
		m.coachStreaming = false
		full := strings.TrimSpace(m.currentStream)
		if full != "" {
			m.coachHistory = append(m.coachHistory, CoachTurn{
				Role:    "coach",
				Content: full,
			})
		}
		m.currentStream = ""
		m.rebuildContent()
		m.viewport.GotoBottom()
		return m, nil

	case coach.CoachStreamErrorMsg:
		if !m.coachStreaming {
			// A user-initiated cancel surfaces as a context error; it must not
			// be reported as a provider failure.
			return m, nil
		}
		if msg.StreamID != 0 && m.activeStreamID != 0 && msg.StreamID != m.activeStreamID {
			return m, nil
		}
		m.coachStreaming = false
		m.coachHistory = append(m.coachHistory, CoachTurn{
			Role:    "system",
			Content: fmt.Sprintf("Coach unavailable: %v", msg.Err),
		})
		m.currentStream = ""
		m.rebuildContent()
		return m, nil

	case tea.KeyMsg:
		if m.activeTab == TabTestResults && (msg.String() == "enter" || msg.String() == "d") {
			if m.failedCount() > 0 {
				m.showDetails = !m.showDetails
				m.rebuildContent()
				return m, nil
			}
		}
		if m.activeTab == TabCustom {
			switch msg.String() {
			case "tab":
				m.customField = (m.customField + 1) % 2
				m.syncCustomFocus()
				m.rebuildContent()
				return m, nil
			case "enter":
				input, err := parseCustomJSON(m.customInput.Value(), "input")
				if err != nil {
					m.customErr = err.Error()
					m.customRunning = false
					m.rebuildContent()
					return m, nil
				}
				expected, err := parseCustomJSON(m.customExpected.Value(), "expected")
				if err != nil {
					m.customErr = err.Error()
					m.customRunning = false
					m.rebuildContent()
					return m, nil
				}
				m.customErr = ""
				m.customRunning = true
				m.customResult = nil
				m.rebuildContent()
				return m, func() tea.Msg {
					return CustomTestRequestedMsg{Input: input, Expected: expected}
				}
			}

			var inputCmd tea.Cmd
			if m.customField == 0 {
				m.customInput, inputCmd = m.customInput.Update(msg)
			} else {
				m.customExpected, inputCmd = m.customExpected.Update(msg)
			}
			m.rebuildContent()
			return m, inputCmd
		}
		if m.activeTab == TabSocraticCoach && m.coachInput.Focused() {
			switch msg.String() {
			case "enter":
				val := strings.TrimSpace(m.coachInput.Value())
				if val != "" && !m.coachStreaming {
					m.coachInput.Reset()
					action := coach.ActionQuestion
					query := val

					if strings.HasPrefix(val, "/hint") {
						action = coach.ActionHint
						m.hintsUsed++
						m.hintAvailable = false
					} else if strings.HasPrefix(val, "/explain") {
						action = coach.ActionExplain
					} else if strings.HasPrefix(val, "/complexity") {
						action = coach.ActionComplexity
					} else if strings.HasPrefix(val, "/diagnose") {
						action = coach.ActionDiagnose
						m.hintAvailable = false
					}

					m.coachHistory = append(m.coachHistory, CoachTurn{
						Role:    "user",
						Content: val,
						Action:  action,
					})
					m.rebuildContent()

					return m, m.dispatchCoachRequest(action, query)
				}
				return m, nil

			case "tab":
				// Autocomplete slash command
				val := m.coachInput.Value()
				if completed, ok := m.completeSlashPrefix(val); ok {
					m.coachInput.SetValue(completed)
					m.coachInput.CursorEnd()
					return m, nil
				}

			case "esc", "ctrl+c":
				if m.coachStreaming {
					m.CancelStream()
					return m, nil
				}
				m.coachInput.Blur()
				return m, nil
			}

			var tiCmd tea.Cmd
			m.coachInput, tiCmd = m.coachInput.Update(msg)
			return m, tiCmd
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ConsoleModel) completeSlashPrefix(val string) (string, bool) {
	if !strings.HasPrefix(val, "/") || strings.Contains(val, " ") {
		return "", false
	}
	var matches []string
	for _, p := range m.slashPrefixes {
		if strings.HasPrefix(p, val) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0] + " ", true
	}
	return "", false
}

func (m ConsoleModel) dispatchCoachRequest(action coach.Action, query string) tea.Cmd {
	if m.coachManager == nil {
		return func() tea.Msg {
			return coach.CoachStreamErrorMsg{Err: fmt.Errorf("coach manager not configured")}
		}
	}

	req := coach.CoachRequest{
		Action:       action,
		Problem:      m.problem,
		UserCode:     m.userCode,
		Language:     m.language,
		TestResults:  m.result,
		UserQuestion: query,
	}

	mgr := m.coachManager
	return func() tea.Msg {
		channels, providerName, streamID, err := mgr.Stream(context.Background(), req)
		if err != nil {
			return coach.CoachStreamErrorMsg{Err: err}
		}
		return coach.CoachStreamStartedMsg{
			Provider: providerName,
			Action:   action,
			Channels: channels,
			StreamID: streamID,
		}
	}
}

// CancelStream terminates any in-progress coach streaming.
func (m *ConsoleModel) CancelStream() {
	if m.coachStreaming {
		m.coachStreaming = false
		if m.currentStream != "" {
			m.coachHistory = append(m.coachHistory, CoachTurn{
				Role:    "coach",
				Content: m.currentStream + " [cancelled]",
			})
			m.currentStream = ""
		}
	}
	if m.coachManager != nil {
		m.coachManager.CancelActiveStream()
	}
	m.rebuildContent()
}

// SetHintAvailable updates the failure hint badge state.
func (m *ConsoleModel) SetHintAvailable(available bool) {
	m.hintAvailable = available
	m.rebuildContent()
}

// HintsUsed reports the number of hints requested from the coach.
func (m ConsoleModel) HintsUsed() int {
	return m.hintsUsed
}

// Result returns the latest test result held by the console.
func (m ConsoleModel) Result() *model.ExecutionResult {
	return m.result
}

// tabLabels returns the text rendered in each console tab.
func (m ConsoleModel) tabLabels() []string {
	return []string{"Test Results", "Socratic Coach", "Hint", "Custom"}
}

func customInputWidth(innerW int) int {
	width := innerW - 12
	if width < 10 {
		width = 10
	}
	return width
}

// tabGap is the spacer between tabs. The border title and hit-testing both use
// it so the painted title and the clickable cells cannot drift.
func (m ConsoleModel) tabGap() string {
	if m.viewport.Width < 52 {
		return " "
	}
	return "   "
}

type tabSpan struct {
	tab        ConsoleTab
	start, end int
}

// tabSpans reports the half-open cell ranges of the labels inside the console
// border title.
func (m ConsoleModel) tabSpans() []tabSpan {
	labels := m.tabLabels()
	spans := make([]tabSpan, 0, len(labels))
	x := 0
	gap := lipgloss.Width(m.tabGap())
	for i, label := range labels {
		if i > 0 {
			x += gap
		}
		width := lipgloss.Width(label)
		spans = append(spans, tabSpan{tab: ConsoleTab(i), start: x, end: x + width})
		x += width
	}
	return spans
}

// TabAt resolves a cell inside the console border title to a tab.
func (m ConsoleModel) TabAt(relX int) (ConsoleTab, bool) {
	for _, span := range m.tabSpans() {
		if relX >= span.start && relX < span.end {
			return span.tab, true
		}
	}
	return TabTestResults, false
}

// TabTitle renders the tab labels for the console pane's top border.
func (m ConsoleModel) TabTitle() string {
	activeColor := m.theme.Text
	if m.focused {
		activeColor = m.theme.Accent
	}
	labels := m.tabLabels()
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		if ConsoleTab(i) == m.activeTab {
			parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(activeColor).Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Muted).Render(label))
		}
	}
	return strings.Join(parts, m.tabGap())
}

// viewportHeight is the content height for the active tab. The coach tab
// reserves one row for its input prompt.
func (m ConsoleModel) viewportHeight() int {
	height := m.height
	if m.activeTab == TabSocraticCoach {
		height--
	}
	if height < 1 {
		height = 1
	}
	return height
}

// View renders only the console body. Tabs live in the pane border.
func (m ConsoleModel) View() string {
	m.viewport.Height = m.viewportHeight()
	body := m.viewport.View()
	if m.activeTab == TabSocraticCoach {
		inputLine := lipgloss.NewStyle().
			Foreground(m.theme.Accent).
			Render("> ") + m.coachInput.View()
		return lipgloss.JoinVertical(lipgloss.Left, body, inputLine)
	}
	return body
}

func (m *ConsoleModel) rebuildContent() {
	m.viewport.Height = m.viewportHeight()
	var content string
	switch m.activeTab {
	case TabTestResults:
		content = m.renderTestResults()
	case TabSocraticCoach:
		content = m.renderSocraticCoach()
	case TabHint:
		content = m.renderHint()
	case TabCustom:
		content = m.renderCustom()
	}
	m.viewport.SetContent(content)
}

func (m ConsoleModel) renderTestResults() string {
	width := m.viewport.Width
	if m.isRunning {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Accent).
			Render("⏳ Running tests against host runner...")
	}

	if m.result == nil {
		return lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render("Ready. Press ctrl+r to run the fixtures.")
	}

	res := m.result
	var lines []string
	success := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Success)
	failure := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Danger)
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	text := lipgloss.NewStyle().Foreground(m.theme.Text)

	// Summary line: green check + bold summary, runtime right-aligned.
	switch {
	case res.IsSuccessful():
		left := success.Render(fmt.Sprintf("✔ ALL TESTS PASSED (%d/%d)", res.PassedTests, res.TotalTests))
		right := muted.Render(res.Duration.Round(time.Millisecond).String())
		lines = append(lines, uikit.JoinLeftRight(width, left, right))
	case res.StdErr != "":
		lines = append(lines, failure.Render("✗ EXECUTION / SYNTAX ERROR"))
		for _, line := range strings.Split(strings.TrimRight(res.StdErr, "\n"), "\n") {
			lines = append(lines, uikit.FitExact(text.Render(line), width))
		}
	default:
		left := failure.Render(fmt.Sprintf("✗ FAILED (%d/%d)", res.PassedTests, res.TotalTests))
		right := muted.Render(res.Duration.Round(time.Millisecond).String())
		lines = append(lines, uikit.JoinLeftRight(width, left, right))
	}

	if len(res.Results) > 0 {
		lines = append(lines, "")
	}
	for _, tr := range res.Results {
		runtime := tr.Duration.Round(time.Microsecond).String()
		marker := "✔"
		if !tr.Passed {
			marker = "✗"
		}
		status := success
		if !tr.Passed {
			status = failure
		}
		left := status.Render(marker) + " " + text.Render("["+tr.TestCaseID+"]")
		lines = append(lines, uikit.JoinLeftRight(width, left, muted.Render(runtime)))

		if tr.Passed {
			continue
		}
		if m.showDetails {
			input := m.testInput(tr.TestCaseID)
			lines = append(lines,
				muted.Render("  ▾ input: ")+text.Render(formatJSONValue(input)),
				muted.Render("    expected: ")+success.Render(formatJSONValue(tr.Expected)),
				muted.Render("    got: ")+failure.Render(formatJSONValue(tr.Actual)),
			)
			if tr.Error != "" {
				for _, errLine := range strings.Split(strings.TrimRight(tr.Error, "\n"), "\n") {
					lines = append(lines, muted.Render("    "+uikit.Truncate(errLine, maxInt(width-4, 1), "")))
				}
			}
		} else {
			lines = append(lines, muted.Render("  ▸ press enter for input / expected / got"))
		}
	}

	for _, log := range m.logs {
		lines = append(lines, muted.Render("  "+log))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderHint is the local Hint tab. It turns the latest result into a focused
// next step without contacting the coach provider.
func (m ConsoleModel) renderHint() string {
	width := m.viewport.Width
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	text := lipgloss.NewStyle().Foreground(m.theme.Text)
	success := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Success)
	failure := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Danger)

	var lines []string
	lines = append(lines, muted.Render("HINT"))

	if m.result == nil {
		lines = append(lines,
			text.Render("Run the tests first (ctrl+r), then the Hint tab"),
			text.Render("will show the failing case and the next thing to try."),
		)
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	res := m.result
	if res.IsSuccessful() {
		lines = append(lines,
			success.Render("All fixtures pass."),
			text.Render("Check the edge cases, then review your time and space"),
			text.Render("complexity before moving to the next level."),
		)
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	for _, tr := range res.Results {
		if tr.Passed {
			continue
		}
		lines = append(lines,
			failure.Render(fmt.Sprintf("Failing case: %s", tr.TestCaseID)),
			muted.Render("input: ")+text.Render(formatJSONValue(m.testInput(tr.TestCaseID))),
			muted.Render("expected: ")+text.Render(formatJSONValue(tr.Expected)),
			muted.Render("got: ")+failure.Render(formatJSONValue(tr.Actual)),
		)
		if tr.Error != "" {
			for _, errLine := range strings.Split(strings.TrimRight(tr.Error, "\n"), "\n") {
				lines = append(lines, muted.Render(uikit.Truncate(errLine, width, "")))
			}
		}
		break
	}
	lines = append(lines, "", text.Render("Try the smallest failing input by hand, then"))
	lines = append(lines, text.Render("re-check the branch that produces the wrong value."))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func parseCustomJSON(raw, label string) (interface{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%s is empty", label)
	}
	var value interface{}
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return nil, fmt.Errorf("%s JSON: %v", label, err)
	}
	return value, nil
}

// renderCustom draws the user-authored fixture form and its latest result.
func (m ConsoleModel) renderCustom() string {
	width := m.viewport.Width
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	accent := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Accent)
	success := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Success)
	failure := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Danger)

	inputMarker := "  input  "
	expectedMarker := "  expected  "
	if m.customField == 0 {
		inputMarker = "▌ input  "
	} else {
		expectedMarker = "▌ expected  "
	}
	if m.focused {
		if m.customField == 0 {
			inputMarker = "▌ input  "
		} else {
			expectedMarker = "▌ expected  "
		}
	}

	inputLine := accent.Render(inputMarker) + m.customInput.View()
	expectedLine := accent.Render(expectedMarker) + m.customExpected.View()

	lines := []string{
		muted.Render("CUSTOM TEST"),
		muted.Render("Input JSON — object = named args, array = positional"),
		uikit.FitExact(inputLine, width),
		muted.Render("Expected JSON"),
		uikit.FitExact(expectedLine, width),
		muted.Render("tab field · enter run · ctrl+r all fixtures"),
	}

	if m.customRunning {
		lines = append(lines, "", accent.Render("Running custom case..."))
	}
	if m.customErr != "" {
		lines = append(lines, "", failure.Render(m.customErr))
	}
	if m.customResult != nil {
		res := m.customResult
		lines = append(lines, "")
		switch {
		case res.StdErr != "":
			lines = append(lines, failure.Render("✗ CUSTOM EXECUTION ERROR"))
			for _, errLine := range strings.Split(strings.TrimRight(res.StdErr, "\n"), "\n") {
				lines = append(lines, muted.Render(uikit.Truncate(errLine, width, "")))
			}
		case res.IsSuccessful():
			lines = append(lines, success.Render(fmt.Sprintf("✔ CUSTOM PASSED  %s", res.Duration.Round(time.Millisecond))))
		default:
			lines = append(lines, failure.Render(fmt.Sprintf("✗ CUSTOM FAILED  %s", res.Duration.Round(time.Millisecond))))
		}
		for _, tr := range res.Results {
			if tr.Passed {
				continue
			}
			lines = append(lines,
				muted.Render("expected: ")+success.Render(formatJSONValue(tr.Expected)),
				muted.Render("got: ")+failure.Render(formatJSONValue(tr.Actual)),
			)
			if tr.Error != "" {
				for _, errLine := range strings.Split(strings.TrimRight(tr.Error, "\n"), "\n") {
					lines = append(lines, muted.Render(uikit.Truncate(errLine, width, "")))
				}
			}
		}
	}

	for i, line := range lines {
		lines[i] = uikit.FitExact(line, width)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatJSONValue(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

// SetCustomRunning marks the custom fixture as executing.
func (m *ConsoleModel) SetCustomRunning(running bool) {
	m.customRunning = running
	m.rebuildContent()
}

// SetCustomResult installs the result of a custom fixture run.
func (m *ConsoleModel) SetCustomResult(res *model.ExecutionResult) {
	m.customRunning = false
	m.customErr = ""
	m.customResult = res
	m.rebuildContent()
}

// SetCustomError reports a request that could not be dispatched.
func (m *ConsoleModel) SetCustomError(message string) {
	m.customRunning = false
	m.customErr = message
	m.rebuildContent()
}

// syncCustomFocus moves the text cursor to the selected custom field.
func (m *ConsoleModel) syncCustomFocus() {
	if m.activeTab != TabCustom || !m.focused {
		m.customInput.Blur()
		m.customExpected.Blur()
		return
	}
	if m.customField == 0 {
		m.customInput.Focus()
		m.customExpected.Blur()
	} else {
		m.customExpected.Focus()
		m.customInput.Blur()
	}
}

func (m ConsoleModel) failedCount() int {
	if m.result == nil {
		return 0
	}
	count := 0
	for _, tr := range m.result.Results {
		if !tr.Passed {
			count++
		}
	}
	return count
}

func (m ConsoleModel) testInput(id string) interface{} {
	if m.problem == nil {
		return "?"
	}
	for _, tc := range m.problem.TestCases {
		if tc.ID == id {
			return tc.Input
		}
	}
	return "?"
}

func (m ConsoleModel) renderSocraticCoach() string {
	innerW := m.viewport.Width
	contentW := innerW - 10
	if contentW < 20 {
		contentW = 20
	}

	var lines []string

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Accent).
		Render("🧠 Socratic AI Coach")

	pills := lipgloss.NewStyle().Foreground(m.theme.Subtext).
		Render("Commands: [/hint]  [/complexity]  [/explain]  [/diagnose]")

	lines = append(lines, header, pills, "")

	text := lipgloss.NewStyle().Foreground(m.theme.Text)
	if len(m.coachHistory) == 0 && !m.coachStreaming {
		intro := "I will guide your problem-solving process without spoiling the solution."
		hint := "Type a command or ask a question below (e.g. '/hint', '/complexity', 'Why does my pointer cross bounds?')."
		wrappedIntro := wrapToWidth(intro, contentW)
		wrappedHint := wrapToWidth(hint, contentW)
		lines = append(lines, text.Render(wrappedIntro), "",
			lipgloss.NewStyle().Foreground(m.theme.Subtext).Render(wrappedHint))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	// Render conversation turns with word wrapping
	for _, turn := range m.coachHistory {
		wrapped := wrapToWidth(turn.Content, contentW)
		switch turn.Role {
		case "user":
			line := lipgloss.NewStyle().Foreground(m.theme.Primary).Render("🧑 You: ") + text.Render(wrapped)
			lines = append(lines, line)
		case "coach":
			line := lipgloss.NewStyle().Foreground(m.theme.Accent).Render("🧠 Coach: ") + text.Render(wrapped)
			lines = append(lines, line, "")
		case "system":
			line := lipgloss.NewStyle().Foreground(m.theme.Warning).Render("⚙ System: ") + text.Render(wrapped)
			lines = append(lines, line)
		}
	}

	// Render in-progress stream with live cursor. The cursor occupies cells, so
	// it must come out of the wrapping budget or the row bleeds into the
	// neighbouring pane.
	if m.coachStreaming {
		cursor := " ▋"
		streamWidth := contentW - lipgloss.Width(cursor)
		if streamWidth < 1 {
			streamWidth = 1
		}
		wrapped := wrapToWidth(m.currentStream, streamWidth)
		line := lipgloss.NewStyle().Foreground(m.theme.Accent).Render("🧠 Coach: ") + text.Render(wrapped) +
			lipgloss.NewStyle().Foreground(m.theme.Accent).Render(cursor)
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func wrapToWidth(text string, maxCols int) string {
	if maxCols <= 0 {
		return text
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		if ansi.StringWidth(paragraph) <= maxCols {
			result.WriteString(paragraph)
			result.WriteByte('\n')
			continue
		}
		words := strings.Fields(paragraph)
		lineWidth := 0
		for i, word := range words {
			wordWidth := ansi.StringWidth(word)
			if wordWidth > maxCols {
				// A single token longer than the pane (URL, minified code,
				// CJK run) must still fit on one row.
				word = ansi.Truncate(word, maxCols, "…")
				wordWidth = ansi.StringWidth(word)
			}
			if i > 0 && lineWidth > 0 {
				if lineWidth+1+wordWidth <= maxCols {
					result.WriteByte(' ')
					lineWidth++
				} else {
					result.WriteByte('\n')
					lineWidth = 0
				}
			}
			result.WriteString(word)
			lineWidth += wordWidth
		}
		result.WriteByte('\n')
	}
	return strings.TrimRight(result.String(), "\n")
}

// SetLanguage updates the active language on console.
func (m *ConsoleModel) SetLanguage(lang string) {
	m.language = lang
}

// SetContext updates problem and user code context for the coach.
func (m *ConsoleModel) SetContext(p *model.Problem, code, lang string) {
	m.problem = p
	m.userCode = code
	m.language = lang
}

// SetResult updates the test execution result.
func (m *ConsoleModel) SetResult(res *model.ExecutionResult) {
	m.result = res
	m.isRunning = false
	m.showDetails = res != nil && m.failedCount() > 0
	m.rebuildContent()
}

// SetRunning toggles the running progress banner.
func (m *ConsoleModel) SetRunning(running bool) {
	m.isRunning = running
	m.rebuildContent()
}

// AppendLog adds a log line to test output.
func (m *ConsoleModel) AppendLog(msg string) {
	m.logs = append(m.logs, msg)
	m.rebuildContent()
}

// SwitchTab cycles Test Results -> Socratic Coach -> Hint -> Custom.
func (m *ConsoleModel) SwitchTab() {
	next := m.activeTab + 1
	if next > TabCustom {
		next = TabTestResults
	}
	m.SwitchToTab(next)
}

// SwitchToTab sets the active console tab.
func (m *ConsoleModel) SwitchToTab(tab ConsoleTab) {
	if tab < TabTestResults || tab > TabCustom {
		tab = TabTestResults
	}
	m.activeTab = tab
	if m.activeTab == TabSocraticCoach && m.focused {
		m.coachInput.Focus()
	} else {
		m.coachInput.Blur()
	}
	m.syncCustomFocus()
	m.rebuildContent()
}

// SetFocused updates the console pane's focus state.
func (m *ConsoleModel) SetFocused(focused bool) {
	m.focused = focused
	if m.activeTab == TabSocraticCoach {
		if focused {
			m.coachInput.Focus()
		} else {
			m.coachInput.Blur()
		}
	}
	m.syncCustomFocus()
}

// IsFocused reports whether the console pane currently has focus.
func (m ConsoleModel) IsFocused() bool {
	return m.focused
}

// Reset clears the execution result.
func (m *ConsoleModel) Reset() {
	m.CancelStream()
	m.result = nil
	m.isRunning = false
	m.logs = m.logs[:0]
	m.activeTab = TabTestResults
	m.hintAvailable = false
	m.hintsUsed = 0
	m.showDetails = false
	m.coachHistory = m.coachHistory[:0]
	m.customInput.Reset()
	m.customExpected.Reset()
	m.customField = 0
	m.customRunning = false
	m.customErr = ""
	m.customResult = nil
	m.customInput.Blur()
	m.customExpected.Blur()
	m.rebuildContent()
}
