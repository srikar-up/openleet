package storage

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"openleet/assets"
	"openleet/internal/model"
)

// ExtractEmbeddedProblems extracts seed problems from the embedded filesystem
// into the problems directory if they do not already exist.
func ExtractEmbeddedProblems(problemsDir string) error {
	if err := os.MkdirAll(problemsDir, 0755); err != nil {
		return err
	}

	return fs.WalkDir(assets.ProblemsFS, "problems", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "problems" {
			return nil
		}

		relPath, err := filepath.Rel("problems", path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(problemsDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Don't overwrite existing user problem data
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}

		data, err := assets.ProblemsFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

// LoadEmbeddedProblem loads a problem directly from the embedded binary assets.
func LoadEmbeddedProblem(dirName string) (*model.Problem, error) {
	problemPath := fmt.Sprintf("problems/%s/problem.json", dirName)
	data, err := assets.ProblemsFS.ReadFile(problemPath)
	if err != nil {
		return nil, fmt.Errorf("embedded problem %s not found: %w", dirName, err)
	}

	var p model.Problem
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse embedded problem %s: %w", dirName, err)
	}

	descPath := fmt.Sprintf("problems/%s/description.md", dirName)
	if descData, err := assets.ProblemsFS.ReadFile(descPath); err == nil {
		p.Description = string(descData)
	}

	return &p, nil
}
