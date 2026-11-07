package workspace

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"

	"openleet/internal/model"
)

// DescriptionModel handles the left-panel scrollable problem statement viewport.
type DescriptionModel struct {
	viewport viewport.Model
	problem  *model.Problem
	theme    WorkspaceTheme
	width    int
	height   int
	focused  bool
}

// NewDescriptionModel constructs an initialized DescriptionModel.
func NewDescriptionModel(problem *model.Problem, theme WorkspaceTheme, width, height int) DescriptionModel {
	innerW := width
	innerH := height - 1 // reserve one row for the scroll indicator
	if innerW < 10 {
		innerW = 10
	}
	if innerH < 4 {
		innerH = 4
	}

	vp := viewport.New(innerW, innerH)
	dm := DescriptionModel{
		viewport: vp,
		problem:  problem,
		theme:    theme,
		width:    width,
		height:   height,
	}
	dm.rebuildContent()
	return dm
}

// Update processes viewport scrolling and resizing.
func (m DescriptionModel) Update(msg tea.Msg) (DescriptionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		innerW := msg.Width
		innerH := msg.Height - 1 // reserve one row for the scroll indicator
		if innerW < 10 {
			innerW = 10
		}
		if innerH < 4 {
			innerH = 4
		}
		m.viewport.Width = innerW
		m.viewport.Height = innerH
		m.rebuildContent()
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View returns the rendered description panel content.
func (m DescriptionModel) View() string {
	scrollLabel := m.scrollLabel()
	scrollBar := lipgloss.NewStyle().
		Foreground(m.theme.Subtext).
		Align(lipgloss.Right).
		Width(m.viewport.Width).
		Render(scrollLabel)

	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), scrollBar)
}

// SetProblem updates the active problem and re-renders the markdown description.
func (m *DescriptionModel) SetProblem(p *model.Problem) {
	m.problem = p
	m.viewport.GotoTop()
	m.rebuildContent()
}

func (m *DescriptionModel) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.focused = focused
	m.rebuildContent()
}

// SetTheme updates the active workspace theme and re-renders the markdown description.
func (m *DescriptionModel) SetTheme(t WorkspaceTheme) {
	m.theme = t
	m.rebuildContent()
}

// IsFocused reports whether the description pane currently has focus.
func (m DescriptionModel) IsFocused() bool {
	return m.focused
}

func (m *DescriptionModel) rebuildContent() {
	if m.problem == nil {
		m.viewport.SetContent("No problem loaded.")
		return
	}

	p := m.problem
	innerW := m.viewport.Width
	if innerW < 10 {
		innerW = 10
	}

	// 1. Title
	titleText := fmt.Sprintf("#%s  %s", p.ID, p.Title)
	title := lipgloss.NewStyle().
		Width(innerW).
		MaxWidth(innerW).
		Bold(true).
		Foreground(m.theme.Text).
		Render(titleText)

	// 2. Difficulty badge and acceptance on one row
	var diffBadge string
	switch p.Difficulty {
	case model.DifficultyEasy:
		diffBadge = lipgloss.NewStyle().Bold(true).Foreground(m.theme.Success).Render("● EASY")
	case model.DifficultyMedium:
		diffBadge = lipgloss.NewStyle().Bold(true).Foreground(m.theme.Warning).Render("● MEDIUM")
	case model.DifficultyHard:
		diffBadge = lipgloss.NewStyle().Bold(true).Foreground(m.theme.Danger).Render("● HARD")
	default:
		diffBadge = lipgloss.NewStyle().Foreground(m.theme.Muted).Render("● " + string(p.Difficulty))
	}
	acceptance := fmt.Sprintf("Acceptance: %.1f%%", p.AcceptanceRate)
	metaLine := fmt.Sprintf("%s   %s", diffBadge, lipgloss.NewStyle().Foreground(m.theme.Muted).Render(acceptance))

	// 3. One-line topics/companies metadata
	var metaParts []string
	if len(p.Topics) > 0 {
		metaParts = append(metaParts,
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Topics: ")+
				lipgloss.NewStyle().Foreground(m.theme.Text).Render(strings.Join(p.Topics, ", ")),
		)
	}
	if len(p.Companies) > 0 {
		metaParts = append(metaParts,
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Companies: ")+
				lipgloss.NewStyle().Foreground(m.theme.Text).Render(strings.Join(p.Companies, ", ")),
		)
	}
	metaText := strings.Join(metaParts, lipgloss.NewStyle().Foreground(m.theme.Border).Render("  ·  "))
	if metaText != "" {
		metaText = xansi.Truncate(metaText, innerW, "")
	}

	divider := lipgloss.NewStyle().
		Foreground(m.theme.Border).
		Render(strings.Repeat("─", innerW))

	// 4. Render Markdown via Glamour
	cleanedDesc := stripDuplicateDescriptionHeading(p.Description, p.Title)
	renderedMarkdown := renderMarkdown(cleanedDesc, innerW, m.theme)

	sections := []string{
		title,
		metaLine,
	}
	if metaText != "" {
		sections = append(sections, metaText)
	}
	sections = append(sections, divider, renderedMarkdown)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	m.viewport.SetContent(content)
}

func renderMarkdown(raw string, width int, wt WorkspaceTheme) string {
	if width < 10 {
		width = 10
	}
	cfg := markdownStyleConfig(wt)

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(cfg),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return raw
	}
	out, err := r.Render(raw)
	if err != nil {
		return raw
	}
	return strings.TrimRight(out, "\n")
}

