package ui

import (
	"openleet/internal/model"
)

// ProblemLoadedMsg is dispatched when a problem is loaded into memory.
type ProblemLoadedMsg struct {
	Problem *model.Problem
	Err     error
}

// ProblemListLoadedMsg is dispatched when all problems are enumerated.
type ProblemListLoadedMsg struct {
	Problems []*model.Problem
	Err      error
}

// UserStateLoadedMsg is dispatched when user progression state is loaded.
type UserStateLoadedMsg struct {
	State   *model.UserState
	Err     error
	LocalID string
}

// ExecutionCompletedMsg is dispatched when runner finishes executing test cases.
type ExecutionCompletedMsg struct {
	Result *model.ExecutionResult
	Err    error
}

// CodeSavedMsg is dispatched when user buffer is saved to disk.
type CodeSavedMsg struct {
	ProblemID string
	Language  string
	Err       error
}

// ErrorMsg indicates a transient error to display in status bar.
type ErrorMsg struct {
	Err error
}
