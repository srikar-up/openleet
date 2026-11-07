package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/config"
	"openleet/internal/runner"
	"openleet/internal/secrets"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/uikit"
)

// Focus identifies which Settings region owns navigation.
type Focus int

const (
	FocusSidebar Focus = iota
	FocusCanvas
	FocusSearch
)

// CloseMsg asks the app to restore the screen that opened Settings.
type CloseMsg struct{}

// PreviewThemeMsg applies a theme without persisting it.
type PreviewThemeMsg struct{ Name string }

// ApplyThemeMsg applies and persists a theme.
type ApplyThemeMsg struct{ Name string }

// RevertThemeMsg restores the theme that was active before the preview.
type RevertThemeMsg struct{ Name string }

// SaveMsg reports the result of persisting settings.
type SaveMsg struct{ Err error }

// ProviderTestMsg reports a provider key/model probe.
type ProviderTestMsg struct {
	Provider string
	Latency  time.Duration
	Err      error
	Models   []string
}

// DetectMsg reports discovered language runtimes.
type DetectMsg struct {
	Runtimes []runner.Runtime
	Err      error
}

// ActionMsg asks the app to perform a data/account action.
type ActionMsg struct {
	Action string
	Value  string
}

// ActionResultMsg reports a completed data action back to Settings.
type ActionResultMsg struct {
	Action string
	Detail string
	Err    error
}

// Confirm is a typed-confirmation modal for destructive actions.
type Confirm struct {
	Action string
	Title  string
	Word   string
	Input  textinput.Model
	Err    string
}

// Model is the schema-driven Settings screen.
type Model struct {
	cfg     *config.Config
	pal     theme.Palette
	secrets *secrets.Store
	width   int
	height  int

	sections        []Section
	selectedSection SectionID
	focus           Focus
	selectedField   int // index into the active section's filtered field list

	// dirtySections marks the sidebar entries that contain unsaved values.
	dirtySections map[SectionID]bool
	// searchSavedSection/searchSavedField restore the tab when Esc clears a filter.
	searchSavedSection SectionID
	searchSavedField   int

	search       textinput.Model
	searchActive bool
	query        string

	editing    bool
	editInput  textinput.Model
	editField  *Field
	reveal     bool
	captureKey bool

	dirty    bool
	status   string
	saveErr  error
	detected []runner.Runtime

	themes        []theme.Palette
	themeIndex    int
	originalTheme string
	previewing    bool

	providers      []Provider
	providerIndex  int
	providerDetail bool
	providerStatus map[string]ProviderTestMsg

	keybindings []Keybinding
	captureIdx  int
	conflict    string

	confirm *Confirm

	account     AccountInfo
	auth        *AuthForm
	googleBusy  bool
	integration *integrationPicker
}

// New constructs the Settings model and migrates the config schema.
func New(cfg *config.Config, pal theme.Palette, width, height int) Model {
	EnsureDefaults(cfg)

	search := textinput.New()
	search.Prompt = ""
	search.Placeholder = "/ search settings"
	search.CharLimit = 64
	search.Width = sidebarWidth - 2

	edit := textinput.New()
	edit.CharLimit = 512
	edit.Width = 40

	m := Model{
		cfg:             cfg,
		pal:             pal,
		width:           width,
		height:          height,
		sections:        Sections(),
		selectedSection: SectionGeneral,
		focus:           FocusSidebar,
		search:          search,
		editInput:       edit,
		themes:          theme.All(),
		providers:       Providers(),
		providerStatus:  make(map[string]ProviderTestMsg),
		keybindings:     DefaultKeybindings(),
		dirtySections:   make(map[SectionID]bool),
	}
	m.originalTheme = cfg.Theme
	m.selectTheme(cfg.Theme)
	if cfg.ConfigDir() != "" {
		store, err := secrets.NewStore(filepath.Join(cfg.ConfigDir(), "secrets.enc"))
		if err != nil {
			m.status = "Secret vault unavailable: " + err.Error()
		} else {
			m.secrets = store
		}
	}
	m.OpenSection(SectionGeneral)
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// SetSize updates the viewport.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetPalette repaints Settings after a live theme preview.
func (m *Model) SetPalette(p theme.Palette) {
	m.pal = p
}

// SetStatus overrides the transient status line, used by the app for the
// quit-confirmation prompt.
func (m *Model) SetStatus(text string) { m.status = text }

// Width returns the current width.
func (m Model) Width() int { return m.width }

// Height returns the current height.
func (m Model) Height() int { return m.height }

// ActiveSection returns the explicitly selected settings tab.
func (m Model) ActiveSection() SectionID {
	for _, section := range m.sections {
		if section.ID == m.selectedSection {
			return m.selectedSection
		}
	}
	return SectionGeneral
}

// Dirty reports whether unsaved settings exist.
func (m Model) Dirty() bool { return m.dirty }

// OpenSection clears any filter and selects one tab.
func (m *Model) OpenSection(id SectionID) {
	m.query = ""
	m.searchActive = false
	m.search.Reset()
	m.search.Blur()
	m.providerDetail = false
	m.focus = FocusCanvas
	m.setSection(id)
	m.selectedField = 0
}

// setSection validates and assigns the active tab.
func (m *Model) setSection(id SectionID) {
	valid := false
	for _, section := range m.sections {
		if section.ID == id {
			valid = true
			break
		}
	}
	if !valid {
		id = SectionGeneral
	}
	if id != m.selectedSection {
		m.selectedField = 0
		m.providerDetail = false
	}
	m.selectedSection = id
}

// changeSection moves to the previous or next tab.
func (m *Model) changeSection(delta int) {
	if len(m.sections) == 0 {
		return
	}
	current := 0
	for i, section := range m.sections {
		if section.ID == m.ActiveSection() {
			current = i
			break
		}
	}
	next := current + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.sections) {
		next = len(m.sections) - 1
	}
	m.setSection(m.sections[next].ID)
	m.selectedField = 0
	m.providerDetail = false
}

