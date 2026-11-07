package runner

import (
	"context"
	"math"
	"testing"
	"time"

	"openleet/internal/model"
)

func TestScaleRunnerPythonLinearCurve(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	prob := &model.Problem{
		ID:   "scale-001",
		Slug: "sum_all",
	}

	makeOnes := func(n int) []int {
		nums := make([]int, n)
		for i := range nums {
			nums[i] = 1
		}
		return nums
	}

	cases := map[string][]model.TestCase{
		"small": {
			{
				ID:       "s1",
				Input:    map[string]interface{}{"nums": makeOnes(10)},
				Expected: 10,
			},
		},
		"medium": {
			{
				ID:       "m1",
				Input:    map[string]interface{}{"nums": makeOnes(1000)},
				Expected: 1000,
			},
		},
		"large": {
			{
				ID:       "l1",
				Input:    map[string]interface{}{"nums": makeOnes(100000)},
				Expected: 100000,
			},
		},
	}

	code := `def sum_all(nums):
    total = 0
    for n in nums:
        total += n
    return total
`

	runner := NewCurveRunner(d, 10*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "python",
		Code:     code,
		Cases:    cases,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleAccepted {
		t.Fatalf("expected Accepted status, got %s (stderr: %s)", res.Status, res.StdErr)
	}
	if len(res.Scales) != 3 {
		t.Fatalf("expected 3 scales, got %d", len(res.Scales))
	}
	for _, scale := range []string{"small", "medium", "large"} {
		v, ok := res.Times[scale]
		if !ok || v < 0 {
			t.Fatalf("expected scale %q in Times with value >= 0, got %v", scale, v)
		}
	}
	if res.GrowthFactor <= 0 {
		t.Fatalf("expected GrowthFactor > 0, got %f", res.GrowthFactor)
	}
}

func TestScaleRunnerPythonWrongAnswer(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	prob := &model.Problem{
		ID:   "scale-001",
		Slug: "sum_all",
	}

	makeOnes := func(n int) []int {
		nums := make([]int, n)
		for i := range nums {
			nums[i] = 1
		}
		return nums
	}

	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": makeOnes(10)}, Expected: 10},
		},
		"large": {
			{ID: "l1", Input: map[string]interface{}{"nums": makeOnes(100000)}, Expected: 100000},
		},
	}

	code := `def sum_all(nums):
    return 0
`

	runner := NewCurveRunner(d, 10*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "python",
		Code:     code,
		Cases:    cases,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleWrongAnswer {
		t.Fatalf("expected %s status, got %s", model.ScaleWrongAnswer, res.Status)
	}
}

func TestScaleRunnerPythonRuntimeError(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	prob := &model.Problem{
		ID:   "scale-001",
		Slug: "sum_all",
	}

	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": []int{1}}, Expected: 1},
		},
		"large": {
			{ID: "l1", Input: map[string]interface{}{"nums": []int{1}}, Expected: 1},
		},
	}

	code := `def sum_all(nums):
    raise ValueError("boom")
`

	runner := NewCurveRunner(d, 10*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "python",
		Code:     code,
		Cases:    cases,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleRuntimeError {
		t.Fatalf("expected %s status, got %s", model.ScaleRuntimeError, res.Status)
	}
}

func TestScaleRunnerPythonTimeout(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	prob := &model.Problem{
		ID:   "scale-001",
		Slug: "sum_all",
	}

	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": []int{1}}, Expected: 1},
		},
		"large": {
			{ID: "l1", Input: map[string]interface{}{"nums": []int{1}}, Expected: 1},
		},
	}

	code := `def sum_all(nums):
    while True:
        pass
`

	runner := NewCurveRunner(d, 10*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "python",
		Code:     code,
		Cases:    cases,
		Timeout:  300 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleTimeout {
		t.Fatalf("expected %s status, got %s", model.ScaleTimeout, res.Status)
	}
}

