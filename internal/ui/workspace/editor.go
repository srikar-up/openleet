package workspace

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/model"
	"openleet/internal/ui/uikit"
)

const languageMenuHeaderRows = 4

// formatIcon is the clickable beautify action shown at the end of the editor
// header. It is decorative to terminals that lack emoji support but still
// occupies a predictable cell range for mouse hit-testing.
const formatIcon = "✦"

// EditorOptions carries the user-configurable editor behaviors sourced from
// the Settings schema.
type EditorOptions struct {
	TabWidth        int
	ShowLineNumbers bool
	AutoIndent      bool
	BracketPairing  bool
	FormatOnSave    bool
}

// DefaultEditorOptions returns the schema defaults.
func DefaultEditorOptions() EditorOptions {
	return EditorOptions{
		TabWidth:        4,
		ShowLineNumbers: true,
		AutoIndent:      true,
		BracketPairing:  true,
		FormatOnSave:    true,
	}
}

// bracketCloser maps an opening bracket or quote to its matching closer.
func bracketCloser(r rune) (string, bool) {
	switch r {
	case '(':
		return ")", true
	case '[':
		return "]", true
	case '{':
		return "}", true
	case '"':
		return "\"", true
	case '\'':
		return "'", true
	}
	return "", false
}

// EditorModel manages the high-density syntax-highlighted code editor buffer.
type EditorModel struct {
	lines      []string // at least one line; "" is one empty line
	row, col   int      // col is a rune index into lines[row]
	scrollRow  int
	scrollCol  int // measured in terminal cells
	language   string
	theme      WorkspaceTheme
	width      int // full panel width, including the PanelStyle horizontal padding
	height     int // full panel height
	focused    bool
	isModified bool
	isSaving   bool
	version    int
	undo       []editorSnapshot
	redo       []editorSnapshot
	cache      *highlightCache

	tabWidth        int
	showLineNumbers bool
	autoIndent      bool
	bracketPairing  bool
}

type editorSnapshot struct {
	lines    []string
	row, col int
}

type highlightCache struct {
	version int
	lang    string
	lines   []codeLine
}

type codeLine []codeSpan

type codeSpan struct {
	Text string
	Type chroma.TokenType
}

type styledSpan struct {
	text  string
	style lipgloss.Style
}

// NewEditorModel constructs the code editor component.
func NewEditorModel(problem *model.Problem, language string, theme WorkspaceTheme, width, height int) EditorModel {
	normLang := normalizeLanguage(language)
	defaults := DefaultEditorOptions()
	m := EditorModel{
		lines:           []string{""},
		language:        normLang,
		theme:           theme,
		width:           width,
		height:          height,
		focused:         true,
		version:         1,
		cache:           &highlightCache{},
		tabWidth:        defaults.TabWidth,
		showLineNumbers: defaults.ShowLineNumbers,
		autoIndent:      defaults.AutoIndent,
		bracketPairing:  defaults.BracketPairing,
	}

	initialCode := ""
	if problem != nil && problem.Templates != nil {
		if tmpl, ok := problem.Templates[normLang]; ok && tmpl != "" {
			initialCode = tmpl
		} else if tmpl, ok := problem.Templates[language]; ok && tmpl != "" {
			initialCode = tmpl
		}
	}
	if initialCode != "" {
		m.SetContent(initialCode)
	} else {
		m.ensureCursorVisible()
	}

	return m
}

