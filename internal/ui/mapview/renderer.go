package mapview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/model"
)

// ViewData is the complete, UI-independent input needed to draw the roadmap.
type ViewData struct {
	Width        int
	Height       int
	Title        string
	Problems     []*model.Problem
	UserState    *model.UserState
	Selected     int
	ActiveFilter string
	Filters      []string
	LockedIDs    map[string]bool
	Command      string
	Status       string
}

// Renderer draws the roadmap and its inspector with the Figma palette.
type Renderer struct {
	Theme MapTheme
}

// NewRenderer constructs a roadmap renderer with the Figma palette.
func NewRenderer() Renderer {
	return Renderer{Theme: DefaultMapTheme()}
}

// Render composes the full frame inside the caller-provided terminal bounds.
func (r Renderer) Render(data ViewData) string {
	if data.Width <= 0 || data.Height <= 0 {
		return ""
	}
	if r.Theme.Primary.Light == "" && r.Theme.Primary.Dark == "" {
		r.Theme = DefaultMapTheme()
	}

	header := r.renderHeader(data)
	footer := r.renderFooter(data.Width)

	command := ""
	if data.Command != "" {
		command = r.renderCommandBar(data)
	}

	bodyHeight := data.Height - lipgloss.Height(header) - lipgloss.Height(footer) - lipgloss.Height(command)
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	body := r.renderBody(data, bodyHeight)

	parts := []string{header, body}
	if command != "" {
		parts = append(parts, command)
	}
	parts = append(parts, footer)

	frame := padFrame(lipgloss.JoinVertical(lipgloss.Left, parts...), data.Width, data.Height)
	return paintBackground(frame, r.Theme.Background)
}

// renderHeader draws the progress title row and the solved line. The app
// chrome owns the breadcrumb row and stats, so the roadmap starts at its
// content.
func (r Renderer) renderHeader(data ViewData) string {
	total, solved := progressCounts(data)
	percent := 0
	if total > 0 {
		percent = (solved * 100) / total
	}
	topic := topicName(data)
	title := lipgloss.NewStyle().Bold(true).Foreground(r.Theme.Primary).Render("OPENLEET/" + topic)

	// The progress track spans roughly 44% of the mockup slide width. It is
	// derived from the live terminal width so narrow screens still fit.
	barWidth := data.Width * 44 / 100
	if maxBar := data.Width - lipgloss.Width(title) - 4; barWidth > maxBar {
		barWidth = maxBar
	}
	if barWidth < 8 {
		barWidth = 8
	}
	filled := 0
	if total > 0 {
		filled = barWidth * solved / total
	}
	if filled > barWidth {
		filled = barWidth
	}
	progress := "[" +
		r.Theme.ProgressFill.Render(strings.Repeat("█", filled)) +
		r.Theme.ProgressEmpty.Render(strings.Repeat("|", barWidth-filled)) +
		"]"
	lineTwo := fitLine(title+" "+progress, data.Width)

	lineThree := fitLine(
		r.Theme.HeaderStyle.Render(fmt.Sprintf("%d/%d solved  .%d%%", solved, total, percent)),
		data.Width,
	)

	return lipgloss.JoinVertical(lipgloss.Left, lineTwo, lineThree)
}

func progressCounts(data ViewData) (total, solved int) {
	total = len(data.Problems)
	if data.UserState == nil {
		return total, 0
	}
	for _, problem := range data.Problems {
		if problem != nil && data.UserState.HasSolved(problem.ID) {
			solved++
		}
	}
	return total, solved
}

// topicName resolves the category currently being shown.
func topicName(data ViewData) string {
	if data.ActiveFilter != "" && !strings.EqualFold(data.ActiveFilter, "all") {
		return strings.ToUpper(data.ActiveFilter)
	}
	if i := strings.Index(data.Title, "/"); i >= 0 {
		name := strings.TrimSpace(data.Title[i+1:])
		if name != "" {
			return strings.ToUpper(name)
		}
	}
	return "DSA"
}

// renderBody draws the topic rail, the winding map and the inspector.
func (r Renderer) renderBody(data ViewData, height int) string {
	if height <= 0 {
		return ""
	}
	if data.Width < 50 {
		return fitPanel(r.renderCanvas(data, data.Width, height), data.Width, height)
	}

	layout := computeBodyLayout(data.Width)
	parts := []string{
		r.renderTopicRail(data, layout.RailWidth, height),
		r.renderScrollRail(height, topicIndex(data.ActiveFilter)),
		r.renderCanvas(data, layout.CanvasWidth, height),
	}
	if layout.ShowInspector {
		parts = append(parts,
			r.renderOrangeDivider(height),
			r.renderInspector(data, layout.InspectorWidth, height),
		)
	}
	return lipgloss.NewStyle().
		Width(data.Width).
		Height(height).
		MaxWidth(data.Width).
		MaxHeight(height).
		Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}

func topicIndex(filter string) int {
	for i, topic := range topicRailItems() {
		if topicActive(topic, filter) {
			return i
		}
	}
	return 0
}