func markdownStyleConfig(wt WorkspaceTheme) ansi.StyleConfig {
	cfg := styles.DarkStyleConfig
	if wt.Light {
		cfg = styles.LightStyleConfig
	}

	cfg.Document.Color = stringPtr(colorHex(wt.Text))

	cfg.H1.Prefix = ""
	cfg.H2.Prefix = ""
	cfg.H3.Prefix = ""
	cfg.H4.Prefix = ""

	primaryHex := stringPtr(colorHex(wt.Primary))
	cfg.H1.Color = primaryHex
	cfg.H1.BackgroundColor = nil
	cfg.H2.Color = primaryHex
	cfg.H3.Color = primaryHex
	cfg.H4.Color = primaryHex
	cfg.H5.Color = primaryHex
	cfg.H6.Color = primaryHex

	cfg.Heading.Color = primaryHex

	cfg.Code.Color = stringPtr(colorHex(wt.Secondary))
	cfg.Code.BackgroundColor = stringPtr(colorHex(wt.Surface))

	cfg.CodeBlock.Color = stringPtr(colorHex(wt.Text))
	cfg.CodeBlock.BackgroundColor = stringPtr(colorHex(wt.Surface))

	if cfg.CodeBlock.Chroma != nil {
		chroma := *cfg.CodeBlock.Chroma
		chroma.Text.Color = stringPtr(colorHex(wt.Text))
		chroma.Comment.Color = stringPtr(colorHex(wt.Muted))
		chroma.CommentPreproc.Color = stringPtr(colorHex(wt.Secondary))
		chroma.Keyword.Color = stringPtr(colorHex(wt.SyntaxKeyword))
		chroma.KeywordReserved.Color = stringPtr(colorHex(wt.SyntaxKeyword))
		chroma.KeywordNamespace.Color = stringPtr(colorHex(wt.SyntaxFunction))
		chroma.KeywordType.Color = stringPtr(colorHex(wt.SyntaxType))
		chroma.Operator.Color = stringPtr(colorHex(wt.SyntaxOperator))
		chroma.Punctuation.Color = stringPtr(colorHex(wt.SyntaxPunctuation))
		chroma.NameBuiltin.Color = stringPtr(colorHex(wt.SyntaxBuiltin))
		chroma.NameClass.Color = stringPtr(colorHex(wt.SyntaxType))
		chroma.NameFunction.Color = stringPtr(colorHex(wt.SyntaxFunction))
		chroma.NameDecorator.Color = stringPtr(colorHex(wt.SyntaxFunction))
		chroma.LiteralNumber.Color = stringPtr(colorHex(wt.SyntaxNumber))
		chroma.LiteralString.Color = stringPtr(colorHex(wt.SyntaxString))
		chroma.LiteralStringEscape.Color = stringPtr(colorHex(wt.SyntaxString))
		chroma.GenericDeleted.Color = stringPtr(colorHex(wt.Danger))
		chroma.GenericInserted.Color = stringPtr(colorHex(wt.Success))
		chroma.GenericSubheading.Color = stringPtr(colorHex(wt.Secondary))
		chroma.Error.Color = stringPtr(colorHex(wt.Text))
		chroma.Error.BackgroundColor = stringPtr(colorHex(wt.Danger))
		chroma.Background.BackgroundColor = stringPtr(colorHex(wt.Surface))
		cfg.CodeBlock.Chroma = &chroma
	}

	cfg.Link.Color = stringPtr(colorHex(wt.Secondary))
	cfg.LinkText.Color = stringPtr(colorHex(wt.Secondary))
	cfg.Image.Color = stringPtr(colorHex(wt.Secondary))
	cfg.ImageText.Color = stringPtr(colorHex(wt.Muted))
	cfg.HorizontalRule.Color = stringPtr(colorHex(wt.Muted))

	return cfg
}

func colorHex(c lipgloss.AdaptiveColor) string {
	if c.Dark != "" {
		return c.Dark
	}
	return c.Light
}

func stringPtr(s string) *string {
	return &s
}

func (m DescriptionModel) scrollLabel() string {
	total := strings.Count(m.viewport.View(), "\n") + 1
	vpHeight := m.viewport.Height
	yOffset := m.viewport.YOffset

	if total <= vpHeight {
		return ""
	}
	if yOffset == 0 {
		return "scroll: top"
	}
	if yOffset+vpHeight >= total {
		return "scroll: bottom"
	}
	pct := (yOffset * 100) / max(1, total-vpHeight)
	return fmt.Sprintf("scroll: %d%%", pct)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stripDuplicateDescriptionHeading(raw, title string) string {
	normalizedRaw := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalizedRaw, "\n")
	firstIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			firstIdx = i
			break
		}
	}
	if firstIdx == -1 {
		return raw
	}
	firstLine := strings.TrimSpace(lines[firstIdx])
	if !strings.HasPrefix(firstLine, "#") {
		return raw
	}
	normHeading := normalizeHeading(firstLine)
	normTitle := normalizeHeading(title)
	if normTitle == "" || (!strings.EqualFold(normHeading, normTitle) && !strings.Contains(normHeading, normTitle)) {
		return raw
	}

	nextIdx := firstIdx + 1
	for nextIdx < len(lines) && strings.TrimSpace(lines[nextIdx]) == "" {
		nextIdx++
	}
	return strings.Join(lines[nextIdx:], "\n")
}

func normalizeHeading(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
