package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/coach"
)

const providerProbeTimeout = 5 * time.Second

func (m Model) testProviderCmd() tea.Cmd {
	provider := m.currentProvider()
	if provider.ID == "" {
		return nil
	}
	key := ""
	if m.secrets != nil {
		key, _ = m.secrets.Get(provider.ID)
	}
	base := m.providerBase(provider)
	model := m.providerModel(provider)
	return func() tea.Msg {
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), providerProbeTimeout)
		defer cancel()
		models, err := probeProvider(ctx, provider, key, base, model)
		return ProviderTestMsg{
			Provider: provider.ID,
			Latency:  time.Since(started),
			Err:      err,
			Models:   models,
		}
	}
}

func (m Model) providerBase(provider Provider) string {
	return ProviderBase(m.cfg, provider)
}

func (m Model) providerModel(provider Provider) string {
	return ProviderModel(m.cfg, provider)
}

// coachChangedCmd refreshes the shared coach manager only when the coach
// provider selection changes. Other selects use the normal Settings flow.
func coachChangedCmd(field Field, next string) tea.Cmd {
	if field.ID != "ai.coach" {
		return nil
	}
	return func() tea.Msg { return AIProviderChangedMsg{ProviderID: next} }
}

// providerIDFromField extracts the provider ID from an
// ai.provider.<id>.<setting> key.
func providerIDFromField(id string) string {
	const prefix = "ai.provider."
	rest := strings.TrimPrefix(id, prefix)
	if rest == id {
		return ""
	}
	dot := strings.IndexByte(rest, '.')
	if dot <= 0 {
		return ""
	}
	return rest[:dot]
}

func probeProvider(ctx context.Context, provider Provider, key, base, model string) ([]string, error) {
	protocol := provider.Protocol
	if protocol == "" && provider.Ollama {
		protocol = ProtocolOllama
	}
	switch protocol {
	case ProtocolOllama:
		if base == "" {
			base = "http://localhost:11434"
		}
		return probeJSON(ctx, http.MethodGet, coach.OllamaTagsURL(base), nil, "", 0)

	case ProtocolGemini:
		if key == "" {
			return nil, errors.New("no API key saved")
		}
		if base == "" {
			base = provider.BaseURL
		}
		models, err := probeJSON(ctx, http.MethodGet, coach.GeminiModelsURL(base, key), nil, "", 0)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(model) != "" {
			if chatErr := coach.ProbeGeminiChat(ctx, base, key, model); chatErr != nil {
				return models, chatErr
			}
		}
		return models, nil

	case ProtocolAnthropic:
		if key == "" {
			return nil, errors.New("no API key saved")
		}
		if base == "" {
			base = provider.BaseURL
		}
		if base == "" {
			return nil, errors.New("base URL is required")
		}
		headers := map[string]string{
			"x-api-key":         key,
			"anthropic-version": "2023-06-01",
		}
		return probeJSON(ctx, http.MethodGet, coach.AnthropicModelsURL(base), headers, key, 0)

	case ProtocolOpenAI:
		fallthrough
	default:
		if key == "" {
			return nil, errors.New("no API key saved")
		}
		if base == "" {
			base = provider.BaseURL
		}
		if base == "" {
			return nil, errors.New("base URL is required")
		}
		headers := map[string]string{
			"Authorization": "Bearer " + key,
		}
		models, err := probeJSON(ctx, http.MethodGet, coach.OpenAIModelsURL(base), headers, key, 0)
		if err != nil {
			return nil, err
		}
		// Listing models only proves the key is accepted. Verify generation too:
		// providers such as NVIDIA can allow /models while blocking chat.
		if strings.TrimSpace(model) != "" {
			if chatErr := coach.ProbeChat(ctx, base, key, model); chatErr != nil {
				return models, chatErr
			}
		}
		return models, nil
	}
}

func probeJSON(ctx context.Context, method, endpoint string, headers map[string]string, _ string, _ int) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "openleet/coach")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := (&http.Client{Timeout: providerProbeTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err == nil {
			if errRaw, ok := raw["error"]; ok {
				var errStr string
				if err := json.Unmarshal(errRaw, &errStr); err == nil && strings.TrimSpace(errStr) != "" {
					return nil, fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(errStr))
				}
				var errObj struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(errRaw, &errObj); err == nil && strings.TrimSpace(errObj.Message) != "" {
					return nil, fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(errObj.Message))
				}
			}
			if detailRaw, ok := raw["detail"]; ok {
				var detailStr string
				if err := json.Unmarshal(detailRaw, &detailStr); err == nil && strings.TrimSpace(detailStr) != "" {
					return nil, fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(detailStr))
				}
			}
		}
		return nil, fmt.Errorf("provider returned %s", resp.Status)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode provider response: %w", err)
	}
	var models []string
	for _, item := range payload.Data {
		if item.ID != "" && isCoachModelName(item.ID) {
			models = append(models, item.ID)
		}
	}
	for _, item := range payload.Models {
		// Gemini reports the methods each model supports; keep chat-capable
		// models only so embeddings, TTS, and image endpoints do not pollute
		// the Socratic Coach selector.
		if len(item.SupportedGenerationMethods) > 0 {
			chat := false
			for _, method := range item.SupportedGenerationMethods {
				if method == "generateContent" || method == "streamGenerateContent" {
					chat = true
					break
				}
			}
			if !chat {
				continue
			}
		}
		name := strings.TrimPrefix(item.Name, "models/")
		if name != "" && isCoachModelName(name) {
			models = append(models, name)
		}
	}
	return models, nil
}

// isCoachModelName filters model catalogs down to entries that can plausibly
// answer a text coaching prompt. The provider /models lists also include
// embeddings, rerankers, speech, image, and moderation endpoints.
func isCoachModelName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{
		"embed", "rerank", "moderation", "whisper", "tts", "image",
		"imagen", "veo", "lyria", "aqa", "safety-guard", "nemotron-parse",
		"transcribe", "computer-use", "safeguard", "guard", "orpheus",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}