// Restore keeps the remembered tab and focus when Settings is reopened from
// another screen. It only repairs geometry and selection.
func (m *Model) Restore() {
	m.setSection(m.ActiveSection())
	m.focus = FocusCanvas
	fields := m.currentFields()
	if len(fields) == 0 {
		m.selectedField = 0
	} else if m.selectedField < 0 || m.selectedField >= len(fields) {
		m.selectedField = 0
	}
}

func (m *Model) selectTheme(name string) {
	for i, candidate := range m.themes {
		if candidate.Name == name {
			m.themeIndex = i
			return
		}
	}
	m.themeIndex = 0
}

func (m Model) currentTheme() theme.Palette {
	if len(m.themes) == 0 {
		return theme.Default()
	}
	if m.themeIndex < 0 || m.themeIndex >= len(m.themes) {
		return m.themes[0]
	}
	return m.themes[m.themeIndex]
}

// allFields returns every schema row in authoring order, with the AI model
// options resolved from the active provider.
func (m Model) allFields() []Field {
	fields := Fields()
	for i := range fields {
		if fields[i].ID == "ai.model" {
			fields[i].Options = m.modelOptions()
			fields[i].Default = m.effectiveModel()
		}
	}
	return fields
}

// visibleFields applies the search filter to the full field list.
func (m Model) visibleFields() []Field {
	all := m.allFields()
	query := strings.TrimSpace(m.query)
	if query == "" {
		return all
	}
	matches := make(map[string]bool)
	for _, field := range Search(query) {
		matches[field.ID] = true
	}
	out := make([]Field, 0, len(all))
	for _, field := range all {
		if matches[field.ID] {
			out = append(out, field)
		}
	}
	return out
}

// sectionFields returns the fields of the selected tab, filtered by search.
func (m Model) sectionFields() []Field { return m.currentFields() }

func (m Model) fieldsFor(section SectionID) []Field {
	var out []Field
	for _, field := range m.visibleFields() {
		if field.Section == section {
			out = append(out, field)
		}
	}
	return out
}

// currentFields is the selected tab's field list.
func (m Model) currentFields() []Field { return m.fieldsFor(m.ActiveSection()) }

func (m Model) indexOfField(id string) int {
	for i, field := range m.currentFields() {
		if field.ID == id {
			return i
		}
	}
	return -1
}

func (m Model) modelProvider() Provider {
	if id := CoachProviderID(m.cfg); id != "" {
		if p, ok := ProviderByID(id); ok {
			return p
		}
	}
	priority := []string{"opencode", "openai", "groq", "anthropic", "gemini", "nvidia", "openrouter", "ollama", "custom"}
	for _, pid := range priority {
		if p, ok := ProviderByID(pid); ok {
			if p.Ollama || m.SecretMasked(p.ID) != "" {
				return p
			}
		}
	}
	return m.currentProvider()
}

func (m Model) effectiveModel() string {
	return ProviderModel(m.cfg, m.modelProvider())
}

func (m Model) modelOptions() []Option {
	p := m.modelProvider()
	discovered := m.providerStatus[p.ID].Models
	if len(discovered) == 0 {
		discovered = CachedProviderModels(m.cfg, p.ID)
	}
	active := m.effectiveModel()
	names := ProviderModels(p, active, discovered)
	if len(names) == 0 {
		return nil
	}
	opts := make([]Option, len(names))
	for i, name := range names {
		label := name
		if strings.EqualFold(name, active) {
			label = name + "  (active)"
		}
		opts[i] = Option{Value: name, Label: label}
	}
	return opts
}

