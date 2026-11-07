// Package quickdialog provides one reusable centered list/input/confirm
// surface for the OpenLeet quick actions. It paints only with Palette tokens.
package quickdialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/ui/theme"
)

// Mode selects the dialog body.
type Mode int

const (
	ModeList Mode = iota
	ModeInput
	ModeConfirm
)

// Item is one selectable row.
type Item struct {
	ID     string
	Label  string
	Value  string
	Active bool
}

// Group is a labelled collection of items.
type Group struct {
	Title string
	Items []Item
}

// Options configures a list dialog.
type Options struct {
	Title       string
	Search      bool
	ReadOnly    bool
	Hint        string
	Placeholder string
}

type row struct {
	header bool
	label  string
	item   Item
}

// Model is a reusable centered dialog.
type Model struct {
	mode       Mode
	opts       Options
	groups     []Group
	rows       []row
	selected   int
	offset     int
	search     textinput.Model
	input      textinput.Model
	inputLabel string
	question   string
	confirmYes bool
	width      int
	height     int
	pal        theme.Palette
	errText    string
}

// NewList constructs a searchable list dialog.
func NewList(opts Options, groups []Group, pal theme.Palette, width, height int) Model {
	if opts.Title == "" {
		opts.Title = "SELECT"
	}
	if opts.Placeholder == "" {
		opts.Placeholder = "Search"
	}
	search := textinput.New()
	search.Prompt = ""
	search.Placeholder = opts.Placeholder
	search.CharLimit = 64
	search.Width = max(8, panelContentWidth(width)-2)
	if opts.Search {
		search.Focus()
	}
	m := Model{
		mode:     ModeList,
		opts:     opts,
		search:   search,
		selected: -1,
		width:    width,
		height:   height,
		pal:      pal,
	}
	m.SetGroups(groups)
	m.syncSearchCursor()
	return m
}

// NewInput constructs a single text-input dialog.
func NewInput(title, label, value string, masked bool, pal theme.Palette, width, height int) Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 512
	input.Width = max(8, panelContentWidth(width)-2)
	input.SetValue(value)
	input.CursorEnd()
	if masked {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
	}
	input.Focus()
	return Model{
		mode:       ModeInput,
		opts:       Options{Title: title, Hint: "enter save  esc back"},
		input:      input,
		inputLabel: label,
		selected:   -1,
		width:      width,
		height:     height,
		pal:        pal,
	}
}

// NewConfirm constructs a yes/no confirmation dialog.
func NewConfirm(title, question string, pal theme.Palette, width, height int) Model {
	return Model{
		mode:       ModeConfirm,
		opts:       Options{Title: title, Hint: "enter confirm  esc cancel"},
		question:   question,
		confirmYes: true,
		selected:   -1,
		width:      width,
		height:     height,
		pal:        pal,
	}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.search.Width = max(8, panelContentWidth(width)-2)
	m.input.Width = max(8, panelContentWidth(width)-2)
}

// SetPalette repaints the dialog.
func (m *Model) SetPalette(p theme.Palette) {
	m.pal = p
}

// SetGroups replaces the list content. The previously selected ID is kept when
// it is still visible, otherwise the first active item is selected.
func (m *Model) SetGroups(groups []Group) {
	previous := m.SelectedID()
	m.groups = groups
	m.rebuild(previous)
}

// syncSearchCursor hides the block cursor while the search box is empty so it
// cannot sit on top of the placeholder's first rune.
func (m *Model) syncSearchCursor() tea.Cmd {
	if !m.opts.Search {
		return nil
	}
	if m.search.Value() == "" {
		if m.search.Cursor.Mode() != cursor.CursorHide {
			return m.search.Cursor.SetMode(cursor.CursorHide)
		}
		return nil
	}
	if m.search.Cursor.Mode() != cursor.CursorBlink {
		return m.search.Cursor.SetMode(cursor.CursorBlink)
	}
	return nil
}

// SetError sets the inline validation message for input dialogs.
func (m *Model) SetError(text string) {
	m.errText = text
}

// Mode returns the dialog body kind.
func (m Model) Mode() Mode { return m.mode }

// Title returns the dialog title.
func (m Model) Title() string { return m.opts.Title }

