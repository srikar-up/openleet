package ui

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/model"
	"openleet/internal/ui/commands"
	"openleet/internal/ui/quickdialog"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/toast"
)

// quickState owns the transient command palette, one quick dialog, and the
// one-line toast. None of these change currentView.
type quickState struct {
	paletteOpen bool
	palette     quickdialog.Model
	context     commands.Context

	dialogOpen    bool
	dialog        quickdialog.Model
	action        commands.Action
	stage         int
	provider      settings.Provider
	baseURL       string
	key           string
	originalTheme string
}

const (
	connectStageProvider = iota
	connectStageEndpoint
	connectStageKey
	connectStageModel
)

type quickHandler func(AppModel, string) (AppModel, tea.Cmd)

var quickHandlers = map[commands.Action]quickHandler{
	commands.ActionPickProblem:   AppModel.handlePickProblem,
	commands.ActionRoadmap:       AppModel.handleRoadmap,
	commands.ActionGoogleLogin:   AppModel.handleGoogleLogin,
	commands.ActionRandom:        AppModel.handleRandom,
	commands.ActionFilter:        AppModel.handleFilter,
	commands.ActionTopic:         AppModel.handleTopic,
	commands.ActionThemes:        AppModel.handleThemes,
	commands.ActionLang:          AppModel.handleLang,
	commands.ActionConnect:       AppModel.handleConnect,
	commands.ActionModel:         AppModel.handleModel,
	commands.ActionStats:         AppModel.handleStats,
	commands.ActionSettings:      AppModel.handleSettings,
	commands.ActionThemeAudit:    AppModel.handleThemeAudit,
	commands.ActionNotifications: AppModel.handleNotifications,
	commands.ActionNotifyTest:    AppModel.handleNotifyTest,
	commands.ActionUpdate:        AppModel.handleUpdate,
	commands.ActionQuit:          AppModel.handleQuit,
	commands.ActionRun:           AppModel.handleRun,
	commands.ActionSave:          AppModel.handleSave,
	commands.ActionCoach:         AppModel.handleCoach,
	commands.ActionHint:          AppModel.handleHint,
	commands.ActionReset:         AppModel.handleReset,
}

// commandContext maps the active screen onto the command catalog.
func (m AppModel) commandContext() commands.Context {
	switch m.currentView {
	case ViewHome:
		return commands.ContextHome
	case ViewSolve:
		return commands.ContextSolve
	case ViewRoadmap:
		return commands.ContextRoadmap
	case ViewStats:
		return commands.ContextStats
	case ViewSettings:
		return commands.ContextSettings
	default:
		return commands.ContextHome
	}
}

// openCommandPalette builds the searchable command list for the current screen.
func (m AppModel) openCommandPalette() AppModel {
	ctx := m.commandContext()
	m.quick.context = ctx
	m.quick.palette = quickdialog.NewList(
		quickdialog.Options{Title: "COMMANDS", Search: true, Placeholder: "Search commands"},
		commandGroups(ctx),
		m.palette,
		m.width,
		m.height,
	)
	m.quick.paletteOpen = true
	m.quick.dialogOpen = false
	return m
}

func commandGroups(ctx commands.Context) []quickdialog.Group {
	var quickActions, manage []quickdialog.Item
	for _, cmd := range commands.CommandsFor(ctx) {
		item := quickdialog.Item{
			ID:    string(cmd.Action),
			Label: cmd.Name,
			Value: cmd.Description,
		}
		if cmd.Action == commands.ActionSettings {
			manage = append(manage, item)
		} else {
			quickActions = append(quickActions, item)
		}
	}
	groups := make([]quickdialog.Group, 0, 2)
	if len(quickActions) > 0 {
		groups = append(groups, quickdialog.Group{Title: "QUICK ACTIONS", Items: quickActions})
	}
	if len(manage) > 0 {
		groups = append(groups, quickdialog.Group{Title: "MANAGE", Items: manage})
	}
	return groups
}

func lookupCommand(ctx commands.Context, actionID string) (commands.Command, bool) {
	for _, cmd := range commands.CommandsFor(ctx) {
		if string(cmd.Action) == actionID {
			return cmd, true
		}
	}
	return commands.Command{}, false
}