// Update processes editor events and resizes.
func (m EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		return m, nil

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *EditorModel) handleKey(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	keyStr := msg.String()

	switch {
	case msg.Type == tea.KeyCtrlZ || keyStr == "ctrl+z":
		m.performUndo()
		return *m, nil

	case msg.Type == tea.KeyCtrlY || keyStr == "ctrl+y":
		m.performRedo()
		return *m, nil

	case msg.Type == tea.KeyCtrlA || keyStr == "ctrl+a" || msg.Type == tea.KeyHome || keyStr == "home":
		m.col = 0
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyCtrlE || keyStr == "ctrl+e" || msg.Type == tea.KeyEnd || keyStr == "end":
		if len(m.lines) > 0 {
			m.col = len([]rune(m.lines[m.row]))
		}
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyCtrlHome || keyStr == "ctrl+home":
		m.row = 0
		m.col = 0
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyCtrlEnd || keyStr == "ctrl+end":
		if len(m.lines) > 0 {
			m.row = len(m.lines) - 1
			m.col = len([]rune(m.lines[m.row]))
		}
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyPgUp || keyStr == "pgup":
		h := m.ViewportHeight()
		m.row -= h
		if m.row < 0 {
			m.row = 0
		}
		lineLen := len([]rune(m.lines[m.row]))
		if m.col > lineLen {
			m.col = lineLen
		}
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyPgDown || keyStr == "pgdown":
		h := m.ViewportHeight()
		m.row += h
		if m.row >= len(m.lines) {
			m.row = len(m.lines) - 1
		}
		lineLen := len([]rune(m.lines[m.row]))
		if m.col > lineLen {
			m.col = lineLen
		}
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyUp || keyStr == "up":
		if m.row > 0 {
			m.row--
			lineLen := len([]rune(m.lines[m.row]))
			if m.col > lineLen {
				m.col = lineLen
			}
			m.ensureCursorVisible()
		}
		return *m, nil

	case msg.Type == tea.KeyDown || keyStr == "down":
		if m.row < len(m.lines)-1 {
			m.row++
			lineLen := len([]rune(m.lines[m.row]))
			if m.col > lineLen {
				m.col = lineLen
			}
			m.ensureCursorVisible()
		}
		return *m, nil

	case msg.Type == tea.KeyCtrlLeft || (msg.Alt && msg.Type == tea.KeyLeft) || keyStr == "ctrl+left" || keyStr == "alt+left":
		m.moveWordLeft()
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyCtrlRight || (msg.Alt && msg.Type == tea.KeyRight) || keyStr == "ctrl+right" || keyStr == "alt+right":
		m.moveWordRight()
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyLeft || keyStr == "left":
		if m.col > 0 {
			m.col--
		} else if m.row > 0 {
			m.row--
			m.col = len([]rune(m.lines[m.row]))
		}
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyRight || keyStr == "right":
		lineLen := len([]rune(m.lines[m.row]))
		if m.col < lineLen {
			m.col++
		} else if m.row < len(m.lines)-1 {
			m.row++
			m.col = 0
		}
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyCtrlK || keyStr == "ctrl+k":
		lineRunes := []rune(m.lines[m.row])
		if m.col < len(lineRunes) {
			m.pushUndo()
			m.lines[m.row] = string(lineRunes[:m.col])
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		} else if m.row < len(m.lines)-1 {
			m.pushUndo()
			m.lines[m.row] = m.lines[m.row] + m.lines[m.row+1]
			m.lines = append(m.lines[:m.row+1], m.lines[m.row+2:]...)
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		}
		return *m, nil

	case msg.Type == tea.KeyCtrlU || keyStr == "ctrl+u":
		if m.col > 0 {
			m.pushUndo()
			lineRunes := []rune(m.lines[m.row])
			m.lines[m.row] = string(lineRunes[m.col:])
			m.col = 0
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		}
		return *m, nil

	case msg.Type == tea.KeyBackspace || keyStr == "backspace":
		if m.col > 0 {
			m.pushUndo()
			lineRunes := []rune(m.lines[m.row])
			m.lines[m.row] = string(lineRunes[:m.col-1]) + string(lineRunes[m.col:])
			m.col--
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		} else if m.row > 0 {
			m.pushUndo()
			prevRunes := []rune(m.lines[m.row-1])
			prevLen := len(prevRunes)
			m.lines[m.row-1] = m.lines[m.row-1] + m.lines[m.row]
			m.lines = append(m.lines[:m.row], m.lines[m.row+1:]...)
			m.row--
			m.col = prevLen
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		}
		return *m, nil

	case msg.Type == tea.KeyDelete || keyStr == "delete":
		lineRunes := []rune(m.lines[m.row])
		if m.col < len(lineRunes) {
			m.pushUndo()
			m.lines[m.row] = string(lineRunes[:m.col]) + string(lineRunes[m.col+1:])
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		} else if m.row < len(m.lines)-1 {
			m.pushUndo()
			m.lines[m.row] = m.lines[m.row] + m.lines[m.row+1]
			m.lines = append(m.lines[:m.row+1], m.lines[m.row+2:]...)
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
		}
		return *m, nil

	case msg.Type == tea.KeyEnter || keyStr == "enter":
		m.pushUndo()
		lineRunes := []rune(m.lines[m.row])
		left := string(lineRunes[:m.col])
		right := string(lineRunes[m.col:])

		leadingSpaces := 0
		for _, r := range m.lines[m.row] {
			if r == ' ' {
				leadingSpaces++
			} else {
				break
			}
		}
		indent := ""
		if m.autoIndent {
			indent = strings.Repeat(" ", leadingSpaces)
			if m.language == "python" && strings.HasSuffix(strings.TrimRight(left, " \t"), ":") {
				indent += strings.Repeat(" ", m.tabWidth)
			}
		}

		trimmedRight := right
		if m.col <= leadingSpaces {
			trimmedRight = strings.TrimLeft(right, " ")
		}

		newLine := indent + trimmedRight
		m.lines[m.row] = left
		m.lines = append(m.lines[:m.row+1], append([]string{newLine}, m.lines[m.row+1:]...)...)
		m.row++
		m.col = len([]rune(indent))
		m.version++
		m.isModified = true
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyTab || keyStr == "tab":
		lineRunes := []rune(m.lines[m.row])
		visualCol := ansi.StringWidth(string(lineRunes[:m.col]))
		spaces := m.tabWidth - (visualCol % m.tabWidth)
		if spaces == 0 {
			spaces = m.tabWidth
		}
		spaceStr := strings.Repeat(" ", spaces)
		m.pushUndo()
		m.lines[m.row] = string(lineRunes[:m.col]) + spaceStr + string(lineRunes[m.col:])
		m.col += spaces
		m.version++
		m.isModified = true
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeySpace || keyStr == " ":
		m.pushUndo()
		lineRunes := []rune(m.lines[m.row])
		m.lines[m.row] = string(lineRunes[:m.col]) + " " + string(lineRunes[m.col:])
		m.col++
		m.version++
		m.isModified = true
		m.ensureCursorVisible()
		return *m, nil

	case msg.Type == tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return *m, nil
		}
		if len(msg.Runes) == 1 {
			m.pushUndo()
			r := msg.Runes[0]
			insert := string(r)
			if m.bracketPairing && !msg.Alt {
				if closer, ok := bracketCloser(r); ok {
					insert += closer
				}
			}
			lineRunes := []rune(m.lines[m.row])
			m.lines[m.row] = string(lineRunes[:m.col]) + insert + string(lineRunes[m.col:])
			m.col++
			m.version++
			m.isModified = true
			m.ensureCursorVisible()
			return *m, nil
		}

		pasteStr := string(msg.Runes)
		pasteStr = strings.ReplaceAll(pasteStr, "\r\n", "\n")
		pasteStr = strings.ReplaceAll(pasteStr, "\r", "\n")
		pasteStr = strings.ReplaceAll(pasteStr, "\t", strings.Repeat(" ", m.tabWidth))
		pasteLines := strings.Split(pasteStr, "\n")

		m.pushUndo()
		lineRunes := []rune(m.lines[m.row])
		prefix := string(lineRunes[:m.col])
		suffix := string(lineRunes[m.col:])

		if len(pasteLines) == 1 {
			m.lines[m.row] = prefix + pasteLines[0] + suffix
			m.col = len([]rune(prefix + pasteLines[0]))
		} else {
			newLines := make([]string, 0, len(m.lines)+len(pasteLines)-1)
			newLines = append(newLines, m.lines[:m.row]...)
			newLines = append(newLines, prefix+pasteLines[0])
			for i := 1; i < len(pasteLines)-1; i++ {
				newLines = append(newLines, pasteLines[i])
			}
			newLines = append(newLines, pasteLines[len(pasteLines)-1]+suffix)
			newLines = append(newLines, m.lines[m.row+1:]...)
			m.lines = newLines
			m.row = m.row + len(pasteLines) - 1
			m.col = len([]rune(pasteLines[len(pasteLines)-1]))
		}
		m.version++
		m.isModified = true
		m.ensureCursorVisible()
		return *m, nil
	}

	return *m, nil
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (m *EditorModel) moveWordLeft() {
	if m.col == 0 {
		if m.row > 0 {
			m.row--
			m.col = len([]rune(m.lines[m.row]))
		}
		return
	}
	lineRunes := []rune(m.lines[m.row])
	c := m.col
	for c > 0 && !isWordChar(lineRunes[c-1]) {
		c--
	}
	for c > 0 && isWordChar(lineRunes[c-1]) {
		c--
	}
	m.col = c
}

