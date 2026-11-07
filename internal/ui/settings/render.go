package settings

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/theme"
	"openleet/internal/ui/uikit"
)

const (
	sidebarWidth = 22

	// The canvas badge is a bordered block three rows tall. It is split into
	// real grid rows before the canvas is composed so it can never push the
	// status bar out of the frame.
	canvasBadgeHeight = 3

	// Geometry used by both the Appearance theme grid and its mouse hit test.
	themeCardHeight = 4 // top border, swatch, name, bottom border
	themeCardGap    = 1

	// Provider cards are border + heading + blank + three rows + border.
	providerCardHeight = 7
)

// canvasWidth is the usable width right of the sidebar rail.
func (m Model) canvasWidth() int {
	if m.width < 100 {
		return m.width
	}
	return m.width - sidebarWidth - 1
}

// canvasOriginX is the screen column where renderCanvas starts painting.
func (m Model) canvasOriginX() int {
	if m.width < 100 {
		return 0
	}
	return sidebarWidth + 1
}

// sectionWidth matches renderSection(width - 2) in renderCanvas.
func (m Model) sectionWidth() int { return m.canvasWidth() - 2 }

// themeGridLayout derives the column count and card width from the section
// width. The renderer and themeAt both call it so pointer targets cannot drift.
func themeGridLayout(sectionWidth int) (cols, cardW int) {
	gridWidth := sectionWidth - 4
	cols = 3
	if gridWidth < 74 {
		cols = 2
	}
	if gridWidth < 24 {
		cols = 1
	}
	cardW = (gridWidth - (cols-1)*themeCardGap) / cols
	if cardW < 12 {
		cardW = 12
	}
	return cols, cardW
}

// View renders the complete Settings screen on an exact terminal grid. The
// AppModel chrome owns the outer breadcrumb/footer bands; settings renders its
// body plus its own status row.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Initializing settings..."
	}
	status := m.renderStatusBar()
	bodyHeight := m.height - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body string
	if m.width < 100 {
		body = m.renderNarrow(bodyHeight)
	} else {
		body = m.renderWide(bodyHeight)
	}

	frame := lipgloss.JoinVertical(lipgloss.Left, body, status)
	if m.integration != nil {
		frame = m.renderIntegrationOverlay(frame)
	} else if m.auth != nil {
		frame = m.renderAuthOverlay(frame)
	} else {
		frame = m.overlayConfirm(frame)
	}
	return uikit.PadFrame(uikit.PaintBackground(frame, m.pal.Background), m.width, m.height)
}

