// Package settings implements the schema-driven Settings screen.
package settings

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"openleet/internal/config"
)

// SchemaVersion increments whenever a stored settings key changes meaning.
const SchemaVersion = 2

// SectionID identifies a left-rail settings category.
type SectionID string

const (
	SectionGeneral    SectionID = "general"
	SectionAppearance SectionID = "appearance"
	SectionEditor     SectionID = "editor"
	SectionRunner     SectionID = "runner"
	SectionAI         SectionID = "ai"
	SectionKeys       SectionID = "keybindings"
	SectionAccount    SectionID = "account"
	SectionData       SectionID = "data"
	SectionAbout      SectionID = "about"
)

// Section is one entry in the Settings category rail.
type Section struct {
	ID    SectionID
	Label string
	Short string
}

// Kind is the control rendered for a field.
type Kind int

const (
	KindToggle Kind = iota
	KindSelect
	KindStepper
	KindText
	KindSecret
	KindButton
	KindThemeGrid
	KindAccent
	KindKeyTable
	KindAccount
	KindAbout
	KindProviderList
)

// Option is one select/dropdown value.
type Option struct {
	Value string
	Label string
}

// Field is one schema row. Cards group fields into 3-6 row concerns.
type Field struct {
	ID          string
	Section     SectionID
	Card        string
	Label       string
	Help        string
	Kind        Kind
	Default     string
	Options     []Option
	Min         int
	Max         int
	Step        int
	Placeholder string
	Validate    func(string) error
}

// Sections returns the fixed category order.
func Sections() []Section {
	return []Section{
		{SectionGeneral, "GENERAL", "GEN"},
		{SectionAppearance, "APPEARANCE", "THEME"},
		{SectionEditor, "EDITOR", "EDIT"},
		{SectionRunner, "RUNNER", "RUN"},
		{SectionAI, "AI MODELS", "AI"},
		{SectionKeys, "KEYBINDINGS", "KEYS"},
		{SectionAccount, "ACCOUNT", "ACCT"},
		{SectionData, "DATA", "DATA"},
		{SectionAbout, "ABOUT", "ABOUT"},
	}
}