func (m *EditorModel) moveWordRight() {
	lineRunes := []rune(m.lines[m.row])
	if m.col >= len(lineRunes) {
		if m.row < len(m.lines)-1 {
			m.row++
			m.col = 0
		}
		return
	}
	c := m.col
	if isWordChar(lineRunes[c]) {
		for c < len(lineRunes) && isWordChar(lineRunes[c]) {
			c++
		}
	} else {
		for c < len(lineRunes) && !isWordChar(lineRunes[c]) {
			c++
		}
	}
	for c < len(lineRunes) && unicode.IsSpace(lineRunes[c]) {
		c++
	}
	m.col = c
}

func (m *EditorModel) pushUndo() {
	linesCopy := make([]string, len(m.lines))
	copy(linesCopy, m.lines)
	snap := editorSnapshot{
		lines: linesCopy,
		row:   m.row,
		col:   m.col,
	}
	m.undo = append(m.undo, snap)
	if len(m.undo) > 100 {
		m.undo = m.undo[len(m.undo)-100:]
	}
	m.redo = nil
}

func (m *EditorModel) performUndo() {
	if len(m.undo) == 0 {
		return
	}
	linesCopy := make([]string, len(m.lines))
	copy(linesCopy, m.lines)
	m.redo = append(m.redo, editorSnapshot{
		lines: linesCopy,
		row:   m.row,
		col:   m.col,
	})
	last := m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
	m.lines = last.lines
	m.row = last.row
	m.col = last.col
	m.version++
	m.isModified = true
	m.ensureCursorVisible()
}

