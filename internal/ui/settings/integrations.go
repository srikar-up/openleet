package settings

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

type pickerRow struct {
	Header      bool
	HeaderLabel string
	Provider    Provider
}

type integrationPicker struct {
	search   textinput.Model
	rows     []pickerRow
	selected int
	offset   int
}

func (m *integrationPicker) moveSelection(delta int) {
	n := len(m.rows)
	if n == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	steps := delta
	if steps < 0 {
		steps = -steps
	}
	for k := 0; k < steps; k++ {
		next := m.selected + step
		for next >= 0 && next < n && m.rows[next].Header {
			next += step
		}
		if next < 0 || next >= n {
			return // clamped at the end of the list
		}
		m.selected = next
	}
}

func (m *integrationPicker) ensureVisible(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+maxVisible {
		m.offset = m.selected - maxVisible + 1
	}
	maxOffset := len(m.rows) - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// OpenIntegrationPicker displays the search modal for connecting an integration.
func (m *Model) OpenIntegrationPicker() {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Search providers"
	input.CharLimit = 64
	input.Focus()

	m.integration = &integrationPicker{
		search: input,
	}
	m.refreshIntegrationRows()
	m.status = "Choose an integration for the Socratic Coach"
}

func (m Model) openIntegrationPicker() (Model, tea.Cmd) {
	m.OpenIntegrationPicker()
	return m, textinput.Blink
}

func (m *Model) refreshIntegrationRows() {
	if m.integration == nil {
		return
	}

	catalog := m.providers
	if len(catalog) == 0 {
		catalog = Providers()
	}
	byID := make(map[string]Provider, len(catalog))
	for _, p := range catalog {
		byID[strings.ToLower(p.ID)] = p
	}

	popularOrder := []string{"opencode", "openai", "groq", "anthropic", "gemini"}
	servicesOrder := []string{"nvidia", "openrouter", "ollama", "custom"}

	used := make(map[string]bool)
	var popularList []Provider
	for _, id := range popularOrder {
		if p, ok := byID[id]; ok {
			popularList = append(popularList, p)
			used[id] = true
		}
	}

	var servicesList []Provider
	for _, id := range servicesOrder {
		if p, ok := byID[id]; ok {
			servicesList = append(servicesList, p)
			used[id] = true
		}
	}
	for _, p := range catalog {
		id := strings.ToLower(p.ID)
		if !used[id] {
			servicesList = append(servicesList, p)
			used[id] = true
		}
	}

	query := strings.ToLower(strings.TrimSpace(m.integration.search.Value()))
	matches := func(p Provider) bool {
		if query == "" {
			return true
		}
		return strings.Contains(strings.ToLower(p.Label), query) ||
			strings.Contains(strings.ToLower(p.ID), query) ||
			strings.Contains(strings.ToLower(p.DefaultModel), query) ||
			strings.Contains(strings.ToLower(p.BaseURL), query)
	}

	var matchedPopular []Provider
	for _, p := range popularList {
		if matches(p) {
			matchedPopular = append(matchedPopular, p)
		}
	}

	var matchedServices []Provider
	for _, p := range servicesList {
		if matches(p) {
			matchedServices = append(matchedServices, p)
		}
	}

	var rows []pickerRow
	if len(matchedPopular) > 0 {
		rows = append(rows, pickerRow{Header: true, HeaderLabel: "POPULAR"})
		for _, p := range matchedPopular {
			rows = append(rows, pickerRow{Header: false, Provider: p})
		}
	}
	if len(matchedServices) > 0 {
		rows = append(rows, pickerRow{Header: true, HeaderLabel: "SERVICES"})
		for _, p := range matchedServices {
			rows = append(rows, pickerRow{Header: false, Provider: p})
		}
	}

	m.integration.rows = rows
	m.integration.selected = 0
	for i, r := range rows {
		if !r.Header {
			m.integration.selected = i
			break
		}
	}
	m.integration.offset = 0
}

func (m Model) integrationPanelSize() (int, int) {
	width := min(76, max(30, m.width-4))
	height := min(22, max(10, m.height-4))
	return width, height
}

func (m Model) integrationVisibleRows() int {
	_, panelH := m.integrationPanelSize()
	return max(3, panelH-6)
}

func (m Model) updateIntegration(msg tea.Msg) (Model, tea.Cmd) {
	if m.integration == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.integration = nil
			m.status = "Integration picker closed"
			return m, nil
		case "up", "ctrl+p":
			m.integration.moveSelection(-1)
			m.integration.ensureVisible(m.integrationVisibleRows())
			return m, nil
		case "down", "ctrl+n":
			m.integration.moveSelection(1)
			m.integration.ensureVisible(m.integrationVisibleRows())
			return m, nil
		case "enter":
			return m.chooseIntegration()
		default:
			var cmd tea.Cmd
			m.integration.search, cmd = m.integration.search.Update(msg)
			m.refreshIntegrationRows()
			return m, cmd
		}

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			m.integration.moveSelection(-1)
			m.integration.ensureVisible(m.integrationVisibleRows())
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.integration.moveSelection(1)
			m.integration.ensureVisible(m.integrationVisibleRows())
			return m, nil
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if idx, ok := m.integrationAt(msg.X, msg.Y); ok {
				m.integration.selected = idx
				m.integration.ensureVisible(m.integrationVisibleRows())
				return m.chooseIntegration()
			}
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.integration.search, cmd = m.integration.search.Update(msg)
		return m, cmd
	}
}

