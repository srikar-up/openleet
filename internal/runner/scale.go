package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"openleet/internal/model"
)

// ScaleRequest configures a scaling-curve execution run.
type ScaleRequest struct {
	Problem    *model.Problem
	Language   string
	Code       string
	Cases      map[string][]model.TestCase // optional; if empty use Problem.ScaleTestCases
	ScaleSizes map[string]int              // optional; defaults small=10, medium=1000, large=100000
	Repeat     int                         // optional; default 1, clamped to 1..20
	Timeout    time.Duration               // optional; overrides runner timeout
}

// ScaleRunner defines the contract for benchmarking algorithmic growth curves.
// Run returns an error only for invalid requests. Compilation, runtime, wrong
// answer, and timeout failures are reported through ScaleResult.Status.
type ScaleRunner interface {
	Run(ctx context.Context, req ScaleRequest) (*model.ScaleResult, error)
}

// CurveRunner implements ScaleRunner using isolated subprocess sandboxes.
type CurveRunner struct {
	detector *Detector
	timeout  time.Duration
}

// NewScaleRunner constructs a ScaleRunner using host language detection.
func NewScaleRunner(detector *Detector, timeout time.Duration) ScaleRunner {
	return NewCurveRunner(detector, timeout)
}

// NewCurveRunner constructs a concrete CurveRunner.
func NewCurveRunner(detector *Detector, timeout time.Duration) *CurveRunner {
	if detector == nil {
		detector = NewDetector()
	}
	if timeout <= 0 {
		timeout = DefaultScaleTimeout()
	}
	return &CurveRunner{
		detector: detector,
		timeout:  timeout,
	}
}

// DefaultScaleTimeout returns the default timeout boundary for curve benchmarking (15s).
func DefaultScaleTimeout() time.Duration {
	return 15 * time.Second
}

func normalizeScaleLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "py", "python3":
		return "python"
	case "golang":
		return "go"
	default:
		return strings.ToLower(strings.TrimSpace(language))
	}
}

func normalizeScaleKey(key string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(key)), "scale_")
}

func scaleSize(sizes map[string]int, name string, fallback int) int {
	for key, value := range sizes {
		if normalizeScaleKey(key) == name && value > 0 {
			return value
		}
	}
	return fallback
}