func (m *Model) selectModel(model string) (Model, tea.Cmd) {
	p := m.modelProvider()
	key := "ai.provider." + p.ID + ".model"
	SetValue(m.cfg, key, model)
	m.markDirty(SectionAI)
	m.status = fmt.Sprintf("%s · model: %s", p.Label, model)
	return *m, func() tea.Msg { return AIProviderChangedMsg{ProviderID: p.ID} }
}

func (m Model) selectedFieldValue() *Field {
	fields := m.sectionFields()
	if len(fields) == 0 {
		return nil
	}
	if m.selectedField < 0 {
		return &fields[0]
	}
	if m.selectedField >= len(fields) {
		return &fields[len(fields)-1]
	}
	return &fields[m.selectedField]
}

// Update processes keyboard, mouse, and async provider/runtime messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m.update(msg)
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	}
	if m.integration != nil {
		return m.updateIntegration(msg)
	}

	switch msg := msg.(type) {
	case ProviderTestMsg:
		m.providerStatus[msg.Provider] = msg
		if len(msg.Models) > 0 {
			SetCachedProviderModels(m.cfg, msg.Provider, msg.Models)
			m.markDirty(SectionAI)
		}
		if msg.Err != nil {
			m.status = fmt.Sprintf("%s test failed: %v", msg.Provider, msg.Err)
		} else if len(msg.Models) > 0 {
			m.status = fmt.Sprintf("%s connected · %d models · choose one in Assign models", msg.Provider, len(msg.Models))
		} else {
			m.status = fmt.Sprintf("%s connected in %s", msg.Provider, msg.Latency.Round(time.Millisecond))
		}
		return m, nil

	case DetectMsg:
		if msg.Err != nil {
			m.status = "Detect interpreters: " + msg.Err.Error()
			return m, nil
		}
		m.detected = msg.Runtimes
		m.status = fmt.Sprintf("Detected %d runtimes", len(msg.Runtimes))
		return m, nil

	case SaveMsg:
		m.saveErr = msg.Err
		if msg.Err != nil {
			m.status = "Save failed: " + msg.Err.Error()
		}
		return m, nil

	case ActionResultMsg:
		if msg.Err != nil {
			m.status = "Action failed: " + msg.Err.Error()
		} else {
			m.status = msg.Detail
		}
		return m, nil

	case AuthResultMsg:
		m.googleBusy = false
		if msg.Err != nil {
			if m.auth != nil {
				m.auth.Busy = false
				m.auth.Err = msg.Err.Error()
				m.status = "Sign-in failed"
			} else {
				m.status = "Sign-in failed: " + msg.Err.Error()
			}
			return m, nil
		}
		m.account = msg.Info
		m.auth = nil
		if msg.Message != "" {
			m.status = msg.Message
		} else if msg.Info.SignedIn {
			m.status = "Signed in as " + msg.Info.Email
		} else {
			m.status = "Signed out"
		}
		return m, nil

	case tea.MouseMsg:
		return m.updateMouse(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if m.editing {
			var cmd tea.Cmd
			m.editInput, cmd = m.editInput.Update(msg)
			return m, cmd
		}
		if m.searchActive {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.query = m.search.Value()
			return m, cmd
		}
		return m, nil
	}

	if m.confirm != nil {
		return m.updateConfirm(keyMsg)
	}
	if m.auth != nil {
		return m.updateAuth(keyMsg)
	}
	if m.captureKey {
		return m.updateKeyCapture(keyMsg)
	}
	if m.editing {
		return m.updateEditing(keyMsg)
	}
	if m.searchActive {
		return m.updateSearch(keyMsg)
	}
	return m.updateNormal(keyMsg)
}