// Fields returns every settings row in authoring order.
func Fields() []Field {
	return []Field{
		// GENERAL
		{ID: "general.startup", Section: SectionGeneral, Card: "Startup", Label: "Startup screen",
			Help: "Screen shown when OpenLeet launches.", Kind: KindSelect, Default: "home",
			Options: []Option{{"home", "Home"}, {"roadmap", "Roadmap"}, {"stats", "Stats"}}},
		{ID: "general.confirm_quit", Section: SectionGeneral, Card: "Startup", Label: "Confirm on quit",
			Help: "Ask before closing the app.", Kind: KindToggle, Default: "off"},

		// APPEARANCE
		{ID: "appearance.theme", Section: SectionAppearance, Card: "Theme", Label: "Theme",
			Help: "Moving the cursor previews live; Enter applies, Esc reverts.", Kind: KindThemeGrid, Default: "openleet"},
		{ID: "appearance.accent", Section: SectionAppearance, Card: "Interface", Label: "Accent",
			Help: "Use a theme accent preset or a six-digit hex value.", Kind: KindAccent, Default: "theme"},
		{ID: "appearance.border", Section: SectionAppearance, Card: "Interface", Label: "Border style",
			Help: "Rounded uses ╭╮; single uses ┌┐.", Kind: KindSelect, Default: "rounded",
			Options: []Option{{"rounded", "Rounded"}, {"single", "Single"}}},
		{ID: "appearance.icons", Section: SectionAppearance, Card: "Interface", Label: "Icons",
			Help: "Auto detects Unicode support; On forces icons; Off keeps text labels only.", Kind: KindSelect, Default: "auto",
			Options: []Option{{"auto", "Auto"}, {"on", "On"}, {"off", "Off"}}},

		// EDITOR
		{ID: "editor.tab_width", Section: SectionEditor, Card: "Typography", Label: "Tab width",
			Help: "Spaces inserted by Tab and used by format.", Kind: KindStepper, Default: "4", Min: 2, Max: 8, Step: 1},
		{ID: "editor.auto_indent", Section: SectionEditor, Card: "Input", Label: "Auto-indent",
			Help: "Maintain block indentation on Enter.", Kind: KindToggle, Default: "on"},
		{ID: "editor.bracket_pairing", Section: SectionEditor, Card: "Input", Label: "Bracket pairing",
			Help: "Insert matching brackets and quotes.", Kind: KindToggle, Default: "on"},
		{ID: "editor.line_numbers", Section: SectionEditor, Card: "Display", Label: "Line numbers",
			Help: "Show the gutter line numbers.", Kind: KindToggle, Default: "on"},
		{ID: "editor.format_on_save", Section: SectionEditor, Card: "Display", Label: "Format on save",
			Help: "Run the language formatter before saving.", Kind: KindToggle, Default: "on"},
		{ID: "editor.default_language", Section: SectionEditor, Card: "Display", Label: "Default language",
			Help: "Starter language for a new problem.", Kind: KindSelect, Default: "python",
			Options: []Option{{"python", "Python"}, {"go", "Go"}, {"cpp", "C++"}, {"rust", "Rust"}, {"javascript", "JavaScript"}, {"typescript", "TypeScript"}}},

		// RUNNER
		{ID: "runner.timeout", Section: SectionRunner, Card: "Execution", Label: "Timeout (seconds)",
			Help: "Maximum wall-clock time for one submission.", Kind: KindStepper, Default: "5", Min: 1, Max: 120, Step: 1},
		{ID: "runner.python_path", Section: SectionRunner, Card: "Interpreters", Label: "Python path",
			Help: "Executable used for Python submissions.", Kind: KindText, Default: "python", Placeholder: "python3"},
		{ID: "runner.go_path", Section: SectionRunner, Card: "Interpreters", Label: "Go path",
			Help: "Go toolchain executable.", Kind: KindText, Default: "go", Placeholder: "go"},
		{ID: "runner.detect", Section: SectionRunner, Card: "Interpreters", Label: "Run diagnostics",
			Help: "Check Python and Go toolchains and report the detected runtime paths.", Kind: KindButton, Default: ""},

		// AI MODELS. ai.providers is authored last so the canvas can order the
		// Integrations card, Assign models, Generation, and Privacy above the
		// provider stack while keyboard navigation still reaches every row.
		{ID: "ai.connect", Section: SectionAI, Card: "Integrations", Label: "Connect an integration",
			Help: "Search popular providers and services. Enter opens API-key setup.", Kind: KindButton, Default: ""},
		{ID: "ai.coach", Section: SectionAI, Card: "Assign models", Label: "Socratic Coach",
			Help: "Selects the provider for coaching; the Model row selects its model.", Kind: KindSelect, Default: "auto",
			Options: CoachOptions()},
		{ID: "ai.model", Section: SectionAI, Card: "Assign models", Label: "Model", Help: "Model used by the selected provider. Test a provider to list its available models.", Kind: KindSelect, Default: ""},
		{ID: "ai.temperature", Section: SectionAI, Card: "Generation", Label: "Temperature",
			Help: "0.0 is deterministic; 1.0 is creative. Range 0–2.", Kind: KindText, Default: "0.2",
			Validate: validateTemperature},
		{ID: "ai.max_tokens", Section: SectionAI, Card: "Generation", Label: "Max tokens",
			Help: "Upper bound for one response.", Kind: KindStepper, Default: "1024", Min: 128, Max: 8192, Step: 128},
		{ID: "ai.never_send_code", Section: SectionAI, Card: "Privacy", Label: "Never send solution code",
			Help: "Removes the editor buffer from every AI request.", Kind: KindToggle, Default: "on"},
		{ID: "ai.providers", Section: SectionAI, Card: "Providers", Label: "Providers",
			Help: "Enter opens the selected provider; left/right changes it.", Kind: KindProviderList, Default: ""},

		// KEYBINDINGS
		{ID: "keys.table", Section: SectionKeys, Card: "Bindings", Label: "Action bindings",
			Help: "Enter captures a new key; conflicts are rejected inline.", Kind: KindKeyTable, Default: ""},

		// ACCOUNT
		{ID: "account.panel", Section: SectionAccount, Card: "Account", Label: "Account",
			Help: "Sign in to sync progress across devices.", Kind: KindAccount, Default: ""},
		{ID: "sync.enabled", Section: SectionAccount, Card: "Sync", Label: "Cloud sync",
			Help: "Off keeps every change on this device. Manual sync is disabled.", Kind: KindToggle, Default: "on"},
		{ID: "sync.policy", Section: SectionAccount, Card: "Sync", Label: "Conflict policy",
			Help: "Newest wins merges cloud progress; local wins pushes this device unchanged.", Kind: KindSelect, Default: "newest",
			Options: []Option{{"newest", "Newest wins"}, {"local", "Local wins"}}},

		// DATA
		{ID: "data.export", Section: SectionData, Card: "Progress", Label: "Export progress",
			Help: "Write JSON and CSV snapshots beside the data directory.", Kind: KindButton, Default: ""},
		{ID: "data.import", Section: SectionData, Card: "Progress", Label: "Import progress",
			Help: "Read openleet-progress.json and restore progress.", Kind: KindButton, Default: ""},
		{ID: "data.backup", Section: SectionData, Card: "Progress", Label: "Backup now",
			Help: "Copy progress, layout, and config into a timestamped folder.", Kind: KindButton, Default: ""},
		{ID: "data.clear_cache", Section: SectionData, Card: "Progress", Label: "Clear cache",
			Help: "Drop the saved workspace layout; progress is kept.", Kind: KindButton, Default: ""},
		{ID: "data.reset_roadmap", Section: SectionData, Card: "Reset", Label: "Reset roadmap progress",
			Help: "Clear solved problems and streaks; keep code buffers.", Kind: KindButton, Default: ""},
		{ID: "data.reset_settings", Section: SectionData, Card: "Reset", Label: "Reset all settings",
			Help: "Restore schema defaults and the OpenLeet theme.", Kind: KindButton, Default: ""},
		{ID: "data.delete_local", Section: SectionData, Card: "Danger zone", Label: "Delete local data",
			Help: "Remove progress, code buffers, and layout. Keys are kept.", Kind: KindButton, Default: ""},

		// ABOUT
		{ID: "about.panel", Section: SectionAbout, Card: "About", Label: "OpenLeet",
			Help: "Version, paths, links, diagnostics.", Kind: KindAbout, Default: ""},
	}
}

