package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"openleet/internal/model"
)

// Executor manages sandboxed execution of user solutions against test fixtures.
type Executor struct {
	mu       sync.RWMutex
	detector *Detector
	timeout  time.Duration
}

// NewExecutor constructs an Executor with runtime detector and timeout boundary.
func NewExecutor(detector *Detector, timeout time.Duration) *Executor {
	if detector == nil {
		detector = NewDetector()
	}
	if timeout <= 0 {
		timeout = DefaultTimeout()
	}
	return &Executor{
		detector: detector,
		timeout:  timeout,
	}
}

// SetTimeout updates the per-execution wall-clock limit for future runs.
func (e *Executor) SetTimeout(timeout time.Duration) {
	if e == nil {
		return
	}
	if timeout <= 0 {
		timeout = DefaultTimeout()
	}
	e.mu.Lock()
	e.timeout = timeout
	e.mu.Unlock()
}

// SetDetector swaps the runtime detector for future executions.
func (e *Executor) SetDetector(detector *Detector) {
	if e == nil || detector == nil {
		return
	}
	e.mu.Lock()
	e.detector = detector
	e.mu.Unlock()
}

func (e *Executor) runtimeSnapshot() (*Detector, time.Duration) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	detector := e.detector
	timeout := e.timeout
	if detector == nil {
		detector = NewDetector()
	}
	if timeout <= 0 {
		timeout = DefaultTimeout()
	}
	return detector, timeout
}

// ExecuteCustomCase runs a single user-authored fixture. The original problem
// metadata is kept for templates/slug resolution, but only the supplied
// input/expected pair is executed.
func (e *Executor) ExecuteCustomCase(ctx context.Context, problem *model.Problem, language, userCode string, input, expected interface{}) *model.ExecutionResult {
	if problem == nil {
		return &model.ExecutionResult{
			Language:  language,
			Code:      userCode,
			Timestamp: time.Now(),
			StdErr:    "no problem selected",
		}
	}
	custom := *problem
	custom.TestCases = []model.TestCase{{
		ID:       "custom",
		Input:    input,
		Expected: expected,
	}}
	return e.Execute(ctx, &custom, language, userCode)
}

// Execute wraps userCode with test fixtures, runs in a sandboxed subprocess, and parses results.
func (e *Executor) Execute(ctx context.Context, problem *model.Problem, language string, userCode string) *model.ExecutionResult {
	result := &model.ExecutionResult{
		ProblemID: problem.ID,
		Language:  language,
		Code:      userCode,
		Timestamp: time.Now(),
		Results:   make([]model.TestResult, 0),
	}

	// 1. Verify host runtime
	detector, timeout := e.runtimeSnapshot()
	rt := detector.Detect(language)
	if !rt.Available {
		result.StdErr = fmt.Sprintf("Runtime not available: '%s' is not installed or not in PATH.", language)
		return result
	}

	// 2. Generate harness source
	harness := NewHarness()
	source, err := harness.Generate(language, problem, userCode)
	if err != nil {
		result.StdErr = fmt.Sprintf("Harness generation error: %v", err)
		return result
	}

	// 3. Write source into sandbox directory
	tmpDir, err := os.MkdirTemp("", "openleet-run-*")
	if err != nil {
		result.StdErr = fmt.Sprintf("Failed to initialize sandbox directory: %v", err)
		return result
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, SourceFileName(language))
	if err := os.WriteFile(srcFile, []byte(source), 0644); err != nil {
		result.StdErr = fmt.Sprintf("Failed to write sandbox source file: %v", err)
		return result
	}

	// Python tracebacks point into the generated harness file. Translate frames
	// that land inside the user's code back to editor line numbers and hide the
	// ephemeral sandbox path.
	userStart := userCodeStartLine(source)
	userLines := strings.Count(userCode, "\n") + 1
	remapError := func(text string) string {
		if text == "" {
			return text
		}
		if !strings.EqualFold(language, "python") {
			return text
		}
		text = strings.ReplaceAll(text, srcFile, SourceFileName(language))
		return remapPythonTraceback(text, userStart, userLines)
	}

	if strings.ToLower(language) == "go" {
		goMod := "module openleet_run\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			result.StdErr = fmt.Sprintf("Failed to initialize sandbox go.mod: %v", err)
			return result
		}
	}

	// 4. Set up process with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := e.buildCommand(cmdCtx, rt, srcFile, tmpDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Start()
	if runErr != nil {
		result.StdErr = fmt.Sprintf("Failed to launch subprocess: %v", runErr)
		return result
	}

	// Channel to wait for completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-cmdCtx.Done():
		// Timeout or context cancelled: kill the entire process tree immediately
		_ = killProcessTree(cmd)
		<-done // wait for release
		result.Duration = time.Since(startTime)
		result.StdErr = fmt.Sprintf("Time Limit Exceeded: Execution timed out after %.1f seconds.", timeout.Seconds())
		return result

	case waitErr := <-done:
		result.Duration = time.Since(startTime)
		outcome := classifyOutcome(waitErr, cmdCtx.Err(), stderr.String(), stdout.String())

		if outcome == OutcomeCompileError || outcome == OutcomeRuntimeError {
			if stdout.Len() == 0 {
				result.StdErr = remapError(stderr.String())
				if result.StdErr == "" && waitErr != nil {
					result.StdErr = waitErr.Error()
				}
				return result
			}
		}
	}

	// 5. Parse test results from stdout JSON
	harnessCases, parseErr := parseHarnessOutput(stdout.Bytes())
	if parseErr != nil {
		result.StdErr = remapError(fmt.Sprintf("Failed to parse runner output: %v\nOutput: %s\nStderr: %s", parseErr, stdout.String(), stderr.String()))
		return result
	}

	for _, hc := range harnessCases {
		tr := model.TestResult{
			TestCaseID: hc.TestCaseID,
			Passed:     hc.Passed,
			Actual:     hc.Actual,
			Expected:   hc.Expected,
			Error:      remapError(hc.Error),
			Duration:   time.Duration(hc.DurationMs * float64(time.Millisecond)),
		}
		result.Results = append(result.Results, tr)
		if hc.Passed {
			result.PassedTests++
		}
	}
	result.TotalTests = len(result.Results)

	if stderr.Len() > 0 && result.StdErr == "" {
		result.StdErr = stderr.String()
	}

	return result
}

