package main

import (
	"testing"

	"openleet/internal/config"
)

func TestResolveStartViewDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	if got := resolveStartView("", cfg); got != "home" {
		t.Fatalf("default start view = %q, want home", got)
	}
}

func TestResolveStartViewUsesSetting(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Settings["general.startup"] = " Stats "
	if got := resolveStartView("", cfg); got != "stats" {
		t.Fatalf("configured start view = %q, want stats", got)
	}
}

func TestResolveStartViewFlagWins(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Settings["general.startup"] = "stats"
	if got := resolveStartView(" Roadmap ", cfg); got != "roadmap" {
		t.Fatalf("explicit flag start view = %q, want roadmap", got)
	}
}
