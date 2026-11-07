package storage

import (
	"openleet/internal/model"
)

// Manager defines the contract for problem and user state persistence.
type Manager interface {
	// Problem operations
	LoadProblem(problemID string) (*model.Problem, error)
	ListProblems() ([]*model.Problem, error)
	ListProblemsByTopic(topic string) ([]*model.Problem, error)
	ListProblemsByCompany(company string) ([]*model.Problem, error)

	// User state operations
	LoadUserState() (*model.UserState, error)
	SaveUserState(state *model.UserState) error

	// Code buffer operations
	LoadCodeBuffer(problemID, language string) (string, error)
	SaveCodeBuffer(problemID, language, code string) error

	// Workspace layout persistence
	LoadWorkspaceLayout() (WorkspaceLayout, error)
	SaveWorkspaceLayout(layout WorkspaceLayout) error
}