func (m Model) updateNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "<":
		m.changeSection(-1)
		return m, nil
	case ">":
		m.changeSection(1)
		return m, nil
	}

	if key == "/" {
		m.searchSavedSection = m.ActiveSection()
		m.searchSavedField = m.selectedField
		m.focus = FocusSearch
		m.searchActive = true
		m.search.Focus()
		return m, textinput.Blink
	}
	if key == "ctrl+s" {
		return m.save()
	}
	if key == "esc" {
		return m.escape()
	}

	// The Account card's action keys are reachable from any focus pane, so the
	// printed hint never silently no-ops: Settings opens with sidebar focus,
	// where canvas-only keys were unreachable.
	if m.ActiveSection() == SectionAccount && m.account.SignedIn {
		switch key {
		case "l", "L":
			return m, func() tea.Msg { return ActionMsg{Action: "account_logout"} }
		case "s", "S":
			if !SyncEnabled(m.cfg) {
				m.status = "Cloud sync is off · enable it in Sync settings"
				return m, nil
			}
			return m, func() tea.Msg { return ActionMsg{Action: "account_sync"} }
		}
	}

	// Special canvases own their cursor movement before generic field rows.
	if m.focus == FocusCanvas {
		if key == "left" || key == "right" {
			delta := 1
			if key == "left" {
				delta = -1
			}
			return m.adjustSelected(delta)
		}
		switch m.ActiveSection() {
		case SectionAI:
			if m.providerDetail {
				provider := m.currentProvider()
				switch key {
				case "enter":
					m.startProviderKeyEdit()
					return m, textinput.Blink
				case "t":
					return m, m.testProviderCmd()
				case "x":
					if m.secrets != nil {
						_ = m.secrets.Delete(provider.ID)
						delete(m.cfg.Settings, providerModelsCacheKey(provider.ID))
						delete(m.providerStatus, provider.ID)
						m.dirty = true
						m.markDirty(SectionAI)
						m.status = provider.Label + " key removed"
						return m, func() tea.Msg { return AIProviderChangedMsg{ProviderID: provider.ID} }
					}
					return m, nil
				case "b":
					field := Field{
						ID:          "ai.provider." + provider.ID + ".base_url",
						Label:       "Base URL",
						Kind:        KindText,
						Default:     m.providerBase(provider),
						Placeholder: provider.BaseURL,
						Validate:    validateURLOrEmpty,
					}
					m.startEditing(field)
					return m, textinput.Blink
				case "m":
					field := Field{
						ID:          "ai.provider." + provider.ID + ".model",
						Label:       "Model",
						Kind:        KindText,
						Default:     m.providerModel(provider),
						Placeholder: provider.DefaultModel,
					}
					m.startEditing(field)
					return m, textinput.Blink
				}
				return m, nil
			}
			switch key {
			case "c":
				return m.openIntegrationPicker()
			}
		case SectionAccount:
			if !m.account.SignedIn {
				switch key {
				case "g":
					if m.googleBusy {
						return m, nil
					}
					m.googleBusy = true
					m.status = "Opening Google sign-in in your browser…"
					return m, func() tea.Msg { return GoogleAuthRequestMsg{} }
				case "e":
					m.openAuthForm()
					return m, textinput.Blink
				case "u":
					m.openAuthFormMode(AuthSignUp)
					return m, textinput.Blink
				}
			}
		case SectionKeys:
			switch key {
			case "j", "down":
				if m.captureIdx < len(m.keybindings)-1 {
					m.captureIdx++
					return m, nil
				}
				return m.moveDown(), nil
			case "k", "up":
				if m.captureIdx > 0 {
					m.captureIdx--
					return m, nil
				}
				return m.moveUp(), nil
			case "enter":
				m.captureKey = true
				m.conflict = ""
				return m, nil
			case "r":
				if m.cfg.Settings == nil {
					m.cfg.Settings = make(map[string]string)
				}
				delete(m.cfg.Settings, "keys."+m.keybindings[m.captureIdx].Action)
				m.dirty = true
				m.markDirty(SectionKeys)
				m.status = m.keybindings[m.captureIdx].Label + " reset"
				return m, nil
			case "R":
				for _, binding := range m.keybindings {
					delete(m.cfg.Settings, "keys."+binding.Action)
				}
				m.dirty = true
				m.markDirty(SectionKeys)
				m.status = "All keybindings reset"
				return m, nil
			}
		}
	}

	switch key {
	case "tab":
		if m.focus == FocusSidebar {
			m.focus = FocusCanvas
		} else {
			m.focus = FocusSidebar
		}
		return m, nil
	case "h":
		m.focus = FocusSidebar
		return m, nil
	case "l":
		m.focus = FocusCanvas
		if field := m.selectedFieldValue(); field == nil || field.Section != m.ActiveSection() {
			fields := m.fieldsFor(m.ActiveSection())
			if len(fields) > 0 {
				m.selectedField = m.indexOfField(fields[0].ID)
			}
		}
		return m, nil
	case "j", "down":
		return m.moveDown(), nil
	case "k", "up":
		return m.moveUp(), nil
	case "enter":
		return m.activate()
	case " ":
		return m.toggleSelected()
	}
	return m, nil
}

func (m Model) adjustSelected(delta int) (Model, tea.Cmd) {
	field := m.selectedFieldValue()
	if field == nil {
		return m, nil
	}
	if field.ID == "ai.model" {
		if len(field.Options) == 0 {
			return m, nil
		}
		next := cycleOption(*field, m.effectiveModel(), delta)
		return m.selectModel(next)
	}
	switch field.Kind {
	case KindSelect:
		next := cycleOption(*field, Value(m.cfg, *field), delta)
		SetValue(m.cfg, field.ID, next)
		m.markDirty(field.Section)
		m.status = field.Label + ": " + optionLabel(*field, next)
		return m, coachChangedCmd(*field, next)
	case KindStepper:
		return m.stepSelected(delta)
	case KindThemeGrid:
		if len(m.themes) == 0 {
			return m, nil
		}
		m.themeIndex = (m.themeIndex + delta + len(m.themes)) % len(m.themes)
		return m.previewTheme()
	case KindProviderList:
		if m.providerDetail || len(m.providers) == 0 {
			return m, nil
		}
		next := m.providerIndex + delta
		if next < 0 {
			next = 0
		}
		if next >= len(m.providers) {
			next = len(m.providers) - 1
		}
		m.providerIndex = next
		m.status = m.currentProvider().Label
		return m, nil
	}
	return m, nil
}