// topicRailItems is the fixed category rail order from the design.
func topicRailItems() []string {
	return []string{"DSA", "CP", "DP", "ARRAY", "LINK LIST", "GRAPH"}
}

// renderTopicRail draws the stacked topic buttons.
func (r Renderer) renderTopicRail(data ViewData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{r.railBlank(width)}

	for _, topic := range topicRailItems() {
		if len(lines)+3 > height {
			break
		}
		active := topicActive(topic, data.ActiveFilter)
		bg := r.Theme.Card
		fg := r.Theme.Subtext
		if active {
			bg = r.Theme.CardHighlight
			fg = r.Theme.Text
		}

		var content string
		if active {
			marker := lipgloss.NewStyle().Foreground(r.Theme.Accent).Background(bg).Render("▌")
			label := lipgloss.NewStyle().Bold(true).Foreground(fg).Background(bg).Render(" " + topic)
			content = marker + label
		} else {
			content = lipgloss.NewStyle().Foreground(fg).Background(bg).Render("  " + topic)
		}
		if gap := width - lipgloss.Width(content); gap > 0 {
			content += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
		}

		// Exactly three rows, label vertically centered.
		button := lipgloss.NewStyle().
			Width(width).
			MaxWidth(width).
			Height(3).
			MaxHeight(3).
			Background(bg).
			Align(lipgloss.Left, lipgloss.Center).
			Render(content)
		lines = append(lines, button)
		if len(lines) < height {
			lines = append(lines, r.railBlank(width))
		}
	}

	for len(lines) < height {
		lines = append(lines, r.railBlank(width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// railBlank returns an app-background filler row for the topic rail.
func (r Renderer) railBlank(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Background(r.Theme.Background).
		Width(width).
		Render("")
}

func topicActive(topic, filter string) bool {
	normalized := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(filter)))
	if normalized == "" || normalized == "all" {
		return topic == "DSA"
	}
	key := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(topic))
	return key == normalized
}

// renderScrollRail draws the orange rail and its white thumb.
func (r Renderer) renderScrollRail(height, active int) string {
	if height <= 0 {
		return ""
	}
	thumb := 1 + active*4 + 1
	if thumb >= height {
		thumb = height - 1
	}
	if thumb < 0 {
		thumb = 0
	}
	lines := make([]string, height)
	for i := range lines {
		if i == thumb {
			lines[i] = lipgloss.NewStyle().
				Foreground(r.Theme.Text).
				Background(r.Theme.Background).
				Render("█")
			continue
		}
		lines[i] = lipgloss.NewStyle().
			Foreground(r.Theme.Accent).
			Background(r.Theme.Background).
			Render("┃")
	}
	return strings.Join(lines, "\n")
}

func (r Renderer) renderOrangeDivider(height int) string {
	if height <= 0 {
		return ""
	}
	line := lipgloss.NewStyle().
		Foreground(r.Theme.Accent).
		Background(r.Theme.Background).
		Render("┃")
	lines := make([]string, height)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// renderCanvas draws the roadmap badge and the card grid.
func (r Renderer) renderCanvas(data ViewData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	badgeStyle := r.Theme.RoadmapBadge
	badge := badgeStyle.Render("[ " + topicName(data) + "  roadmap ]")
	if lipgloss.Width(badge) > width {
		badge = lipgloss.NewStyle().MaxWidth(width).Render(badge)
	}

	badgeBlock := lipgloss.NewStyle().PaddingLeft(canvasIndent).Render(badge)
	canvasLayout := computeCanvasLayout(height)

	grid := computeGridLayout(width, canvasLayout.GridHeight)
	_, count := pageBounds(len(data.Problems), data.Selected)
	scroll := gridScrollOffset(len(data.Problems), data.Selected, grid.Columns, canvasLayout.GridHeight)
	contentHeight := gridContentHeight(count, grid.Columns)

	legend := ""
	if canvasLayout.LegendHeight > 0 {
		legend = r.renderLegend(width, scroll, contentHeight, canvasLayout.GridHeight)
	}

	canvas := lipgloss.JoinVertical(
		lipgloss.Left,
		badgeBlock,
		"",
		r.renderStaggeredGrid(data, width, canvasLayout.GridHeight),
		legend,
	)
	return paintBackground(fitPanel(canvas, width, height), r.Theme.Surface)
}

// renderInspector draws the problem details and the solve action.
func (r Renderer) renderInspector(data ViewData, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	problem := selectedProblem(data)
	if problem == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			PaddingLeft(1).
			Foreground(r.Theme.Muted).
			Render("Choose a level to preview its details.")
	}

	innerWidth := width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	lines := []string{
		r.Theme.InspectorHeading.Render(fmt.Sprintf("# %d. %s", data.Selected+1, problem.Title)),
		r.renderDifficulty(problem.Difficulty),
		lipgloss.NewStyle().Foreground(r.Theme.Text).Render(fmt.Sprintf("Acceptance: %.1f%%", problem.AcceptanceRate)),
		r.Theme.HeaderStyle.Render("Topics:"),
	}
	lines = append(lines, bulletLines(problem.Topics, r.Theme.Text)...)
	lines = append(lines, r.Theme.HeaderStyle.Render("Companies:"))
	lines = append(lines, bulletLines(problem.Companies, r.Theme.Text)...)
	lines = append(lines, r.Theme.HeaderStyle.Render("Description:"), protectedBlank)
	// renderDescription returns plain copy; give it an explicit foreground so
	// a light palette cannot inherit a light terminal foreground.
	description := r.renderDescription(problem, innerWidth)
	descriptionStyle := lipgloss.NewStyle().Foreground(r.Theme.Text)
	for i, line := range description {
		if line == "" {
			continue
		}
		description[i] = descriptionStyle.Render(line)
	}
	lines = append(lines, description...)

	contentHeight := height - 2
	if contentHeight < 0 {
		contentHeight = 0
	}
	if len(lines) > contentHeight {
		lines = compactLines(lines)
	}
	for i := range lines {
		if lines[i] == protectedBlank {
			lines[i] = ""
		}
	}
	content := fitLines(lines, innerWidth, contentHeight)

	button := r.renderSolveButton(problem, data, innerWidth)

	return lipgloss.NewStyle().
		PaddingLeft(1).
		Background(r.Theme.Background).
		Width(width).
		Height(height).
		MaxWidth(width).
		MaxHeight(height).
		Render(lipgloss.JoinVertical(lipgloss.Left, content, button))
}

// renderDescription converts the stored markdown into the flat inspector copy.
func (r Renderer) renderDescription(problem *model.Problem, width int) []string {
	source := strings.TrimSpace(problem.Description)
	if source == "" {
		return r.renderTestCaseFallback(problem, width)
	}

	var out []string
	inFence := false
	blank := false
	for _, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if line == "" {
			blank = true
			continue
		}
		if !inFence && strings.HasPrefix(line, "# ") {
			// The card number and title are rendered by the inspector itself.
			continue
		}
		if !inFence {
			if strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") {
				line = strings.TrimSpace(strings.TrimLeft(line, "#"))
			}
			if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "- ") {
				line = "- " + strings.TrimSpace(line[2:])
			}
		}
		line = stripMarkdown(line)
		if blank && len(out) > 0 {
			out = append(out, "")
		}
		blank = false
		out = append(out, wrapText(line, width)...)
	}
	if len(out) == 0 {
		return r.renderTestCaseFallback(problem, width)
	}
	return out
}