// Run executes the user code across multiple scale fixtures and records timings.
func (r *CurveRunner) Run(ctx context.Context, req ScaleRequest) (*model.ScaleResult, error) {
	// 1. Validate request
	if req.Problem == nil {
		return nil, errors.New("problem must not be nil")
	}
	if strings.TrimSpace(req.Language) == "" {
		return nil, errors.New("language must not be empty")
	}
	if strings.TrimSpace(req.Code) == "" {
		return nil, errors.New("code must not be empty")
	}

	// 2. Normalize language
	language := normalizeScaleLanguage(req.Language)

	// 4. Choose cases & normalize keys
	rawCases := req.Cases
	if len(rawCases) == 0 && req.Problem != nil {
		rawCases = req.Problem.ScaleTestCases
	}
	if len(rawCases) == 0 {
		return nil, errors.New("no scale test cases provided")
	}

	cases := make(map[string][]model.TestCase)
	for k, v := range rawCases {
		normKey := normalizeScaleKey(k)
		if len(v) == 0 {
			return nil, fmt.Errorf("scale %q has an empty case list", normKey)
		}
		cases[normKey] = v
	}

	if len(cases) < 2 {
		return nil, errors.New("must contain at least two scales")
	}
	if _, ok := cases[string(model.ScaleSmall)]; !ok {
		return nil, errors.New("missing required 'small' scale")
	}
	if _, ok := cases[string(model.ScaleLarge)]; !ok {
		return nil, errors.New("missing required 'large' scale")
	}

	// 3. Detect runtime
	rt := r.detector.Detect(language)
	if !rt.Available {
		return &model.ScaleResult{
			ProblemID: req.Problem.ID,
			Language:  language,
			Status:    model.ScaleRuntimeError,
			StdErr:    fmt.Sprintf("Runtime not available: '%s' is not installed or not in PATH.", language),
			Timestamp: time.Now(),
			Times:     make(map[string]float64),
			Scales:    make(map[string]model.ScaleMeasurement),
		}, nil
	}

	// 5. Choose repeat & timeout
	repeat := req.Repeat
	if repeat <= 0 {
		repeat = 1
	} else if repeat > 20 {
		repeat = 20
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = r.timeout
	}
	if timeout <= 0 {
		timeout = DefaultScaleTimeout()
	}

	// 6. Set up context and start timer
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()

	result := &model.ScaleResult{
		ProblemID: req.Problem.ID,
		Language:  language,
		Times:     make(map[string]float64),
		Scales:    make(map[string]model.ScaleMeasurement),
		Timestamp: time.Now(),
	}

	// 7. Generate harness
	harness := newScaleHarness()
	files, runFile, err := harness.Generate(language, req.Problem, req.Code, cases, repeat)
	if err != nil {
		result.Duration = time.Since(started)
		result.Status = model.ScaleRuntimeError
		result.StdErr = fmt.Sprintf("Harness generation error: %v", err)
		return result, nil
	}

	// 8. Create temporary sandbox directory
	tmpDir, err := os.MkdirTemp("", "openleet-scale-*")
	if err != nil {
		result.Duration = time.Since(started)
		result.Status = model.ScaleRuntimeError
		result.StdErr = fmt.Sprintf("Failed to initialize sandbox directory: %v", err)
		return result, nil
	}
	defer os.RemoveAll(tmpDir)

	// 9. Write sandbox files
	for name, content := range files {
		filePath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			result.Duration = time.Since(started)
			result.Status = model.ScaleRuntimeError
			result.StdErr = fmt.Sprintf("Failed to write sandbox file %s: %v", name, err)
			return result, nil
		}
	}
	if language == "go" {
		goMod := "module openleet_scale\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			result.Duration = time.Since(started)
			result.Status = model.ScaleRuntimeError
			result.StdErr = fmt.Sprintf("Failed to initialize sandbox go.mod: %v", err)
			return result, nil
		}
	}

	// 10. Build and run command
	var cmd *exec.Cmd
	switch language {
	case "python":
		sourceFile := filepath.Join(tmpDir, runFile)
		cmd = exec.CommandContext(ctx, rt.ExecPath, sourceFile)
		cmd.Dir = tmpDir
	case "go":
		binaryName := "solver"
		if runtime.GOOS == "windows" {
			binaryName = "solver.exe"
		}
		buildCmd := exec.CommandContext(ctx, rt.ExecPath, "build", "-o", binaryName, ".")
		buildCmd.Dir = tmpDir
		var buildStdout, buildStderr bytes.Buffer
		buildCmd.Stdout = &buildStdout
		buildCmd.Stderr = &buildStderr

		if buildErr := buildCmd.Run(); buildErr != nil {
			result.Duration = time.Since(started)
			if ctx.Err() == context.DeadlineExceeded || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				result.Status = model.ScaleTimeout
				result.StdErr = fmt.Sprintf("Scale run exceeded the global time limit of %v.", timeout)
				return result, nil
			}
			result.Status = model.ScaleRuntimeError
			trimErr := strings.TrimSpace(buildStderr.String())
			if trimErr == "" {
				trimErr = buildErr.Error()
			}
			result.StdErr = trimErr
			return result, nil
		}
		cmd = exec.CommandContext(ctx, filepath.Join(tmpDir, binaryName))
		cmd.Dir = tmpDir
	default:
		result.Duration = time.Since(started)
		result.Status = model.ScaleRuntimeError
		result.StdErr = fmt.Sprintf("unsupported language: %s", language)
		return result, nil
	}

	// 11. Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmdErr := cmd.Run()

	// 12. Check context deadline
	if ctx.Err() == context.DeadlineExceeded || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Duration = time.Since(started)
		result.Status = model.ScaleTimeout
		result.StdErr = fmt.Sprintf("Scale run exceeded the global time limit of %v.", timeout)
		return result, nil
	}

	// 13. Command error without sentinel
	stdoutStr := stdout.String()
	sentinel := "__OPENLEET_SCALE__"
	idx := strings.LastIndex(stdoutStr, sentinel)
	if idx == -1 {
		result.Duration = time.Since(started)
		result.Status = model.ScaleRuntimeError
		errText := strings.TrimSpace(stderr.String())
		if errText == "" && cmdErr != nil {
			errText = cmdErr.Error()
		}
		result.StdErr = fmt.Sprintf("Missing harness sentinel in stdout.\nStdout: %s\nStderr: %s", strings.TrimSpace(stdoutStr), errText)
		return result, nil
	}

	// 14. Parse harness output
	jsonPayload := strings.TrimSpace(stdoutStr[idx+len(sentinel):])
	if nlIdx := strings.Index(jsonPayload, "\n"); nlIdx != -1 {
		jsonPayload = strings.TrimSpace(jsonPayload[:nlIdx])
	}

	type rawOutput struct {
		Scales map[string]struct {
			Cases []struct {
				TestCaseID string      `json:"test_case_id"`
				Passed     bool        `json:"passed"`
				Actual     interface{} `json:"actual"`
				Expected   interface{} `json:"expected"`
				Error      string      `json:"error"`
				DurationNs int64       `json:"duration_ns"`
			} `json:"cases"`
		} `json:"scales"`
	}

	var parsed rawOutput
	if err := json.Unmarshal([]byte(jsonPayload), &parsed); err != nil {
		result.Duration = time.Since(started)
		result.Status = model.ScaleRuntimeError
		result.StdErr = fmt.Sprintf("Failed to parse harness JSON: %v\nPayload: %s\nStderr: %s", err, jsonPayload, stderr.String())
		return result, nil
	}
	if len(parsed.Scales) == 0 {
		result.Duration = time.Since(started)
		result.Status = model.ScaleRuntimeError
		result.StdErr = "Harness returned no scale measurements."
		return result, nil
	}
	for _, required := range []string{string(model.ScaleSmall), string(model.ScaleLarge)} {
		if _, ok := parsed.Scales[required]; !ok {
			result.Duration = time.Since(started)
			result.Status = model.ScaleRuntimeError
			result.StdErr = fmt.Sprintf("Harness did not return the required %q scale.", required)
			return result, nil
		}
	}

	// 15. Build measurements and calculate status
	hasRuntimeError := false
	hasWrongAnswer := false
	var firstCaseError string

	for scaleName, scaleData := range parsed.Scales {
		measurement := model.ScaleMeasurement{
			Scale: model.Scale(scaleName),
			Cases: make([]model.ScaleCaseResult, 0, len(scaleData.Cases)),
			Total: len(scaleData.Cases),
		}
		var totalDur time.Duration
		passedCount := 0

		for _, c := range scaleData.Cases {
			dur := time.Duration(c.DurationNs) * time.Nanosecond
			totalDur += dur
			if c.Passed {
				passedCount++
			} else {
				hasWrongAnswer = true
			}
			if c.Error != "" {
				hasRuntimeError = true
				if firstCaseError == "" {
					firstCaseError = c.Error
				}
			}
			measurement.Cases = append(measurement.Cases, model.ScaleCaseResult{
				TestCaseID: c.TestCaseID,
				Passed:     c.Passed,
				Actual:     c.Actual,
				Expected:   c.Expected,
				Error:      c.Error,
				Duration:   dur,
			})
		}
		measurement.Duration = totalDur
		measurement.Passed = passedCount
		result.Scales[scaleName] = measurement
		result.Times[scaleName] = float64(totalDur.Nanoseconds()) / 1000.0
	}

	if hasRuntimeError {
		result.Status = model.ScaleRuntimeError
	} else if hasWrongAnswer {
		result.Status = model.ScaleWrongAnswer
	} else {
		result.Status = model.ScaleAccepted
	}

	if stderr.Len() > 0 {
		result.StdErr = strings.TrimSpace(stderr.String())
	} else if firstCaseError != "" {
		result.StdErr = firstCaseError
	}

	// 16. Growth factor
	smallTime := result.Times[string(model.ScaleSmall)]
	largeTime := result.Times[string(model.ScaleLarge)]
	if smallTime > 0 && largeTime > 0 {
		result.GrowthFactor = largeTime / smallTime
	}

	// 17. Normalized growth and score
	smallN := scaleSize(req.ScaleSizes, string(model.ScaleSmall), 10)
	largeN := scaleSize(req.ScaleSizes, string(model.ScaleLarge), 100000)

	if result.GrowthFactor > 0 && smallN > 0 && largeN > 0 {
		ratio := float64(largeN) / float64(smallN)
		if ratio > 0 {
			result.NormalizedGrowth = result.GrowthFactor / ratio
			diff := result.NormalizedGrowth - 1.0
			if diff < 0 {
				diff = 0
			}
			result.Score = 1000.0 / (1.0 + diff)
		}
	}

	// 18. Duration and timestamp
	result.Duration = time.Since(started)
	result.Timestamp = time.Now()
	return result, nil
}

