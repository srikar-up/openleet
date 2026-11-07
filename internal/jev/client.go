package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultEndpoint = "https://opencode.ai/inference/systemone/v1/systemone"
	ModelFree       = "jev-1.13-free"
	ModelStandard   = "jev-1.13"
)

// AgentRoute maps Jev's triage choice to system agents defined in AGENTS.md.
type AgentRoute struct {
	Choice      string   `json:"choice"`
	TargetAgent string   `json:"target_agent"`
	Description string   `json:"description"`
	Model       string   `json:"model"`
	Subagents   []string `json:"subagents,omitempty"`
}

// RoutingDecision is the rich result returned by Jev triage.
type RoutingDecision struct {
	TaskScope     string             `json:"task_scope"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Complexity    float64            `json:"complexity_score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Route         AgentRoute         `json:"route"`
	CostUSD       float64            `json:"cost_usd"`
	RawResponse   *Response          `json:"raw_response,omitempty"`
}

// Request is the payload sent to Jev System One.
type Request struct {
	Model     string              `json:"model"`
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Question is a typed question for Jev.
type Question struct {
	Type         string      `json:"type"` // "choice", "noul", "score"
	Instructions string      `json:"instructions"`
	Criteria     interface{} `json:"criteria,omitempty"`
}

// Response is the structured output from Jev System One.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Answer represents the evaluated decision for a question.
type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Confidence    float64            `json:"confidence"`
	Score         float64            `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// Usage reports token counts and billing costs.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// Client interacts with the Jev System One endpoint.
type Client struct {
	apiKey     string
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewClient constructs a Jev client, discovering the API key from env or pass.env.
func NewClient(apiKey string) (*Client, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = os.Getenv("OPENCODE_API_KEY")
	}
	if key == "" {
		key = loadKeyFromPassEnv()
	}
	if key == "" {
		return nil, fmt.Errorf("no OPENCODE_API_KEY found in environment or pass.env")
	}

	return &Client{
		apiKey:   key,
		endpoint: DefaultEndpoint,
		model:    ModelFree,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

// SetModel configures the model (default is "jev-1.13-free").
func (c *Client) SetModel(model string) {
	c.model = model
}

// RouteTask evaluates a development task using the AGENTS.md Stage 2 rubric.
func (c *Client) RouteTask(ctx context.Context, taskPrompt string) (*RoutingDecision, error) {
	reqBody := Request{
		Model: c.model,
		State: taskPrompt,
		Questions: map[string]Question{
			"task_route": {
				Type:         "choice",
				Instructions: "Route this development task to the appropriate agent category according to AGENTS.md",
				Criteria: map[string]string{
					"LOCAL_QUICKFIX":         "Minor bug fix, typo, single function tweak, or immediate syntax error fixable in under 10 lines",
					"TUI_STYLING":            "Lip Gloss colors, box borders, layout joins, ANSI palettes, cell padding, or terminal visual alignment",
					"STATE_ARCHITECTURE":     "Bubble Tea Elm architecture, Model struct, Init/Update/View lifecycle, tea.Cmd, or message passing",
					"MULTI_PACKAGE_REFACTOR": "Cross-cutting feature spanning multiple packages, host runners, storage engines, or major architectural refactors",
				},
			},
			"complexity": {
				Type:         "score",
				Instructions: "Rate the task complexity on a 1-5 scale (0=Trivial, 1=Simple, 2=Moderate, 3=Complex, 4=Architectural)",
				Criteria:     []string{"Trivial", "Simple", "Moderate", "Complex", "Architectural"},
			},
		},
	}

	resp, err := c.Evaluate(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	taskAns, ok := resp.Answers["task_route"]
	if !ok {
		return nil, fmt.Errorf("missing 'task_route' answer in Jev response")
	}

	compAns := resp.Answers["complexity"]

	decision := &RoutingDecision{
		TaskScope:     taskPrompt,
		Choice:        taskAns.Choice,
		Confidence:    taskAns.Confidence,
		Complexity:    compAns.Score,
		Probabilities: taskAns.Probabilities,
		CostUSD:       resp.Usage.Cost,
		RawResponse:   resp,
		Route:         mapChoiceToRoute(taskAns.Choice),
	}

	return decision, nil
}

// Evaluate sends an arbitrary question set to Jev.
func (c *Client) Evaluate(ctx context.Context, req Request) (*Response, error) {
	if req.Model == "" {
		req.Model = c.model
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jev API returned status %d: %s", res.StatusCode, string(bodyBytes))
	}

	var parsed Response
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (body: %s)", err, string(bodyBytes))
	}

	return &parsed, nil
}

func mapChoiceToRoute(choice string) AgentRoute {
	switch choice {
	case "LOCAL_QUICKFIX":
		return AgentRoute{
			Choice:      choice,
			TargetAgent: "@orchestrator",
			Description: "Gemini 3.8 Flash direct patch via read/edit/bash",
			Model:       "google/gemini-3.8-flash",
		}
	case "TUI_STYLING":
		return AgentRoute{
			Choice:      choice,
			TargetAgent: "@tui-styler",
			Description: "Lip Gloss styles, borders, dynamic sizing, and ANSI palettes",
			Model:       "opencode/gpt-6-luna",
		}
	case "STATE_ARCHITECTURE":
		return AgentRoute{
			Choice:      choice,
			TargetAgent: "@tui-architect",
			Description: "Bubble Tea Elm architecture, Model state, and tea.Cmd pipelines",
			Model:       "opencode/qwen3.7-plus",
		}
	case "MULTI_PACKAGE_REFACTOR":
		return AgentRoute{
			Choice:      choice,
			TargetAgent: "@tui-architect",
			Description: "Multi-package architecture blueprinting supported by @scanner",
			Model:       "opencode/qwen3.7-plus",
			Subagents:   []string{"@scanner (opencode/deepseek-v4.1-flash)"},
		}
	default:
		return AgentRoute{
			Choice:      choice,
			TargetAgent: "@orchestrator",
			Description: "Default fallback orchestrator",
			Model:       "google/gemini-3.8-flash",
		}
	}
}

var keyRegex = regexp.MustCompile(`OPENCODE_API_KEY\s*=\s*([a-zA-Z0-9_\-]+)`)

// loadKeyFromPassEnv checks the user-level secret store first, then the legacy
// project-local file. Keeping credentials outside the repository prevents an
// accidental commit.
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