func (m *EditorModel) performRedo() {
	if len(m.redo) == 0 {
		return
	}
	linesCopy := make([]string, len(m.lines))
	copy(linesCopy, m.lines)
	m.undo = append(m.undo, editorSnapshot{
		lines: linesCopy,
		row:   m.row,
		col:   m.col,
	})
	if len(m.undo) > 100 {
		m.undo = m.undo[len(m.undo)-100:]
	}
	last := m.redo[len(m.redo)-1]
	m.redo = m.redo[:len(m.redo)-1]
	m.lines = last.lines
	m.row = last.row
	m.col = last.col
	m.version++
	m.isModified = true
	m.ensureCursorVisible()
}

func countDigits(n int) int {
	if n <= 0 {
		return 1
	}
	count := 0
	for n > 0 {
		count++
		n /= 10
	}
	return count
}

func (m EditorModel) GutterWidth() int {
	if !m.showLineNumbers {
		return 0
	}
	digits := countDigits(len(m.lines))
	w := digits + 1
	if w < 2 {
		return 2
	}
	return w
}

func (m EditorModel) ViewportHeight() int {
	h := m.height - 1 // one row for the cursor position footer
	if h < 1 {
		return 1
	}
	return h
}

func (m EditorModel) ScrollRow() int {
	return m.scrollRow
}

func (m EditorModel) ScrollCol() int {
	return m.scrollCol
}

func (m EditorModel) LineCount() int {
	return len(m.lines)
}

func (m EditorModel) Line() int {
	return m.row
}

func (m EditorModel) Column() int {
	return m.col
}