func (m Model) renderWide(height int) string {
	side := strings.Split(m.renderSidebar(height), "\n")
	canvas := strings.Split(m.renderCanvas(m.width-sidebarWidth-1, height), "\n")
	rail := lipgloss.NewStyle().Foreground(m.pal.Border).Background(m.pal.Background).Render("│")
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		left, right := "", ""
		if i < len(side) {
			left = side[i]
		}
		if i < len(canvas) {
			right = canvas[i]
		}
		rows[i] = uikit.FitExact(left, sidebarWidth) + rail + uikit.FitExact(right, m.width-sidebarWidth-1)
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderNarrow(height int) string {
	tabs := m.renderNarrowTabs()
	canvasHeight := height - 1
	if canvasHeight < 1 {
		canvasHeight = 1
	}
	canvas := m.renderCanvas(m.width, canvasHeight)
	return tabs + "\n" + canvas
}

func (m Model) renderNarrowTabs() string {
	labels := make([]string, 0, len(m.sections))
	for _, section := range m.sections {
		label := section.Short
		if section.ID == m.ActiveSection() {
			label = "[" + label + "]"
		}
		labels = append(labels, label)
	}
	line := strings.Join(labels, " ")
	return lipgloss.NewStyle().
		Foreground(m.pal.Muted).
		Background(m.pal.Surface).
		Render(uikit.Fit(line, m.width))
}

func (m Model) renderSidebar(height int) string {
	lines := make([]string, 0, height)
	search := m.search.View()
	if strings.TrimSpace(search) == "" {
		search = lipgloss.NewStyle().Foreground(m.pal.Muted).Render("/ search settings")
	}
	lines = append(lines, uikit.Fit(" "+uikit.Truncate(search, sidebarWidth-1, "…"), sidebarWidth))
	lines = append(lines, lipgloss.NewStyle().Background(m.pal.Background).Render(strings.Repeat(" ", sidebarWidth)))

	rowHeight := 3
	if height < 38 {
		rowHeight = 2
	}
	matches := make(map[SectionID]bool)
	if m.query != "" {
		for _, field := range Search(m.query) {
			matches[field.Section] = true
		}
	}

	for _, section := range m.sections {
		if len(lines)+rowHeight > height {
			break
		}
		active := section.ID == m.ActiveSection()
		lines = append(lines, m.sidebarRow(section, active, matches[section.ID], m.dirtySections[section.ID], rowHeight))
		if len(lines) < height {
			lines = append(lines, lipgloss.NewStyle().Background(m.pal.Background).Render(strings.Repeat(" ", sidebarWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, lipgloss.NewStyle().Background(m.pal.Background).Render(strings.Repeat(" ", sidebarWidth)))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) sidebarRow(section Section, active, match, dirty bool, height int) string {
	bg := m.pal.Background
	fg := m.pal.Muted
	if active {
		bg = m.pal.CardHighlight
		fg = m.pal.Text
	}
	marker := "  "
	if active {
		marker = lipgloss.NewStyle().Foreground(m.pal.Accent).Background(bg).Render("▌") + " "
	}
	label := section.Label
	if match && !active {
		label += " ·"
	}
	text := marker + lipgloss.NewStyle().Bold(active).Foreground(fg).Background(bg).Render(label)
	if dirty {
		text += lipgloss.NewStyle().Foreground(m.pal.Warning).Background(bg).Render(" ●")
	}
	text = uikit.FitExact(text, sidebarWidth)
	block := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(height).
		MaxHeight(height).
		Background(bg).
		Align(lipgloss.Left, lipgloss.Center).
		Render(text)
	return block
}

func (m Model) renderCanvas(width, height int) string {
	if width <= 0 || height <= 0 {
		return strings.Repeat(" ", max(0, width))
	}

	title := strings.ToUpper(m.sectionLabel(m.ActiveSection()))
	badge := lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		Foreground(m.pal.Accent).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.pal.Accent).
		Padding(0, 1).
		Render("[ SETTINGS · " + title + " ]")

	lines := make([]string, 0, height)
	for _, badgeLine := range strings.Split(badge, "\n") {
		if len(lines) >= height {
			break
		}
		lines = append(lines, uikit.FitExact(badgeLine, width))
	}
	if len(lines) < height {
		lines = append(lines, "")
	}

	content := strings.Split(m.sectionContent(), "\n")
	for _, line := range content {
		if len(lines) >= height {
			break
		}
		lines = append(lines, uikit.FitExact(" "+line, width))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// renderSectionFor paints only the selected tab. It must never consult any
// other section.
func (m Model) renderSectionFor(id SectionID, fields []Field, width int) string {
	switch id {
	case SectionAppearance:
		return m.renderAppearance(fields, width)
	case SectionAI:
		return m.renderAI(fields, width)
	case SectionKeys:
		return m.renderKeybindings(width)
	case SectionAccount:
		return m.renderAccount(width)
	case SectionData:
		return m.renderData(fields, width)
	case SectionAbout:
		return m.renderAbout(width)
	}
	return m.renderFieldCards(fields, width)
}

func (m Model) renderSearchResults(width int) string {
	results := Search(m.query)
	if len(results) == 0 {
		return m.renderCard("No matches", []string{
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("Try a section, card, or setting name."),
		}, width, false)
	}
	rows := make([]string, 0, len(results))
	for _, field := range results {
		section := m.sectionLabel(field.Section)
		left := lipgloss.NewStyle().Foreground(m.pal.Text).Render(uikit.Truncate(field.Label, 34, "…"))
		right := lipgloss.NewStyle().Foreground(m.pal.Muted).Render(section)
		rows = append(rows, uikit.JoinLeftRight(width-4, left, right))
	}
	return m.renderCard("Search results · enter jumps to first match", rows, width, true)
}

func (m Model) renderFieldCards(fields []Field, width int) string {
	if len(fields) == 0 {
		return lipgloss.NewStyle().Foreground(m.pal.Muted).Render("No settings in this section.")
	}
	var blocks []string
	for start := 0; start < len(fields); {
		card := fields[start].Card
		end := start + 1
		for end < len(fields) && fields[end].Card == card {
			end++
		}
		danger := strings.EqualFold(card, "Danger zone")
		blocks = append(blocks, m.renderFieldCardStyled(card, fields[start:end], width, danger))
		start = end
	}
	return strings.Join(blocks, "\n\n")
}

func (m Model) renderFieldCardStyled(title string, fields []Field, width int, danger bool) string {
	selectedInCard := false
	rows := make([]string, 0, len(fields)+2)
	for _, field := range fields {
		selected := m.isSelected(field)
		if selected {
			selectedInCard = true
		}
		rows = append(rows, m.renderFieldRow(field, width-4, selected))
		if selected && field.Help != "" {
			rows = append(rows, m.renderHelp(field.Help, width-4))
		}
	}
	if danger {
		return m.renderDangerCard(title, rows, width)
	}
	return m.renderCard(title, rows, width, selectedInCard)
}

func (m Model) isSelected(field Field) bool {
	if m.focus != FocusCanvas {
		return false
	}
	selected := m.selectedFieldValue()
	return selected != nil && selected.ID == field.ID
}

func (m Model) renderFieldRow(field Field, width int, selected bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(m.pal.Muted)
	valueStyle := lipgloss.NewStyle().Foreground(m.pal.Text)
	if selected {
		valueStyle = lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent)
	}
	label := labelStyle.Render(uikit.FitExact(uikit.Truncate(field.Label, 22, "…"), 22))
	display := m.fieldValue(field)
	if m.editing && m.editField != nil && m.editField.ID == field.ID {
		display = m.editInput.View()
	}
	value := valueStyle.Render(uikit.Truncate(display, max(1, width-23), "…"))
	line := label + " " + value
	return uikit.FitExact(line, width)
}

func (m Model) fieldValue(field Field) string {
	value := Value(m.cfg, field)
	switch field.Kind {
	case KindToggle:
		if value == "on" {
			return "[ ● ]"
		}
		return "[   ]"
	case KindSelect:
		return optionLabel(field, value) + "  ▾"
	case KindStepper:
		return "‹ " + value + " ›"
	case KindSecret:
		provider := strings.TrimPrefix(field.ID, "ai.provider.")
		provider = strings.TrimSuffix(provider, ".key")
		if masked := m.SecretMasked(provider); masked != "" {
			return masked
		}
		return "not set · enter to add"
	case KindButton:
		return "[ enter ]"
	case KindThemeGrid:
		return m.currentTheme().Label
	case KindAccent:
		return value
	case KindText:
		if value == "" {
			return field.Placeholder
		}
		return value
	}
	return value
}

func (m Model) renderHelp(text string, width int) string {
	return lipgloss.NewStyle().Foreground(m.pal.Muted).
		Render(uikit.Fit(strings.Repeat(" ", 23)+text, width))
}

func (m Model) renderCard(title string, rows []string, width int, focused bool) string {
	borderColor := m.pal.Border
	if focused {
		borderColor = m.pal.Accent
	}
	return m.renderCardBorder(title, rows, width, borderColor)
}

// renderDangerCard draws a destructive-action card with the error border,
// keeping the danger zone visually distinct even when it is not focused.
func (m Model) renderDangerCard(title string, rows []string, width int) string {
	return m.renderCardBorder(title, rows, width, m.pal.Danger)
}

func (m Model) renderCardBorder(title string, rows []string, width int, borderColor lipgloss.TerminalColor) string {
	if width < 8 {
		return ""
	}
	border := lipgloss.RoundedBorder()
	if Value(m.cfg, Field{ID: "appearance.border"}) == "single" {
		border = lipgloss.NormalBorder()
	}
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1)
	heading := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Subtext).Render(strings.ToUpper(title))
	body := append([]string{heading, ""}, rows...)
	return style.Width(width - 2).Render(strings.Join(body, "\n"))
}

func (m Model) renderAppearance(fields []Field, width int) string {
	var themeField *Field
	rest := make([]Field, 0, len(fields))
	for i := range fields {
		if fields[i].ID == "appearance.theme" {
			themeField = &fields[i]
			continue
		}
		rest = append(rest, fields[i])
	}
	if themeField == nil {
		return m.renderFieldCards(rest, width)
	}
	grid := m.renderThemeGrid(width)
	grid = m.renderCard("Theme preview", []string{grid}, width, m.isSelected(*themeField))
	restCards := m.renderFieldCards(rest, width)
	if restCards == "" {
		return grid
	}
	return grid + "\n\n" + restCards
}

func (m Model) renderThemeGrid(sectionWidth int) string {
	if len(m.themes) == 0 {
		return ""
	}
	cols, cardW := themeGridLayout(sectionWidth)
	var rows []string
	for start := 0; start < len(m.themes); start += cols {
		row := make([]string, 0, cols*2-1)
		for col := 0; col < cols; col++ {
			index := start + col
			if col > 0 {
				row = append(row, strings.Repeat(" ", themeCardGap))
			}
			if index >= len(m.themes) {
				row = append(row, strings.Repeat(" ", cardW))
				continue
			}
			row = append(row, m.renderThemeCard(m.themes[index], index == m.themeIndex, cardW))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderThemeCard(p theme.Palette, selected bool, width int) string {
	border := lipgloss.RoundedBorder()
	borderColor := m.pal.Border
	if selected {
		borderColor = m.pal.Accent
	}
	swatch := func(c lipgloss.AdaptiveColor, char string) string {
		return lipgloss.NewStyle().Foreground(c).Render(char)
	}
	swatches := " " + swatch(p.Background, "█") + swatch(p.Text, "█") + swatch(p.Accent, "█") +
		swatch(p.Success, "█") + swatch(p.Danger, "█")
	name := uikit.Truncate(p.Label, width-4, "…")
	body := swatches + "\n" + lipgloss.NewStyle().Foreground(m.pal.Text).Render(name)
	return lipgloss.NewStyle().
		Width(width-2).
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(body)
}

// genericAIFields returns the AI schema rows rendered as field cards between
// the Integrations card and the provider stack.
func (m Model) genericAIFields(fields []Field) []Field {
	var generic []Field
	for _, field := range fields {
		if field.Kind != KindProviderList && field.ID != "ai.connect" {
			generic = append(generic, field)
		}
	}
	return generic
}

// renderAIPrelude renders the AI content above the provider stack: the
// Integrations card followed by the generic field cards.
func (m Model) renderAIPrelude(fields []Field) string {
	width := m.sectionWidth()
	var blocks []string
	for i := range fields {
		if fields[i].ID == "ai.connect" {
			blocks = append(blocks, m.renderCard("Integrations", []string{
				m.renderFieldRow(fields[i], width-4, m.isSelected(fields[i])),
			}, width, m.isSelected(fields[i])))
			break
		}
	}
	if generic := m.renderFieldCards(m.genericAIFields(fields), width); generic != "" {
		blocks = append(blocks, generic)
	}
	return strings.Join(blocks, "\n\n")
}

// renderAI paints the AI Models canvas in visual order: Integrations, the
// generic field cards, the provider stack, and finally the provider detail.
func (m Model) renderAI(fields []Field, width int) string {
	block := m.renderAIPrelude(fields)
	showProviders := false
	for _, field := range fields {
		if field.Kind == KindProviderList {
			showProviders = true
			break
		}
	}
	if !showProviders {
		return block
	}

	var cards []string
	for i, provider := range m.providers {
		status := m.providerStatus[provider.ID]
		discovered := status.Models
		if len(discovered) == 0 {
			discovered = CachedProviderModels(m.cfg, provider.ID)
		}
		connected := m.SecretMasked(provider.ID) != ""
		tested := status.Err == nil && status.Latency > 0
		dot := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("●")
		state := "not connected"
		if connected {
			// A saved key is not proof that generation works. Keep the warning
			// colour until a chat probe succeeds.
			dot = lipgloss.NewStyle().Foreground(m.pal.Warning).Render("●")
			state = "key saved (untested)"
		}
		if tested {
			dot = lipgloss.NewStyle().Foreground(m.pal.Success).Render("●")
			state = "connected in " + status.Latency.Round(time.Millisecond).String()
		}
		if status.Err != nil {
			dot = lipgloss.NewStyle().Foreground(m.pal.Danger).Render("●")
			state = "test failed"
		}
		keyState := "not set"
		if provider.Ollama {
			keyState = "not required (local)"
		}
		if masked := m.SecretMasked(provider.ID); masked != "" {
			keyState = masked
		}
		lines := []string{
			" " + dot + " " + lipgloss.NewStyle().Foreground(m.pal.Subtext).Render(state),
			"   " + lipgloss.NewStyle().Foreground(m.pal.Muted).Render("model: "+m.providerModel(provider)),
			"   " + lipgloss.NewStyle().Foreground(m.pal.Muted).Render("key: "+keyState),
		}
		if len(discovered) > 0 {
			lines = append(lines, "   "+lipgloss.NewStyle().Foreground(m.pal.Muted).Render(fmt.Sprintf("models: %d discovered", len(discovered))))
		}
		focused := i == m.providerIndex && (m.isSelected(Field{ID: "ai.providers"}) || m.providerDetail)
		cards = append(cards, m.renderCard(provider.Label, lines, width, focused))
	}
	block += "\n\n" + strings.Join(cards, "\n")

	if m.providerDetail {
		provider := m.currentProvider()
		detail := []string{
			m.renderFieldRow(Field{ID: "ai.provider." + provider.ID + ".key", Label: "API key", Kind: KindSecret}, width-4, true),
			m.renderFieldRow(Field{
				ID:          "ai.provider." + provider.ID + ".base_url",
				Label:       "Base URL",
				Kind:        KindText,
				Default:     m.providerBase(provider),
				Placeholder: provider.BaseURL,
				Validate:    validateURLOrEmpty,
			}, width-4, false),
			m.renderFieldRow(Field{
				ID:          "ai.provider." + provider.ID + ".model",
				Label:       "Model",
				Kind:        KindText,
				Default:     m.providerModel(provider),
				Placeholder: provider.DefaultModel,
			}, width-4, false),
		}
		discovered := m.providerStatus[provider.ID].Models
		if len(discovered) == 0 {
			discovered = CachedProviderModels(m.cfg, provider.ID)
		}
		if len(discovered) > 0 {
			limit := min(3, len(discovered))
			summary := strings.Join(discovered[:limit], ", ")
			if len(discovered) > 3 {
				summary += fmt.Sprintf(", +%d more", len(discovered)-3)
			}
			detail = append(detail, lipgloss.NewStyle().Foreground(m.pal.Muted).Render(uikit.Truncate(summary, width-4, "…")))
		}
		detail = append(detail, lipgloss.NewStyle().Foreground(m.pal.Muted).Render("enter edit key  b base URL  m model  t test  x remove"))
		block += "\n\n" + m.renderCard("Provider detail · "+provider.Label, detail, width, true)
	}

	return block
}

func (m Model) renderKeybindings(width int) string {
	selected := m.selectedFieldValue()
	keyFocused := m.focus == FocusCanvas && selected != nil && selected.ID == "keys.table"
	rows := make([]string, 0, len(m.keybindings)+3)
	for i, binding := range m.keybindings {
		key := bindingKey(m.cfg, binding)
		label := uikit.FitExact(uikit.Truncate(binding.Label, 24, "…"), 24)
		row := label + " " + key
		if i == m.captureIdx && keyFocused {
			row = lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent).Render(row)
		} else {
			row = lipgloss.NewStyle().Foreground(m.pal.Text).Render(row)
		}
		rows = append(rows, uikit.FitExact(row, width-4))
	}
	if m.captureKey {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.pal.Warning).Render("press the new key · esc cancels"))
	}
	if m.conflict != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.pal.Danger).Render(m.conflict))
	}
	return m.renderCard("Bindings", rows, width, keyFocused)
}