func (m *Model) previewTheme() (Model, tea.Cmd) {
	name := m.currentTheme().Name
	SetValue(m.cfg, "appearance.theme", name)
	m.cfg.Theme = name
	m.previewing = true
	m.status = "Previewing " + m.currentTheme().Label + " · Enter applies · Esc reverts"
	return *m, func() tea.Msg { return PreviewThemeMsg{Name: name} }
}

func (m Model) moveDown() Model {
	if m.focus == FocusSidebar {
		m.changeSection(1)
		return m
	}
	fields := m.currentFields()
	if len(fields) == 0 {
		return m
	}
	if m.selectedField < len(fields)-1 {
		m.selectedField++
	}
	m.providerDetail = false
	return m
}

func (m Model) moveUp() Model {
	if m.focus == FocusSidebar {
		m.changeSection(-1)
		return m
	}
	if m.selectedField > 0 {
		m.selectedField--
	}
	m.providerDetail = false
	return m
}

// selectedFieldLine finds the first row whose printable text starts with the
// field label. Labels are rendered in the 22-cell label column, so a prefix
// match is stable even when the value column is truncated.
func selectedFieldLine(contents, label string) int {
	if label == "" {
		return -1
	}
	for i, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimLeft(ansi.Strip(line), "│ ")
		if strings.HasPrefix(trimmed, uikit.Truncate(label, 22, "…")+" ") {
			return i
		}
	}
	return -1
}

func (m Model) activate() (Model, tea.Cmd) {
	if m.focus == FocusSidebar {
		m.focus = FocusCanvas
		if len(m.currentFields()) > 0 {
			m.selectedField = 0
		}
		return m, nil
	}
	field := m.selectedFieldValue()
	if field == nil {
		return m, nil
	}
	if field.ID == "ai.model" {
		if len(field.Options) == 0 {
			return m, nil
		}
		next := cycleOption(*field, m.effectiveModel(), 1)
		return m.selectModel(next)
	}
	switch field.Kind {
	case KindToggle:
		return m.toggleSelected()
	case KindSelect:
		next := nextOption(*field, Value(m.cfg, *field))
		SetValue(m.cfg, field.ID, next)
		m.markDirty(field.Section)
		m.status = field.Label + ": " + optionLabel(*field, next)
		return m, coachChangedCmd(*field, next)
	case KindStepper:
		return m.stepSelected(1)
	case KindText, KindSecret, KindAccent:
		m.startEditing(*field)
		return m, textinput.Blink
	case KindButton:
		return m.activateButton(*field)
	case KindThemeGrid:
		name := m.currentTheme().Name
		SetValue(m.cfg, field.ID, name)
		m.cfg.Theme = name
		m.originalTheme = name
		m.previewing = false
		m.markDirty(SectionAppearance)
		m.status = "Theme applied"
		return m, func() tea.Msg { return ApplyThemeMsg{Name: name} }
	case KindProviderList:
		m.providerDetail = true
		m.status = "Enter edits the API key · t tests · esc closes"
		return m, nil
	case KindKeyTable:
		m.captureKey = true
		m.conflict = ""
		return m, nil
	case KindAccount:
		if m.account.SignedIn {
			m.status = "Signed in as " + m.account.Email + " · L logs out"
			return m, nil
		}
		m.openAuthForm()
		return m, textinput.Blink
	case KindAbout:
		m.status = "Diagnostics: " + m.cfg.ConfigDir()
		return m, nil
	}
	return m, nil
}

func (m Model) activateButton(field Field) (Model, tea.Cmd) {
	switch field.ID {
	case "runner.detect":
		return m, func() tea.Msg {
			detector := runner.NewDetector()
			detector.SetExecutable("python", Value(m.cfg, Field{ID: "runner.python_path", Default: "python"}))
			detector.SetExecutable("go", Value(m.cfg, Field{ID: "runner.go_path", Default: "go"}))
			runtimes := detector.DetectAll()
			return DetectMsg{Runtimes: runtimes}
		}
	case "data.export", "data.import", "data.backup", "data.clear_cache":
		action := strings.ReplaceAll(field.ID, ".", "_")
		return m, func() tea.Msg { return ActionMsg{Action: action} }
	case "data.reset_roadmap":
		m.openConfirm("data_reset_roadmap", "Reset roadmap progress?", "RESET")
		return m, textinput.Blink
	case "data.reset_settings":
		m.openConfirm("reset_settings", "Reset all settings?", "RESET")
		return m, textinput.Blink
	case "data.delete_local":
		m.openConfirm("data_delete_local", "Delete local data?", "DELETE")
		return m, textinput.Blink
	case "ai.connect":
		return m.openIntegrationPicker()
	}
	return m, nil
}