// RankWithinLanguage ranks accepted scale results within each language group.
func RankWithinLanguage(results []model.ScaleResult) map[string][]model.RankedScale {
	groups := make(map[string][]model.ScaleResult)
	for _, res := range results {
		if res.Status != model.ScaleAccepted {
			continue
		}
		lang := strings.ToLower(strings.TrimSpace(res.Language))
		if lang == "" {
			continue
		}
		groups[lang] = append(groups[lang], res)
	}

	out := make(map[string][]model.RankedScale)
	for lang, list := range groups {
		sort.SliceStable(list, func(i, j int) bool {
			a := list[i].GrowthFactor
			b := list[j].GrowthFactor
			if a > 0 && b <= 0 {
				return true
			}
			if a <= 0 && b > 0 {
				return false
			}
			return a < b
		})

		n := len(list)
		ranked := make([]model.RankedScale, n)
		for i, item := range list {
			rank := i + 1
			var score float64
			if n == 1 {
				score = 1000.0
			} else {
				score = 1000.0 * float64(n-rank+1) / float64(n)
			}
			ranked[i] = model.RankedScale{
				Language:     item.Language,
				ProblemID:    item.ProblemID,
				GrowthFactor: item.GrowthFactor,
				Score:        score,
				Rank:         rank,
			}
		}
		out[lang] = ranked
	}
	return out
}
