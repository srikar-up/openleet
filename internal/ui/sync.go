package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/model"
	"openleet/internal/network"
	"openleet/internal/storage"
	"openleet/internal/ui/settings"
)

// profileSyncResultMsg reports the outcome of one cloud sync. The local UserState
// is replaced only when the remote snapshot was merged successfully.
type profileSyncResultMsg struct {
	state  *model.UserState
	at     time.Time
	detail string
	err    error
}

// persistSyncMeta saves the sync timestamp and note to the metadata store.
func persistSyncMeta(store *storage.SyncMetaStore, uid string, at time.Time, note string) string {
	if store == nil {
		return ""
	}
	meta := &storage.SyncMeta{
		UID:        uid,
		LastSynced: at.Unix(),
		Note:       note,
	}
	if err := store.Save(meta); err != nil {
		return " · metadata not saved"
	}
	return ""
}

// profileFromState compresses the rich UserState into the Firestore profile
// schema. It is a solved-set summary; code buffers remain device-local.
func profileFromState(state *model.UserState, username string) *storage.LocalProfile {
	profile := storage.NewLocalProfile(username)
	if state == nil {
		profile.LastUpdated = 0
		return profile
	}
	profile.GlobalScore = state.TotalPoints
	profile.LastUpdated = state.LastSolvedDate.Unix()
	for _, solved := range state.SolvedProblems {
		score := 10
		if solved.OptimalSolution {
			score = 15
		}
		profile.SetSolved(solved.ProblemID, score)
	}
	return profile
}

// mergeRemoteProfile adds solved IDs that exist only remotely. Existing local
// records keep their attempt history, language, and solution code.
func mergeRemoteProfile(state *model.UserState, remote *storage.LocalProfile) (*model.UserState, int) {
	if state == nil {
		state = model.NewUserState()
	}
	merged := *state
	merged.SolvedProblems = append([]model.SolvedProblem(nil), state.SolvedProblems...)
	if merged.CodeBuffers == nil {
		merged.CodeBuffers = make(map[string]map[string]string)
	}
	if remote == nil {
		return &merged, 0
	}
	when := time.Now()
	if remote.LastUpdated > 0 {
		when = time.Unix(remote.LastUpdated, 0)
	}
	added := 0
	for id, score := range remote.Solved {
		if merged.HasSolved(id) {
			continue
		}
		merged.SolvedProblems = append(merged.SolvedProblems, model.SolvedProblem{
			ProblemID:       id,
			SolvedAt:        when,
			Attempts:        1,
			OptimalSolution: score >= 15,
		})
		added++
	}
	if remote.GlobalScore > merged.TotalPoints {
		merged.TotalPoints = remote.GlobalScore
	}
	if remote.LastUpdated > merged.LastSolvedDate.Unix() {
		merged.LastSolvedDate = when
	}
	return &merged, added
}

// syncNowCmd pulls the remote snapshot, merges it into the local UserState, and
// pushes the merged summary back. The encrypted local profile is written before
// the network push so a failed sync never loses local progress.
func (m AppModel) syncNowCmd() tea.Cmd {
	session := m.account
	state := m.userState
	store := m.profileStore
	cfgPath := m.authConfigPath()
	firestoreBase := m.firestoreBaseURL
	enabled := settings.SyncEnabled(m.config)
	policy := settings.ConflictPolicy(m.config)
	metaStore := m.syncMetaStore
	return func() tea.Msg {
		if !enabled {
			return profileSyncResultMsg{err: errors.New("cloud sync is off")}
		}
		if session == nil {
			return profileSyncResultMsg{err: errors.New("sign in before syncing")}
		}
		if store == nil {
			return profileSyncResultMsg{err: errors.New("local profile store is unavailable")}
		}
		cfg, err := network.LoadSyncConfig(cfgPath)
		if err != nil {
			return profileSyncResultMsg{err: fmt.Errorf("Firestore is not configured: %w", err)}
		}
		opts := []network.Option{}
		if firestoreBase != "" {
			opts = append(opts, network.WithFirestoreBaseURL(firestoreBase))
		}
		remote, err := network.NewFirestoreService(cfg.FirebaseProjectID, opts...)
		if err != nil {
			return profileSyncResultMsg{err: err}
		}
		syncSvc, err := network.NewSyncService(remote, store)
		if err != nil {
			return profileSyncResultMsg{err: err}
		}

		if policy == "local" {
			local := profileFromState(state, session.DisplayName)
			local.Touch()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := syncSvc.SaveProfile(ctx, session.IDToken, session.UID, local); err != nil {
				return profileSyncResultMsg{err: err}
			}
			at := time.Now()
			detail := fmt.Sprintf("Pushed local progress · %d problems (local wins)", len(local.Solved))
			detail += persistSyncMeta(metaStore, session.UID, at, detail)
			return profileSyncResultMsg{at: at, detail: detail}
		}

		local := profileFromState(state, session.DisplayName)
		if err := store.SavePreservingTimestamp(local); err != nil {
			return profileSyncResultMsg{err: fmt.Errorf("save local profile: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		loaded, err := syncSvc.LoadProfile(ctx, session.IDToken, session.UID)
		if err != nil {
			return profileSyncResultMsg{err: err}
		}

		merged := state
		added := 0
		if loaded.Source == network.ProfileSourceRemote && loaded.Profile != nil {
			merged, added = mergeRemoteProfile(state, loaded.Profile)
		}
		out := profileFromState(merged, session.DisplayName)
		if loaded.Profile != nil {
			for id, score := range loaded.Profile.Solved {
				if _, ok := out.Solved[id]; ok {
					out.SetSolved(id, score)
				}
			}
		}
		out.Touch()
		pushErr := syncSvc.SaveProfile(ctx, session.IDToken, session.UID, out)
		if pushErr != nil {
			detail := ""
			if added > 0 {
				detail = "Remote progress merged locally; push failed"
			}
			return profileSyncResultMsg{
				state:  merged,
				detail: detail,
				err:    pushErr,
			}
		}
		detail := fmt.Sprintf("Synced %d problems · %d new", len(out.Solved), added)
		if errors.Is(loaded.RemoteErr, network.ErrRemoteProfileNotFound) {
			detail = fmt.Sprintf("Created cloud profile · %d problems", len(out.Solved))
		} else if loaded.RemoteErr != nil {
			detail = fmt.Sprintf("Pushed local progress · %d problems (pull warning)", len(out.Solved))
		}
		at := time.Now()
		detail += persistSyncMeta(metaStore, session.UID, at, detail)
		return profileSyncResultMsg{state: merged, at: at, detail: detail}
	}
}
