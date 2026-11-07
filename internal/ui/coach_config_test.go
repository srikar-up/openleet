package ui

import (
	"path/filepath"
	"runtime"
	"testing"

	"openleet/internal/coach"
	"openleet/internal/config"
	"openleet/internal/secrets"
	"openleet/internal/ui/settings"
)

// coachTestConfig loads a config rooted in a per-test temp directory, matching
// the pattern used by the settings package tests.
func coachTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("XDG_DATA_HOME", dir)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func coachVault(t *testing.T, cfg *config.Config) *secrets.Store {
	t.Helper()
	store, err := secrets.NewStore(filepath.Join(cfg.ConfigDir(), "secrets.enc"))
	if err != nil {
		t.Fatalf("open secret vault: %v", err)
	}
	return store
}

func TestBuildCoachProvidersGeminiSelection(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	cfg.Settings["ai.coach"] = "gemini"
	if err := coachVault(t, cfg).Set("gemini", "test-key"); err != nil {
		t.Fatal(err)
	}

	local, cloud := buildCoachProviders(cfg)
	if local != nil {
		t.Fatalf("local = %#v, want nil for an explicit cloud choice", local)
	}
	if _, ok := cloud.(*coach.GeminiProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.GeminiProvider", cloud)
	}
}

func TestBuildCoachProvidersAnthropicSelection(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	cfg.Settings["ai.coach"] = "anthropic"
	if err := coachVault(t, cfg).Set("anthropic", "sk-ant-test"); err != nil {
		t.Fatal(err)
	}

	local, cloud := buildCoachProviders(cfg)
	if local != nil {
		t.Fatalf("local = %#v, want nil for an explicit cloud choice", local)
	}
	if _, ok := cloud.(*coach.AnthropicProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.AnthropicProvider", cloud)
	}
}

func TestBuildCoachProvidersNVIDIAUsesOpenAICompatibleCloud(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	cfg.Settings["ai.coach"] = "nvidia"
	if err := coachVault(t, cfg).Set("nvidia", "nvapi-test"); err != nil {
		t.Fatal(err)
	}

	local, cloud := buildCoachProviders(cfg)
	if local != nil {
		t.Fatalf("local = %#v, want nil for an explicit cloud choice", local)
	}
	if _, ok := cloud.(*coach.CloudProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.CloudProvider for NVIDIA NIM", cloud)
	}
}

func TestBuildCoachProvidersExplicitOllama(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	cfg.Settings["ai.coach"] = "ollama"

	local, cloud := buildCoachProviders(cfg)
	if local == nil || local.Name() != "ollama" {
		t.Fatalf("local = %#v, want the ollama provider", local)
	}
	if cloud != nil {
		t.Fatalf("cloud = %#v, want nil beside explicit ollama", cloud)
	}
}

func TestBuildCoachProvidersAutoPrefersPriorityKey(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	store := coachVault(t, cfg)
	if err := store.Set("gemini", "gemini-key"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("opencode", "opencode-key"); err != nil {
		t.Fatal(err)
	}

	local, cloud := buildCoachProviders(cfg)
	if local == nil || local.Name() != "ollama" {
		t.Fatalf("local = %#v, want ollama in auto mode", local)
	}
	// opencode outranks gemini in the auto priority list, so the cloud side
	// must be the OpenAI-compatible client, not the Gemini provider.
	if _, ok := cloud.(*coach.CloudProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.CloudProvider for the opencode key", cloud)
	}
}

func TestBuildCoachProvidersAutoFallsBackToGeminiKey(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	if err := coachVault(t, cfg).Set("gemini", "gemini-key"); err != nil {
		t.Fatal(err)
	}

	_, cloud := buildCoachProviders(cfg)
	if _, ok := cloud.(*coach.GeminiProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.GeminiProvider when only gemini has a key", cloud)
	}
}

func TestBuildCoachProvidersAutoWithoutKeys(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)

	local, cloud := buildCoachProviders(cfg)
	if local == nil {
		t.Fatal("local must still be ollama in auto mode")
	}
	if cloud == nil {
		t.Fatal("cloud must be non-nil so OPENCODE_API_KEY/pass.env can drive the coach")
	}
	if _, ok := cloud.(*coach.CloudProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.CloudProvider", cloud)
	}
}

func TestSecretStoreForEmptyConfigDir(t *testing.T) {
	if got := secretStoreFor(config.DefaultConfig()); got != nil {
		t.Fatal("expected a nil store when the config has no directory")
	}
	if got := secretStoreFor(nil); got != nil {
		t.Fatal("expected a nil store for a nil config")
	}
}

func TestCoachGenerationOptionsMapping(t *testing.T) {
	if got := coachGenerationOptions(nil); got != coach.DefaultGenerationOptions() {
		t.Fatalf("nil config = %#v, want coach defaults", got)
	}

	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	// The schema default sends no code: ai.never_send_code defaults to "on".
	opts := coachGenerationOptions(cfg)
	if opts.Temperature != 0.2 {
		t.Fatalf("default temperature = %v, want 0.2", opts.Temperature)
	}
	if opts.MaxTokens != 1024 {
		t.Fatalf("default max tokens = %d, want 1024", opts.MaxTokens)
	}
	if opts.IncludeCode {
		t.Fatal("schema default ai.never_send_code=on must exclude code")
	}

	cfg.Settings["ai.temperature"] = "0.7"
	cfg.Settings["ai.max_tokens"] = "2048"
	cfg.Settings["ai.never_send_code"] = "off"
	opts = coachGenerationOptions(cfg)
	if opts.Temperature != 0.7 {
		t.Fatalf("temperature = %v, want 0.7", opts.Temperature)
	}
	if opts.MaxTokens != 2048 {
		t.Fatalf("max tokens = %d, want 2048", opts.MaxTokens)
	}
	if !opts.IncludeCode {
		t.Fatal("ai.never_send_code=off must include code")
	}

	cfg.Settings["ai.temperature"] = "hot"
	cfg.Settings["ai.max_tokens"] = "lots"
	opts = coachGenerationOptions(cfg)
	if opts.Temperature != 0.2 || opts.MaxTokens != 1024 {
		t.Fatalf("malformed values should fall back to defaults: %#v", opts)
	}
}

func TestBuildCoachProvidersGroqUsesOpenAICompatibleCloud(t *testing.T) {
	cfg := coachTestConfig(t)
	settings.EnsureDefaults(cfg)
	cfg.Settings["ai.coach"] = "groq"
	if err := coachVault(t, cfg).Set("groq", "gsk_test"); err != nil {
		t.Fatal(err)
	}

	local, cloud := buildCoachProviders(cfg)
	if local != nil {
		t.Fatalf("local = %#v, want nil for an explicit cloud choice", local)
	}
	if _, ok := cloud.(*coach.CloudProvider); !ok {
		t.Fatalf("cloud = %T, want *coach.CloudProvider for Groq", cloud)
	}
}
