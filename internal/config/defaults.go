package config

// DefaultConfig returns a Config initialized with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Theme:            "openleet",
		DefaultLanguage:  "python",
		OllamaEndpoint:   "http://localhost:11434",
		OllamaModel:      "qwen2.5-coder:7b",
		ExecutionTimeout: 5,
		ShowLineNumbers:  true,
		AutoSave:         true,
		SettingsVersion:  1,
		Settings:         make(map[string]string),
	}
}
