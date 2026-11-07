package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds user and application configurations.
type Config struct {
	Theme            string `json:"theme"`
	DefaultLanguage  string `json:"default_language"`
	OllamaEndpoint   string `json:"ollama_endpoint"`
	OllamaModel      string `json:"ollama_model"`
	ExecutionTimeout int    `json:"execution_timeout"`
	ShowLineNumbers  bool   `json:"show_line_numbers"`
	AutoSave         bool   `json:"auto_save"`

	// SettingsVersion versions the schema-generated settings map so future
	// releases can migrate values without guessing at missing keys.
	SettingsVersion int               `json:"settings_version"`
	Settings        map[string]string `json:"settings"`

	// Runtime computed directories (not persisted in json)
	configDir string
	dataDir   string
}

// ConfigDir returns the configuration directory path.
func (c *Config) ConfigDir() string {
	return c.configDir
}

// DataDir returns the data directory path.
func (c *Config) DataDir() string {
	return c.dataDir
}

// Load reads configuration from disk or creates default configuration.
func Load() (*Config, error) {
	configDir := GetConfigDir()
	dataDir := GetDataDir()

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	cfgPath := filepath.Join(configDir, "config.json")
	cfg := DefaultConfig()
	cfg.configDir = configDir
	cfg.dataDir = dataDir

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := save(cfg, cfgPath); err != nil {
			return nil, err
		}
	} else {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		cfg.configDir = configDir
		cfg.dataDir = dataDir
	}

	if cfg.Settings == nil {
		cfg.Settings = make(map[string]string)
	}
	migrateSettings(cfg)
	if cfg.SettingsVersion == 0 {
		cfg.SettingsVersion = 1
	}

	return cfg, nil
}

// migrateSettings upgrades legacy values whose meaning changed across schema
// versions. It is idempotent and never rewrites unrelated keys.
func migrateSettings(cfg *Config) {
	switch cfg.Settings["appearance.icons"] {
	case "unicode":
		cfg.Settings["appearance.icons"] = "on"
	case "ascii":
		cfg.Settings["appearance.icons"] = "off"
	}
}

// save writes configuration to disk as formatted JSON.
func save(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Save persists the current configuration to the user's config directory.
func (c *Config) Save() error {
	if c.configDir == "" {
		return fmt.Errorf("config: no configuration directory")
	}
	return save(c, filepath.Join(c.configDir, "config.json"))
}
