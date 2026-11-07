package runner

import "time"

// Runtime describes a discovered language toolchain on the host.
type Runtime struct {
	Language   string `json:"language"`
	Available  bool   `json:"available"`
	ExecPath   string `json:"exec_path"`
	Version    string `json:"version"`
	RawVersion string `json:"raw_version"`
}

// HarnessTestCase is a single result row emitted by the harness.
type HarnessTestCase struct {
	TestCaseID string      `json:"test_case_id"`
	Passed     bool        `json:"passed"`
	Actual     interface{} `json:"actual"`
	Expected   interface{} `json:"expected"`
	Error      string      `json:"error,omitempty"`
	DurationMs float64     `json:"duration_ms"`
}

// ExecutionOutcome categorizes the result of a sandboxed run.
type ExecutionOutcome int

const (
	OutcomeSuccess ExecutionOutcome = iota
	OutcomeCompileError
	OutcomeRuntimeError
	OutcomeTimeout
	OutcomeInternalError
)

func (o ExecutionOutcome) String() string {
	switch o {
	case OutcomeSuccess:
		return "success"
	case OutcomeCompileError:
		return "compile_error"
	case OutcomeRuntimeError:
		return "runtime_error"
	case OutcomeTimeout:
		return "timeout"
	default:
		return "internal_error"
	}
}

// DefaultTimeout returns the default timeout boundary (5s).
func DefaultTimeout() time.Duration {
	return 5 * time.Second
}