func validateTemperature(value string) error {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("enter a number between 0 and 2")
	}
	if parsed < 0 || parsed > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	return nil
}

// ValidateHexColor accepts #RRGGBB or a theme preset name.
func ValidateHexColor(value string) error {
	if value == "" || value == "theme" {
		return nil
	}
	if !strings.HasPrefix(value, "#") {
		return nil
	}
	if ok, _ := regexp.MatchString(`^#[0-9a-fA-F]{6}$`, value); !ok {
		return fmt.Errorf("use #RRGGBB")
	}
	return nil
}

func validateURLOrEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("enter a valid base URL")
	}
	return nil
}

// ProviderProtocol distinguishes API formats across LLM vendors.
type ProviderProtocol string

const (
	ProtocolOpenAI    ProviderProtocol = "openai"
	ProtocolGemini    ProviderProtocol = "gemini"
	ProtocolAnthropic ProviderProtocol = "anthropic"
	ProtocolOllama    ProviderProtocol = "ollama"
)

// Provider describes one AI provider card. Provider secrets live in the
// encrypted vault, never in config.json.
type Provider struct {
	ID           string
	Label        string
	DefaultModel string
	Models       []string
	BaseURL      string
	KeyPrefix    string
	Protocol     ProviderProtocol
	Ollama       bool
	Popular      bool
}

