package model

import "time"

// TestResult represents the outcome of executing a single test case.
type TestResult struct {
	TestCaseID string        `json:"test_case_id"`
	Passed     bool          `json:"passed"`
	Actual     interface{}   `json:"actual"`
	Expected   interface{}   `json:"expected"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// ExecutionResult represents the complete run output across all test cases.
type ExecutionResult struct {
	ProblemID   string        `json:"problem_id"`
	Language    string        `json:"language"`
	Code        string        `json:"code"`
	Results     []TestResult  `json:"results"`
	TotalTests  int           `json:"total_tests"`
	PassedTests int           `json:"passed_tests"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	StdErr      string        `json:"stderr,omitempty"`
}

// IsSuccessful returns true if all test cases passed without compilation or runtime errors.
func (r *ExecutionResult) IsSuccessful() bool {
	return r.TotalTests > 0 && r.PassedTests == r.TotalTests && r.StdErr == ""
}
