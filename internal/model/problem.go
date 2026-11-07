package model

import (
	"strings"
	"time"
)

// Difficulty represents the problem difficulty level.
type Difficulty string

const (
	DifficultyEasy   Difficulty = "Easy"
	DifficultyMedium Difficulty = "Medium"
	DifficultyHard   Difficulty = "Hard"
)

// TestCase represents a single test fixture for an algorithm problem.
type TestCase struct {
	ID       string      `json:"id"`
	Input    interface{} `json:"input"`
	Expected interface{} `json:"expected"`
	IsHidden bool        `json:"is_hidden"`
}

// Problem represents a DSA problem with metadata, test fixtures, and starter templates.
type Problem struct {
	ID             string                `json:"id"`
	Slug           string                `json:"slug"`
	Title          string                `json:"title"`
	Category       string                `json:"category"`
	Difficulty     Difficulty            `json:"difficulty"`
	Description    string                `json:"description"`
	Topics         []string              `json:"topics"`
	Companies      []string              `json:"companies"`
	TestCases      []TestCase            `json:"test_cases"`
	ScaleTestCases map[string][]TestCase `json:"scale_test_cases,omitempty"`
	Templates      map[string]string     `json:"templates"`
	AcceptanceRate float64               `json:"acceptance_rate"`
	CreatedAt      time.Time             `json:"created_at"`
}

// CategoryKey returns the normalized roadmap bucket for the problem. Problems
// written before the category field existed belong to the DSA roadmap.
func (p *Problem) CategoryKey() string {
	if p == nil {
		return ""
	}
	return NormalizeCategory(p.Category)
}

// NormalizeCategory lowercases a category label and strips separators so
// "LINK LIST", "link_list" and "linked-list" all compare equal.
func NormalizeCategory(value string) string {
	if value == "" {
		return "dsa"
	}
	cleaned := strings.ToLower(strings.TrimSpace(value))
	cleaned = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(cleaned)
	if cleaned == "" {
		return "dsa"
	}
	return cleaned
}

// GetVisibleTestCases returns non-hidden test fixtures.
func (p *Problem) GetVisibleTestCases() []TestCase {
	var visible []TestCase
	for _, tc := range p.TestCases {
		if !tc.IsHidden {
			visible = append(visible, tc)
		}
	}
	return visible
}
