package mapview

import (
	"openleet/internal/model"
)

// OpenProblemMsg is dispatched when user presses Enter to solve the selected problem.
type OpenProblemMsg struct {
	Problem *model.Problem
}

// ProblemSelectedMsg is dispatched when the active cursor moves to another problem.
type ProblemSelectedMsg struct {
	Problem *model.Problem
	Index   int
}

// FilterChangedMsg is dispatched when the active filter changes.
type FilterChangedMsg struct {
	Filter string
}