// parseTypedCommand resolves a palette query that starts with "/" into a
// command plus its argument, e.g. "/settings ai". Only command names and
// aliases valid in the active context match; any other query keeps filtering
// the visible list as usual.
func parseTypedCommand(ctx commands.Context, query string) (commands.Command, string, bool) {
	trimmed := strings.TrimSpace(query)
	if !strings.HasPrefix(trimmed, "/") {
		return commands.Command{}, "", false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return commands.Command{}, "", false
	}
	name := fields[0]
	for _, cmd := range commands.CommandsFor(ctx) {
		if strings.EqualFold(cmd.Name, name) {
			return cmd, strings.Join(fields[1:], " "), true
		}
		for _, alias := range cmd.Aliases {
			if strings.EqualFold(alias, name) {
				return cmd, strings.Join(fields[1:], " "), true
			}
		}
	}
	return commands.Command{}, "", false
}

// executeCommand is the only dispatch point for slash commands. The handler
// table is tested against every command in every context.
func (m AppModel) executeCommand(cmd commands.Command, argument string) (AppModel, tea.Cmd) {
	handler, ok := quickHandlers[cmd.Action]
	if !ok {
		return m, m.toast("No handler for " + cmd.Name)
	}
	return handler(m, argument)
}

// handleQuickKey owns the keyboard while the command palette or a quick dialog
// is open. The screen underneath keeps its focus, scroll, cursor, and buffers.
func (m AppModel) handleQuickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.quick.paletteOpen {
		switch msg.String() {
		case "esc", "ctrl+p":
			m.quick.paletteOpen = false
			return m, nil
		case "enter":
			if cmd, argument, ok := parseTypedCommand(m.quick.context, m.quick.palette.SearchValue()); ok {
				m.quick.paletteOpen = false
				return m.executeCommand(cmd, argument)
			}
			item, ok := m.quick.palette.SelectedItem()
			if !ok {
				return m, nil
			}
			cmd, ok := lookupCommand(m.quick.context, item.ID)
			if !ok {
				return m, nil
			}
			m.quick.paletteOpen = false
			return m.executeCommand(cmd, "")
		}
		var cmd tea.Cmd
		m.quick.palette, cmd = m.quick.palette.Update(msg)
		return m, cmd
	}
	if m.quick.dialogOpen {
		return m.updateQuickDialog(msg)
	}
	return m, nil
}

func (m AppModel) updateQuickDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.quick.action == commands.ActionReset {
		switch key {
		case "esc":
			m.closeQuickDialog()
			return m, nil
		case "enter":
			if m.quick.dialog.Confirmed() {
				cmd := m.workspace.ResetCode()
				m.closeQuickDialog()
				return m, tea.Batch(cmd, m.toast("Code reset"))
			}
			m.closeQuickDialog()
			return m, nil
		}
		var cmd tea.Cmd
		m.quick.dialog, cmd = m.quick.dialog.Update(msg)
		return m, cmd
	}

	switch key {
	case "esc":
		return m.quickDialogBack()
	case "enter":
		return m.quickDialogEnter()
	}

	previous := m.quick.dialog.SelectedID()
	var cmd tea.Cmd
	m.quick.dialog, cmd = m.quick.dialog.Update(msg)
	if m.quick.action == commands.ActionThemes {
		if next := m.quick.dialog.SelectedID(); next != "" && next != previous {
			m.applyPaletteColors(theme.Resolve(next))
			m.quick.dialog.SetPalette(m.palette)
		}
	}
	return m, cmd
}