func (m Model) renderData(fields []Field, width int) string {
	path := m.cfg.DataDir()
	if path == "" {
		path = "unknown"
	}
	size := m.StorageSize()
	lines := []string{
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render("LOCATION"),
		lipgloss.NewStyle().Foreground(m.pal.Text).Render(uikit.Truncate(path, width-4, "…")),
		"",
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render(fmt.Sprintf("SIZE  %s", humanBytes(size))),
		"",
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render("EXPORT  openleet-progress.json / .csv"),
	}
	body := m.renderCard("Storage", lines, width, false)
	if cards := m.renderFieldCards(fields, width); cards != "" {
		body += "\n\n" + cards
	}
	return body
}

// humanBytes renders a compact, human-readable storage size.
func humanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGT"[exp])
}

func (m Model) renderAbout(width int) string {
	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		version = info.Main.Version
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render("VERSION"),
		lipgloss.NewStyle().Foreground(m.pal.Text).Render(version),
		"",
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render("CONFIG"),
		lipgloss.NewStyle().Foreground(m.pal.Text).Render(uikit.Truncate(m.cfg.ConfigDir(), width-4, "…")),
		"",
		lipgloss.NewStyle().Foreground(m.pal.Text).Render("[ Copy diagnostics ]  [ Documentation ]"),
	}
	about, _ := fieldByID("about.panel")
	return m.renderCard("About OpenLeet", lines, width, m.isSelected(about))
}