func (m EditorModel) ColumnAtCell(row, cell int) int {
	if len(m.lines) == 0 {
		return 0
	}
	if row < 0 {
		row = 0
	} else if row >= len(m.lines) {
		row = len(m.lines) - 1
	}
	if cell <= 0 {
		return 0
	}
	runes := []rune(m.lines[row])
	curCell := 0
	for i, r := range runes {
		rw := ansi.StringWidth(string(r))
		if curCell+rw > cell {
			return i
		}
		curCell += rw
	}
	return len(runes)
}

func (m *EditorModel) ensureCursorVisible() {
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	if m.row < 0 {
		m.row = 0
	} else if m.row >= len(m.lines) {
		m.row = len(m.lines) - 1
	}

	lineRunes := []rune(m.lines[m.row])
	if m.col < 0 {
		m.col = 0
	} else if m.col > len(lineRunes) {
		m.col = len(lineRunes)
	}

	bodyH := m.ViewportHeight()
	innerW := InnerContentWidth(m.width, 1)
	if innerW < 10 {
		innerW = 10
	}
	codeW := innerW - m.GutterWidth()
	if codeW < 4 {
		codeW = 4
	}

	// Adjust scrollRow
	if m.row < m.scrollRow {
		m.scrollRow = m.row
	} else if m.row >= m.scrollRow+bodyH {
		m.scrollRow = m.row - bodyH + 1
	}
	maxScrollRow := len(m.lines) - bodyH
	if maxScrollRow < 0 {
		maxScrollRow = 0
	}
	if m.scrollRow > maxScrollRow {
		m.scrollRow = maxScrollRow
	}
	if m.scrollRow < 0 {
		m.scrollRow = 0
	}

	// Adjust scrollCol
	cursorCell := ansi.StringWidth(string(lineRunes[:m.col]))
	if cursorCell < m.scrollCol {
		m.scrollCol = cursorCell
	} else if cursorCell >= m.scrollCol+codeW {
		m.scrollCol = cursorCell - codeW + 1
	}
	if m.scrollCol < 0 {
		m.scrollCol = 0
	}
}

func (m *EditorModel) SetCursor(row, col int) {
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	if row < 0 {
		row = 0
	} else if row >= len(m.lines) {
		row = len(m.lines) - 1
	}
	m.row = row
	lineRunes := []rune(m.lines[m.row])
	if col < 0 {
		col = 0
	} else if col > len(lineRunes) {
		col = len(lineRunes)
	}
	m.col = col
	m.ensureCursorVisible()
}

func (m EditorModel) Content() string {
	return strings.Join(m.lines, "\n")
}

func (m *EditorModel) expandTabs(s string) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := m.tabWidth - (col % m.tabWidth)
			for i := 0; i < spaces; i++ {
				b.WriteByte(' ')
			}
			col += spaces
		} else {
			b.WriteRune(r)
			col += ansi.StringWidth(string(r))
		}
	}
	return b.String()
}

// SetOptions applies the user-configurable editor behaviors. TabWidth is
// clamped to the 2..8 schema range.
func (m *EditorModel) SetOptions(opts EditorOptions) {
	m.tabWidth = opts.TabWidth
	if m.tabWidth < 2 {
		m.tabWidth = 2
	}
	if m.tabWidth > 8 {
		m.tabWidth = 8
	}
	m.showLineNumbers = opts.ShowLineNumbers
	m.autoIndent = opts.AutoIndent
	m.bracketPairing = opts.BracketPairing
}

func (m *EditorModel) SetContent(content string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	var rawLines []string
	if content == "" {
		rawLines = []string{""}
	} else {
		rawLines = strings.Split(content, "\n")
	}

	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = m.expandTabs(l)
	}

	m.lines = lines
	m.row = 0
	m.col = 0
	m.scrollRow = 0
	m.scrollCol = 0
	m.undo = nil
	m.redo = nil
	m.version++
	m.isModified = false
	m.isSaving = false
	m.ensureCursorVisible()
}

func (m *EditorModel) SetLanguage(lang string) {
	m.language = normalizeLanguage(lang)
	m.version++
}

func (m *EditorModel) SetModified(mod bool) {
	m.isModified = mod
}

