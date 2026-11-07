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

func TestGeminiProviderDefaults(t *testing.T) {
	p := NewGeminiProvider("", "", "")
	if p.Name() != "gemini" {
		t.Fatalf("Name() = %q, want gemini", p.Name())
	}
	if p.endpoint != "https://generativelanguage.googleapis.com" {
		t.Fatalf("endpoint = %q", p.endpoint)
	}
	if p.model != "gemini-3.8-flash" {
		t.Fatalf("model = %q, want gemini-3.8-flash", p.model)
	}

	trailing := NewGeminiProvider("https://generativelanguage.googleapis.com/", "k", "m")
	if trailing.endpoint != "https://generativelanguage.googleapis.com" {
		t.Fatalf("trailing slash not trimmed: %q", trailing.endpoint)
	}
}

func TestGeminiHealthCheckRequiresKey(t *testing.T) {
	p := NewGeminiProvider("", "", "")
	if err := p.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected an error when no key is configured")
	}
	withKey := NewGeminiProvider("", "test-key", "")
	if err := withKey.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error with a key: %v", err)
	}
}

func TestGeminiStreamParsesSSE(t *testing.T) {
	var gotPath, gotQuery string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.RawQuery
		gotBody, _ = io.ReadAll(r.Body)
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello \"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"world\"}]}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := NewGeminiProvider(srv.URL, "test-key", "gemini-2.5 pro")
	channels := NewStreamChannels()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Stream(ctx, CoachRequest{Action: ActionHint}, channels); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	wantPath := "/v1beta/models/gemini-2.5%20pro:streamGenerateContent"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if !strings.Contains(gotQuery, "alt=sse") || !strings.Contains(gotQuery, "key=test-key") {
		t.Fatalf("query = %q, want alt=sse and escaped key", gotQuery)
	}

	var payload geminiStreamRequest
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if len(payload.SystemInstruction.Parts) != 1 || payload.SystemInstruction.Parts[0].Text != SocraticSystemPrompt {
		t.Fatal("system instruction missing from the request body")
	}
	if len(payload.Contents) != 1 || payload.Contents[0].Role != "user" {
		t.Fatalf("contents = %#v, want one user turn", payload.Contents)
	}
	if payload.GenerationConfig.Temperature != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", payload.GenerationConfig.Temperature)
	}
	if payload.Contents[0].Parts[0].Text != BuildUserPrompt(CoachRequest{Action: ActionHint}) {
		t.Fatal("user prompt mismatch in the request body")
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

func TestGeminiStreamNon200ReturnsDescriptiveError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"API key not valid"}}`)
	}))
	defer srv.Close()

	p := NewGeminiProvider(srv.URL, "bad-key", "gemini-2.5-pro")
	err := p.Stream(context.Background(), CoachRequest{Action: ActionHint}, NewStreamChannels())
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("error = %q, want the remote message", err.Error())
	}

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer plain.Close()
	err = NewGeminiProvider(plain.URL, "k", "m").Stream(context.Background(), CoachRequest{Action: ActionHint}, NewStreamChannels())
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v, want status fallback", err)
	}
}

func TestGeminiRetiredModelErrorPointsAt38Flash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":404,"message":"This model models/gemini-2.5-flash is no longer available to new users. Please update your code to use models/gemini-3.8-flash."}}`)
	}))
	defer srv.Close()

	p := NewGeminiProvider(srv.URL, "test-key", "gemini-2.5-flash")
	err := p.Stream(context.Background(), CoachRequest{Action: ActionHint}, NewStreamChannels())
	if err == nil {
		t.Fatal("expected an error for a retired model")
	}
	if !strings.Contains(err.Error(), "gemini-3.8-flash") {
		t.Fatalf("error = %q, want the 3.8 Flash recommendation", err.Error())
	}
}

func TestGemini503ReportsTransientOverload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":{"code":503,"message":"This model is currently experiencing high demand. Spikes in demand are usually temporary. Please try again later.","status":"UNAVAILABLE"}}`)
	}))
	defer srv.Close()

	p := NewGeminiProvider(srv.URL, "test-key", "gemini-3.8-flash")
	err := p.Stream(context.Background(), CoachRequest{Action: ActionHint}, NewStreamChannels())
	if err == nil || !strings.Contains(err.Error(), "retry in a moment") {
		t.Fatalf("error = %v, want a transient-overload hint", err)
	}
}
