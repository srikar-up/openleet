package coach

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"openleet/internal/model"
)

func TestPromptBuilding(t *testing.T) {
	prob := &model.Problem{
		ID:          "001",
		Title:       "Two Sum",
		Difficulty:  model.DifficultyEasy,
		Topics:      []string{"Array", "Hash Table"},
		Description: "Given an array of integers nums and an integer target...",
	}

	req := CoachRequest{
		Action:   ActionHint,
		Problem:  prob,
		UserCode: "def twoSum(nums, target):\n    pass",
		Language: "python",
	}

	prompt := BuildUserPrompt(req)
	if !strings.Contains(prompt, "Two Sum") {
		t.Fatal("expected problem title in prompt")
	}
	if !strings.Contains(prompt, "Array, Hash Table") {
		t.Fatal("expected topics in prompt")
	}
	if !strings.Contains(prompt, "Do NOT write solution code") {
		t.Fatal("expected Socratic directive in prompt")
	}
}

type mockProvider struct {
	name      string
	isHealthy bool
	tokens    []string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) HealthCheck(ctx context.Context) error {
	if !m.isHealthy {
		return context.DeadlineExceeded
	}
	return nil
}
func (m *mockProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	// The manager owns channel closure; a provider only sends.
	for _, tok := range m.tokens {
		channels.Tokens <- tok
	}
	return nil
}

// TestManagerSetProvidersSwapsSelection locks in the mutex-safe provider swap:
// a live manager adopts the new local/cloud pair without being reconstructed,
// and a nil manager tolerates the call.
func TestManagerSetProvidersSwapsSelection(t *testing.T) {
	mgr := NewManager(
		&mockProvider{name: "ollama", isHealthy: true},
		&mockProvider{name: "cloud", isHealthy: true},
	)
	ctx := context.Background()
	if got := mgr.selectProvider(ctx); got == nil || got.Name() != "ollama" {
		t.Fatalf("selectProvider = %v, want ollama", got)
	}

	mgr.SetProviders(
		&mockProvider{name: "new-local", isHealthy: false},
		&mockProvider{name: "new-cloud", isHealthy: true, tokens: []string{"ok"}},
	)
	if got := mgr.selectProvider(ctx); got == nil || got.Name() != "new-cloud" {
		t.Fatalf("selectProvider after swap = %v, want new-cloud", got)
	}

	var nilMgr *Manager
	nilMgr.SetProviders(nil, nil) // must not panic
}

func TestDualEngineManagerFallback(t *testing.T) {
	local := &mockProvider{
		name:      "ollama",
		isHealthy: false, // offline
	}
	cloud := &mockProvider{
		name:      "cloud",
		isHealthy: true,
		tokens:    []string{"Think ", "about ", "hash maps."},
	}

	mgr := NewManager(local, cloud)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	channels, providerName, streamID, err := mgr.Stream(ctx, CoachRequest{Action: ActionHint})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if streamID == 0 {
		t.Fatal("expected non-zero streamID")
	}
	if providerName != "cloud" {
		t.Fatalf("expected fallback to 'cloud', got %s", providerName)
	}

	var received []string
	for tok := range channels.Tokens {
		received = append(received, tok)
	}

	full := strings.Join(received, "")
	if full != "Think about hash maps." {
		t.Fatalf("expected 'Think about hash maps.', got '%s'", full)
	}
}

type failingProvider struct{ err error }

func (p *failingProvider) Name() string                      { return "failing" }
func (p *failingProvider) HealthCheck(context.Context) error { return nil }
func (p *failingProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	channels.Tokens <- "partial"
	return p.err
}

// TestManagerQueuesErrorBeforeClosingTokens locks in the channel-ownership
// contract. Providers used to close Tokens themselves and the manager sent the
// error afterwards, which let a consumer observe a closed stream before the
// error was queued and silently lose the failure reason.
func TestManagerQueuesErrorBeforeClosingTokens(t *testing.T) {
	boom := errors.New("boom")
	mgr := NewManager(&failingProvider{err: boom}, nil)

	channels, _, _, err := mgr.Stream(context.Background(), CoachRequest{Action: ActionHint})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	var tokens []string
	for token := range channels.Tokens {
		tokens = append(tokens, token)
	}
	if len(tokens) != 1 || tokens[0] != "partial" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}

	select {
	case got := <-channels.Errors:
		if !errors.Is(got, boom) {
			t.Fatalf("got error %v, want boom", got)
		}
	case <-time.After(time.Second):
		t.Fatal("provider error was lost after Tokens closed")
	}

	select {
	case <-channels.Done:
	case <-time.After(time.Second):
		t.Fatal("manager did not close Done")
	}
}

type captureProvider struct {
	captured CoachRequest
}

func (c *captureProvider) Name() string                      { return "capture" }
func (c *captureProvider) HealthCheck(context.Context) error { return nil }
func (c *captureProvider) Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error {
	c.captured = req
	channels.Tokens <- "done"
	return nil
}

func TestManagerSetGenerationOptionsAppliesToStream(t *testing.T) {
	mock := &captureProvider{}
	mgr := NewManager(mock, nil)

	mgr.SetGenerationOptions(GenerationOptions{
		Temperature: 0.8,
		MaxTokens:   512,
		IncludeCode: false,
	})

	req := CoachRequest{
		Action:   ActionHint,
		UserCode: "def solve(): pass",
	}

	channels, _, _, err := mgr.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Stream error: %v", err)
	}
	<-channels.Tokens

	if mock.captured.UserCode != "" {
		t.Fatalf("expected UserCode to be cleared when IncludeCode=false, got %q", mock.captured.UserCode)
	}
	if !mock.captured.TemperatureSet || mock.captured.Temperature != 0.8 {
		t.Fatalf("expected Temperature=0.8 and TemperatureSet=true, got %v (%v)", mock.captured.Temperature, mock.captured.TemperatureSet)
	}
	if mock.captured.MaxTokens != 512 {
		t.Fatalf("expected MaxTokens=512, got %d", mock.captured.MaxTokens)
	}
}

func TestManagerSetGenerationOptionsClamping(t *testing.T) {
	var nilMgr *Manager
	nilMgr.SetGenerationOptions(GenerationOptions{}) // must not panic

	mgr := NewManager(nil, nil)
	mgr.SetGenerationOptions(GenerationOptions{
		Temperature: 5.0,
		MaxTokens:   10000,
	})
	if mgr.generation.Temperature != 2.0 {
		t.Fatalf("expected clamped temperature 2.0, got %v", mgr.generation.Temperature)
	}
	if mgr.generation.MaxTokens != 8192 {
		t.Fatalf("expected clamped max tokens 8192, got %d", mgr.generation.MaxTokens)
	}

	mgr.SetGenerationOptions(GenerationOptions{
		Temperature: -1.0,
		MaxTokens:   50,
	})
	if mgr.generation.Temperature != 0.0 {
		t.Fatalf("expected clamped temperature 0.0, got %v", mgr.generation.Temperature)
	}
	if mgr.generation.MaxTokens != 128 {
		t.Fatalf("expected clamped max tokens 128, got %d", mgr.generation.MaxTokens)
	}

	mgr.SetGenerationOptions(GenerationOptions{
		Temperature: 0.0,
		MaxTokens:   0,
	})
	if mgr.generation.Temperature != 0.0 {
		t.Fatalf("expected temperature 0.0 preserved, got %v", mgr.generation.Temperature)
	}
	if mgr.generation.MaxTokens != 1024 {
		t.Fatalf("expected default max tokens 1024, got %d", mgr.generation.MaxTokens)
	}
}