func (m *EditorModel) SetSaving(saving bool) {
	m.isSaving = saving
}

func (m *EditorModel) Focus() tea.Cmd {
	m.focused = true
	return nil
}

func (m *EditorModel) Blur() {
	m.focused = false
}

func (m EditorModel) IsFocused() bool {
	return m.focused
}

func (m *EditorModel) SetFocused(focused bool) {
	m.focused = focused
}

func (m *EditorModel) SetTheme(t WorkspaceTheme) {
	m.theme = t
}

func normalizeLanguage(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch l {
	case "py":
		return "python"
	case "golang":
		return "go"
	case "c++":
		return "cpp"
	case "js":
		return "javascript"
	case "ts":
		return "typescript"
	default:
		return l
	}
}

func languageLabel(lang string) string {
	norm := normalizeLanguage(lang)
	switch norm {
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "cpp":
		return "C++"
	case "rust":
		return "Rust"
	case "javascript":
		return "JavaScript"
	case "typescript":
		return "TypeScript"
	default:
		if len(norm) == 0 {
			return ""
		}
		runes := []rune(norm)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
}

func fileExtension(lang string) string {
	switch lang {
	case "python":
		return "py"
	case "go", "golang":
		return "go"
	case "cpp", "c++":
		return "cpp"
	case "rust":
		return "rs"
	case "javascript":
		return "js"
	case "typescript":
		return "ts"
	default:
		return "txt"
	}
}

func lexerForLanguage(lang string) chroma.Lexer {
	norm := normalizeLanguage(lang)
	lex := lexers.Get(norm)
	if lex != nil {
		return lex
	}
	return lexers.Fallback
}

func (m EditorModel) highlightLines() []codeLine {
	if m.cache != nil && m.cache.version == m.version && m.cache.lang == m.language && len(m.cache.lines) == len(m.lines) {
		return m.cache.lines
	}
	src := m.Content()
	lexer := lexerForLanguage(m.language)
	tokens, _ := chroma.Tokenise(lexer, nil, src)

	res := make([]codeLine, len(m.lines))
	lineIdx := 0
	for _, t := range tokens {
		parts := strings.Split(t.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lineIdx++
			}
			if part != "" && lineIdx < len(res) {
				res[lineIdx] = append(res[lineIdx], codeSpan{
					Text: part,
					Type: t.Type,
				})
			}
		}
	}
	for len(res) < len(m.lines) {
		res = append(res, codeLine{})
	}
	if len(res) > len(m.lines) {
		res = res[:len(m.lines)]
	}

	if m.cache != nil {
		m.cache.version = m.version
		m.cache.lang = m.language
		m.cache.lines = res
	}
	return res
}

func (m EditorModel) syntaxStyle(tt chroma.TokenType) lipgloss.Style {
	switch {
	case tt.InCategory(chroma.Comment):
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxComment)
	case tt.InCategory(chroma.Keyword):
		if tt.InSubCategory(chroma.KeywordType) || tt == chroma.KeywordType {
			return lipgloss.NewStyle().Foreground(m.theme.SyntaxType)
		}
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxKeyword)
	case tt.InSubCategory(chroma.LiteralString):
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxString)
	case tt.InSubCategory(chroma.LiteralNumber):
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxNumber)
	case tt.InSubCategory(chroma.NameFunction) ||
		tt == chroma.NameClass ||
		tt == chroma.NameNamespace ||
		tt == chroma.NameDecorator:
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxFunction)
	case tt.InSubCategory(chroma.NameBuiltin):
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxBuiltin)
	case tt.InCategory(chroma.Operator):
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxOperator)
	case tt.InCategory(chroma.Punctuation):
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxPunctuation)
	case tt.InCategory(chroma.Error) || tt == chroma.Error || tt == chroma.GenericError:
		return lipgloss.NewStyle().Foreground(m.theme.SyntaxError)
	default:
		return lipgloss.NewStyle().Foreground(m.theme.Text)
	}
}