func TestScaleRunnerGoCurve(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("go") {
		t.Skip("go not available in environment, skipping")
	}

	prob := &model.Problem{
		ID:   "scale-001",
		Slug: "sum_all",
	}

	makeOnes := func(n int) []int {
		nums := make([]int, n)
		for i := range nums {
			nums[i] = 1
		}
		return nums
	}

	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": makeOnes(10)}, Expected: 10},
		},
		"medium": {
			{ID: "m1", Input: map[string]interface{}{"nums": makeOnes(1000)}, Expected: 1000},
		},
		"large": {
			{ID: "l1", Input: map[string]interface{}{"nums": makeOnes(100000)}, Expected: 100000},
		},
	}

	code := `package main

func sumAll(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}
`

	runner := NewCurveRunner(d, 15*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "go",
		Code:     code,
		Cases:    cases,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleAccepted {
		t.Fatalf("expected %s status, got %s (stderr: %s)", model.ScaleAccepted, res.Status, res.StdErr)
	}
	if len(res.Times) < 3 {
		t.Fatalf("expected at least 3 scale timings, got %d", len(res.Times))
	}
	for _, scale := range []string{"small", "medium", "large"} {
		v, ok := res.Times[scale]
		if !ok || v < 0 {
			t.Fatalf("expected scale %q in Times with value >= 0, got %v", scale, v)
		}
	}
	if res.GrowthFactor <= 0 {
		t.Fatalf("expected GrowthFactor > 0, got %f", res.GrowthFactor)
	}
}

func TestScaleRunnerUsesProblemScaleCasesAndAlias(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	prob := &model.Problem{
		ID:   "scale-002",
		Slug: "sum_all",
		ScaleTestCases: map[string][]model.TestCase{
			"Scale_Small": {
				{ID: "s1", Input: map[string]interface{}{"nums": []int{1, 2, 3}}, Expected: 6},
			},
			"Scale_Large": {
				{ID: "l1", Input: map[string]interface{}{"nums": makeInts(1000)}, Expected: 1000},
			},
		},
	}

	code := `def sum_all(nums):
    total = 0
    for n in nums:
        total += n
    return total
`
	runner := NewCurveRunner(d, 5*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:    prob,
		Language:   "py",
		Code:       code,
		ScaleSizes: map[string]int{"Scale_Small": 3, "Scale_Large": 1000},
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleAccepted {
		t.Fatalf("expected Accepted status, got %s (stderr: %s)", res.Status, res.StdErr)
	}
	if res.Language != "python" {
		t.Fatalf("expected normalized language python, got %q", res.Language)
	}
	if _, ok := res.Times[string(model.ScaleSmall)]; !ok {
		t.Fatal("expected the 'Scale_Small' fixture to normalize to 'small'")
	}
	if res.Score <= 0 || res.Score > 1000 {
		t.Fatalf("expected a curve score in (0,1000], got %f", res.Score)
	}
}

func makeInts(n int) []int {
	nums := make([]int, n)
	for i := range nums {
		nums[i] = 1
	}
	return nums
}

func TestScaleRunnerPythonSolutionMethodResolution(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("python") {
		t.Skip("python not available in environment, skipping")
	}

	prob := &model.Problem{ID: "scale-003", Slug: "two_sum"}
	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": []int{2, 7}, "target": 9}, Expected: []int{0, 1}},
		},
		"large": {
			{ID: "l1", Input: map[string]interface{}{"nums": []int{2, 7}, "target": 9}, Expected: []int{0, 1}},
		},
	}
	code := `class Solution:
    def helper(self, value):
        return value

    def twoSum(self, nums, target):
        seen = {}
        for i, n in enumerate(nums):
            if target - n in seen:
                return [seen[target - n], i]
            seen[n] = i
        return []
`
	runner := NewCurveRunner(d, 5*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "python",
		Code:     code,
		Cases:    cases,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleAccepted {
		t.Fatalf("expected the camelCase twoSum method to be selected, got %s (stderr: %s)", res.Status, res.StdErr)
	}
}