// StatusHints returns the key hints for the current Settings context. The
// shared app footer renders them so Settings never draws a second hint band.
func (m Model) StatusHints() []string {
	switch {
	case m.auth != nil:
		return []string{"tab field", "enter submit", "esc offline"}
	case m.integration != nil:
		return []string{"arrow select", "enter connect", "esc close"}
	case m.providerDetail:
		return []string{"enter edit", "t test", "esc close"}
	case m.captureKey:
		return []string{"press key", "esc cancel"}
	case m.editing:
		return []string{"enter save", "esc cancel", "ctrl+r reveal"}
	default:
		return []string{"j/k move", "ctrl+s save", "esc back"}
	}
}

// renderStatusBar keeps only the save state. Hints live in the app footer so
// the bottom band is identical everywhere and never repeats itself.
func (m Model) renderStatusBar() string {
	right := ""
	if m.dirty {
		right = lipgloss.NewStyle().Foreground(m.pal.Warning).Render("● unsaved")
	} else {
		right = lipgloss.NewStyle().Foreground(m.pal.Success).Render("● saved")
	}
	return lipgloss.NewStyle().Background(m.pal.Background).Render(
		uikit.JoinLeftRight(m.width, " ", right+" "),
	)
}

// overlayConfirm is kept small and centred over the canvas instead of using a
// blocking popup. Destructive actions must type the exact word.
func (m Model) overlayConfirm(base string) string {
	if m.confirm == nil {
		return base
	}
	panelW := min(56, m.width-4)
	panelW = max(panelW, 24)
	box := lipgloss.NewStyle().
		Width(panelW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.pal.Danger).
		Padding(1, 2).
		Render(
			lipgloss.NewStyle().Bold(true).Foreground(m.pal.Danger).Render(m.confirm.Title) + "\n\n" +
				lipgloss.NewStyle().Foreground(m.pal.Text).Render("Type "+m.confirm.Word+" to confirm:") + "\n" +
				m.confirm.Input.View() + "\n" +
				lipgloss.NewStyle().Foreground(m.pal.Muted).Render("enter confirm · esc cancel"),
		)
	return centerPanelOver(base, box, m.width)
}

