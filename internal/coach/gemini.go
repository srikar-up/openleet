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
	"strings"
	"time"
)

// GeminiProvider implements the Google Gemini streaming protocol. It speaks
// the generativelanguage.googleapis.com SSE dialect, not the OpenAI shape.
type GeminiProvider struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGeminiProvider constructs a Google Gemini coach provider.
func NewGeminiProvider(endpoint, apiKey, model string) *GeminiProvider {
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com"
	}
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if model == "" {
		model = "gemini-3.8-flash"
	}

	return &GeminiProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns "gemini".
func (g *GeminiProvider) Name() string {
	return "gemini"
}

// HealthCheck verifies that an API key is available.
func (g *GeminiProvider) HealthCheck(ctx context.Context) error {
	if g.apiKey == "" {
		return fmt.Errorf("no Gemini API key configured")
	}
	return nil
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiStreamRequest struct {
	SystemInstruction geminiContent          `json:"systemInstruction"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// Stream streams tokens over SSE from the Gemini generateContent endpoint. The
// manager owns channel closure; this method must not close Tokens.
func (g *GeminiProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	if g.apiKey == "" {
		return fmt.Errorf("gemini provider requires an API key")
	}

	temp := 0.2
	if req.TemperatureSet {
		temp = req.Temperature
	}
	maxTokens := 1024
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	payload := geminiStreamRequest{
		SystemInstruction: geminiContent{
			Parts: []geminiPart{{Text: SocraticSystemPrompt}},
		},
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: BuildUserPrompt(req)}}},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     temp,
			MaxOutputTokens: maxTokens,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	model := strings.TrimPrefix(strings.TrimSpace(g.model), "models/")
	url := GeminiStreamURL(g.endpoint, model, g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := g.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return geminiResponseError(res.StatusCode, res.Status, model, body)
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

		var chunk geminiStreamChunk
		if err := json.Unmarshal([]byte(dataPart), &chunk); err != nil {
			continue
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text == "" {
					continue
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-channels.Done:
					return nil
				case channels.Tokens <- part.Text:
				}
			}
		}
	}

	return scanner.Err()
}

// geminiResponseError turns a Gemini HTTP failure into an actionable error.
func geminiResponseError(statusCode int, status, model string, body []byte) error {
	var remote struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	detail := ""
	if json.Unmarshal(body, &remote) == nil {
		if remote.Error.Status != "" && remote.Error.Message != "" {
			detail = fmt.Sprintf("[%s] %s", remote.Error.Status, remote.Error.Message)
		} else if remote.Error.Message != "" {
			detail = remote.Error.Message
		}
	}

	lowerDetail := strings.ToLower(detail)
	if strings.Contains(lowerDetail, "no longer available") || strings.Contains(lowerDetail, "not found for api version") {
		return fmt.Errorf("gemini model %q is retired; choose gemini-3.8-flash in Settings ? AI Models (provider said: %s)", model, detail)
	}
	if statusCode == http.StatusServiceUnavailable {
		if detail == "" {
			detail = status
		}
		return fmt.Errorf("gemini is temporarily overloaded (503); retry in a moment or choose gemini-3.7-flash in Settings → AI Models: %s", detail)
	}
	if detail != "" {
		return fmt.Errorf("gemini endpoint returned %s: %s", status, detail)
	}
	return fmt.Errorf("gemini endpoint returned %s", status)
}

// ProbeGeminiChat sends a one-token generateContent request so the Settings
// test verifies the selected model, not just the models listing.
func ProbeGeminiChat(ctx context.Context, endpoint, apiKey, model string) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("no API key saved")
	}
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if model == "" {
		model = "gemini-3.8-flash"
	}

	payload := geminiStreamRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: "ping"}}},
		},
		GenerationConfig: geminiGenerationConfig{MaxOutputTokens: 1},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, GeminiGenerateURL(endpoint, model, apiKey), bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return geminiResponseError(res.StatusCode, res.Status, model, body)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return nil
}