func (m Model) chooseIntegration() (Model, tea.Cmd) {
	if m.integration == nil {
		return m, nil
	}
	if m.integration.selected < 0 || m.integration.selected >= len(m.integration.rows) {
		return m, nil
	}
	row := m.integration.rows[m.integration.selected]
	if row.Header {
		return m, nil
	}
	provider := row.Provider

	SetValue(m.cfg, "ai.coach", provider.ID)
	m.markDirty(SectionAI)
	m.integration = nil

	for i, p := range m.providers {
		if p.ID == provider.ID {
			m.providerIndex = i
			break
		}
	}
	m.providerDetail = true
	if index := m.indexOfField("ai.providers"); index >= 0 {
		m.selectedField = index
	}
	m.status = fmt.Sprintf("Connect %s: enter API key", provider.Label)

	return m, func() tea.Msg {
		return AIProviderChangedMsg{ProviderID: provider.ID}
	}
}

func (m Model) renderIntegrationOverlay(base string) string {
	if m.integration == nil {
		return base
	}
	return centerPanelOver(base, m.renderIntegrationPicker(), m.width)
}

func (m Model) renderIntegrationPicker() string {
	if m.integration == nil {
		return ""
	}
	panelW, panelH := m.integrationPanelSize()
	contentW := panelW - 4
	visRows := m.integrationVisibleRows()

	contentRows := make([]string, 0, panelH-2)

	// 1. title row: uikit.JoinLeftRight(contentW, bold("CONNECT AN INTEGRATION"), muted("esc"))
	titleLeft := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Text).Render("CONNECT AN INTEGRATION")
	titleRight := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("esc")
	contentRows = append(contentRows, uikit.JoinLeftRight(contentW, titleLeft, titleRight))

	// 2. search row: muted "Search  " + a copy of the textinput with Width set to max(8, contentW-10)
	searchPrefix := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("/ ")
	searchCopy := m.integration.search
	searchCopy.Width = max(8, contentW-10)
	contentRows = append(contentRows, uikit.FitExact(searchPrefix+searchCopy.View(), contentW))

	// 3. blank
	contentRows = append(contentRows, strings.Repeat(" ", contentW))

	// 4. integrationVisibleRows() list rows from offset
	if len(m.integration.rows) == 0 {
		noMatch := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("No integrations match")
		contentRows = append(contentRows, uikit.FitExact(noMatch, contentW))
		for i := 1; i < visRows; i++ {
			contentRows = append(contentRows, strings.Repeat(" ", contentW))
		}
	} else {
		for i := 0; i < visRows; i++ {
			rowIdx := m.integration.offset + i
			if rowIdx >= len(m.integration.rows) {
				contentRows = append(contentRows, strings.Repeat(" ", contentW))
				continue
			}
			row := m.integration.rows[rowIdx]
			if row.Header {
				header := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Subtext).Render(strings.ToUpper(row.HeaderLabel))
				contentRows = append(contentRows, uikit.FitExact(header, contentW))
			} else {
				marker := " "
				if row.Provider.Ollama {
					marker = "•"
				} else if m.SecretMasked(row.Provider.ID) != "" {
					marker = "✓"
				}

				status := "not connected"
				if row.Provider.Ollama {
					status = "local"
				} else if m.SecretMasked(row.Provider.ID) != "" {
					status = "connected"
				}

				if rowIdx == m.integration.selected {
					line := uikit.JoinLeftRight(contentW, marker+" "+row.Provider.Label, status)
					selectedLine := lipgloss.NewStyle().
						Bold(true).
						Foreground(m.pal.Background).
						Background(m.pal.Accent).
						Render(uikit.FitExact(line, contentW))
					contentRows = append(contentRows, selectedLine)
				} else {
					checkStyle := lipgloss.NewStyle().Foreground(m.pal.Success)
					labelStyle := lipgloss.NewStyle().Foreground(m.pal.Text)
					statusStyle := lipgloss.NewStyle().Foreground(m.pal.Muted)

					left := checkStyle.Render(marker) + " " + labelStyle.Render(row.Provider.Label)
					right := statusStyle.Render(status)
					contentRows = append(contentRows, uikit.JoinLeftRight(contentW, left, right))
				}
			}
		}
	}

	// 5. hint row: "↑/↓ select  enter connect  esc close" (truncate/fit)
	hint := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("↑/↓ select  enter connect  esc close")
	contentRows = append(contentRows, uikit.FitExact(uikit.Fit(hint, contentW), contentW))

	for len(contentRows) < panelH-2 {
		contentRows = append(contentRows, strings.Repeat(" ", contentW))
	}
	if len(contentRows) > panelH-2 {
		contentRows = contentRows[:panelH-2]
	}

	style := lipgloss.NewStyle().
		Background(m.pal.Surface).
		Padding(1, 2).
		Width(panelW)
	return style.Render(strings.Join(contentRows, "\n"))
}

func (m Model) integrationAt(x, y int) (int, bool) {
	if m.integration == nil {
		return 0, false
	}
	panelW, panelH := m.integrationPanelSize()
	panelX := (m.width - panelW) / 2
	panelY := (m.height - panelH) / 2
	listY := panelY + 1 + 3 // padding(1) + title(1) + search(1) + blank(1)
	visRows := m.integrationVisibleRows()

	if x < panelX || x >= panelX+panelW {
		return 0, false
	}
	if y < listY || y >= listY+visRows {
		return 0, false
	}
	rowIdx := m.integration.offset + (y - listY)
	if rowIdx < 0 || rowIdx >= len(m.integration.rows) {
		return 0, false
	}
	if m.integration.rows[rowIdx].Header {
		return 0, false
	}
	return rowIdx, true
}