func (m Model) toggleSelected() (Model, tea.Cmd) {
	field := m.selectedFieldValue()
	if field != nil && field.Kind == KindToggle {
		value := Value(m.cfg, *field)
		next := "on"
		if value == "on" {
			next = "off"
		}
		SetValue(m.cfg, field.ID, next)
		m.markDirty(field.Section)
		m.status = field.Label + ": " + next
		return m, nil
	}
	return m, nil
}

func (m Model) stepSelected(delta int) (Model, tea.Cmd) {
	field := m.selectedFieldValue()
	if field == nil || field.Kind != KindStepper {
		return m, nil
	}
	current, _ := strconv.Atoi(Value(m.cfg, *field))
	current += delta * field.Step
	if current < field.Min {
		current = field.Min
	}
	if current > field.Max {
		current = field.Max
	}
	SetValue(m.cfg, field.ID, strconv.Itoa(current))
	m.markDirty(field.Section)
	m.status = fmt.Sprintf("%s: %d", field.Label, current)
	return m, nil
}

func (m *Model) startEditing(field Field) {
	if strings.HasSuffix(field.ID, ".base_url") {
		field.Validate = validateURLOrEmpty
	}
	m.editing = true
	m.editField = &field
	m.reveal = false
	m.editInput.Reset()
	m.editInput.EchoMode = textinput.EchoNormal
	value := Value(m.cfg, field)
	if field.Kind != KindSecret {
		m.editInput.SetValue(value)
	} else {
		m.editInput.EchoMode = textinput.EchoPassword
	}
	m.editInput.Placeholder = field.Placeholder
	m.editInput.Focus()
	m.editInput.CursorEnd()
}

func (m *Model) startProviderKeyEdit() {
	provider := m.currentProvider()
	if provider.ID == "" {
		return
	}
	field := Field{ID: "ai.provider." + provider.ID + ".key", Section: SectionAI, Label: provider.Label + " API key", Kind: KindSecret}
	m.startEditing(field)
	m.editInput.Placeholder = provider.KeyPrefix + "…"
	m.status = "Enter saves to the encrypted vault; ctrl+r reveals while typing."
}

func (m Model) currentProvider() Provider {
	if len(m.providers) == 0 {
		return Provider{}
	}
	if m.providerIndex < 0 || m.providerIndex >= len(m.providers) {
		return m.providers[0]
	}
	return m.providers[m.providerIndex]
}

func (m Model) updateEditing(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		m.editInput.Blur()
		m.editInput.Reset()
		return m, nil
	case "ctrl+r":
		m.reveal = !m.reveal
		if m.reveal {
			m.editInput.EchoMode = textinput.EchoNormal
		} else {
			m.editInput.EchoMode = textinput.EchoPassword
		}
		return m, nil
	case "enter":
		return m.commitEdit()
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m Model) commitEdit() (Model, tea.Cmd) {
	field := m.editField
	if field == nil {
		m.editing = false
		return m, nil
	}
	value := strings.TrimSpace(m.editInput.Value())
	if field.Kind == KindSecret {
		provider := m.currentProvider()
		if provider.ID == "" || value == "" {
			m.status = "API key cannot be empty"
			return m, nil
		}
		if m.secrets == nil {
			m.status = "Secret vault is unavailable"
			return m, nil
		}
		if err := m.secrets.Set(provider.ID, value); err != nil {
			m.status = err.Error()
			return m, nil
		}
		// A new key can expose a different model catalog.
		delete(m.cfg.Settings, providerModelsCacheKey(provider.ID))
		delete(m.providerStatus, provider.ID)
		m.markDirty(SectionAI)
		m.status = provider.Label + " key saved to the encrypted vault"
		m.editing = false
		m.editInput.Reset()
		m.editInput.Blur()
		return m, func() tea.Msg { return AIProviderChangedMsg{ProviderID: provider.ID} }
	}
	if field.Validate != nil {
		if err := field.Validate(value); err != nil {
			m.status = field.Label + ": " + err.Error()
			return m, nil
		}
	}
	SetValue(m.cfg, field.ID, value)
	m.markDirty(field.Section)
	m.status = field.Label + " updated"
	m.editing = false
	m.editInput.Reset()
	m.editInput.Blur()
	if strings.HasPrefix(field.ID, "ai.provider.") {
		if id := providerIDFromField(field.ID); id != "" {
			if strings.HasSuffix(field.ID, ".base_url") {
				// A different gateway can expose a different catalog.
				delete(m.cfg.Settings, providerModelsCacheKey(id))
				delete(m.providerStatus, id)
			}
			return m, func() tea.Msg { return AIProviderChangedMsg{ProviderID: id} }
		}
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchActive = false
		m.search.Blur()
		m.search.Reset()
		m.query = ""
		m.setSection(m.searchSavedSection)
		m.selectedField = m.searchSavedField
		if fields := m.currentFields(); len(fields) > 0 {
			if m.selectedField < 0 || m.selectedField >= len(fields) {
				m.selectedField = 0
			}
		} else {
			m.selectedField = 0
		}
		m.focus = FocusCanvas
		return m, nil
	case "enter":
		query := strings.TrimSpace(m.query)
		m.searchActive = false
		m.search.Blur()
		m.search.Reset()
		m.query = ""
		m.focus = FocusCanvas
		if query == "" {
			return m, nil
		}
		if results := Search(query); len(results) > 0 {
			m.OpenSection(results[0].Section)
			if index := m.indexOfField(results[0].ID); index >= 0 {
				m.selectedField = index
			}
		}
		return m, nil
	case "down", "up":
		// The result list is jumped to with Enter; keep typing while the
		// input is focused.
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.query = m.search.Value()
	if strings.TrimSpace(m.query) == "" {
		m.setSection(m.searchSavedSection)
		m.selectedField = m.searchSavedField
		if fields := m.currentFields(); len(fields) > 0 && m.selectedField >= len(fields) {
			m.selectedField = 0
		}
	}
	return m, cmd
}

func (m Model) updateKeyCapture(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.captureKey = false
		m.conflict = ""
		return m, nil
	case "enter":
		return m, nil
	}
	captured := msg.String()
	for i, binding := range m.keybindings {
		if i != m.captureIdx && bindingKey(m.cfg, binding) == captured {
			m.conflict = fmt.Sprintf("%s is already used by %s", captured, binding.Label)
			return m, nil
		}
	}
	if m.cfg.Settings == nil {
		m.cfg.Settings = make(map[string]string)
	}
	m.cfg.Settings["keys."+m.keybindings[m.captureIdx].Action] = captured
	m.markDirty(SectionKeys)
	m.captureKey = false
	m.conflict = ""
	m.status = m.keybindings[m.captureIdx].Label + " → " + captured
	return m, nil
}