// SelectedItem returns the selected item, if any.
func (m Model) SelectedItem() (Item, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) || m.rows[m.selected].header {
		return Item{}, false
	}
	return m.rows[m.selected].item, true
}

// SelectedID returns the selected item ID.
func (m Model) SelectedID() string {
	item, ok := m.SelectedItem()
	if !ok {
		return ""
	}
	return item.ID
}

// SearchValue returns the current filter query.
func (m Model) SearchValue() string { return m.search.Value() }

// InputValue returns the text input value.
func (m Model) InputValue() string { return m.input.Value() }

// Confirmed reports whether the confirm dialog is on Yes.
func (m Model) Confirmed() bool { return m.confirmYes }

// Update processes navigation, filtering, input, and confirm state.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			m.move(-1)
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.move(1)
			return m, nil
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case ModeList:
			switch msg.String() {
			case "up", "ctrl+p":
				m.move(-1)
				return m, nil
			case "down", "ctrl+n":
				m.move(1)
				return m, nil
			case "k":
				if !m.opts.Search {
					m.move(-1)
					return m, nil
				}
			case "j":
				if !m.opts.Search {
					m.move(1)
					return m, nil
				}
			case "pgup":
				m.move(-5)
				return m, nil
			case "pgdown":
				m.move(5)
				return m, nil
			case "home":
				m.selectFirst()
				return m, nil
			case "end":
				m.selectLast()
				return m, nil
			case "enter", "esc":
				return m, nil
			}
			if m.opts.Search {
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.rebuild("")
				if blink := m.syncSearchCursor(); blink != nil {
					cmd = tea.Batch(cmd, blink)
				}
				return m, cmd
			}
			return m, nil

		case ModeInput:
			if msg.String() == "enter" || msg.String() == "esc" {
				return m, nil
			}
			m.errText = ""
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		case ModeConfirm:
			switch msg.String() {
			case "left", "h", "shift+tab", "y", "Y":
				m.confirmYes = true
			case "right", "l", "tab", "n", "N":
				m.confirmYes = false
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) rebuild(keepID string) {
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	var rows []row
	for _, group := range m.groups {
		items := group.Items
		if query != "" {
			items = nil
			for _, item := range group.Items {
				haystack := strings.ToLower(strings.Join([]string{group.Title, item.ID, item.Label, item.Value}, " "))
				if strings.Contains(haystack, query) {
					items = append(items, item)
				}
			}
		}
		if len(items) == 0 {
			continue
		}
		rows = append(rows, row{header: true, label: group.Title})
		for _, item := range items {
			rows = append(rows, row{item: item})
		}
	}
	m.rows = rows
	m.selected = -1
	m.offset = 0
	if keepID != "" {
		for i, candidate := range m.rows {
			if !candidate.header && candidate.item.ID == keepID {
				m.selected = i
				break
			}
		}
	}
	if m.selected < 0 {
		for i, candidate := range m.rows {
			if !candidate.header && candidate.item.Active {
				m.selected = i
				break
			}
		}
	}
	if m.selected < 0 {
		for i, candidate := range m.rows {
			if !candidate.header {
				m.selected = i
				break
			}
		}
	}
	m.ensureVisible()
}

func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}
	index := m.selected
	if index < 0 {
		index = 0
	}
	for n := 0; n < delta; n++ {
		next := index
		for {
			next = (next + step + len(m.rows)) % len(m.rows)
			if !m.rows[next].header {
				break
			}
		}
		index = next
	}
	m.selected = index
	m.ensureVisible()
}

func (m *Model) selectFirst() {
	for i, candidate := range m.rows {
		if !candidate.header {
			m.selected = i
			m.ensureVisible()
			return
		}
	}
}

func (m *Model) selectLast() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if !m.rows[i].header {
			m.selected = i
			m.ensureVisible()
			return
		}
	}
}

func (m *Model) ensureVisible() {
	visible := m.visibleRows()
	if visible <= 0 || m.selected < 0 {
		return
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+visible {
		m.offset = m.selected - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if maxOffset := len(m.rows) - visible; m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) visibleRows() int {
	limit := 14
	if m.opts.ReadOnly {
		limit = 18
	}
	value := m.height - 12
	if value < 4 {
		value = 4
	}
	if value > limit {
		value = limit
	}
	return value
}
