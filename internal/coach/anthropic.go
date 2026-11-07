package coach

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicProvider implements the Anthropic Messages SSE streaming protocol.
type AnthropicProvider struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAnthropicProvider constructs an Anthropic coach provider.
func NewAnthropicProvider(endpoint, apiKey, model string) *AnthropicProvider {
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	return &AnthropicProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns "anthropic".
func (a *AnthropicProvider) Name() string {
	return "anthropic"
}

// HealthCheck verifies that an API key is available.
func (a *AnthropicProvider) HealthCheck(ctx context.Context) error {
	if a.apiKey == "" {
		return fmt.Errorf("no Anthropic API key configured")
	}
	return nil
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicStreamRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system"`
	Messages    []anthropicMessage `json:"messages"`
	Stream      bool               `json:"stream"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicStreamChunk struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Stream streams tokens over SSE from the Anthropic Messages endpoint. The
// manager owns channel closure; this method must not close Tokens.
func (a *AnthropicProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	if a.apiKey == "" {
		return fmt.Errorf("anthropic provider requires an API key")
	}

	maxTokens := 1024
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	payload := anthropicStreamRequest{
		Model:     a.model,
		MaxTokens: maxTokens,
		System:    SocraticSystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: BuildUserPrompt(req)},
		},
		Stream: true,
	}
	if req.TemperatureSet {
		payload.Temperature = req.Temperature
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := AnthropicMessagesURL(a.endpoint)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("User-Agent", "openleet/coach")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	res, err := a.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		var remote struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &remote) == nil && remote.Error.Message != "" {
			return fmt.Errorf("anthropic endpoint returned %s: %s", res.Status, remote.Error.Message)
		}
		return fmt.Errorf("anthropic endpoint returned %s", res.Status)
	}

	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		dataPart := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if dataPart == "[DONE]" {
			break
		}

		var chunk anthropicStreamChunk
		if err := json.Unmarshal([]byte(dataPart), &chunk); err != nil {
			continue
		}

		switch chunk.Type {
		case "content_block_delta":
			if chunk.Delta.Type != "text_delta" || chunk.Delta.Text == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-channels.Done:
				return nil
			case channels.Tokens <- chunk.Delta.Text:
			}
		case "error":
			if chunk.Error.Message != "" {
				return fmt.Errorf("anthropic stream error: %s", chunk.Error.Message)
			}
			return fmt.Errorf("anthropic stream error")
		case "message_stop":
			return nil
		}
	}

	return scanner.Err()
}
