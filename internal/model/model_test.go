package model

import (
	"testing"
	"time"
)

func TestProblemModel(t *testing.T) {
	p := Problem{
		ID:         "001",
		Title:      "Two Sum",
		Difficulty: DifficultyEasy,
		TestCases: []TestCase{
			{ID: "tc1", IsHidden: false},
			{ID: "tc2", IsHidden: true},
			{ID: "tc3", IsHidden: false},
		},
	}

	visible := p.GetVisibleTestCases()
	if len(visible) != 2 {
		t.Fatalf("expected 2 visible test cases, got %d", len(visible))
	}
}

func TestUserState(t *testing.T) {
	state := NewUserState()

	if state.HasSolved("001") {
		t.Fatal("expected problem 001 not solved initially")
	}

	state.MarkSolved("001", "python", "solution_code")
	if !state.HasSolved("001") {
		t.Fatal("expected problem 001 to be solved")
	}
	if state.CurrentStreak != 1 {
		t.Fatalf("expected streak 1, got %d", state.CurrentStreak)
	}

	// Code buffers
	state.SetCodeBuffer("001", "python", "my code")
	buf := state.GetCodeBuffer("001", "python")
	if buf != "my code" {
		t.Fatalf("expected buffer 'my code', got %s", buf)
	}
}

func TestExecutionResult(t *testing.T) {
	res := ExecutionResult{
		ProblemID:   "001",
		Language:    "python",
		TotalTests:  3,
		PassedTests: 3,
		Duration:    100 * time.Millisecond,
	}

	if !res.IsSuccessful() {
		t.Fatal("expected result to be successful")
	}

	res.PassedTests = 2
	if res.IsSuccessful() {
		t.Fatal("expected result to fail when passed < total")
	}

	res.PassedTests = 3
	res.StdErr = "syntax error"
	if res.IsSuccessful() {
		t.Fatal("expected result to fail when stderr is non-empty")
	}
}
