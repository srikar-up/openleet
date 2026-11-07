package ui

import (
	"path/filepath"
	"strconv"
	"strings"

	"openleet/internal/coach"
	"openleet/internal/config"
	"openleet/internal/secrets"
	"openleet/internal/ui/settings"
)

// coachAutoCloudPriority orders the providers tried in auto mode. The first
// one with a saved key wins; remaining non-Ollama providers follow in catalog
// order as a tail.
var coachAutoCloudPriority = []string{
	"opencode", "openai", "groq", "anthropic", "gemini", "nvidia", "openrouter",
}

// coachGenerationOptions maps the persisted AI settings into the shared coach
// generation options. A nil config or settings map yields the coach defaults;
// malformed values are ignored so one bad field cannot poison the others.
// The manager clamps out-of-range values itself.
func coachGenerationOptions(cfg *config.Config) coach.GenerationOptions {
	opts := coach.DefaultGenerationOptions()
	if cfg == nil || cfg.Settings == nil {
		return opts
	}
	if raw := strings.TrimSpace(cfg.Settings["ai.temperature"]); raw != "" {
		if temp, err := strconv.ParseFloat(raw, 64); err == nil {
			opts.Temperature = temp
		}
	}
	if raw := strings.TrimSpace(cfg.Settings["ai.max_tokens"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			opts.MaxTokens = n
		}
	}
	opts.IncludeCode = strings.TrimSpace(cfg.Settings["ai.never_send_code"]) != "on"
	return opts
}

// buildCoachProviders resolves the Socratic Coach local/cloud pair from the
// persisted settings and the encrypted vault. The caller owns swapping them
// into the shared manager; nothing here mutates state.
func buildCoachProviders(cfg *config.Config) (coach.CoachProvider, coach.CoachProvider) {
	store := secretStoreFor(cfg)

	if selected := settings.CoachProviderID(cfg); selected != "" {
		if p, ok := settings.ProviderByID(selected); ok {
			if p.Ollama {
				return localCoachProvider(cfg), nil
			}
			// An explicit cloud choice never silently falls back to Ollama.
			return nil, buildCloudProvider(cfg, store, p)
		}
	}

	return localCoachProvider(cfg), autoCloudProvider(cfg, store)
}

// secretStoreFor opens the encrypted vault next to config.json. A config
// without a directory (or a broken vault) yields a nil store, and every
// consumer must tolerate that.
func secretStoreFor(cfg *config.Config) *secrets.Store {
	if cfg == nil || cfg.ConfigDir() == "" {
		return nil
	}
	store, err := secrets.NewStore(filepath.Join(cfg.ConfigDir(), "secrets.enc"))
	if err != nil {
		return nil
	}
	return store
}

// buildCloudProvider constructs the protocol-correct cloud provider for one
// catalog entry, using its saved key and configured base URL/model.
func buildCloudProvider(cfg *config.Config, store *secrets.Store, p settings.Provider) coach.CoachProvider {
	key := ""
	if store != nil {
		key, _ = store.Get(p.ID)
	}
	base := settings.ProviderBase(cfg, p)
	model := settings.ProviderModel(cfg, p)

	switch p.Protocol {
	case settings.ProtocolGemini:
		return coach.NewGeminiProvider(base, key, model)
	case settings.ProtocolAnthropic:
		return coach.NewAnthropicProvider(base, key, model)
	default:
		return coach.NewCloudProvider(base, key, model)
	}
}

// autoCloudProvider picks the first non-Ollama provider with a saved key in
// priority order. With no saved key it still returns an OpenAI-compatible
// CloudProvider so OPENCODE_API_KEY / pass.env can drive the coach.
func autoCloudProvider(cfg *config.Config, store *secrets.Store) coach.CoachProvider {
	byID := make(map[string]settings.Provider, len(settings.Providers()))
	for _, p := range settings.Providers() {
		byID[p.ID] = p
	}

	ordered := make([]settings.Provider, 0, len(byID))
	used := make(map[string]bool, len(byID))
	for _, id := range coachAutoCloudPriority {
		if p, ok := byID[id]; ok {
			ordered = append(ordered, p)
			used[id] = true
		}
	}
	for _, p := range settings.Providers() {
		if !used[p.ID] && !p.Ollama {
			ordered = append(ordered, p)
		}
	}

	if store != nil {
		for _, p := range ordered {
			if store.Has(p.ID) {
				return buildCloudProvider(cfg, store, p)
			}
		}
	}

	return coach.NewCloudProvider("", "", "")
}

// localCoachProvider builds the Ollama local provider. Settings overrides win
// over the legacy top-level config fields, which win over the catalog default.
func localCoachProvider(cfg *config.Config) coach.CoachProvider {
	p, _ := settings.ProviderByID("ollama")
	base := settings.ProviderBase(cfg, p)
	model := settings.ProviderModel(cfg, p)
	if cfg != nil && (cfg.Settings == nil || strings.TrimSpace(cfg.Settings["ai.provider.ollama.base_url"]) == "") {
		if cfg.OllamaEndpoint != "" {
			base = cfg.OllamaEndpoint
		}
	}
	if cfg != nil && (cfg.Settings == nil || strings.TrimSpace(cfg.Settings["ai.provider.ollama.model"]) == "") {
		if cfg.OllamaModel != "" {
			model = cfg.OllamaModel
		}
	}
	return coach.NewOllamaProvider(base, model)
}
