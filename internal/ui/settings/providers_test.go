package settings

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeProviderVerifiesOpenAIChat(t *testing.T) {
	var chatCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"nvidia/nemotron-3-super-120b-a12b"}]}`)
		case "/v1/chat/completions":
			chatCalls++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"content":"pong"}}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := Provider{ID: "nvidia", Protocol: ProtocolOpenAI, BaseURL: srv.URL}
	models, err := probeProvider(context.Background(), provider, "test-key", srv.URL, "nvidia/nemotron-3-super-120b-a12b")
	if err != nil {
		t.Fatalf("probeProvider error: %v", err)
	}
	if len(models) != 1 || models[0] != "nvidia/nemotron-3-super-120b-a12b" {
		t.Fatalf("models = %v", models)
	}
	if chatCalls != 1 {
		t.Fatalf("chat calls = %d, want 1", chatCalls)
	}
}

func TestProbeProviderReturnsChatFailureWithModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"nvidia/nemotron-3-super-120b-a12b"}]}`)
		case "/v1/chat/completions":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"status":404,"detail":"Function 'x': Not found for account 'y'"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := Provider{ID: "nvidia", Protocol: ProtocolOpenAI, BaseURL: srv.URL}
	models, err := probeProvider(context.Background(), provider, "test-key", srv.URL, "nvidia/nemotron-3-super-120b-a12b")
	if err == nil {
		t.Fatal("expected the chat verification to fail")
	}
	if len(models) != 1 {
		t.Fatalf("models = %v, want the discovered model list to survive the chat failure", models)
	}
	if !strings.Contains(err.Error(), "cloud endpoint returned 404") {
		t.Fatalf("error = %q, want the HTTP failure", err.Error())
	}
}

func TestProbeGeminiFiltersNonChatModels(t *testing.T) {
	var chatCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1beta/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"models":[
				{"name":"models/gemini-3.8-flash","supportedGenerationMethods":["generateContent","streamGenerateContent"]},
				{"name":"models/gemini-3.7-flash","supportedGenerationMethods":["generateContent"]},
				{"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]},
				{"name":"models/gemini-2.5-flash-image","supportedGenerationMethods":["generateContent"]},
				{"name":"models/imagen-4","supportedGenerationMethods":["predict"]}
			]}`)
		case strings.HasSuffix(r.URL.Path, ":generateContent"):
			chatCalls++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"pong"}]}}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := Provider{ID: "gemini", Protocol: ProtocolGemini, BaseURL: srv.URL}
	models, err := probeProvider(context.Background(), provider, "test-key", srv.URL, "gemini-3.8-flash")
	if err != nil {
		t.Fatalf("probeProvider error: %v", err)
	}
	if len(models) != 2 || models[0] != "gemini-3.8-flash" || models[1] != "gemini-3.7-flash" {
		t.Fatalf("models = %v, want only the two chat models", models)
	}
	if chatCalls != 1 {
		t.Fatalf("chat calls = %d, want 1", chatCalls)
	}
}

func TestProbeProviderGeminiReportsRetiredModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1beta/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"models":[{"name":"models/gemini-3.8-flash","supportedGenerationMethods":["generateContent"]}]}`)
		case strings.HasSuffix(r.URL.Path, ":generateContent"):
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"code":404,"message":"This model models/gemini-2.5-flash is no longer available to new users. Please update your code to use models/gemini-3.8-flash."}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := Provider{ID: "gemini", Protocol: ProtocolGemini, BaseURL: srv.URL}
	models, err := probeProvider(context.Background(), provider, "test-key", srv.URL, "gemini-2.5-flash")
	if err == nil {
		t.Fatal("expected the retired model to fail the chat probe")
	}
	if !strings.Contains(err.Error(), "retired") || !strings.Contains(err.Error(), "gemini-3.8-flash") {
		t.Fatalf("error = %q, want a retired-model hint", err.Error())
	}
	if len(models) != 1 {
		t.Fatalf("models = %v, want the discovered list to survive", models)
	}
}
