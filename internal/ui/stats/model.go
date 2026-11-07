// Package stats renders the OpenLeet activity dashboard.
package stats

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/model"
)

// TopicStat counts how many solved problems mention a topic.
type TopicStat struct {
	Name  string
	Count int
}

// RankEntry is one row of the minimal public leaderboard. It intentionally
// carries no email, solution code, or per-problem history.
type RankEntry struct {
	UID    string
	Name   string
	Score  int
	Solved int
}

// Summary aggregates the user's roadmap and solve history.
type Summary struct {
	Total  int
	Solved int

	EasyTotal    int
	EasySolved   int
	MediumTotal  int
	MediumSolved int
	HardTotal    int
	HardSolved   int

	ActiveDays int
	BestStreak int
	Sessions   int

	Topics []TopicStat
	Days   map[string]int
}

// Model is the Bubble Tea component for the stats page.
type Model struct {
	width     int
	height    int
	problems  []*model.Problem
	userState *model.UserState
	pal       palette

	leaderboard         []RankEntry
	leaderboardSelf     string
	leaderboardSignedIn bool
	leaderboardLoading  bool
	leaderboardErr      string
}

// New constructs an initialized stats model.
func New(problems []*model.Problem, userState *model.UserState, width, height int) Model {
	return Model{
		width:     width,
		height:    height,
		problems:  problems,
		userState: userState,
		pal:       defaultPalette(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
	}
	return m, nil
}

// SetProblems replaces the problem catalogue.
func (m *Model) SetProblems(problems []*model.Problem) { m.problems = problems }

// SetUserState replaces the progress snapshot.
func (m *Model) SetUserState(state *model.UserState) { m.userState = state }

// SetLeaderboard replaces the public ranking rows and their display state.
// Entries are copied so later network results cannot mutate the rendered model.
func (m *Model) SetLeaderboard(entries []RankEntry, selfUID string, signedIn, loading bool, errText string) {
	m.leaderboard = append([]RankEntry(nil), entries...)
	m.leaderboardSelf = selfUID
	m.leaderboardSignedIn = signedIn
	m.leaderboardLoading = loading
	m.leaderboardErr = errText
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Width returns the current viewport width.
func (m Model) Width() int { return m.width }

// Height returns the current viewport height.
func (m Model) Height() int { return m.height }

// View renders the dashboard.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	return m.render(summarize(m.problems, m.userState))
}

// summarize computes every figure shown on the page.
func summarize(problems []*model.Problem, state *model.UserState) Summary {
	summary := Summary{Days: make(map[string]int)}
	for _, p := range problems {
		if p == nil {
			continue
		}
		summary.Total++
		switch p.Difficulty {
		case model.DifficultyEasy:
			summary.EasyTotal++
		case model.DifficultyMedium:
			summary.MediumTotal++
		case model.DifficultyHard:
			summary.HardTotal++
		}
	}
	if state == nil {
		return summary
	}

	topicCounts := make(map[string]int)
	for _, p := range problems {
		if p == nil || !state.HasSolved(p.ID) {
			continue
		}
		summary.Solved++
		switch p.Difficulty {
		case model.DifficultyEasy:
			summary.EasySolved++
		case model.DifficultyMedium:
			summary.MediumSolved++
		case model.DifficultyHard:
			summary.HardSolved++
		}
		for _, topic := range p.Topics {
			topicCounts[topic]++
		}
	}

	days := make(map[string]bool)
	for _, solved := range state.SolvedProblems {
		if solved.SolvedAt.IsZero() {
			continue
		}
		key := solved.SolvedAt.Local().Format("2006-01-02")
		days[key] = true
		summary.Days[key]++
	}
	summary.ActiveDays = len(days)
	summary.Sessions = state.Sessions
	summary.BestStreak = longestStreak(days)
	if state.LongestStreak > summary.BestStreak {
		summary.BestStreak = state.LongestStreak
	}
	if state.CurrentStreak > summary.BestStreak {
		summary.BestStreak = state.CurrentStreak
	}

	for name, count := range topicCounts {
		summary.Topics = append(summary.Topics, TopicStat{Name: name, Count: count})
	}
	sort.Slice(summary.Topics, func(i, j int) bool {
		if summary.Topics[i].Count != summary.Topics[j].Count {
			return summary.Topics[i].Count > summary.Topics[j].Count
		}
		return summary.Topics[i].Name < summary.Topics[j].Name
	})
	if len(summary.Topics) > 4 {
		summary.Topics = summary.Topics[:4]
	}
	return summary
}

// longestStreak returns the longest run of consecutive active days.
func longestStreak(days map[string]bool) int {
	if len(days) == 0 {
		return 0
	}
	dates := make([]time.Time, 0, len(days))
	for key := range days {
		parsed, err := time.ParseInLocation("2006-01-02", key, time.Local)
		if err != nil {
			continue
		}
		dates = append(dates, parsed)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	best, run := 1, 1
	for i := 1; i < len(dates); i++ {
		if dates[i].Sub(dates[i-1]) == 24*time.Hour {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
	}
	return best
}
