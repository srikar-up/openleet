package coach

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnthropicProviderDefaults(t *testing.T) {
	p := NewAnthropicProvider("", "", "")
	if p.Name() != "anthropic" {
		t.Fatalf("Name() = %q, want anthropic", p.Name())
	}
	if p.endpoint != "https://api.anthropic.com" {
		t.Fatalf("endpoint = %q", p.endpoint)
	}
	if p.model != "claude-sonnet-4-6" {
		t.Fatalf("model = %q, want claude-sonnet-4-6", p.model)
	}

	trailing := NewAnthropicProvider("https://api.anthropic.com/", "k", "m")
	if trailing.endpoint != "https://api.anthropic.com" {
		t.Fatalf("trailing slash not trimmed: %q", trailing.endpoint)
	}
}

func TestAnthropicHealthCheckRequiresKey(t *testing.T) {
	p := NewAnthropicProvider("", "", "")
	if err := p.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected an error when no key is configured")
	}
	withKey := NewAnthropicProvider("", "test-key", "")
	if err := withKey.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error with a key: %v", err)
	}
}

func TestAnthropicStreamParsesSSE(t *testing.T) {
	var gotPath, gotAuth, gotVersion string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello \"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"world\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	defer srv.Close()

	p := NewAnthropicProvider(srv.URL, "test-key", "claude-sonnet-4-6")
	channels := NewStreamChannels()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Stream(ctx, CoachRequest{Action: ActionExplain}, channels); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if gotPath != "/v1/messages" {
		t.Fatalf("path = %q, want /v1/messages", gotPath)
	}
	if gotAuth != "test-key" {
		t.Fatalf("x-api-key = %q, want test-key", gotAuth)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want 2023-06-01", gotVersion)
	}

	var payload anthropicStreamRequest
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if payload.Model != "claude-sonnet-4-6" || payload.MaxTokens != 1024 || !payload.Stream {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.System != SocraticSystemPrompt {
		t.Fatal("system prompt missing from the request body")
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Role != "user" ||
		payload.Messages[0].Content != BuildUserPrompt(CoachRequest{Action: ActionExplain}) {
		t.Fatalf("messages = %#v", payload.Messages)
	}

	var tokens []string
	for i := 0; i < 2; i++ {
		select {
		case token := <-channels.Tokens:
			tokens = append(tokens, token)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for token %d", i)
		}
	}
	if full := strings.Join(tokens, ""); full != "Hello world" {
		t.Fatalf("tokens = %q, want 'Hello world'", full)
	}
}

func TestAnthropicStreamStopsOnDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"never sent\"}}\n\n")
	}))
	defer srv.Close()

	p := NewAnthropicProvider(srv.URL, "test-key", "claude-sonnet-4-6")
	channels := NewStreamChannels()
	if err := p.Stream(context.Background(), CoachRequest{Action: ActionHint}, channels); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	select {
	case token := <-channels.Tokens:
		if token != "partial" {
			t.Fatalf("token = %q, want partial", token)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the first token")
	}
	select {
	case token := <-channels.Tokens:
		t.Fatalf("token after [DONE] leaked: %q", token)
	default:
	}
}

func TestAnthropicStreamNon200ReturnsDescriptiveError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(srv.URL, "bad-key", "claude-sonnet-4-6")
	err := p.Stream(context.Background(), CoachRequest{Action: ActionHint}, NewStreamChannels())
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("error = %q, want the remote message", err.Error())
	}
}
