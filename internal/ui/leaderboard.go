package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/network"
	"openleet/internal/ui/stats"
)

// leaderboardResultMsg carries one leaderboard read or write result back to the
// Elm loop. Failures never block solving or syncing; they only affect the
// compact Stats ranking line.
type leaderboardResultMsg struct {
	entries []network.LeaderboardEntry
	err     error
}

// leaderboardWriteResultMsg is deliberately separate from a read result: a
// successful publish must never replace the cached ranking with an empty list.
type leaderboardWriteResultMsg struct {
	err error
}

// leaderboardEntry builds the minimal record that may be published. It is only
// available while a Firebase session exists, so signed-out users never upload
// anything.
func (m AppModel) leaderboardEntry() (network.LeaderboardEntry, bool) {
	if m.account == nil || m.account.UID == "" || m.account.IDToken == "" || m.userState == nil {
		return network.LeaderboardEntry{}, false
	}
	name := strings.TrimSpace(m.account.PublicName)
	if name == "" {
		name = "Anonymous"
	}
	return network.LeaderboardEntry{
		UID:     m.account.UID,
		Name:    name,
		Score:   m.userState.TotalPoints,
		Solved:  len(m.userState.SolvedProblems),
		Updated: time.Now().Unix(),
	}, true
}

func (m AppModel) leaderboardService() (*network.FirestoreService, error) {
	cfg, err := network.LoadSyncConfig(m.authConfigPath())
	if err != nil {
		return nil, err
	}
	opts := []network.Option{}
	if m.firestoreBaseURL != "" {
		opts = append(opts, network.WithFirestoreBaseURL(m.firestoreBaseURL))
	}
	return network.NewFirestoreService(cfg.FirebaseProjectID, opts...)
}

// publishLeaderboardCmd writes the caller's own leaderboard document. The
// write is a no-op when the app is signed out.
func (m AppModel) publishLeaderboardCmd() tea.Cmd {
	entry, ok := m.leaderboardEntry()
	if !ok {
		return nil
	}
	token := m.account.IDToken
	return func() tea.Msg {
		svc, err := m.leaderboardService()
		if err != nil {
			return leaderboardWriteResultMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := svc.PublishLeaderboard(ctx, token, entry); err != nil {
			return leaderboardWriteResultMsg{err: err}
		}
		return leaderboardWriteResultMsg{}
	}
}

// loadLeaderboardCmd reads the top ranking rows. It is a no-op when signed out.
func (m AppModel) loadLeaderboardCmd() tea.Cmd {
	if m.account == nil || m.account.IDToken == "" {
		return nil
	}
	token := m.account.IDToken
	return func() tea.Msg {
		svc, err := m.leaderboardService()
		if err != nil {
			return leaderboardResultMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		entries, err := svc.TopLeaderboard(ctx, token, network.DefaultLeaderboardLimit)
		if err != nil {
			return leaderboardResultMsg{err: err}
		}
		return leaderboardResultMsg{entries: entries}
	}
}

func (m AppModel) leaderboardRanks() []stats.RankEntry {
	ranks := make([]stats.RankEntry, 0, len(m.leaderboard))
	for _, entry := range m.leaderboard {
		ranks = append(ranks, stats.RankEntry{
			UID:    entry.UID,
			Name:   entry.Name,
			Score:  entry.Score,
			Solved: entry.Solved,
		})
	}
	return ranks
}

func (m *AppModel) refreshLeaderboardView() {
	self := ""
	if m.account != nil {
		self = m.account.UID
	}
	m.statsView.SetLeaderboard(
		m.leaderboardRanks(),
		self,
		m.account != nil,
		m.leaderboardLoading,
		m.leaderboardErr,
	)
}
