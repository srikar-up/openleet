package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"openleet/internal/model"
)

type diskManager struct {
	dataDir             string
	problemsDir         string
	userStatePath       string
	workspaceLayoutPath string
}

// NewManager initializes the storage manager and extracts embedded seed problems if missing.
func NewManager(dataDir string) (Manager, error) {
	problemsDir := filepath.Join(dataDir, "problems")
	if err := os.MkdirAll(problemsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create problems directory: %w", err)
	}

	// Extract embedded seed problems on first run
	if err := ExtractEmbeddedProblems(problemsDir); err != nil {
		// Log or proceed without fatal error if extraction encounters minor issues
		fmt.Fprintf(os.Stderr, "Notice: could not extract embedded problems: %v\n", err)
	}

	return &diskManager{
		dataDir:             dataDir,
		problemsDir:         problemsDir,
		userStatePath:       filepath.Join(dataDir, "user_state.json"),
		workspaceLayoutPath: filepath.Join(dataDir, "workspace_layout.json"),
	}, nil
}

// ForUser returns a manager rooted at users/<localID>. Problem files remain
// global; user state, code buffers, profile data, and layout are scoped.
func ForUser(root, localID string) (Manager, error) {
	root = strings.TrimSpace(root)
	localID = strings.TrimSpace(localID)
	if root == "" {
		return nil, fmt.Errorf("data root is required")
	}
	if !validLocalID(localID) {
		return nil, fmt.Errorf("invalid local profile id %q", localID)
	}
	userDir := filepath.Join(root, "users", localID)
	if err := os.MkdirAll(userDir, 0700); err != nil {
		return nil, fmt.Errorf("create user data directory: %w", err)
	}
	problemsDir := filepath.Join(root, "problems")
	if err := os.MkdirAll(problemsDir, 0755); err != nil {
		return nil, fmt.Errorf("create problems directory: %w", err)
	}
	return &diskManager{
		dataDir:             userDir,
		problemsDir:         problemsDir,
		userStatePath:       filepath.Join(userDir, "user_state.json"),
		workspaceLayoutPath: filepath.Join(userDir, "workspace_layout.json"),
	}, nil
}

// UserDir returns the scoped directory for a local profile.
func UserDir(root, localID string) string {
	return filepath.Join(root, "users", localID)
}

func validLocalID(localID string) bool {
	if !strings.HasPrefix(localID, "u_") || len(localID) < 4 || len(localID) > 64 {
		return false
	}
	for _, r := range localID {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

// RootDir exposes the data root for the app's identity and scoped factories.
func (m *diskManager) RootDir() string { return m.dataDir }

// LoadProblem loads a single problem by ID or directory slug.
func (m *diskManager) LoadProblem(problemID string) (*model.Problem, error) {
	entries, err := os.ReadDir(m.problemsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read problems directory: %w", err)
	}

	var matchedDir string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == problemID || strings.HasPrefix(name, problemID+"_") || strings.HasPrefix(name, problemID) {
			matchedDir = name
			break
		}
	}

	if matchedDir == "" {
		return nil, fmt.Errorf("problem %s not found", problemID)
	}

	problemDir := filepath.Join(m.problemsDir, matchedDir)
	metaPath := filepath.Join(problemDir, "problem.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem metadata at %s: %w", metaPath, err)
	}

	var p model.Problem
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse problem json for %s: %w", matchedDir, err)
	}

	descPath := filepath.Join(problemDir, "description.md")
	if descData, err := os.ReadFile(descPath); err == nil {
		p.Description = string(descData)
	}

	return &p, nil
}

// ListProblems returns all available problems, sorted by ID.
func (m *diskManager) ListProblems() ([]*model.Problem, error) {
	entries, err := os.ReadDir(m.problemsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read problems directory: %w", err)
	}

	var problems []*model.Problem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, err := m.LoadProblem(entry.Name())
		if err != nil {
			continue
		}
		problems = append(problems, p)
	}

	sort.Slice(problems, func(i, j int) bool {
		return problems[i].ID < problems[j].ID
	})

	return problems, nil
}

// ListProblemsByTopic filters problems matching a specific topic.
func (m *diskManager) ListProblemsByTopic(topic string) ([]*model.Problem, error) {
	all, err := m.ListProblems()
	if err != nil {
		return nil, err
	}

	var filtered []*model.Problem
	search := strings.ToLower(topic)
	for _, p := range all {
		for _, t := range p.Topics {
			if strings.ToLower(t) == search {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered, nil
}

// ListProblemsByCompany filters problems matching a specific company tag.
func (m *diskManager) ListProblemsByCompany(company string) ([]*model.Problem, error) {
	all, err := m.ListProblems()
	if err != nil {
		return nil, err
	}

	var filtered []*model.Problem
	search := strings.ToLower(company)
	for _, p := range all {
		for _, c := range p.Companies {
			if strings.ToLower(c) == search {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered, nil
}

// LoadUserState reads the user state file or returns an empty one if missing.
func (m *diskManager) LoadUserState() (*model.UserState, error) {
	if _, err := os.Stat(m.userStatePath); os.IsNotExist(err) {
		return model.NewUserState(), nil
	}

	data, err := os.ReadFile(m.userStatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user state: %w", err)
	}

	var state model.UserState
	if err := json.Unmarshal(data, &state); err != nil {
		return model.NewUserState(), nil
	}
	if state.CodeBuffers == nil {
		state.CodeBuffers = make(map[string]map[string]string)
	}
	return &state, nil
}

// SaveUserState writes the user state to disk safely.
func (m *diskManager) SaveUserState(state *model.UserState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode user state: %w", err)
	}

	return os.WriteFile(m.userStatePath, data, 0644)
}

// LoadCodeBuffer retrieves a solution buffer for a problem and language.
func (m *diskManager) LoadCodeBuffer(problemID, language string) (string, error) {
	state, err := m.LoadUserState()
	if err != nil {
		return "", err
	}
	buf := state.GetCodeBuffer(problemID, language)
	if buf != "" {
		return buf, nil
	}

	// Fallback to starter template from problem if available
	p, err := m.LoadProblem(problemID)
	if err == nil && p.Templates != nil {
		if tmpl, ok := p.Templates[language]; ok {
			return tmpl, nil
		}
	}

	return "", nil
}

// SaveCodeBuffer persists user's in-progress code.
func (m *diskManager) SaveCodeBuffer(problemID, language, code string) error {
	state, err := m.LoadUserState()
	if err != nil {
		return err
	}
	state.SetCodeBuffer(problemID, language, code)
	return m.SaveUserState(state)
}

// LoadWorkspaceLayout reads the persisted workspace layout from disk or returns
// a zero WorkspaceLayout if the file does not exist.
func (m *diskManager) LoadWorkspaceLayout() (WorkspaceLayout, error) {
	data, err := os.ReadFile(m.workspaceLayoutPath)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceLayout{}, nil
		}
		return WorkspaceLayout{}, fmt.Errorf("failed to read workspace layout: %w", err)
	}

	var layout WorkspaceLayout
	if err := json.Unmarshal(data, &layout); err != nil {
		return WorkspaceLayout{}, fmt.Errorf("failed to parse workspace layout: %w", err)
	}
	return layout, nil
}

// SaveWorkspaceLayout writes the workspace layout configuration to disk safely.
func (m *diskManager) SaveWorkspaceLayout(layout WorkspaceLayout) error {
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode workspace layout: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.workspaceLayoutPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for workspace layout: %w", err)
	}

	return os.WriteFile(m.workspaceLayoutPath, data, 0644)
}
