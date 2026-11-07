package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "openleet" {
		t.Fatalf("expected theme 'openleet', got %s", cfg.Theme)
	}
	if cfg.DefaultLanguage != "python" {
		t.Fatalf("expected default language 'python', got %s", cfg.DefaultLanguage)
	}
	if cfg.ExecutionTimeout != 5 {
		t.Fatalf("expected timeout 5, got %d", cfg.ExecutionTimeout)
	}
}

func TestGetPaths(t *testing.T) {
	cfgDir := GetConfigDir()
	if cfgDir == "" {
		t.Fatal("expected non-empty config directory")
	}

	dataDir := GetDataDir()
	if dataDir == "" {
		t.Fatal("expected non-empty data directory")
	}

	problemsDir := GetProblemsDir()
	if !filepath.IsAbs(problemsDir) && problemsDir[:1] != "." {
		t.Fatalf("unexpected problems dir: %s", problemsDir)
	}
}

func TestMigrateSettingsLegacyIcons(t *testing.T) {
	cases := map[string]string{
		"unicode": "on",
		"ascii":   "off",
		"on":      "on",
		"auto":    "auto",
	}
	for legacy, want := range cases {
		cfg := DefaultConfig()
		cfg.Settings = map[string]string{"appearance.icons": legacy}
		migrateSettings(cfg)
		if got := cfg.Settings["appearance.icons"]; got != want {
			t.Fatalf("%q migrated to %q, want %q", legacy, got, want)
		}
	}

	cfg := DefaultConfig()
	cfg.Settings = map[string]string{}
	migrateSettings(cfg)
	if _, exists := cfg.Settings["appearance.icons"]; exists {
		t.Fatal("migration must not invent a value when the key is absent")
	}
}

func TestLoadConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	origLocalAppData := os.Getenv("LOCALAPPDATA")
	origXdg := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Setenv("LOCALAPPDATA", origLocalAppData)
		os.Setenv("XDG_CONFIG_HOME", origXdg)
	}()

	os.Setenv("LOCALAPPDATA", tempDir)
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.DefaultLanguage != "python" {
		t.Fatalf("expected default language 'python', got %s", cfg.DefaultLanguage)
	}
}

func TestSavePersistsTheme(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.configDir = dir
	cfg.Theme = "dracula"

	if err := cfg.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if !strings.Contains(string(data), `"theme": "dracula"`) {
		t.Fatalf("saved config does not contain the new theme:\n%s", data)
	}
}

func TestSaveWithoutConfigDirFails(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Save(); err == nil {
		t.Fatal("expected an error when the config directory is unknown")
	}
}