func (r Renderer) renderTestCaseFallback(problem *model.Problem, width int) []string {
	tests := problem.GetVisibleTestCases()
	if len(tests) == 0 {
		return []string{"No description available."}
	}
	var out []string
	for i, test := range tests {
		out = append(out, fmt.Sprintf("Example %d:", i+1))
		out = append(out, wrapText(fmt.Sprintf("Input: %v", test.Input), width)...)
		out = append(out, wrapText(fmt.Sprintf("Output: %v", test.Expected), width)...)
		out = append(out, "")
	}
	return out
}

func (r Renderer) renderSolveButton(problem *model.Problem, data ViewData, width int) string {
	label := "▶ SOLVE LEVEL (Enter / Click)"
	solved := data.UserState != nil && data.UserState.HasSolved(problem.ID)
	if solved {
		label = "✓ SOLVED · Enter to undo"
	}
	return r.Theme.SolveButton.
		Width(width).
		MaxWidth(width).
		Render(label)
}

// protectedBlank marks a separator that survives inspector compaction.
const protectedBlank = "\x00"

// compactLines drops blank separators so long problem copy still fits a short
// inspector. Exact mockup spacing is preserved whenever it fits.
func compactLines(lines []string) []string {
	compact := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		compact = append(compact, line)
	}
	return compact
}

func bulletLines(values []string, color lipgloss.AdaptiveColor) []string {
	style := lipgloss.NewStyle().Foreground(color)
	if len(values) == 0 {
		return []string{style.Render("- —")}
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, style.Render("- "+value))
	}
	return lines
}

func (r Renderer) renderDifficulty(difficulty model.Difficulty) string {
	label := strings.ToUpper(strings.TrimSpace(string(difficulty)))
	if label == "" {
		label = "EASY"
	}
	style := r.Theme.EasyBadge
	switch difficulty {
	case model.DifficultyEasy:
		style = r.Theme.MediumBadge
	case model.DifficultyHard:
		style = r.Theme.HardBadge
	}
	return style.Render("** " + label)
}

func selectedProblem(data ViewData) *model.Problem {
	if len(data.Problems) == 0 || data.Selected < 0 || data.Selected >= len(data.Problems) {
		return nil
	}
	return data.Problems[data.Selected]
}

// renderFooter is retired: the app-level bottom bar owns the hint row so the
// roadmap never competes with the shared chrome.
func (r Renderer) renderFooter(width int) string {
	return ""
}

func (r Renderer) renderCommandBar(data ViewData) string {
	content := r.Theme.PromptStyle.Render("> ") + r.Theme.PromptStyle.Render(data.Command)
	return fitLine(content, data.Width)
}