func TestScaleRunnerGoMultiArgumentMapping(t *testing.T) {
	d := NewDetector()
	if !d.IsAvailable("go") {
		t.Skip("go not available in environment, skipping")
	}

	prob := &model.Problem{ID: "scale-004", Slug: "two_sum"}
	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": []int{2, 7}, "target": 9}, Expected: []int{0, 1}},
		},
		"large": {
			{ID: "l1", Input: map[string]interface{}{"nums": []int{3, 2, 4}, "target": 6}, Expected: []int{1, 2}},
		},
	}
	code := `package main

func twoSum(nums []int, target int) []int {
	seen := make(map[int]int)
	for i, n := range nums {
		if j, ok := seen[target-n]; ok {
			return []int{j, i}
		}
		seen[n] = i
	}
	return nil
}
`
	runner := NewCurveRunner(d, 15*time.Second)
	res, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "go",
		Code:     code,
		Cases:    cases,
	})
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if res.Status != model.ScaleAccepted {
		t.Fatalf("expected multi-argument mapping to pass, got %s (stderr: %s)", res.Status, res.StdErr)
	}
}

func TestScaleRunnerRejectsMissingScales(t *testing.T) {
	d := NewDetector()
	prob := &model.Problem{
		ID:   "scale-001",
		Slug: "sum_all",
	}

	cases := map[string][]model.TestCase{
		"small": {
			{ID: "s1", Input: map[string]interface{}{"nums": []int{1}}, Expected: 1},
		},
	}

	runner := NewCurveRunner(d, 5*time.Second)
	_, err := runner.Run(context.Background(), ScaleRequest{
		Problem:  prob,
		Language: "python",
		Code:     "def sum_all(nums): return 0",
		Cases:    cases,
	})
	if err == nil {
		t.Fatal("expected error for missing required large scale, got nil")
	}
}

func TestRankWithinLanguage(t *testing.T) {
	results := []model.ScaleResult{
		{
			ProblemID:    "p1",
			Language:     "python",
			Status:       model.ScaleAccepted,
			GrowthFactor: 4.0,
		},
		{
			ProblemID:    "p2",
			Language:     "python",
			Status:       model.ScaleAccepted,
			GrowthFactor: 2.0,
		},
		{
			ProblemID:    "p3",
			Language:     "python",
			Status:       model.ScaleAccepted,
			GrowthFactor: 8.0,
		},
		{
			ProblemID:    "g1",
			Language:     "go",
			Status:       model.ScaleAccepted,
			GrowthFactor: 3.0,
		},
		{
			ProblemID:    "p_failed",
			Language:     "python",
			Status:       model.ScaleWrongAnswer,
			GrowthFactor: 1.0,
		},
	}

	ranked := RankWithinLanguage(results)
	pyRanked, ok := ranked["python"]
	if !ok {
		t.Fatal("expected 'python' group in ranked results")
	}
	if len(pyRanked) != 3 {
		t.Fatalf("expected 3 ranked python results, got %d", len(pyRanked))
	}

	// Growth order should be 2.0, 4.0, 8.0
	expectedGrowths := []float64{2.0, 4.0, 8.0}
	for i, exp := range expectedGrowths {
		if pyRanked[i].GrowthFactor != exp {
			t.Errorf("at index %d: expected GrowthFactor %f, got %f", i, exp, pyRanked[i].GrowthFactor)
		}
		if pyRanked[i].Rank != i+1 {
			t.Errorf("at index %d: expected Rank %d, got %d", i, i+1, pyRanked[i].Rank)
		}
	}

	// Scores: ~1000, ~666.67, ~333.33
	if math.Abs(pyRanked[0].Score-1000.0) > 1e-6 {
		t.Errorf("expected score ~1000, got %f", pyRanked[0].Score)
	}
	if math.Abs(pyRanked[1].Score-666.6666666666666) > 0.1 {
		t.Errorf("expected score ~666.67, got %f", pyRanked[1].Score)
	}
	if math.Abs(pyRanked[2].Score-333.3333333333333) > 0.1 {
		t.Errorf("expected score ~333.33, got %f", pyRanked[2].Score)
	}

	goRanked, ok := ranked["go"]
	if !ok {
		t.Fatal("expected 'go' group in ranked results")
	}
	if len(goRanked) != 1 {
		t.Fatalf("expected 1 ranked go result, got %d", len(goRanked))
	}
	if goRanked[0].Rank != 1 || goRanked[0].Score != 1000.0 {
		t.Errorf("expected go single result to have rank 1 and score 1000, got rank %d score %f", goRanked[0].Rank, goRanked[0].Score)
	}
}
