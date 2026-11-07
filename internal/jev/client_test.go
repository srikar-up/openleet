package jev

import (
	"context"
	"testing"
	"time"
)

func TestClientDiscovery(t *testing.T) {
	client, err := NewClient("")
	if err != nil {
		t.Skipf("OPENCODE_API_KEY not configured: %v", err)
	}

	if client.apiKey == "" {
		t.Skip("expected non-empty API key in the environment or ~/.openleet/pass.env")
	}
}

func TestLiveJevRouting(t *testing.T) {
	client, err := NewClient("")
	if err != nil {
		t.Skipf("OPENCODE_API_KEY not configured: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Test TUI Styling route
	task1 := "Fix Lip Gloss borders and padding alignment on the right workspace panel"
	decision1, err := client.RouteTask(ctx, task1)
	if err != nil {
		t.Fatalf("Jev routing failed for task 1: %v", err)
	}

	if decision1.Choice != "TUI_STYLING" {
		t.Logf("Notice: choice for styling was %s (conf: %.2f)", decision1.Choice, decision1.Confidence)
	}
	if decision1.Route.TargetAgent == "" {
		t.Fatal("expected non-empty TargetAgent")
	}

	// 2. Test Multi-package refactor route
	task2 := "Implement Socratic AI Coach HTTP streaming client with local Ollama SSE reader and token buffer across packages"
	decision2, err := client.RouteTask(ctx, task2)
	if err != nil {
		t.Fatalf("Jev routing failed for task 2: %v", err)
	}

	if decision2.Choice != "MULTI_PACKAGE_REFACTOR" && decision2.Choice != "STATE_ARCHITECTURE" {
		t.Logf("Notice: choice for architectural task was %s", decision2.Choice)
	}
	if decision2.CostUSD != 0 {
		t.Fatalf("expected cost $0.00 for free model, got %f", decision2.CostUSD)
	}
}