func (e *Executor) buildCommand(ctx context.Context, rt Runtime, srcFile, workDir string) *exec.Cmd {
	var cmd *exec.Cmd
	switch strings.ToLower(rt.Language) {
	case "python":
		cmd = exec.CommandContext(ctx, rt.ExecPath, srcFile)
	case "go":
		cmd = exec.CommandContext(ctx, rt.ExecPath, "run", ".")
		cmd.Dir = workDir
	default:
		cmd = exec.CommandContext(ctx, rt.ExecPath, srcFile)
	}

	if cmd.Dir == "" {
		cmd.Dir = workDir
	}

	setProcessGroup(cmd)
	return cmd
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return terminateProcessTree(cmd.Process.Pid)
}

func classifyOutcome(waitErr error, ctxErr error, stderrStr, stdoutStr string) ExecutionOutcome {
	if waitErr == nil {
		return OutcomeSuccess
	}
	if ctxErr == context.DeadlineExceeded {
		return OutcomeTimeout
	}
	if ctxErr == context.Canceled {
		return OutcomeTimeout
	}

	lowerStderr := strings.ToLower(stderrStr)
	if stdoutStr == "" {
		if strings.Contains(lowerStderr, "syntaxerror") ||
			strings.Contains(lowerStderr, "syntax error") ||
			strings.Contains(lowerStderr, "compile error") ||
			strings.Contains(lowerStderr, "undefined:") {
			return OutcomeCompileError
		}
		if stderrStr != "" {
			return OutcomeRuntimeError
		}
	}

	return OutcomeRuntimeError
}

func parseHarnessOutput(data []byte) ([]HarnessTestCase, error) {
	// Trim any accidental leading/trailing whitespace or empty lines
	trimmed := bytes.TrimSpace(data)
	// Find the JSON array brackets '[' and ']' in case there were print statements before
	startIdx := bytes.IndexByte(trimmed, '[')
	endIdx := bytes.LastIndexByte(trimmed, ']')

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		trimmed = trimmed[startIdx : endIdx+1]
	}

	var results []HarnessTestCase
	if err := json.Unmarshal(trimmed, &results); err != nil {
		return nil, err
	}
	return results, nil
}