func (m EditorModel) buildRowSpans(hlLines []codeLine, rowIdx int) []styledSpan {
	var spans []codeSpan
	if rowIdx >= 0 && rowIdx < len(hlLines) {
		spans = hlLines[rowIdx]
	}
	lineText := ""
	if rowIdx >= 0 && rowIdx < len(m.lines) {
		lineText = m.lines[rowIdx]
	}
	if len(spans) == 0 && len(lineText) > 0 {
		spans = []codeSpan{{Text: lineText, Type: chroma.Text}}
	}

	isCursorRow := m.focused && (rowIdx == m.row)
	if !isCursorRow {
		styled := make([]styledSpan, 0, len(spans))
		for _, s := range spans {
			if s.Text == "" {
				continue
			}
			styled = append(styled, styledSpan{
				text:  s.Text,
				style: m.syntaxStyle(s.Type),
			})
		}
		return styled
	}

	// Reverse video keeps the cursor opaque without emitting a background
	// sequence. uikit.PaintBackground restores the last background after every
	// reset, so an explicit cursor background would leak across the whole row.
	cursorStyle := lipgloss.NewStyle().Foreground(m.theme.Primary).Reverse(true)
	styled := make([]styledSpan, 0, len(spans)+2)
	totalRunes := 0
	cursorInserted := false

	for _, s := range spans {
		if s.Text == "" {
			continue
		}
		spanRunes := []rune(s.Text)
		spanLen := len(spanRunes)
		baseStyle := m.syntaxStyle(s.Type)

		if !cursorInserted && m.col >= totalRunes && m.col < totalRunes+spanLen {
			offset := m.col - totalRunes
			before := string(spanRunes[:offset])
			cursorGlyph := string(spanRunes[offset : offset+1])
			after := string(spanRunes[offset+1:])

			if before != "" {
				styled = append(styled, styledSpan{text: before, style: baseStyle})
			}
			styled = append(styled, styledSpan{text: cursorGlyph, style: cursorStyle})
			if after != "" {
				styled = append(styled, styledSpan{text: after, style: baseStyle})
			}
			cursorInserted = true
		} else {
			styled = append(styled, styledSpan{text: s.Text, style: baseStyle})
		}
		totalRunes += spanLen
	}

	if !cursorInserted {
		styled = append(styled, styledSpan{text: " ", style: cursorStyle})
	}

	return styled
}

func (m EditorModel) renderStyledSpans(spans []styledSpan, scrollCol, codeW int) string {
	if codeW <= 0 {
		return ""
	}
	skip := scrollCol
	budget := codeW
	var b strings.Builder

	for _, span := range spans {
		if budget <= 0 {
			break
		}
		w := ansi.StringWidth(span.text)
		if w == 0 {
			continue
		}
		if skip >= w {
			skip -= w
			continue
		}

		text := span.text
		if skip > 0 {
			text = ansi.Cut(text, skip, w)
			skip = 0
			if ansi.StringWidth(text) == 0 {
				continue
			}
		}

		rendered := span.style.Render(text)
		if ansi.StringWidth(rendered) > budget {
			rendered = ansi.Truncate(rendered, budget, "")
		}
		renderedW := ansi.StringWidth(rendered)
		b.WriteString(rendered)
		budget -= renderedW
	}

	if budget > 0 {
		b.WriteString(strings.Repeat(" ", budget))
	}
	res := b.String()
	if ansi.StringWidth(res) != codeW {
		res = uikit.FitExact(res, codeW)
	}
	return res
}

// FileName is the editor title shown in the pane border.
func (m EditorModel) FileName() string {
	return "solution." + fileExtension(m.language)
}

// Meta is the language, save state, and beautify action shown at the right end
// of the editor border.
func (m EditorModel) Meta() string {
	statusTag := "[saved]"
	if m.isSaving {
		statusTag = "[saving...]"
	} else if m.isModified {
		statusTag = "[modified]"
	}
	text := fmt.Sprintf("%s ▾ · %s", languageLabel(m.language), statusTag)
	return lipgloss.NewStyle().Foreground(m.theme.Muted).Render(text) +
		"  " + lipgloss.NewStyle().Bold(true).Foreground(m.theme.Accent).Render(formatIcon)
}

