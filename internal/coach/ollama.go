package coach

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider targets a local Ollama instance with NDJSON streaming.
type OllamaProvider struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider constructs an Ollama provider.
func NewOllamaProvider(endpoint, model string) *OllamaProvider {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "qwen2.5-coder:7b"
	}

	return &OllamaProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		model:    model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns "ollama".
func (o *OllamaProvider) Name() string {
	return "ollama"
}

// HealthCheck checks if local Ollama daemon is responsive.
func (o *OllamaProvider) HealthCheck(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	url := OllamaTagsURL(o.endpoint)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	res, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("local ollama offline: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", res.StatusCode)
	}
	return nil
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// Stream sends chat request to Ollama and pipes tokens into channels.Tokens.
// The manager owns channel closure; this method must not close Tokens.
func (o *OllamaProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	var options map[string]any
	if req.TemperatureSet || req.MaxTokens > 0 {
		options = make(map[string]any)
		if req.TemperatureSet {
			options["temperature"] = req.Temperature
		}
		if req.MaxTokens > 0 {
			options["num_predict"] = req.MaxTokens
		}
	}

	payload := ollamaChatRequest{
		Model: o.model,
		Messages: []ollamaChatMessage{
			{Role: "system", Content: SocraticSystemPrompt},
			{Role: "user", Content: BuildUserPrompt(req)},
		},
		Stream:  true,
		Options: options,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := OllamaChatURL(o.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := o.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama chat error: status %d", res.StatusCode)
	}

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk ollamaChatResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-channels.Done:
				return nil
			case channels.Tokens <- chunk.Message.Content:
			}
		}

		if chunk.Done {
			break
		}
	}

	return scanner.Err()
}
