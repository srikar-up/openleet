package storage

import (
	"os"
	"testing"
)

func TestStorageManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_storage_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mgr, err := NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}

	// 1. List problems (should include extracted seed problems)
	problems, err := mgr.ListProblems()
	if err != nil {
		t.Fatalf("failed to list problems: %v", err)
	}
	if len(problems) < 5 {
		t.Fatalf("expected at least 5 embedded seed problems, got %d", len(problems))
	}

	// 2. Load single problem
	p, err := mgr.LoadProblem("001")
	if err != nil {
		t.Fatalf("failed to load problem 001: %v", err)
	}
	if p.Title != "Two Sum" {
		t.Fatalf("expected title 'Two Sum', got %s", p.Title)
	}
	if len(p.TestCases) < 3 {
		t.Fatalf("expected test cases for problem 001, got %d", len(p.TestCases))
	}

	// 3. User state persistence
	state, err := mgr.LoadUserState()
	if err != nil {
		t.Fatalf("failed to load user state: %v", err)
	}
	if state.CurrentStreak != 0 {
		t.Fatalf("expected initial streak 0, got %d", state.CurrentStreak)
	}

	state.MarkSolved("001", "python", "class Solution: ...")
	if err := mgr.SaveUserState(state); err != nil {
		t.Fatalf("failed to save user state: %v", err)
	}

	reloaded, err := mgr.LoadUserState()
	if err != nil {
		t.Fatalf("failed to reload user state: %v", err)
	}
	if !reloaded.HasSolved("001") {
		t.Fatal("expected problem 001 to be marked solved")
	}
	if reloaded.CurrentStreak != 1 {
		t.Fatalf("expected streak 1, got %d", reloaded.CurrentStreak)
	}

	// 4. Code buffer persistence
	code := "def twoSum(nums, target): return [0, 1]"
	if err := mgr.SaveCodeBuffer("001", "python", code); err != nil {
		t.Fatalf("failed to save code buffer: %v", err)
	}

	loadedCode, err := mgr.LoadCodeBuffer("001", "python")
	if err != nil {
		t.Fatalf("failed to load code buffer: %v", err)
	}
	if loadedCode != code {
		t.Fatalf("expected code %q, got %q", code, loadedCode)
	}

	// 5. Filter problems by topic
	arrayProblems, err := mgr.ListProblemsByTopic("array")
	if err != nil {
		t.Fatalf("failed to filter by topic: %v", err)
	}
	if len(arrayProblems) == 0 {
		t.Fatal("expected array problems to be found")
	}

	// 6. Workspace layout persistence
	initialLayout, err := mgr.LoadWorkspaceLayout()
	if err != nil {
		t.Fatalf("expected non-error for absent workspace layout, got: %v", err)
	}
	if initialLayout != (WorkspaceLayout{}) {
		t.Fatalf("expected empty initial workspace layout, got: %+v", initialLayout)
	}

	toSave := WorkspaceLayout{
		DescriptionWidth:  35,
		SidebarWidth:      25,
		DescriptionHidden: false,
		SidebarHidden:     true,
	}
	if err := mgr.SaveWorkspaceLayout(toSave); err != nil {
		t.Fatalf("failed to save workspace layout: %v", err)
	}

	loadedLayout, err := mgr.LoadWorkspaceLayout()
	if err != nil {
		t.Fatalf("failed to reload workspace layout: %v", err)
	}
	if loadedLayout != toSave {
		t.Fatalf("expected reloaded layout %+v, got %+v", toSave, loadedLayout)
	}
}

func TestEmbeddedJSONProblemLoads(t *testing.T) {
	p, err := LoadEmbeddedProblem("017_contains_duplicate")
	if err != nil {
		t.Fatalf("failed to load the embedded sample problem: %v", err)
	}
	if p.ID != "017" || p.Title != "Contains Duplicate" {
		t.Fatalf("unexpected problem identity: %s %q", p.ID, p.Title)
	}
	if p.CategoryKey() != "array" {
		t.Fatalf("expected the array roadmap section, got %q", p.CategoryKey())
	}
	if p.Difficulty != "Easy" {
		t.Fatalf("expected Easy difficulty, got %q", p.Difficulty)
	}
	if len(p.TestCases) != 4 {
		t.Fatalf("expected 4 fixtures, got %d", len(p.TestCases))
	}
	if visible := len(p.GetVisibleTestCases()); visible != 3 {
		t.Fatalf("expected 3 visible fixtures, got %d", visible)
	}
	if p.Description == "" {
		t.Fatal("expected the description.md statement to load")
	}
	if p.Templates["python"] == "" || p.Templates["go"] == "" {
		t.Fatal("expected both starter templates")
	}
}

func TestNewProblemIsAutoDiscovered(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openleet_problem_discovery_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mgr, err := NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}
	problems, err := mgr.ListProblems()
	if err != nil {
		t.Fatalf("failed to list problems: %v", err)
	}

	for _, p := range problems {
		if p.ID == "017" {
			if p.Title != "Contains Duplicate" {
				t.Fatalf("unexpected title for 017: %q", p.Title)
			}
			return
		}
	}
	t.Fatal("expected the new JSON folder to be discovered without code changes")
}
