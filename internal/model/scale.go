package model

import "time"

// Scale identifies an input size fixture tier.
type Scale string

const (
	ScaleSmall  Scale = "small"
	ScaleMedium Scale = "medium"
	ScaleLarge  Scale = "large"
)

// CanonicalScaleOrder returns the standard progression of scales.
func CanonicalScaleOrder() []Scale {
	return []Scale{ScaleSmall, ScaleMedium, ScaleLarge}
}

// ScaleStatus represents the outcome of running scaling-curve fixtures.
type ScaleStatus string

const (
	ScaleAccepted     ScaleStatus = "Accepted"
	ScaleWrongAnswer  ScaleStatus = "Wrong Answer"
	ScaleTimeout      ScaleStatus = "Timeout"
	ScaleRuntimeError ScaleStatus = "Runtime Error"
)

// ScaleCaseResult is the result of executing a single test case within a scale tier.
type ScaleCaseResult struct {
	TestCaseID string        `json:"test_case_id"`
	Passed     bool          `json:"passed"`
	Actual     interface{}   `json:"actual"`
	Expected   interface{}   `json:"expected"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// ScaleMeasurement aggregates metrics for a single scale tier.
type ScaleMeasurement struct {
	Scale    Scale             `json:"scale"`
	Cases    []ScaleCaseResult `json:"cases"`
	Duration time.Duration     `json:"duration"`
	Passed   int               `json:"passed"`
	Total    int               `json:"total"`
}

// ScaleResult contains the complete scaling-curve benchmark outcome.
type ScaleResult struct {
	ProblemID string      `json:"problem_id"`
	Language  string      `json:"language"`
	Status    ScaleStatus `json:"status"`
	// Times maps each scale name to the total in-harness execution time in
	// microseconds. Process startup and compilation are deliberately excluded.
	Times map[string]float64 `json:"times_us"`
	// GrowthFactor is Times[large] / Times[small]. It is dimensionless, so slow
	// and fast machines produce comparable curves.
	GrowthFactor float64 `json:"growth_factor"`
	// NormalizedGrowth divides GrowthFactor by the large/small input-size ratio.
	// Linear work is near 1, quadratic work is near the size ratio.
	NormalizedGrowth float64 `json:"normalized_growth"`
	// Score is a hardware-independent 0..1000 curve score derived from
	// NormalizedGrowth. A perfectly linear curve scores 1000.
	Score     float64                     `json:"score"`
	Scales    map[string]ScaleMeasurement `json:"scales,omitempty"`
	Duration  time.Duration               `json:"duration"`
	StdErr    string                      `json:"stderr,omitempty"`
	Timestamp time.Time                   `json:"timestamp"`
}

// Passed returns true if the scaling run completed with accepted status.
func (r *ScaleResult) Passed() bool {
	return r != nil && r.Status == ScaleAccepted
}

// Micros returns the per-scale durations rounded down to whole microseconds.
// Times itself keeps sub-microsecond precision as float64 microseconds so very
// small fixtures do not collapse to zero.
func (r *ScaleResult) Micros() map[string]int64 {
	if r == nil {
		return nil
	}
	out := make(map[string]int64, len(r.Times))
	for name, value := range r.Times {
		out[name] = int64(value)
	}
	return out
}

// RankedScale represents a cohort percentile score within one language. Feed
// RankWithinLanguage results from the same problem and scale set; Python and Go
// never compete directly.
type RankedScale struct {
	Language     string  `json:"language"`
	ProblemID    string  `json:"problem_id"`
	GrowthFactor float64 `json:"growth_factor"`
	Score        float64 `json:"score"`
	Rank         int     `json:"rank"`
}