// View renders the syntax-highlighted buffer and the cursor position footer.
// The filename and save state live in the pane border instead of a second
// header row.
func (m EditorModel) View() string {
	innerW := m.width
	if innerW < 10 {
		innerW = 10
	}
	innerH := m.ViewportHeight()
	gutterW := m.GutterWidth()
	codeW := innerW - gutterW
	if codeW < 4 {
		codeW = 4
	}

	m.ensureCursorVisible()

	// 1. Body
	hlLines := m.highlightLines()
	bodyRows := make([]string, innerH)
	for i := 0; i < innerH; i++ {
		idx := m.scrollRow + i
		if idx >= len(m.lines) {
			bodyRows[i] = strings.Repeat(" ", innerW)
		} else {
			rowSpans := m.buildRowSpans(hlLines, idx)
			codeRendered := m.renderStyledSpans(rowSpans, m.scrollCol, codeW)
			rowLine := codeRendered
			if gutterW > 0 {
				numWidth := gutterW - 1
				numStr := fmt.Sprintf("%*d ", numWidth, idx+1)
				gutterStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
				if m.focused && idx == m.row {
					gutterStyle = lipgloss.NewStyle().Foreground(m.theme.Primary)
				}
				rowLine = gutterStyle.Render(numStr) + codeRendered
			}
			if ansi.StringWidth(rowLine) != innerW {
				rowLine = uikit.FitExact(rowLine, innerW)
			}
			bodyRows[i] = rowLine
		}
	}

	// 2. Status
	cursorPos := fmt.Sprintf("Ln %d / %d", m.row+1, len(m.lines))
	statusLine := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Align(lipgloss.Right).
		Width(innerW).
		Render(cursorPos)
	if ansi.StringWidth(statusLine) != innerW {
		statusLine = uikit.FitExact(statusLine, innerW)
	}

	elements := make([]string, 0, innerH+1)
	elements = append(elements, bodyRows...)
	elements = append(elements, statusLine)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// ViewLanguageMenu renders a language selection menu in place of the editor buffer.
func (m EditorModel) ViewLanguageMenu(options []string, selected int) string {
	innerW := m.width
	if innerW < 10 {
		innerW = 10
	}
	innerH := m.ViewportHeight()

	// 1. Menu body rows
	ruleW := 28
	if innerW < ruleW {
		ruleW = innerW
	}
	rule := lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", ruleW))

	var rawRows []string
	rawRows = append(rawRows, "")
	rawRows = append(rawRows, lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text).Render("Select language"))
	rawRows = append(rawRows, rule)
	rawRows = append(rawRows, "")

	for i, opt := range options {
		label := languageLabel(opt)
		paddedLabel := uikit.PadRight(label, 14)
		extPart := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("." + fileExtension(opt))

		if i == selected {
			rowStyled := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Accent).Render("▌ " + paddedLabel)
			rawRows = append(rawRows, rowStyled+extPart)
		} else {
			rowStyled := lipgloss.NewStyle().Foreground(m.theme.Text).Render("  " + paddedLabel)
			rawRows = append(rawRows, rowStyled+extPart)
		}
	}

	rawRows = append(rawRows, "")
	rawRows = append(rawRows, lipgloss.NewStyle().Foreground(m.theme.Subtext).Render("enter select · esc cancel"))

	bodyRows := make([]string, innerH)
	for i := 0; i < innerH; i++ {
		if i < len(rawRows) {
			bodyRows[i] = uikit.FitExact(rawRows[i], innerW)
		} else {
			bodyRows[i] = strings.Repeat(" ", innerW)
		}
	}

	// 2. Status
	cursorPos := fmt.Sprintf("Ln %d / %d", m.row+1, len(m.lines))
	statusLine := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Align(lipgloss.Right).
		Width(innerW).
		Render(cursorPos)
	if ansi.StringWidth(statusLine) != innerW {
		statusLine = uikit.FitExact(statusLine, innerW)
	}

	elements := make([]string, 0, innerH+1)
	elements = append(elements, bodyRows...)
	elements = append(elements, statusLine)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}