func (m AppModel) quickDialogEnter() (AppModel, tea.Cmd) {
	item, _ := m.quick.dialog.SelectedItem()
	switch m.quick.action {
	case commands.ActionThemes:
		name := item.ID
		if name == "" {
			return m, nil
		}
		pal := theme.Resolve(name)
		m.applyPalette(pal.Name)
		m.quick.dialogOpen = false
		m.quick.originalTheme = ""
		return m, m.notify(toastInfo, "Theme changed", pal.Label, toast.Options{})

	case commands.ActionLang:
		lang := item.ID
		if lang == "" {
			return m, nil
		}
		m.setDefaultLanguage(lang)
		var cmd tea.Cmd
		if m.currentView == ViewSolve {
			cmd = m.workspace.SwitchLanguage(lang)
		}
		m.quick.dialogOpen = false
		return m, tea.Batch(cmd, m.notify(toastInfo, "Language changed", quickLanguageLabel(lang), toast.Options{}))

	case commands.ActionFilter:
		value := item.ID
		m.mapView.SetFilter(value)
		m.quick.dialogOpen = false
		label := "All problems"
		if value != "" && !strings.EqualFold(value, "all") {
			label = capitalize(value)
		}
		return m, m.notify(toastInfo, "Filter changed", label, toast.Options{})

	case commands.ActionTopic:
		value := item.ID
		m.mapView.SetFilter(value)
		m.quick.dialogOpen = false
		label := "All topics"
		if value != "" && !strings.EqualFold(value, "all") {
			label = value
		}
		return m, m.notify(toastInfo, "Topic changed", label, toast.Options{})

	case commands.ActionPickProblem:
		if item.ID == "" {
			return m, nil
		}
		for i, p := range m.problems {
			if p.ID == item.ID {
				m.selectedIndex = i
				m.mapView.SelectProblem(p.ID)
				m.quick.dialogOpen = false
				return m, m.toast(fmt.Sprintf("Selected: #%s %s · F2 to solve", p.ID, p.Title))
			}
		}
		return m, nil

	case commands.ActionStats:
		m.quick.dialogOpen = false
		return m, nil

	case commands.ActionConnect:
		return m.connectEnter(item)
	case commands.ActionModel:
		provider := m.settingsView.ActiveModelProvider()
		modelName := item.ID
		if modelName == "" {
			return m, nil
		}
		settings.SetValue(m.config, "ai.provider."+provider.ID+".model", modelName)
		m.settingsView.SetActiveProvider(provider.ID)
		m.settingsView.SetActiveModel(modelName)
		m.refreshCoachFromConfig()
		_ = m.config.Save()
		m.quick.dialogOpen = false
		return m, m.notify(toastInfo, "Coach model changed", modelName, toast.Options{})
	}
	m.quick.dialogOpen = false
	return m, nil
}

func (m AppModel) quickDialogBack() (AppModel, tea.Cmd) {
	switch m.quick.action {
	case commands.ActionThemes:
		if m.quick.originalTheme != "" {
			m.applyPaletteColors(theme.Resolve(m.quick.originalTheme))
		}
		m.closeQuickDialog()
		return m, nil

	case commands.ActionConnect:
		switch m.quick.stage {
		case connectStageModel:
			if m.quick.provider.Ollama {
				m.openConnectProviderDialog()
			} else {
				m.openConnectKeyDialog()
			}
			return m, nil
		case connectStageKey:
			if m.quick.provider.ID == "custom" {
				m.openConnectEndpointDialog()
			} else {
				m.openConnectProviderDialog()
			}
			return m, nil
		case connectStageEndpoint:
			m.openConnectProviderDialog()
			return m, nil
		default:
			m.closeQuickDialog()
			return m, nil
		}
	}
	m.closeQuickDialog()
	return m, nil
}

func (m *AppModel) closeQuickDialog() {
	m.quick.dialogOpen = false
	m.quick.action = ""
	m.quick.stage = 0
	m.quick.provider = settings.Provider{}
	m.quick.baseURL = ""
	m.quick.key = ""
	m.quick.originalTheme = ""
}

// ---------------------------------------------------------------------------
// Command handlers

