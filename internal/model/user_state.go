package model

import "time"

// SolvedProblem tracks completion details of a problem by the user.
type SolvedProblem struct {
	ProblemID       string    `json:"problem_id"`
	SolvedAt        time.Time `json:"solved_at"`
	Attempts        int       `json:"attempts"`
	OptimalSolution bool      `json:"optimal_solution"`
	LastCode        string    `json:"last_code"`
	Language        string    `json:"language"`
}

// UserState tracks the user's progress, solve history, streaks, and working buffers.
type UserState struct {
	SolvedProblems []SolvedProblem              `json:"solved_problems"`
	CurrentStreak  int                          `json:"current_streak"`
	LongestStreak  int                          `json:"longest_streak"`
	LastSolvedDate time.Time                    `json:"last_solved_date"`
	TotalPoints    int                          `json:"total_points"`
	Sessions       int                          `json:"sessions"`
	CodeBuffers    map[string]map[string]string `json:"code_buffers"` // problemID -> language -> code
}

// NewUserState creates an initialized UserState.
func NewUserState() *UserState {
	return &UserState{
		SolvedProblems: make([]SolvedProblem, 0),
		CodeBuffers:    make(map[string]map[string]string),
	}
}

// HasSolved returns true if the specified problem has been solved.
func (u *UserState) HasSolved(problemID string) bool {
	for _, sp := range u.SolvedProblems {
		if sp.ProblemID == problemID {
			return true
		}
	}
	return false
}

// GetSolvedProblem retrieves solved problem records, or nil if not yet solved.
func (u *UserState) GetSolvedProblem(problemID string) *SolvedProblem {
	for i := range u.SolvedProblems {
		if u.SolvedProblems[i].ProblemID == problemID {
			return &u.SolvedProblems[i]
		}
	}
	return nil
}

// MarkSolved marks a problem solved, updates streaks and points.
func (u *UserState) MarkSolved(problemID, language, code string) {
	now := time.Now()

	if existing := u.GetSolvedProblem(problemID); existing != nil {
		existing.Attempts++
		existing.LastCode = code
		existing.Language = language
		return
	}

	u.SolvedProblems = append(u.SolvedProblems, SolvedProblem{
		ProblemID: problemID,
		SolvedAt:  now,
		Attempts:  1,
		LastCode:  code,
		Language:  language,
	})

	if u.LastSolvedDate.IsZero() {
		u.CurrentStreak = 1
	} else {
		daysSince := int(now.Sub(u.LastSolvedDate).Hours() / 24)
		if daysSince == 1 {
			u.CurrentStreak++
		} else if daysSince > 1 {
			u.CurrentStreak = 1
		}
	}

	if u.CurrentStreak > u.LongestStreak {
		u.LongestStreak = u.CurrentStreak
	}

	u.LastSolvedDate = now
	u.TotalPoints += 10
}

// GetCodeBuffer returns the saved working buffer for a problem and language.
func (u *UserState) GetCodeBuffer(problemID, language string) string {
	if langs, ok := u.CodeBuffers[problemID]; ok {
		return langs[language]
	}
	return ""
}

// SetCodeBuffer stores working code in memory.
func (u *UserState) SetCodeBuffer(problemID, language, code string) {
	if u.CodeBuffers == nil {
		u.CodeBuffers = make(map[string]map[string]string)
	}
	if u.CodeBuffers[problemID] == nil {
		u.CodeBuffers[problemID] = make(map[string]string)
	}
	u.CodeBuffers[problemID][language] = code
}
