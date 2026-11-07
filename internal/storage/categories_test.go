package storage

import (
	"testing"
)

// TestSeedCategoriesPresent guarantees every roadmap rail category has at
// least one problem, so the topic rail never opens onto an empty canvas.
func TestSeedCategoriesPresent(t *testing.T) {
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	problems, err := mgr.ListProblems()
	if err != nil {
		t.Fatalf("ListProblems: %v", err)
	}
	if len(problems) == 0 {
		t.Fatal("expected embedded seed problems")
	}

	counts := make(map[string]int)
	for _, p := range problems {
		counts[p.CategoryKey()]++
	}

	for _, category := range []string{"dsa", "cp", "dp", "array", "linklist", "graph"} {
		if counts[category] == 0 {
			t.Fatalf("category %q has no seed problems (all: %v)", category, counts)
		}
	}

	// The DSA roadmap keeps the six-card layout from the design.
	if counts["dsa"] != 6 {
		t.Fatalf("expected 6 DSA problems, got %d", counts["dsa"])
	}
}