// centerPanelOver draws box over base, clearing the rest of each covered row so
// the underlying card border cannot leak around a modal.
func centerPanelOver(base, box string, width int) string {
	lines := strings.Split(base, "\n")
	panel := strings.Split(box, "\n")
	start := max(0, (len(lines)-len(panel))/2)
	for i := 0; i < len(panel) && start+i < len(lines); i++ {
		pad := max(0, (width-lipgloss.Width(panel[i]))/2)
		tail := width - pad - lipgloss.Width(panel[i])
		if tail < 0 {
			tail = 0
		}
		lines[start+i] = strings.Repeat(" ", pad) + panel[i] + strings.Repeat(" ", tail)
	}
	return strings.Join(lines, "\n")
}

// updateMouse handles sidebar tabs, theme/provider cards, and field rows in
// the current section.
func (m Model) updateMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.integration != nil {
		return m, nil
	}
	if m.confirm != nil {
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.width >= 100 && msg.X < sidebarWidth {
		if index, ok := m.sidebarIndexAt(msg.Y); ok {
			m.OpenSection(m.sections[index].ID)
			return m, nil
		}
	}
	if index, ok := m.themeAt(msg.X, msg.Y); ok {
		m.themeIndex = index
		if fieldIndex := m.indexOfField("appearance.theme"); fieldIndex >= 0 {
			m.selectedField = fieldIndex
		}
		m.focus = FocusCanvas
		m.previewing = true
		SetValue(m.cfg, "appearance.theme", m.themes[index].Name)
		return m, func() tea.Msg { return PreviewThemeMsg{Name: m.themes[index].Name} }
	}
	if index, ok := m.providerAt(msg.X, msg.Y); ok {
		m.providerIndex = index
		m.providerDetail = true
		if fieldIndex := m.indexOfField("ai.providers"); fieldIndex >= 0 {
			m.selectedField = fieldIndex
		}
		m.focus = FocusCanvas
		return m, nil
	}
	if msg.X >= m.canvasOriginX()+1 {
		if index, ok := m.fieldRowAt(msg.Y); ok {
			m.focus = FocusCanvas
			m.selectedField = index
			field := m.selectedFieldValue()
			if field != nil && field.Kind == KindToggle && msg.X >= m.fieldValueColumnX() {
				return m.toggleSelected()
			}
			if field != nil && field.Kind == KindButton && msg.X >= m.fieldValueColumnX() {
				return m.activate()
			}
			return m, nil
		}
	}
	m.focus = FocusCanvas
	return m, nil
}

// fieldValueColumnX is the first cell of the value column inside a card.
func (m Model) fieldValueColumnX() int {
	return m.canvasOriginX() + 1 + 1 + 1 + 22 + 1
}

func (m Model) sidebarIndexAt(y int) (int, bool) {
	if y < 1 || y > m.height-2 {
		return 0, false
	}
	rowHeight := 3
	if m.height-2 < 38 {
		rowHeight = 2
	}
	// Screen row 0 is the breadcrumb; the sidebar starts at body row 1 with
	// the search field, then a spacer, then the category blocks.
	if y < 3 {
		return 0, false
	}
	index := (y - 3) / (rowHeight + 1)
	if index < 0 || index >= len(m.sections) {
		return 0, false
	}
	return index, true
}

func (m Model) sectionLabel(id SectionID) string {
	for _, section := range m.sections {
		if section.ID == id {
			return section.Label
		}
	}
	return string(id)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = theme.Default
