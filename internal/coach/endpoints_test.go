package coach

import (
	"testing"
)

func TestOpenAIEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantBase   string
		wantChat   string
		wantModels string
	}{
		{
			name:       "api.openai.com host without version",
			endpoint:   "https://api.openai.com",
			wantBase:   "https://api.openai.com/v1",
			wantChat:   "https://api.openai.com/v1/chat/completions",
			wantModels: "https://api.openai.com/v1/models",
		},
		{
			name:       "api.openai.com with trailing slash",
			endpoint:   "https://api.openai.com/",
			wantBase:   "https://api.openai.com/v1",
			wantChat:   "https://api.openai.com/v1/chat/completions",
			wantModels: "https://api.openai.com/v1/models",
		},
		{
			name:       "openrouter.ai/api path without version",
			endpoint:   "https://openrouter.ai/api",
			wantBase:   "https://openrouter.ai/api/v1",
			wantChat:   "https://openrouter.ai/api/v1/chat/completions",
			wantModels: "https://openrouter.ai/api/v1/models",
		},
		{
			name:       "integrate.api.nvidia.com/v1 already has version",
			endpoint:   "https://integrate.api.nvidia.com/v1",
			wantBase:   "https://integrate.api.nvidia.com/v1",
			wantChat:   "https://integrate.api.nvidia.com/v1/chat/completions",
			wantModels: "https://integrate.api.nvidia.com/v1/models",
		},
		{
			name:       "api.groq.com/openai/v1 already has version",
			endpoint:   "https://api.groq.com/openai/v1",
			wantBase:   "https://api.groq.com/openai/v1",
			wantChat:   "https://api.groq.com/openai/v1/chat/completions",
			wantModels: "https://api.groq.com/openai/v1/models",
		},
		{
			name:       "full /v1/chat/completions suffix",
			endpoint:   "https://api.openai.com/v1/chat/completions",
			wantBase:   "https://api.openai.com/v1",
			wantChat:   "https://api.openai.com/v1/chat/completions",
			wantModels: "https://api.openai.com/v1/models",
		},
		{
			name:       "custom endpoint with query string",
			endpoint:   "https://custom.host/v1beta?foo=bar",
			wantBase:   "https://custom.host/v1beta?foo=bar",
			wantChat:   "https://custom.host/v1beta/chat/completions?foo=bar",
			wantModels: "https://custom.host/v1beta/models?foo=bar",
		},
		{
			name:       "schemeless api.openai.com",
			endpoint:   "api.openai.com",
			wantBase:   "api.openai.com/v1",
			wantChat:   "api.openai.com/v1/chat/completions",
			wantModels: "api.openai.com/v1/models",
		},
		{
			name:       "custom host without version",
			endpoint:   "https://custom.host",
			wantBase:   "https://custom.host/v1",
			wantChat:   "https://custom.host/v1/chat/completions",
			wantModels: "https://custom.host/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase := NormalizeOpenAIBase(tt.endpoint)
			if gotBase != tt.wantBase {
				t.Errorf("NormalizeOpenAIBase(%q) = %q, want %q", tt.endpoint, gotBase, tt.wantBase)
			}
			gotChat := OpenAIChatURL(tt.endpoint)
			if gotChat != tt.wantChat {
				t.Errorf("OpenAIChatURL(%q) = %q, want %q", tt.endpoint, gotChat, tt.wantChat)
			}
			if tt.wantModels != "" {
				gotModels := OpenAIModelsURL(tt.endpoint)
				if gotModels != tt.wantModels {
					t.Errorf("OpenAIModelsURL(%q) = %q, want %q", tt.endpoint, gotModels, tt.wantModels)
				}
			}
		})
	}
}

func TestAnthropicEndpoints(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		wantBase     string
		wantMessages string
		wantModels   string
	}{
		{
			name:         "api.anthropic.com default",
			endpoint:     "https://api.anthropic.com",
			wantBase:     "https://api.anthropic.com/v1",
			wantMessages: "https://api.anthropic.com/v1/messages",
			wantModels:   "https://api.anthropic.com/v1/models",
		},
		{
			name:         "api.anthropic.com/v1 already has version",
			endpoint:     "https://api.anthropic.com/v1",
			wantBase:     "https://api.anthropic.com/v1",
			wantMessages: "https://api.anthropic.com/v1/messages",
			wantModels:   "https://api.anthropic.com/v1/models",
		},
		{
			name:         "api.anthropic.com/v1/messages trailing messages",
			endpoint:     "https://api.anthropic.com/v1/messages",
			wantBase:     "https://api.anthropic.com/v1",
			wantMessages: "https://api.anthropic.com/v1/messages",
			wantModels:   "https://api.anthropic.com/v1/models",
		},
		{
			name:         "api.anthropic.com/messages without v1",
			endpoint:     "https://api.anthropic.com/messages",
			wantBase:     "https://api.anthropic.com/v1",
			wantMessages: "https://api.anthropic.com/v1/messages",
			wantModels:   "https://api.anthropic.com/v1/models",
		},
		{
			name:         "empty endpoint defaults to anthropic v1",
			endpoint:     "",
			wantBase:     "https://api.anthropic.com/v1",
			wantMessages: "https://api.anthropic.com/v1/messages",
			wantModels:   "https://api.anthropic.com/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase := NormalizeAnthropicBase(tt.endpoint)
			if gotBase != tt.wantBase {
				t.Errorf("NormalizeAnthropicBase(%q) = %q, want %q", tt.endpoint, gotBase, tt.wantBase)
			}
			gotMessages := AnthropicMessagesURL(tt.endpoint)
			if gotMessages != tt.wantMessages {
				t.Errorf("AnthropicMessagesURL(%q) = %q, want %q", tt.endpoint, gotMessages, tt.wantMessages)
			}
			gotModels := AnthropicModelsURL(tt.endpoint)
			if gotModels != tt.wantModels {
				t.Errorf("AnthropicModelsURL(%q) = %q, want %q", tt.endpoint, gotModels, tt.wantModels)
			}
		})
	}
}

func TestGeminiEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		model      string
		apiKey     string
		wantBase   string
		wantStream string
		wantModels string
	}{
		{
			name:       "default endpoint and clean model",
			endpoint:   "https://generativelanguage.googleapis.com",
			model:      "gemini-2.5-pro",
			apiKey:     "test-key",
			wantBase:   "https://generativelanguage.googleapis.com/v1beta",
			wantStream: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse&key=test-key",
			wantModels: "https://generativelanguage.googleapis.com/v1beta/models?key=test-key",
		},
		{
			name:       "model with models/ prefix and spaces (path escaping)",
			endpoint:   "https://generativelanguage.googleapis.com",
			model:      "models/gemini-2.5 pro",
			apiKey:     "key with space",
			wantBase:   "https://generativelanguage.googleapis.com/v1beta",
			wantStream: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5%20pro:streamGenerateContent?alt=sse&key=key+with+space",
			wantModels: "https://generativelanguage.googleapis.com/v1beta/models?key=key+with+space",
		},
		{
			name:       "v1 endpoint preserved",
			endpoint:   "https://generativelanguage.googleapis.com/v1",
			model:      "gemini-1.5-flash",
			apiKey:     "test-key",
			wantBase:   "https://generativelanguage.googleapis.com/v1",
			wantStream: "https://generativelanguage.googleapis.com/v1/models/gemini-1.5-flash:streamGenerateContent?alt=sse&key=test-key",
			wantModels: "https://generativelanguage.googleapis.com/v1/models?key=test-key",
		},
		{
			name:       "empty model defaults to gemini-3.8-flash and empty key omitted",
			endpoint:   "",
			model:      "",
			apiKey:     "",
			wantBase:   "https://generativelanguage.googleapis.com/v1beta",
			wantStream: "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:streamGenerateContent?alt=sse",
			wantModels: "https://generativelanguage.googleapis.com/v1beta/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase := NormalizeGeminiBase(tt.endpoint)
			if gotBase != tt.wantBase {
				t.Errorf("NormalizeGeminiBase(%q) = %q, want %q", tt.endpoint, gotBase, tt.wantBase)
			}
			gotStream := GeminiStreamURL(tt.endpoint, tt.model, tt.apiKey)
			if gotStream != tt.wantStream {
				t.Errorf("GeminiStreamURL(%q, %q, %q) = %q, want %q", tt.endpoint, tt.model, tt.apiKey, gotStream, tt.wantStream)
			}
			gotModels := GeminiModelsURL(tt.endpoint, tt.apiKey)
			if gotModels != tt.wantModels {
				t.Errorf("GeminiModelsURL(%q, %q) = %q, want %q", tt.endpoint, tt.apiKey, gotModels, tt.wantModels)
			}
		})
	}
}

func TestOllamaEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantChat string
		wantTags string
	}{
		{
			name:     "default endpoint localhost",
			endpoint: "http://localhost:11434",
			wantChat: "http://localhost:11434/api/chat",
			wantTags: "http://localhost:11434/api/tags",
		},
		{
			name:     "endpoint with trailing slash",
			endpoint: "http://localhost:11434/",
			wantChat: "http://localhost:11434/api/chat",
			wantTags: "http://localhost:11434/api/tags",
		},
		{
			name:     "endpoint ending in /api does not duplicate",
			endpoint: "http://localhost:11434/api",
			wantChat: "http://localhost:11434/api/chat",
			wantTags: "http://localhost:11434/api/tags",
		},
		{
			name:     "endpoint ending in /api/ does not duplicate",
			endpoint: "http://localhost:11434/api/",
			wantChat: "http://localhost:11434/api/chat",
			wantTags: "http://localhost:11434/api/tags",
		},
		{
			name:     "empty endpoint uses default",
			endpoint: "",
			wantChat: "http://localhost:11434/api/chat",
			wantTags: "http://localhost:11434/api/tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChat := OllamaChatURL(tt.endpoint)
			if gotChat != tt.wantChat {
				t.Errorf("OllamaChatURL(%q) = %q, want %q", tt.endpoint, gotChat, tt.wantChat)
			}
			gotTags := OllamaTagsURL(tt.endpoint)
			if gotTags != tt.wantTags {
				t.Errorf("OllamaTagsURL(%q) = %q, want %q", tt.endpoint, gotTags, tt.wantTags)
			}
		})
	}
}