func bindingKey(cfg *config.Config, binding Keybinding) string {
	if cfg != nil && cfg.Settings != nil {
		if value := cfg.Settings["keys."+binding.Action]; value != "" {
			return value
		}
	}
	return binding.Default
}

func (m Model) escape() (Model, tea.Cmd) {
	if m.previewing {
		m.previewing = false
		SetValue(m.cfg, "appearance.theme", m.originalTheme)
		m.cfg.Theme = m.originalTheme
		m.selectTheme(m.originalTheme)
		return m, func() tea.Msg { return RevertThemeMsg{Name: m.originalTheme} }
	}
	if m.providerDetail {
		m.providerDetail = false
		m.selectedField = 0
		return m, nil
	}
	if m.searchActive {
		m.searchActive = false
		m.search.Blur()
		m.search.Reset()
		m.query = ""
		m.setSection(m.searchSavedSection)
		m.selectedField = m.searchSavedField
		if fields := m.currentFields(); len(fields) > 0 && m.selectedField >= len(fields) {
			m.selectedField = 0
		}
		return m, nil
	}
	return m, func() tea.Msg { return CloseMsg{} }
}

// markDirty records the section that owns an unsaved change for the sidebar.
func (m *Model) markDirty(section SectionID) {
	m.dirty = true
	if m.dirtySections == nil {
		m.dirtySections = make(map[SectionID]bool)
	}
	m.dirtySections[section] = true
}

func (m Model) save() (Model, tea.Cmd) {
	EnsureDefaults(m.cfg)
	m.cfg.SettingsVersion = SchemaVersion
	err := m.cfg.Save()
	m.dirty = err != nil
	if err == nil {
		m.dirtySections = make(map[SectionID]bool)
	}
	if err != nil {
		m.status = "Save failed: " + err.Error()
	} else {
		m.status = "Settings saved"
	}
	return m, func() tea.Msg { return SaveMsg{Err: err} }
}

func nextOption(field Field, current string) string {
	return cycleOption(field, current, 1)
}

// cycleOption advances a select value by delta and wraps in both directions.
func cycleOption(field Field, current string, delta int) string {
	if len(field.Options) == 0 {
		return current
	}
	for i, option := range field.Options {
		if option.Value == current {
			next := (i + delta) % len(field.Options)
			if next < 0 {
				next += len(field.Options)
			}
			return field.Options[next].Value
		}
	}
	return field.Options[0].Value
}

func optionLabel(field Field, value string) string {
	for _, option := range field.Options {
		if option.Value == value {
			return option.Label
		}
	}
	return value
}