func (m AppModel) handleThemes(string) (AppModel, tea.Cmd) {
	items := make([]quickdialog.Item, 0, len(theme.All()))
	for _, pal := range theme.All() {
		items = append(items, quickdialog.Item{
			ID:     pal.Name,
			Label:  pal.Label,
			Value:  pal.Description,
			Active: pal.Name == m.palette.Name,
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "THEMES", Search: true, Placeholder: "Search themes"},
		[]quickdialog.Group{{Title: "PALETTES", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionThemes
	m.quick.originalTheme = m.palette.Name
	return m, nil
}

func (m AppModel) handleLang(string) (AppModel, tea.Cmd) {
	languages := m.workspace.Languages()
	if len(languages) == 0 {
		languages = []string{"python", "go", "cpp", "rust", "javascript", "typescript"}
	}
	active := m.config.DefaultLanguage
	if m.currentView == ViewSolve {
		active = m.workspace.Language()
	}
	items := make([]quickdialog.Item, 0, len(languages))
	for _, lang := range languages {
		items = append(items, quickdialog.Item{
			ID:     lang,
			Label:  quickLanguageLabel(lang),
			Value:  m.interpreterPath(lang),
			Active: lang == active,
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "LANGUAGES", Search: true, Placeholder: "Search languages"},
		[]quickdialog.Group{{Title: "LANGUAGES", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionLang
	return m, nil
}

func (m AppModel) handleConnect(string) (AppModel, tea.Cmd) {
	m.quick.provider = settings.Provider{}
	m.quick.baseURL = ""
	m.quick.key = ""
	m.quick.stage = connectStageProvider
	m.openConnectProviderDialog()
	return m, nil
}

func (m *AppModel) openConnectProviderDialog() {
	popularOrder := []string{"opencode", "openai", "groq", "anthropic", "gemini"}
	servicesOrder := []string{"nvidia", "openrouter", "ollama", "custom"}
	byID := make(map[string]settings.Provider)
	for _, p := range settings.Providers() {
		byID[p.ID] = p
	}
	activeID := settings.CoachProviderID(m.config)
	build := func(order []string) []quickdialog.Item {
		items := make([]quickdialog.Item, 0, len(order))
		for _, id := range order {
			provider, ok := byID[id]
			if !ok {
				continue
			}
			status := "not connected"
			if provider.Ollama {
				status = "local"
			} else if m.settingsView.SecretMasked(provider.ID) != "" {
				status = "connected"
			}
			items = append(items, quickdialog.Item{
				ID:     provider.ID,
				Label:  provider.Label,
				Value:  status,
				Active: provider.ID == activeID,
			})
		}
		return items
	}
	groups := []quickdialog.Group{
		{Title: "POPULAR", Items: build(popularOrder)},
		{Title: "SERVICES", Items: build(servicesOrder)},
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "CONNECT PROVIDER", Search: true, Placeholder: "Search providers"},
		groups,
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionConnect
	m.quick.stage = connectStageProvider
}

func (m *AppModel) openConnectEndpointDialog() {
	m.quick.dialog = quickdialog.NewInput(
		"CONNECT · "+m.quick.provider.Label,
		"Deployment endpoint",
		m.quick.baseURL,
		false,
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.action = commands.ActionConnect
	m.quick.stage = connectStageEndpoint
}

func (m *AppModel) openConnectKeyDialog() {
	m.quick.dialog = quickdialog.NewInput(
		"CONNECT · "+m.quick.provider.Label,
		"API key",
		m.quick.key,
		true,
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.action = commands.ActionConnect
	m.quick.stage = connectStageKey
}

func (m *AppModel) openConnectModelDialog() {
	provider := m.quick.provider
	active := settings.ProviderModel(m.config, provider)
	options := m.settingsView.ProviderModelOptions(provider.ID)
	if len(options) == 0 {
		m.quick.dialog = quickdialog.NewInput(
			"CONNECT · "+provider.Label,
			"Model name",
			active,
			false,
			m.palette,
			m.width,
			m.height,
		)
	} else {
		items := make([]quickdialog.Item, 0, len(options))
		for _, option := range options {
			items = append(items, quickdialog.Item{
				ID:     option.Value,
				Label:  option.Label,
				Active: strings.EqualFold(option.Value, active),
			})
		}
		m.quick.dialog = quickdialog.NewList(
			quickdialog.Options{Title: "CONNECT · " + provider.Label, Search: true, Placeholder: "Search models"},
			[]quickdialog.Group{{Title: "MODELS", Items: items}},
			m.palette,
			m.width,
			m.height,
		)
	}
	m.quick.dialogOpen = true
	m.quick.action = commands.ActionConnect
	m.quick.stage = connectStageModel
}

func (m AppModel) connectEnter(item quickdialog.Item) (AppModel, tea.Cmd) {
	provider := m.quick.provider
	switch m.quick.stage {
	case connectStageProvider:
		selected, ok := settings.ProviderByID(item.ID)
		if !ok {
			return m, nil
		}
		m.quick.provider = selected
		m.quick.baseURL = selected.BaseURL
		m.quick.key = ""
		switch {
		case selected.Ollama:
			m.openConnectModelDialog()
		case selected.ID == "custom":
			m.openConnectEndpointDialog()
		default:
			m.openConnectKeyDialog()
		}
		return m, nil

	case connectStageEndpoint:
		value := strings.TrimSpace(m.quick.dialog.InputValue())
		if value == "" {
			m.quick.dialog.SetError("Endpoint is required")
			return m, nil
		}
		m.quick.baseURL = value
		m.openConnectKeyDialog()
		return m, nil

	case connectStageKey:
		value := strings.TrimSpace(m.quick.dialog.InputValue())
		if value == "" {
			m.quick.dialog.SetError("API key is required")
			return m, nil
		}
		if err := m.settingsView.SetProviderSecret(provider.ID, value); err != nil {
			m.quick.dialog.SetError(err.Error())
			return m, nil
		}
		m.quick.key = value
		m.openConnectModelDialog()
		return m, nil

	case connectStageModel:
		modelName := item.ID
		if m.quick.dialog.Mode() == quickdialog.ModeInput {
			modelName = strings.TrimSpace(m.quick.dialog.InputValue())
		}
		if modelName == "" {
			m.quick.dialog.SetError("Model name is required")
			return m, nil
		}
		if provider.ID == "custom" {
			settings.SetValue(m.config, "ai.provider.custom.base_url", m.quick.baseURL)
		}
		settings.SetValue(m.config, "ai.coach", provider.ID)
		settings.SetValue(m.config, "ai.provider."+provider.ID+".model", modelName)
		m.settingsView.SetActiveProvider(provider.ID)
		m.settingsView.SetActiveModel(modelName)
		m.refreshCoachFromConfig()
		_ = m.config.Save()
		m.closeQuickDialog()
		return m, m.notify(toastPass, "Provider connected", provider.Label, toast.Options{})
	}
	return m, nil
}

func (m AppModel) handleModel(string) (AppModel, tea.Cmd) {
	if !m.coachReady() {
		m.quick.stage = connectStageProvider
		m.openConnectProviderDialog()
		return m, m.toast("Connect an AI provider first")
	}
	provider := m.settingsView.ActiveModelProvider()
	options := m.settingsView.ActiveModelOptions()
	if len(options) == 0 {
		return m.handleConnect("")
	}
	active := settings.ProviderModel(m.config, provider)
	items := make([]quickdialog.Item, 0, len(options))
	for _, option := range options {
		items = append(items, quickdialog.Item{
			ID:     option.Value,
			Label:  option.Label,
			Active: strings.EqualFold(option.Value, active),
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "MODEL · " + provider.Label, Search: true, Placeholder: "Search models"},
		[]quickdialog.Group{{Title: "MODELS", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionModel
	return m, nil
}

func (m AppModel) handleStats(string) (AppModel, tea.Cmd) {
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "STATS", ReadOnly: true, Hint: "↑/↓ scroll  esc close"},
		statsGroups(m.problems, m.userState),
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionStats
	return m, nil
}

func (m AppModel) handleFilter(string) (AppModel, tea.Cmd) {
	current := strings.ToLower(m.mapView.ActiveFilter())
	values := []struct{ id, label string }{
		{"all", "All difficulties"},
		{"easy", "Easy"},
		{"medium", "Medium"},
		{"hard", "Hard"},
	}
	items := make([]quickdialog.Item, 0, len(values))
	for _, value := range values {
		count := len(m.problems)
		if value.id != "all" {
			count = 0
			for _, p := range m.problems {
				if strings.EqualFold(string(p.Difficulty), value.id) {
					count++
				}
			}
		}
		items = append(items, quickdialog.Item{
			ID:     value.id,
			Label:  value.label,
			Value:  problemCount(count),
			Active: current == value.id || (value.id == "all" && (current == "" || current == "all")),
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "FILTER DIFFICULTY", Search: true, Placeholder: "Search difficulty"},
		[]quickdialog.Group{{Title: "DIFFICULTY", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionFilter
	return m, nil
}

func (m AppModel) handleTopic(string) (AppModel, tea.Cmd) {
	counts := make(map[string]int)
	for _, p := range m.problems {
		for _, topic := range p.Topics {
			key := strings.ToLower(strings.TrimSpace(topic))
			if key != "" {
				counts[key]++
			}
		}
	}
	topics := make([]string, 0, len(counts))
	for topic := range counts {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	current := strings.ToLower(m.mapView.ActiveFilter())
	items := []quickdialog.Item{{
		ID:     "all",
		Label:  "All topics",
		Value:  problemCount(len(m.problems)),
		Active: current == "" || current == "all",
	}}
	for _, topic := range topics {
		items = append(items, quickdialog.Item{
			ID:     topic,
			Label:  topic,
			Value:  problemCount(counts[topic]),
			Active: current == topic,
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "FILTER TOPIC", Search: true, Placeholder: "Search topics"},
		[]quickdialog.Group{{Title: "TOPICS", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionTopic
	return m, nil
}

// handleRoadmap is one of the two deliberate navigation actions. It leaves the
// editor buffer, cursor, and console untouched for when the user returns.
// handleGoogleLogin starts the browser flow from any screen without changing
// the active view. A Google sign-in failure is reported as a toast.
func (m AppModel) handleGoogleLogin(string) (AppModel, tea.Cmd) {
	if m.account != nil {
		return m, m.notify(toastInfo, "Already signed in", m.account.Email, toast.Options{})
	}
	m.settingsView.SetGoogleBusy(true)
	return m, tea.Batch(m.googleAuthCmd(), m.notify(toastInfo, "Signing in", "Opening Google sign-in in your browser…", toast.Options{}))
}

func (m AppModel) handleRoadmap(string) (AppModel, tea.Cmd) {
	if m.currentView == ViewRoadmap {
		return m, nil
	}
	m.currentView = ViewRoadmap
	return m, nil
}

func (m AppModel) handlePickProblem(string) (AppModel, tea.Cmd) {
	groups := []quickdialog.Group{}
	for _, difficulty := range []model.Difficulty{model.DifficultyEasy, model.DifficultyMedium, model.DifficultyHard} {
		items := make([]quickdialog.Item, 0)
		for i, p := range m.problems {
			if p.Difficulty != difficulty {
				continue
			}
			items = append(items, quickdialog.Item{
				ID:     p.ID,
				Label:  fmt.Sprintf("#%s %s", p.ID, p.Title),
				Value:  string(p.Difficulty),
				Active: i == m.selectedIndex,
			})
		}
		if len(items) > 0 {
			groups = append(groups, quickdialog.Group{Title: strings.ToUpper(string(difficulty)), Items: items})
		}
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "SOLVE · PICK PROBLEM", Search: true, Placeholder: "Search problems"},
		groups,
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionPickProblem
	return m, nil
}

func (m AppModel) handleRandom(string) (AppModel, tea.Cmd) {
	if len(m.problems) == 0 {
		return m, m.toast("No problems available")
	}
	pool := make([]int, 0, len(m.problems))
	for i, p := range m.problems {
		if m.userState == nil || !m.userState.HasSolved(p.ID) {
			pool = append(pool, i)
		}
	}
	if len(pool) == 0 {
		for i := range m.problems {
			pool = append(pool, i)
		}
	}
	index := pool[rand.Intn(len(pool))]
	m.selectedIndex = index
	p := m.problems[index]
	m.mapView.SelectProblem(p.ID)
	return m, m.toast(fmt.Sprintf("Random: #%s %s · F2 to solve", p.ID, p.Title))
}

func (m AppModel) handleRun(string) (AppModel, tea.Cmd) {
	if m.workspace.Problem() == nil {
		return m, m.toast("No problem selected")
	}
	cmd := m.workspace.RunTests()
	return m, tea.Batch(cmd, m.toast("Running tests…"))
}

func (m AppModel) handleSave(string) (AppModel, tea.Cmd) {
	cmd := m.workspace.RequestSave()
	if cmd == nil {
		return m, m.toast("Nothing to save")
	}
	return m, tea.Batch(cmd, m.toast("Saving…"))
}

func (m AppModel) handleReset(string) (AppModel, tea.Cmd) {
	m.quick.dialog = quickdialog.NewConfirm(
		"RESET CODE",
		"Reset the current code buffer to its starter template?",
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionReset
	return m, nil
}

func (m AppModel) handleCoach(string) (AppModel, tea.Cmd) {
	if !m.coachReady() {
		m.quick.stage = connectStageProvider
		m.openConnectProviderDialog()
		return m, m.toast("Connect an AI provider first")
	}
	if m.currentView != ViewSolve {
		return m, m.toast("Open Solve to use the Coach")
	}
	m.workspace.ShowCoach()
	return m, m.toast("Coach")
}

func (m AppModel) handleHint(string) (AppModel, tea.Cmd) {
	if m.currentView != ViewSolve {
		return m, m.toast("Open Solve to use hints")
	}
	m.workspace.ShowHint()
	return m, m.toast("Hint")
}

func (m AppModel) handleSettings(argument string) (AppModel, tea.Cmd) {
	return m.openSettings(argument), nil
}

// handleNotifications opens the bounded notification history.
func (m AppModel) handleNotifications(string) (AppModel, tea.Cmd) {
	return m.openNotifications()
}

// handleNotifyTest fires one toast of every kind when the debug flag is on.
func (m AppModel) handleNotifyTest(string) (AppModel, tea.Cmd) {
	if !commands.NotifyTestEnabled() {
		return m, m.notify(toastWarn, "Notification test disabled", "Set OPENLEET_DEBUG=1 to enable /notify-test.", toast.Options{})
	}
	return m, m.fireNotificationTest()
}

// handleUpdate opens the update status popup.
func (m AppModel) handleUpdate(string) (AppModel, tea.Cmd) {
	return m.openUpdateDialog()
}

// handleThemeAudit shows the pass/fail table for every registered theme.
func (m AppModel) handleThemeAudit(string) (AppModel, tea.Cmd) {
	rows := runThemeAudit()
	passed := 0
	items := make([]quickdialog.Item, 0, len(rows))
	for _, row := range rows {
		status := "PASS"
		if row.pass() {
			passed++
		} else {
			status = "FAIL · " + strings.Join(row.Failures, "; ")
		}
		items = append(items, quickdialog.Item{
			ID:     row.Name,
			Label:  row.Label,
			Value:  status,
			Active: row.pass(),
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "THEME AUDIT", ReadOnly: true, Hint: "esc close"},
		[]quickdialog.Group{{Title: "THEMES", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionThemeAudit
	if passed == len(rows) {
		return m, m.notify(toastPass, "Theme audit complete", fmt.Sprintf("%d/%d themes passed", passed, len(rows)), toast.Options{})
	}
	return m, m.notify(toastFail, "Theme audit failed", fmt.Sprintf("%d/%d themes passed · %d failed", passed, len(rows), len(rows)-passed), toast.Options{})
}

func (m AppModel) handleQuit(string) (AppModel, tea.Cmd) {
	next, cmd := m.requestQuit()
	return next.(AppModel), cmd
}

// ---------------------------------------------------------------------------
// Shared group builders

func statsGroups(problems []*model.Problem, state *model.UserState) []quickdialog.Group {
	solved := 0
	currentStreak, longestStreak := 0, 0
	if state != nil {
		solved = len(state.SolvedProblems)
		currentStreak = state.CurrentStreak
		longestStreak = state.LongestStreak
	}
	byDifficulty := map[model.Difficulty][2]int{}
	for _, p := range problems {
		counts := byDifficulty[p.Difficulty]
		counts[0]++
		if state != nil && state.HasSolved(p.ID) {
			counts[1]++
		}
		byDifficulty[p.Difficulty] = counts
	}

	streakItems := []quickdialog.Item{
		{Label: "Current streak", Value: dayCount(currentStreak)},
		{Label: "Longest streak", Value: dayCount(longestStreak)},
		{Label: "Problems solved", Value: fmt.Sprintf("%d / %d", solved, len(problems))},
	}
	difficultyItems := make([]quickdialog.Item, 0, 3)
	for _, difficulty := range []model.Difficulty{model.DifficultyEasy, model.DifficultyMedium, model.DifficultyHard} {
		counts := byDifficulty[difficulty]
		difficultyItems = append(difficultyItems, quickdialog.Item{
			Label: string(difficulty),
			Value: fmt.Sprintf("%d / %d", counts[1], counts[0]),
		})
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := make([]quickdialog.Item, 0, 7)
	for offset := 6; offset >= 0; offset-- {
		day := today.AddDate(0, 0, -offset)
		count := 0
		if state != nil {
			for _, record := range state.SolvedProblems {
				solvedAt := record.SolvedAt.In(day.Location())
				solvedDay := time.Date(solvedAt.Year(), solvedAt.Month(), solvedAt.Day(), 0, 0, 0, 0, solvedAt.Location())
				if solvedDay.Equal(day) {
					count++
				}
			}
		}
		days = append(days, quickdialog.Item{
			Label: day.Format("Mon 02 Jan"),
			Value: fmt.Sprintf("%d solved", count),
		})
	}

	return []quickdialog.Group{
		{Title: "STREAK", Items: streakItems},
		{Title: "SOLVED BY DIFFICULTY", Items: difficultyItems},
		{Title: "LAST 7 DAYS", Items: days},
	}
}

// ---------------------------------------------------------------------------
// Single-source-of-truth helpers

func (m *AppModel) setDefaultLanguage(lang string) {
	if m.config == nil || strings.TrimSpace(lang) == "" {
		return
	}
	m.config.DefaultLanguage = lang
	if m.config.Settings == nil {
		m.config.Settings = make(map[string]string)
	}
	m.config.Settings["editor.default_language"] = lang
	_ = m.config.Save()
	m.homeView.SetActiveLanguage(lang)
}

func (m AppModel) interpreterPath(lang string) string {
	if m.config != nil {
		switch strings.ToLower(lang) {
		case "python":
			return settings.Value(m.config, settings.Field{ID: "runner.python_path", Default: "python"})
		case "go":
			return settings.Value(m.config, settings.Field{ID: "runner.go_path", Default: "go"})
		}
	}
	switch strings.ToLower(lang) {
	case "cpp", "c++":
		return "g++"
	case "rust":
		return "rustc"
	case "javascript":
		return "node"
	case "typescript":
		return "tsc"
	default:
		return lang
	}
}

func quickLanguageLabel(lang string) string {
	switch strings.ToLower(lang) {
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "cpp", "c++":
		return "C++"
	case "rust":
		return "Rust"
	case "javascript":
		return "JavaScript"
	case "typescript":
		return "TypeScript"
	default:
		return lang
	}
}

func dayCount(value int) string {
	if value == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", value)
}

func problemCount(value int) string {
	if value == 1 {
		return "1 problem"
	}
	return fmt.Sprintf("%d problems", value)
}

func capitalize(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func (m *AppModel) refreshCoachFromConfig() {
	if m.coachMgr == nil {
		return
	}
	local, cloud := buildCoachProviders(m.config)
	m.coachMgr.SetProviders(local, cloud)
}

// ---------------------------------------------------------------------------
// Overlay and toast composition

// overlayPanel dims the current frame and splices a centered panel over it.
func overlayPanel(bg, panel string, width, height int, pal theme.Palette) string {
	if width <= 0 || height <= 0 {
		return bg
	}
	base := fitToGrid(ansi.Strip(bg), width, height)
	lines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	panelW := 0
	for _, line := range panelLines {
		if w := ansi.StringWidth(line); w > panelW {
			panelW = w
		}
	}
	panelH := len(panelLines)
	if panelW <= 0 || panelH <= 0 || panelW > width || panelH > height {
		return bg
	}

	x := (width - panelW) / 2
	y := (height - panelH) / 2
	if y < 0 {
		y = 0
	}

	// A dark ink scrim keeps a light-theme dialog isolated from the bright
	// live-preview behind it. Dark themes already have a dark canvas, so their
	// existing background is the scrim.
	scrimBg := pal.Background
	if pal.Light {
		scrimBg = pal.Text
	}
	dim := lipgloss.NewStyle().
		Foreground(pal.Muted).
		Background(scrimBg)

	for i := range lines {
		if i >= y && i < y+panelH {
			left := ansi.Cut(lines[i], 0, x)
			right := ansi.Cut(lines[i], x+panelW, width)
			lines[i] = dim.Render(left) + panelLines[i-y] + dim.Render(right)
		} else {
			lines[i] = dim.Render(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

// helpGroups builds the "?" keyboard overlay from the editable keybindings.
func (m AppModel) helpGroups() []quickdialog.Group {
	items := make([]quickdialog.Item, 0)
	for _, binding := range settings.DefaultKeybindings() {
		key := binding.Default
		if m.config != nil && m.config.Settings != nil {
			if value := m.config.Settings["keys."+binding.Action]; value != "" {
				key = value
			}
		}
		items = append(items, quickdialog.Item{Label: binding.Label, Value: key})
	}
	return []quickdialog.Group{{Title: "KEYBOARD", Items: items}}
}

func (m AppModel) openHelpDialog() AppModel {
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "KEYS", Search: true, ReadOnly: true, Placeholder: "Search keys", Hint: "↑/↓ scroll  esc close"},
		m.helpGroups(),
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = ""
	return m
}