// Providers returns the fixed provider order.
func Providers() []Provider {
	return []Provider{
		{
			ID:           "anthropic",
			Label:        "Anthropic",
			DefaultModel: "claude-sonnet-4-6",
			BaseURL:      "https://api.anthropic.com",
			KeyPrefix:    "sk-ant-",
			Protocol:     ProtocolAnthropic,
			Popular:      true,
		},
		{
			ID:           "openai",
			Label:        "OpenAI",
			DefaultModel: "gpt-4.1",
			Models:       []string{"gpt-4.1", "gpt-4.1-mini", "gpt-4o-mini"},
			BaseURL:      "https://api.openai.com/v1",
			KeyPrefix:    "sk-",
			Protocol:     ProtocolOpenAI,
			Popular:      true,
		},
		{
			ID:           "groq",
			Label:        "Groq",
			DefaultModel: "openai/gpt-oss-120b",
			Models: []string{
				"openai/gpt-oss-120b",
				"openai/gpt-oss-20b",
				"llama-3.3-70b-versatile",
				"llama-3.1-8b-instant",
				"qwen/qwen3.8-27b",
			},
			BaseURL:   "https://api.groq.com/openai/v1",
			KeyPrefix: "gsk_",
			Protocol:  ProtocolOpenAI,
			Popular:   true,
		},
		{
			ID:           "gemini",
			Label:        "Google Gemini",
			DefaultModel: "gemini-3.8-flash",
			Models: []string{
				"gemini-3.8-flash",
				"gemini-3.7-flash",
				"gemini-3.6-flash",
				"gemini-3.5-flash",
				"gemini-3.5-flash-lite",
				"gemini-3.1-flash-lite",
				"gemini-2.5-pro",
			},
			BaseURL:   "https://generativelanguage.googleapis.com",
			KeyPrefix: "AIza",
			Protocol:  ProtocolGemini,
			Popular:   true,
		},
		{
			ID:           "nvidia",
			Label:        "NVIDIA NIM",
			DefaultModel: "nvidia/nemotron-3-super-120b-a12b",
			Models: []string{
				"nvidia/nemotron-3-super-120b-a12b",
				"nvidia/llama-3.1-nemotron-70b-instruct",
				"nvidia/llama-3.1-nemotron-ultra-253b-v1",
				"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning",
			},
			BaseURL:   "https://integrate.api.nvidia.com/v1",
			KeyPrefix: "nvapi-",
			Protocol:  ProtocolOpenAI,
			Popular:   false,
		},
		{
			ID:           "opencode",
			Label:        "OpenCode Console",
			DefaultModel: "deepseek-v4.1-flash-free",
			Models:       []string{"deepseek-v4.1-flash-free", "big-pickle", "beam-free"},
			BaseURL:      "https://opencode.ai/inference/v1",
			KeyPrefix:    "sk-",
			Protocol:     ProtocolOpenAI,
			Popular:      true,
		},
		{
			ID:           "openrouter",
			Label:        "OpenRouter",
			DefaultModel: "openrouter/auto",
			BaseURL:      "https://openrouter.ai/api/v1",
			KeyPrefix:    "sk-or-",
			Protocol:     ProtocolOpenAI,
			Popular:      false,
		},
		{
			ID:           "ollama",
			Label:        "Ollama (local)",
			DefaultModel: "qwen2.5-coder:7b",
			Models:       []string{"qwen2.5-coder:7b", "llama3.1:8b"},
			BaseURL:      "http://localhost:11434",
			KeyPrefix:    "",
			Protocol:     ProtocolOllama,
			Ollama:       true,
			Popular:      false,
		},
		{
			ID:           "custom",
			Label:        "Custom OpenAI-compatible",
			DefaultModel: "",
			BaseURL:      "",
			KeyPrefix:    "sk-",
			Protocol:     ProtocolOpenAI,
			Popular:      false,
		},
	}
}

// ProviderByID searches Providers() by ID (case-insensitive and trimmed).
func ProviderByID(id string) (Provider, bool) {
	needle := strings.ToLower(strings.TrimSpace(id))
	if needle == "" {
		return Provider{}, false
	}
	for _, p := range Providers() {
		if strings.ToLower(p.ID) == needle {
			return p, true
		}
	}
	return Provider{}, false
}

// ProviderBase returns the configured base URL for the provider if non-empty,
// falling back to provider.BaseURL.
func ProviderBase(cfg *config.Config, provider Provider) string {
	key := "ai.provider." + provider.ID + ".base_url"
	if cfg != nil && cfg.Settings != nil {
		if value := strings.TrimSpace(cfg.Settings[key]); value != "" {
			return value
		}
	}
	return provider.BaseURL
}

// ProviderModel returns the configured model for the provider if non-empty,
// falling back to provider.DefaultModel.
func ProviderModel(cfg *config.Config, provider Provider) string {
	key := "ai.provider." + provider.ID + ".model"
	if cfg != nil && cfg.Settings != nil {
		if value := strings.TrimSpace(cfg.Settings[key]); value != "" {
			return value
		}
	}
	return provider.DefaultModel
}

