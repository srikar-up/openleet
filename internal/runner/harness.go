package runner

import (
	"encoding/json"
	"fmt"
	"strings"

	"openleet/internal/model"
)

// Harness generates language-specific test wrappers around user solutions.
type Harness struct{}

// NewHarness returns an initialized Harness generator.
func NewHarness() *Harness {
	return &Harness{}
}

// Generate creates a runnable standalone program string for the given language.
func (h *Harness) Generate(language string, problem *model.Problem, userCode string) (string, error) {
	switch strings.ToLower(language) {
	case "python":
		return h.generatePython(problem, userCode)
	case "go":
		return h.generateGo(problem, userCode)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func (h *Harness) generatePython(problem *model.Problem, userCode string) (string, error) {
	testCasesJSON, err := json.Marshal(problem.TestCases)
	if err != nil {
		return "", fmt.Errorf("failed to marshal test cases: %w", err)
	}

	var b strings.Builder
	b.WriteString(`#!/usr/bin/env python3
import json
import sys
import time
import inspect
import traceback

false = False
true = True
null = None

_test_cases = `)
	b.Write(testCasesJSON)
	b.WriteString(`

# ==================== USER SOLUTION BEGIN ====================
`)
	b.WriteString(userCode)
	b.WriteString(`
# ==================== USER SOLUTION END ======================

def _resolve_callable():
    # 1. Check if Solution class exists
    if 'Solution' in globals() and inspect.isclass(Solution):
        instance = Solution()
        # Find first non-dunder method
        methods = [m for m in dir(instance) if not m.startswith('_') and callable(getattr(instance, m))]
        if methods:
            # Prefer slug or method matching problem
            for m in methods:
                if m.lower() == "` + strings.ToLower(problem.Slug) + `":
                    return getattr(instance, m)
            return getattr(instance, methods[0])
    
    # 2. Check for module-level function matching slug or 'solution'
    for candidate in ["` + problem.Slug + `", "solution"]:
        if candidate in globals() and callable(globals()[candidate]):
            return globals()[candidate]

    # 3. Fallback: check any user defined callable
    for k, v in globals().items():
        if not k.startswith('_') and callable(v) and inspect.isfunction(v):
            return v

    raise RuntimeError("No solution callable found. Please define 'class Solution' or function matching problem.")

_fn = _resolve_callable()
_results = []

for _tc in _test_cases:
    _tc_id = _tc["id"]
    _expected = _tc["expected"]
    _inp = _tc["input"]
    _err = ""
    _actual = None
    _t0 = time.monotonic()
    
    try:
        if isinstance(_inp, dict):
            _actual = _fn(**_inp)
        elif isinstance(_inp, list):
            _actual = _fn(*_inp)
        else:
            _actual = _fn(_inp)
    except Exception:
        _err = traceback.format_exc()

    _elapsed_ms = (time.monotonic() - _t0) * 1000.0
    _passed = (_err == "") and (_actual == _expected)

    _results.append({
        "test_case_id": _tc_id,
        "passed": _passed,
        "actual": _actual,
        "expected": _expected,
        "error": _err,
        "duration_ms": round(_elapsed_ms, 3),
    })

json.dump(_results, sys.stdout)
sys.stdout.write("\n")
`)

	return b.String(), nil
}

func (h *Harness) generateGo(problem *model.Problem, userCode string) (string, error) {
	testCasesJSON, err := json.Marshal(problem.TestCases)
	if err != nil {
		return "", fmt.Errorf("failed to marshal test cases: %w", err)
	}

	var b strings.Builder
	b.WriteString(`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"
)

type TestCase struct {
	ID       string      ` + "`json:\"id\"`" + `
	Input    interface{} ` + "`json:\"input\"`" + `
	Expected interface{} ` + "`json:\"expected\"`" + `
	IsHidden bool        ` + "`json:\"is_hidden\"`" + `
}

type HarnessResult struct {
	TestCaseID string      ` + "`json:\"test_case_id\"`" + `
	Passed     bool        ` + "`json:\"passed\"`" + `
	Actual     interface{} ` + "`json:\"actual\"`" + `
	Expected   interface{} ` + "`json:\"expected\"`" + `
	Error      string      ` + "`json:\"error,omitempty\"`" + `
	DurationMs float64     ` + "`json:\"duration_ms\"`" + `
}

var rawTestCasesJSON = ` + "`" + string(testCasesJSON) + "`" + `

// ==================== USER SOLUTION BEGIN ====================
` + userCode + `
// ==================== USER SOLUTION END ======================

func main() {
	var cases []TestCase
	if err := json.Unmarshal([]byte(rawTestCasesJSON), &cases); err != nil {
		fmt.Fprintf(os.Stderr, "harness: failed to parse test fixtures: %v\n", err)
		os.Exit(1)
	}

	var results []HarnessResult
	for _, tc := range cases {
		var actual interface{}
		var errMsg string
		t0 := time.Now()

		func() {
			defer func() {
				if r := recover(); r != nil {
					errMsg = fmt.Sprintf("panic: %v", r)
				}
			}()

			// Helper generic call router: if user implements solve(interface{}) interface{}
			// or problem-specific adapters
			actual = callUserSolution(tc.Input)
		}()

		elapsedMs := float64(time.Since(t0).Microseconds()) / 1000.0
		passed := errMsg == "" && reflect.DeepEqual(actual, tc.Expected)

		results = append(results, HarnessResult{
			TestCaseID: tc.ID,
			Passed:     passed,
			Actual:     actual,
			Expected:   tc.Expected,
			Error:      errMsg,
			DurationMs: elapsedMs,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "harness: failed to encode output: %v\n", err)
		os.Exit(1)
	}
}

// Fallback caller for Go
func callUserSolution(input interface{}) interface{} {
	// If user defined a top-level solve(interface{}) interface{}
	type solver interface {
		Solve(interface{}) interface{}
	}
	return nil
}
`)

	return b.String(), nil
}

// SourceFileName returns the conventional file name for the harness code.
func SourceFileName(language string) string {
	switch strings.ToLower(language) {
	case "python":
		return "solution.py"
	case "go":
		return "main.go"
	default:
		return "solution.txt"
	}
}
