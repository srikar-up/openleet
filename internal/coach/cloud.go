package coach

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CloudProvider implements an OpenAI-compatible SSE streaming client.
type CloudProvider struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewCloudProvider constructs a remote cloud coach provider.
func NewCloudProvider(endpoint, apiKey, model string) *CloudProvider {
	if endpoint == "" {
		endpoint = "https://opencode.ai/inference/v1"
	}
	if apiKey == "" && strings.Contains(endpoint, "opencode.ai") {
		apiKey = os.Getenv("OPENCODE_API_KEY")
		if apiKey == "" {
			apiKey = loadKeyFromPassEnv()
		}
	}
	if model == "" {
		model = "deepseek-v4.1-flash-free"
	}

	return &CloudProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns "cloud".
func (c *CloudProvider) Name() string {
	return "cloud"
}

// HealthCheck verifies that an API key is available.
func (c *CloudProvider) HealthCheck(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("no cloud API key configured")
	}
	return nil
}

type cloudChatRequest struct {
	Model       string             `json:"model"`
	Messages    []cloudChatMessage `json:"messages"`
	Stream      bool               `json:"stream"`
	Temperature float64            `json:"temperature"`
	MaxTokens   int                `json:"max_tokens"`
}

type cloudChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cloudStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// Stream streams tokens using Server-Sent Events (SSE). The manager owns
// channel closure; this method must not close Tokens.
func (c *CloudProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	if c.apiKey == "" {
		return fmt.Errorf("cloud provider requires an API key")
	}

	temp := 0.2
	if req.TemperatureSet {
		temp = req.Temperature
	}
	maxTokens := 1024
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	payload := cloudChatRequest{
		Model: c.model,
		Messages: []cloudChatMessage{
			{Role: "system", Content: SocraticSystemPrompt},
			{Role: "user", Content: BuildUserPrompt(req)},
		},
		Stream:      true,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := OpenAIChatURL(c.endpoint)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("User-Agent", "openleet/coach")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return cloudResponseError(c.endpoint, res.StatusCode, res.Status, c.model, body)
	}

	scanner := bufio.NewScanner(res.Body)
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

		var chunk cloudStreamChunk
		if err := json.Unmarshal([]byte(dataPart), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			token := chunk.Choices[0].Delta.Content
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-channels.Done:
				return nil
			case channels.Tokens <- token:
			}
		}
	}

	return scanner.Err()
}

// cloudResponseError turns a provider HTTP failure into an actionable error.
// Known account/entitlement blockers get a short hint; everything else keeps
// the provider's own detail.
func cloudResponseError(endpoint string, statusCode int, status, model string, body []byte) error {
	if hint := providerHint(endpoint, statusCode, model, body); hint != "" {
		return errors.New(hint)
	}
	if detail := parseCloudError(body); detail != "" {
		return fmt.Errorf("cloud endpoint returned %s: %s", status, detail)
	}
	return fmt.Errorf("cloud endpoint returned %s", status)
}

// providerHint maps the failure signatures users actually hit onto concrete
// next steps. It deliberately avoids echoing an account ID or key material.
func providerHint(endpoint string, statusCode int, model string, body []byte) string {
	host := strings.ToLower(endpoint)
	lowerBody := strings.ToLower(string(body))

	switch {
	case strings.Contains(host, "integrate.api.nvidia.com") &&
		statusCode == http.StatusNotFound &&
		strings.Contains(lowerBody, "not found for account"):
		return "NVIDIA NIM: this account lacks Public API Endpoints access, so chat is blocked even though key checks and model listing succeed. Enable it at build.nvidia.com or email help@build.nvidia.com, then retry; you can also choose another provider in Settings → AI Models."
	case strings.Contains(host, "integrate.api.nvidia.com") && statusCode == http.StatusNotFound:
		return fmt.Sprintf("NVIDIA NIM: model %q was not found for this account. Pick another model in Settings → AI Models.", model)
	case strings.Contains(host, "opencode.ai") && statusCode == http.StatusPaymentRequired:
		return "OpenCode inference: this key has insufficient funds. Top up OpenCode or choose another provider in Settings → AI Models."
	case strings.Contains(host, "opencode.ai") && statusCode == http.StatusTooManyRequests:
		return "OpenCode inference: free usage limit reached. Subscribe to OpenCode Go or choose another provider in Settings → AI Models."
	}
	return ""
}

// ProbeChat sends a one-token, non-streaming request to an OpenAI-compatible
// endpoint. Model listing alone is not proof that generation works: NVIDIA can
// list models for an account that is not entitled to chat completions.
func ProbeChat(ctx context.Context, endpoint, apiKey, model string) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("no API key saved")
	}
	if strings.TrimSpace(model) == "" {
		return errors.New("no model configured")
	}

	payload := cloudChatRequest{
		Model:       model,
		Messages:    []cloudChatMessage{{Role: "user", Content: "ping"}},
		Stream:      false,
		Temperature: 0,
		MaxTokens:   1,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, OpenAIChatURL(endpoint), bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "openleet/coach")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return cloudResponseError(endpoint, res.StatusCode, res.Status, model, body)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return nil
}

func parseCloudError(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var raw struct {
		Error   any    `json:"error"`
		Detail  any    `json:"detail"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return strings.TrimSpace(string(body))
	}

	if raw.Error != nil {
		switch v := raw.Error.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		case map[string]any:
			if m, ok := v["message"].(string); ok && strings.TrimSpace(m) != "" {
				return strings.TrimSpace(m)
			}
		}
	}

	if raw.Detail != nil {
		switch v := raw.Detail.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		case []any:
			if len(v) > 0 {
				if first, ok := v[0].(map[string]any); ok {
					if m, ok := first["msg"].(string); ok && strings.TrimSpace(m) != "" {
						return strings.TrimSpace(m)
					}
				}
			}
		}
	}

	if s := strings.TrimSpace(raw.Message); s != "" {
		return s
	}

	return ""
}

var keyRegex = regexp.MustCompile(`OPENCODE_API_KEY\s*=\s*([a-zA-Z0-9_\-]+)`)

// loadKeyFromPassEnv checks the user-level secret store first, then the legacy
// project-local file.
func loadKeyFromPassEnv() string {
	paths := make([]string, 0, 5)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".openleet", "pass.env"))
	}
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		paths = append(paths, filepath.Join(configDir, "openleet", "pass.env"))
	}
	paths = append(paths,
		"pass.env",
		filepath.Join("..", "pass.env"),
		filepath.Join("..", "..", "pass.env"),
	)

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		match := keyRegex.FindStringSubmatch(string(data))
		if len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}

	return ""
}