// ProviderModels returns the combined model names for a provider in order:
// configured model, static provider.Models, and discovered models from probes.
// Blanks are trimmed and duplicates are removed case-insensitively.
func ProviderModels(provider Provider, configured string, discovered []string) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(m string) {
		trimmed := strings.TrimSpace(m)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if !seen[key] {
			seen[key] = true
			out = append(out, trimmed)
		}
	}
	add(configured)
	for _, m := range provider.Models {
		add(m)
	}
	for _, m := range discovered {
		add(m)
	}
	return out
}

// CoachProviderID reads ai.coach and returns the valid provider ID,
// or empty string for empty, "auto", or unknown provider IDs.
func CoachProviderID(cfg *config.Config) string {
	if cfg == nil || cfg.Settings == nil {
		return ""
	}
	val := strings.ToLower(strings.TrimSpace(cfg.Settings["ai.coach"]))
	if val == "" || val == "auto" {
		return ""
	}
	if p, ok := ProviderByID(val); ok {
		return p.ID
	}
	return ""
}

// CoachOptions returns the options list for the ai.coach setting:
// "auto" followed by each provider in Providers() order.
func CoachOptions() []Option {
	providers := Providers()
	opts := make([]Option, 0, len(providers)+1)
	opts = append(opts, Option{
		Value: "auto",
		Label: "Auto (connected provider)",
	})
	for _, p := range providers {
		opts = append(opts, Option{
			Value: p.ID,
			Label: p.Label,
		})
	}
	return opts
}

// AIProviderChangedMsg indicates that the active AI provider was changed.
type AIProviderChangedMsg struct {
	ProviderID string
}

// Keybinding is one editable action mapping.
type Keybinding struct {
	Action  string
	Label   string
	Default string
}

// DefaultKeybindings returns the action table shown in Settings.
func DefaultKeybindings() []Keybinding {
	return []Keybinding{
		{"run", "Run tests", "ctrl+r"},
		{"save", "Save buffer", "ctrl+s"},
		{"beautify", "Beautify code", "ctrl+b"},
		{"commands", "Command palette", "ctrl+p"},
		{"settings", "Open settings", "ctrl+,"},
		{"next_problem", "Next problem", "ctrl+n"},
		{"prev_problem", "Previous problem", "ctrl+pgup"},
		{"focus", "Cycle focus", "tab"},
		{"statement", "Toggle statement", "alt+d"},
		{"session", "Toggle session", "alt+s"},
		{"left_pane", "Toggle left pane", "alt+["},
		{"right_pane", "Toggle right pane", "alt+]"},
		{"quit", "Quit", "ctrl+c"},
	}
}

// SectionAliases maps palette deep links to sections.
func SectionAliases() map[string]SectionID {
	return map[string]SectionID{
		"general": SectionGeneral, "appearance": SectionAppearance, "theme": SectionAppearance,
		"editor": SectionEditor, "runner": SectionRunner, "ai": SectionAI, "models": SectionAI,
		"keys": SectionKeys, "keybindings": SectionKeys, "account": SectionAccount,
		"data": SectionData, "about": SectionAbout,
	}
}

// ResolveSection resolves a deep-link argument, defaulting to GENERAL.
func ResolveSection(argument string) SectionID {
	arg := strings.ToLower(strings.TrimSpace(argument))
	arg = strings.TrimPrefix(arg, "/settings")
	arg = strings.TrimSpace(arg)
	if id, ok := SectionAliases()[arg]; ok {
		return id
	}
	return SectionGeneral
}

// EnsureDefaults migrates a config to the current settings schema. Legacy
// top-level fields are copied only when their schema key has never been
// written, so an existing config keeps its theme, language, timeout, and line
// number preference even though config.Load has already stamped a version.
func EnsureDefaults(cfg *config.Config) {
	if cfg.Settings == nil {
		cfg.Settings = make(map[string]string)
	}
	migrateRetiredModels(cfg)
	legacy := map[string]string{
		"appearance.theme":        cfg.Theme,
		"editor.default_language": cfg.DefaultLanguage,
		"runner.timeout":          strconv.Itoa(cfg.ExecutionTimeout),
		"editor.line_numbers":     boolValue(cfg.ShowLineNumbers),
	}
	for id, value := range legacy {
		if _, ok := cfg.Settings[id]; ok {
			continue
		}
		if value != "" {
			cfg.Settings[id] = value
		}
	}
	for _, field := range Fields() {
		if field.Default == "" {
			continue
		}
		if _, ok := cfg.Settings[field.ID]; !ok {
			cfg.Settings[field.ID] = field.Default
		}
	}
	if cfg.SettingsVersion < SchemaVersion {
		cfg.SettingsVersion = SchemaVersion
	}
}

