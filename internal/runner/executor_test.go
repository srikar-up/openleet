package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"openleet/internal/model"
)

func TestExecutorPythonSuccess(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	e := NewExecutor(d, 5*time.Second)
	prob := &model.Problem{
		ID:   "001",
		Slug: "two_sum",
		TestCases: []model.TestCase{
			{
				ID:       "tc1",
				Input:    map[string]interface{}{"nums": []int{2, 7, 11, 15}, "target": 9},
				Expected: []interface{}{0, 1},
			},
			{
				ID:       "tc2",
				Input:    map[string]interface{}{"nums": []int{3, 2, 4}, "target": 6},
				Expected: []interface{}{1, 2},
			},
		},
	}

	code := `
class Solution:
    def twoSum(self, nums, target):
        lookup = {}
        for i, num in enumerate(nums):
            diff = target - num
            if diff in lookup:
                return [lookup[diff], i]
            lookup[num] = i
        return []
`

	res := e.Execute(context.Background(), prob, "python", code)
	if res.StdErr != "" {
		t.Fatalf("unexpected execution error: %s", res.StdErr)
	}
	if res.TotalTests != 2 {
		t.Fatalf("expected 2 tests, got %d", res.TotalTests)
	}
	if res.PassedTests != 2 {
		t.Fatalf("expected 2 passed tests, got %d", res.PassedTests)
	}
	if !res.IsSuccessful() {
		t.Fatal("expected execution result to be successful")
	}
}

func TestExecutorPythonSyntaxError(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	e := NewExecutor(d, 5*time.Second)
	prob := &model.Problem{
		ID:   "001",
		Slug: "two_sum",
		TestCases: []model.TestCase{
			{ID: "tc1", Input: map[string]interface{}{"nums": []int{2, 7}, "target": 9}, Expected: []interface{}{0, 1}},
		},
	}

	badCode := `
class Solution
    def twoSum(self:
        broken code syntax!!
`

	res := e.Execute(context.Background(), prob, "python", badCode)
	if res.StdErr == "" {
		t.Fatal("expected stderr to contain syntax error information")
	}
	if res.IsSuccessful() {
		t.Fatal("expected execution with syntax error to fail")
	}
}

func TestExecutorPythonTracebackUsesEditorLines(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	e := NewExecutor(d, 5*time.Second)
	prob := &model.Problem{
		ID:   "001",
		Slug: "two_sum",
		TestCases: []model.TestCase{
			{ID: "tc1", Input: map[string]interface{}{"nums": []int{2, 7}, "target": 9}, Expected: []interface{}{0, 1}},
		},
	}

	code := "class Solution:\n    def twoSum(self, nums, target):\n        raise ValueError('boom')\n"
	res := e.Execute(context.Background(), prob, "python", code)
	if len(res.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(res.Results))
	}
	message := res.Results[0].Error
	if !strings.Contains(message, "line 3, in twoSum") {
		t.Fatalf("expected the editor line 3 in the traceback, got:\n%s", message)
	}
	if strings.Contains(message, "openleet-run-") {
		t.Fatalf("expected the sandbox path to be hidden, got:\n%s", message)
	}
}

func TestExecutorPythonCustomCase(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	e := NewExecutor(d, 5*time.Second)
	prob := &model.Problem{
		ID:   "017",
		Slug: "contains_duplicate",
		TestCases: []model.TestCase{
			{ID: "fixture", Input: map[string]interface{}{"nums": []int{9, 9}}, Expected: false},
		},
	}
	code := "class Solution:\n    def containsDuplicate(self, nums):\n        return len(nums) != len(set(nums))\n"

	res := e.ExecuteCustomCase(
		context.Background(),
		prob,
		"python",
		code,
		map[string]interface{}{"nums": []int{1, 2, 3, 1}},
		true,
	)
	if res.TotalTests != 1 {
		t.Fatalf("expected one custom fixture, got %d", res.TotalTests)
	}
	if !res.IsSuccessful() {
		t.Fatalf("expected the custom fixture to pass, got %+v", res)
	}
}

func TestExecutorTimeout(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	// 1 second timeout boundary
	e := NewExecutor(d, 1*time.Second)
	prob := &model.Problem{
		ID:   "001",
		Slug: "two_sum",
		TestCases: []model.TestCase{
			{ID: "tc1", Input: map[string]interface{}{"nums": []int{2, 7}, "target": 9}, Expected: []interface{}{0, 1}},
		},
	}

	infiniteLoopCode := `
class Solution:
    def twoSum(self, nums, target):
        while True:
            pass
`

	startTime := time.Now()
	res := e.Execute(context.Background(), prob, "python", infiniteLoopCode)
	elapsed := time.Since(startTime)

	if elapsed > 4*time.Second {
		t.Fatalf("timeout boundary exceeded, took %v", elapsed)
	}
	if !strings.Contains(res.StdErr, "Time Limit Exceeded") {
		t.Fatalf("expected timeout error message, got: %s", res.StdErr)
	}
	if res.IsSuccessful() {
		t.Fatal("expected infinite loop execution to fail")
	}
}