func (m Model) updateConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.confirm = nil
		return m, nil
	case "enter":
		if m.confirm == nil {
			return m, nil
		}
		if strings.TrimSpace(m.confirm.Input.Value()) != m.confirm.Word {
			m.confirm.Err = "Type " + m.confirm.Word + " to confirm"
			continueInput := m.confirm.Input
			m.confirm.Input = continueInput
			return m, nil
		}
		action := m.confirm.Action
		m.confirm = nil
		return m, func() tea.Msg { return ActionMsg{Action: action} }
	}
	var cmd tea.Cmd
	m.confirm.Input, cmd = m.confirm.Input.Update(msg)
	return m, cmd
}

func (m *Model) openConfirm(action, title, word string) {
	input := textinput.New()
	input.Placeholder = word
	input.CharLimit = 32
	input.Focus()
	m.confirm = &Confirm{Action: action, Title: title, Word: word, Input: input}
}

// Config returns the mutated config for app-level persistence or inspection.
func (m Model) Config() *config.Config { return m.cfg }

// SecretMasked returns a safe masked key for one provider.
func (m Model) SecretMasked(provider string) string {
	if m.secrets == nil {
		return ""
	}
	return m.secrets.Masked(provider)
}

// CoachReady reports whether the selected Socratic Coach provider can run now.
// A selected local provider is always ready; otherwise the provider must have a
// saved key. Auto mode is ready when any non-local provider has a saved key.
func (m Model) CoachReady() bool {
	selected := CoachProviderID(m.cfg)
	if selected != "" {
		for _, p := range Providers() {
			if p.ID == selected {
				if p.Ollama {
					return true
				}
				return m.SecretMasked(p.ID) != ""
			}
		}
	}
	for _, p := range Providers() {
		if !p.Ollama && m.SecretMasked(p.ID) != "" {
			return true
		}
	}
	return false
}

// SecretsAvailable reports whether the encrypted fallback vault is usable.
func (m Model) SecretsAvailable() bool { return m.secrets != nil }

// SetActiveTheme keeps the Settings theme grid in sync with a theme applied
// elsewhere (for example a /themes quick action). It does not write config.
func (m *Model) SetActiveTheme(name string) {
	m.selectTheme(name)
	m.themes = theme.All()
	m.originalTheme = name
	m.previewing = false
}

// ActiveTheme returns the theme currently selected in the Appearance grid.
func (m Model) ActiveTheme() string {
	if len(m.themes) == 0 {
		return ""
	}
	return m.currentTheme().Name
}

// SetActiveProvider selects one provider in the AI provider stack.
func (m *Model) SetActiveProvider(id string) {
	provider, ok := ProviderByID(id)
	if !ok {
		return
	}
	for i, candidate := range m.providers {
		if candidate.ID == provider.ID {
			m.providerIndex = i
			return
		}
	}
}

// SetProviderSecret writes an API key to the shared encrypted vault. It is the
// same store used by Settings > AI Models, so quick-connect and Settings always
// observe the same connection.
func (m *Model) SetProviderSecret(providerID, key string) error {
	provider, ok := ProviderByID(providerID)
	if !ok {
		return fmt.Errorf("unknown provider %q", providerID)
	}
	if m.secrets == nil {
		return fmt.Errorf("secret vault is unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if err := m.secrets.Set(provider.ID, key); err != nil {
		return err
	}
	if m.cfg != nil {
		delete(m.cfg.Settings, providerModelsCacheKey(provider.ID))
	}
	delete(m.providerStatus, provider.ID)
	m.markDirty(SectionAI)
	return nil
}

// ActiveModelProvider returns the provider whose models the Coach will use.
func (m Model) ActiveModelProvider() Provider {
	return m.modelProvider()
}

// ActiveModelOptions returns the model list for the active provider.
func (m Model) ActiveModelOptions() []Option {
	return m.modelOptions()
}

// ProviderModelOptions returns the model list for one provider, preserving the
// configured model first and then static and discovered entries.
func (m Model) ProviderModelOptions(providerID string) []Option {
	provider, ok := ProviderByID(providerID)
	if !ok {
		return nil
	}
	discovered := m.providerStatus[provider.ID].Models
	if len(discovered) == 0 {
		discovered = CachedProviderModels(m.cfg, provider.ID)
	}
	active := ProviderModel(m.cfg, provider)
	names := ProviderModels(provider, active, discovered)
	options := make([]Option, 0, len(names))
	for _, name := range names {
		options = append(options, Option{Value: name, Label: name})
	}
	return options
}

// SetActiveModel updates the shared ai.provider.<id>.model setting used by both
// Settings and the /model quick action. It does not persist config.
func (m *Model) SetActiveModel(model string) {
	provider := m.modelProvider()
	SetValue(m.cfg, "ai.provider."+provider.ID+".model", model)
	m.markDirty(SectionAI)
	m.status = fmt.Sprintf("%s · model: %s", provider.Label, model)
}

// StorageSize returns the current size of the config/data directory when it is
// available. It is intentionally best-effort for the Settings card.
func (m Model) StorageSize() int64 {
	dir := m.cfg.DataDir()
	if dir == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, statErr := entry.Info(); statErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