func boolValue(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

// migrateRetiredModels rewrites saved model IDs that the provider no longer
// serves. Gemini 2.5 Pro/Flash/Flash-Lite are closed to new API users and
// return a 404 that recommends a current model.
func migrateRetiredModels(cfg *config.Config) {
	switch cfg.Settings["ai.provider.gemini.model"] {
	case "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite":
		cfg.Settings["ai.provider.gemini.model"] = "gemini-3.8-flash"
	}
}

const maxCachedModels = 200

// providerModelsCacheKey stores a provider's last successful discovery so the
// full model list survives closing Settings and restarting the app.
func providerModelsCacheKey(providerID string) string {
	return "ai.provider." + providerID + ".models_discovered"
}

// CachedProviderModels returns the persisted discovery list for a provider.
func CachedProviderModels(cfg *config.Config, providerID string) []string {
	if cfg == nil || cfg.Settings == nil || providerID == "" {
		return nil
	}
	raw := strings.TrimSpace(cfg.Settings[providerModelsCacheKey(providerID)])
	if raw == "" {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		model := strings.TrimSpace(line)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, model)
		if len(out) >= maxCachedModels {
			break
		}
	}
	return out
}

// SetCachedProviderModels persists a discovery list and removes blanks and
// duplicates. An empty list clears the cache.
func SetCachedProviderModels(cfg *config.Config, providerID string, models []string) {
	if cfg == nil || providerID == "" {
		return
	}
	if cfg.Settings == nil {
		cfg.Settings = make(map[string]string)
	}
	key := providerModelsCacheKey(providerID)
	seen := make(map[string]bool)
	clean := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		lower := strings.ToLower(model)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		clean = append(clean, model)
		if len(clean) >= maxCachedModels {
			break
		}
	}
	if len(clean) == 0 {
		delete(cfg.Settings, key)
		return
	}
	cfg.Settings[key] = strings.Join(clean, "\n")
}

// Value returns a schema value or its default.
func Value(cfg *config.Config, field Field) string {
	if cfg != nil && cfg.Settings != nil {
		if value, ok := cfg.Settings[field.ID]; ok {
			return value
		}
	}
	return field.Default
}

// SetValue writes a schema value and keeps legacy config fields in sync.
func SetValue(cfg *config.Config, id, value string) {
	if cfg.Settings == nil {
		cfg.Settings = make(map[string]string)
	}
	cfg.Settings[id] = value
	switch id {
	case "appearance.theme":
		cfg.Theme = value
	case "editor.default_language":
		cfg.DefaultLanguage = value
	case "editor.line_numbers":
		cfg.ShowLineNumbers = value == "on"
	case "runner.timeout":
		if n, err := strconv.Atoi(value); err == nil {
			cfg.ExecutionTimeout = n
		}
	}
}

// SyncEnabled reports whether cloud sync is active.
func SyncEnabled(cfg *config.Config) bool {
	return Value(cfg, Field{ID: "sync.enabled", Default: "on"}) == "on"
}

// ConflictPolicy returns the conflict resolution policy ("local" or "newest").
func ConflictPolicy(cfg *config.Config) string {
	if Value(cfg, Field{ID: "sync.policy", Default: "newest"}) == "local" {
		return "local"
	}
	return "newest"
}

func fieldByID(id string) (Field, bool) {
	for _, field := range Fields() {
		if field.ID == id {
			return field, true
		}
	}
	return Field{}, false
}

// FieldsForSection returns the fields for one section in authoring order.
func FieldsForSection(section SectionID) []Field {
	var out []Field
	for _, field := range Fields() {
		if field.Section == section {
			out = append(out, field)
		}
	}
	return out
}

// Search returns fields matching query across labels, help, cards, and sections.
func Search(query string) []Field {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	var out []Field
	for _, field := range Fields() {
		haystack := strings.ToLower(field.Label + " " + field.Help + " " + field.Card + " " + string(field.Section) + " " + field.ID)
		if strings.Contains(haystack, query) {
			out = append(out, field)
		}
	}
	return out
}
