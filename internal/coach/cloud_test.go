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

func TestCloudStreamSendsNormalizedURLAndBody(t *testing.T) {
	var gotPath, gotAuth, gotAccept, gotUA string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	// Use srv.URL (which is http://127.0.0.1:xxxx with no /v1) to test URL normalization
	p := NewCloudProvider(srv.URL, "test-cloud-key", "test-model")
	channels := NewStreamChannels()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := CoachRequest{
		Action:         ActionHint,
		Temperature:    0.7,
		TemperatureSet: true,
		MaxTokens:      2048,
	}

	if err := p.Stream(ctx, req, channels); err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-cloud-key" {
		t.Fatalf("Authorization = %q, want Bearer test-cloud-key", gotAuth)
	}
	if gotAccept != "text/event-stream" {
		t.Fatalf("Accept = %q, want text/event-stream", gotAccept)
	}
	if gotUA != "openleet/coach" {
		t.Fatalf("User-Agent = %q, want openleet/coach", gotUA)
	}

	var payload cloudChatRequest
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	if payload.Model != "test-model" {
		t.Errorf("model = %q, want test-model", payload.Model)
	}
	if payload.Temperature != 0.7 {
		t.Errorf("temperature = %v, want 0.7", payload.Temperature)
	}
	if payload.MaxTokens != 2048 {
		t.Errorf("max_tokens = %d, want 2048", payload.MaxTokens)
	}
	if !payload.Stream {
		t.Errorf("stream = %v, want true", payload.Stream)
	}

	select {
	case token := <-channels.Tokens:
		if token != "ok" {
			t.Fatalf("token = %q, want ok", token)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for token")
	}
}

func TestCloudStreamSurfaces404JSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"message":"model not found"}}`)
	}))
	defer srv.Close()

	p := NewCloudProvider(srv.URL, "test-cloud-key", "missing-model")
	err := p.Stream(context.Background(), CoachRequest{Action: ActionHint}, NewStreamChannels())
	if err == nil {
		t.Fatal("expected error on 404 response")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "404") || !strings.Contains(errMsg, "model not found") {
		t.Fatalf("error = %q, want 404 status and 'model not found'", errMsg)
	}
}

func TestCloudHealthCheckKeyBehavior(t *testing.T) {
	// Custom endpoint without API key must not fallback to OPENCODE_API_KEY
	p := NewCloudProvider("https://integrate.api.nvidia.com/v1", "", "meta/llama-3")
	if err := p.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected error for empty key on explicit non-opencode provider")
	}

	// With explicit key, health check passes
	withKey := NewCloudProvider("https://integrate.api.nvidia.com/v1", "test-key", "meta/llama-3")
	if err := withKey.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error with key: %v", err)
	}
}

func TestCloudResponseErrorExplainsNVIDIAAccountGate(t *testing.T) {
	body := []byte(`{"status":404,"title":"Not Found","detail":"Function '9b96': Not found for account 'Ud6D'"}`)
	err := cloudResponseError("https://integrate.api.nvidia.com/v1", http.StatusNotFound, "404 Not Found", "nvidia/llama-3.1-nemotron-70b-instruct", body)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{"Public API Endpoints", "build.nvidia.com", "Settings"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want %q", msg, want)
		}
	}
	if strings.Contains(msg, "Ud6D") {
		t.Fatalf("error should not echo the account ID: %q", msg)
	}
}

func TestCloudResponseErrorKeepsGenericDetail(t *testing.T) {
	body := []byte(`{"error":{"message":"model not found"}}`)
	err := cloudResponseError("https://example.com/v1", http.StatusNotFound, "404 Not Found", "missing", body)
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("error = %v, want the provider detail", err)
	}
}

func TestProbeChatSendsMinimalRequest(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody cloudChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"pong"}}]}`)
	}))
	defer srv.Close()

	if err := ProbeChat(context.Background(), srv.URL, "test-key", "test-model"); err != nil {
		t.Fatalf("ProbeChat error: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody.Model != "test-model" || gotBody.MaxTokens != 1 || gotBody.Stream {
		t.Fatalf("unexpected probe body: %+v", gotBody)
	}
	if len(gotBody.Messages) != 1 || gotBody.Messages[0].Role != "user" {
		t.Fatalf("unexpected probe messages: %+v", gotBody.Messages)
	}
}

func TestProbeChatRequiresKeyAndModel(t *testing.T) {
	if err := ProbeChat(context.Background(), "https://example.com/v1", "", "m"); err == nil {
		t.Fatal("expected an error for a missing key")
	}
	if err := ProbeChat(context.Background(), "https://example.com/v1", "k", ""); err == nil {
		t.Fatal("expected an error for a missing model")
	}
}
