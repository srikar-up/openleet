package commands

import (
	"reflect"
	"testing"
)

func names(list []Command) []string {
	out := make([]string, len(list))
	for i, cmd := range list {
		out[i] = cmd.Name
	}
	return out
}

func TestExactContextLists(t *testing.T) {
	cases := []struct {
		context Context
		want    []string
	}{
		{ContextHome, []string{"/solve", "/roadmap", "/login", "/random", "/filter", "/topic", "/themes", "/lang", "/connect", "/stats", "/settings", "/quit", "/theme-audit", "/notifications", "/update"}},
		{ContextSolve, []string{"/run", "/save", "/coach", "/hint", "/reset", "/roadmap", "/login", "/lang", "/themes", "/model", "/connect", "/settings", "/quit", "/theme-audit", "/notifications", "/update"}},
		{ContextRoadmap, []string{"/random", "/login", "/filter", "/topic", "/themes", "/lang", "/connect", "/stats", "/settings", "/quit", "/theme-audit", "/notifications", "/update"}},
		{ContextStats, []string{"/roadmap", "/login", "/themes", "/lang", "/connect", "/stats", "/settings", "/quit", "/theme-audit", "/notifications", "/update"}},
		{ContextThemes, []string{"/themes", "/lang", "/login", "/settings", "/quit", "/theme-audit", "/notifications", "/update"}},
		{ContextSettings, []string{"/themes", "/lang", "/connect", "/model", "/login", "/stats", "/quit", "/theme-audit", "/notifications", "/update"}},
	}
	for _, tc := range cases {
		if got := names(CommandsFor(tc.context)); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.context, got, tc.want)
		}
	}
}

func TestRemovedCommandsAreGone(t *testing.T) {
	removed := map[string]bool{
		"/focus": true, "/pane": true, "/tab": true,
		"/format": true, "/beautify": true, "/doctor": true, "/check": true,
		"/help": true, "/keys": true,
	}
	for _, cmd := range All() {
		if removed[cmd.Name] {
			t.Fatalf("removed command still registered: %s", cmd.Name)
		}
		for _, alias := range cmd.Aliases {
			if removed[alias] {
				t.Fatalf("removed alias still registered: %s", alias)
			}
		}
	}
}

func TestOnlySettingsAndRoadmapNavigate(t *testing.T) {
	var navigating []Action
	for _, cmd := range All() {
		if cmd.Navigates {
			navigating = append(navigating, cmd.Action)
		}
	}
	if !reflect.DeepEqual(navigating, []Action{ActionRoadmap, ActionSettings}) {
		t.Fatalf("only /roadmap and /settings may navigate, got %v", navigating)
	}
}
