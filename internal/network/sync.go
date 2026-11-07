package network

import (
	"context"
	"errors"
	"fmt"

	"openleet/internal/storage"
)

const (
	// ProfileSourceRemote means the returned profile came from Firestore.
	ProfileSourceRemote = "remote"
	// ProfileSourceLocal means the returned profile came from the encrypted
	// local file after a remote failure.
	ProfileSourceLocal = "local"
)

// LocalProfileStore is the offline half of the sync workflow. A
// *storage.ProfileStore satisfies it.
type LocalProfileStore interface {
	Load() (*storage.LocalProfile, error)
	Save(*storage.LocalProfile) error
}

// timestampPreservingStore is implemented by storage.ProfileStore. It lets the
// sync cache retain Firestore's LastActive value instead of stamping the
// snapshot with the local clock.
type timestampPreservingStore interface {
	SavePreservingTimestamp(*storage.LocalProfile) error
}

// SyncService coordinates the encrypted local profile with Firestore. The
// local file is always authoritative for offline availability; Firestore is
// the cross-device copy.
type SyncService struct {
	remote *FirestoreService
	local  LocalProfileStore
}

// NewSyncService wires a Firestore client to a local encrypted profile store.
func NewSyncService(remote *FirestoreService, local LocalProfileStore) (*SyncService, error) {
	if remote == nil {
		return nil, fmt.Errorf("%w: remote Firestore service is required", ErrInvalidInput)
	}
	if local == nil {
		return nil, fmt.Errorf("%w: local profile store is required", ErrInvalidInput)
	}
	return &SyncService{remote: remote, local: local}, nil
}

// ProfileLoad describes the result of LoadProfile. Profile is always usable
// when err is nil. RemoteErr is non-nil when Source is local because the
// network was unavailable, the token expired, or Firestore rejected the read;
// the UI can show an offline/sync warning without losing local progress.
type ProfileLoad struct {
	Profile   *storage.LocalProfile
	Source    string
	RemoteErr error
	// NeedsPush is true when local progress is newer than the remote copy or
	// when the remote was unreachable. The caller can retry SaveProfile later.
	NeedsPush bool
}

// LoadProfile loads the local profile first, then attempts one Firestore read.
// Remote wins when it is at least as new as the local copy; an older remote
// document never overwrites newer local progress. If the remote call fails and
// a local profile exists, the local profile is returned with RemoteErr set.
func (s *SyncService) LoadProfile(ctx context.Context, idToken, uid string) (*ProfileLoad, error) {
	if s == nil || s.remote == nil || s.local == nil {
		return nil, fmt.Errorf("%w: sync service is not initialized", ErrInvalidInput)
	}

	local, localErr := s.local.Load()
	remote, remoteErr := s.remote.PullProfile(ctx, idToken, uid)

	if remoteErr == nil {
		remoteProfile := localProfileFromRemote(remote)
		if localErr == nil && local.LastUpdated > remoteProfile.LastUpdated {
			return &ProfileLoad{Profile: local, Source: ProfileSourceLocal, NeedsPush: true}, nil
		}
		if err := s.cacheLocal(remoteProfile); err != nil {
			return nil, fmt.Errorf("save remote profile locally: %w", err)
		}
		return &ProfileLoad{Profile: remoteProfile, Source: ProfileSourceRemote}, nil
	}

	if localErr == nil {
		return &ProfileLoad{
			Profile:   local,
			Source:    ProfileSourceLocal,
			RemoteErr: remoteErr,
			NeedsPush: true,
		}, nil
	}
	if !errors.Is(localErr, storage.ErrProfileNotFound) {
		return nil, fmt.Errorf("load local profile: %w", localErr)
	}
	return nil, remoteErr
}

// SaveProfile writes the local encrypted file first, then pushes the same
// profile to Firestore. A remote failure therefore never loses local progress;
// the caller can retry SaveProfile when connectivity returns.
func (s *SyncService) SaveProfile(ctx context.Context, idToken, uid string, profile *storage.LocalProfile) error {
	if s == nil || s.remote == nil || s.local == nil {
		return fmt.Errorf("%w: sync service is not initialized", ErrInvalidInput)
	}
	if profile == nil {
		return fmt.Errorf("%w: profile is nil", ErrInvalidInput)
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if err := s.local.Save(profile); err != nil {
		return fmt.Errorf("save local profile: %w", err)
	}
	if err := s.remote.PushProfile(ctx, idToken, remoteProfileFromLocal(uid, profile)); err != nil {
		return fmt.Errorf("push remote profile: %w", err)
	}
	return nil
}

// cacheLocal stores a remote snapshot without advancing its LastUpdated.
func (s *SyncService) cacheLocal(profile *storage.LocalProfile) error {
	if store, ok := s.local.(timestampPreservingStore); ok {
		return store.SavePreservingTimestamp(profile)
	}
	return s.local.Save(profile)
}

// localProfileFromRemote maps the compressed Firestore schema into the local
// profile used by the TUI and encrypted file.
func localProfileFromRemote(data *FirestoreUserData) *storage.LocalProfile {
	solved := make(map[string]int, len(data.SolvedMap))
	for id, score := range data.SolvedMap {
		solved[id] = score
	}
	return &storage.LocalProfile{
		Username:    data.Username,
		GlobalScore: data.GlobalScore,
		LastUpdated: data.LastActive,
		Solved:      solved,
	}
}

// remoteProfileFromLocal maps local progress into the single Firestore
// document. The UID comes from Firebase Auth, not from user-controlled data.
func remoteProfileFromLocal(uid string, profile *storage.LocalProfile) *FirestoreUserData {
	solved := make(map[string]int, len(profile.Solved))
	for id, score := range profile.Solved {
		solved[id] = score
	}
	return &FirestoreUserData{
		UID:         uid,
		Username:    profile.Username,
		GlobalScore: profile.GlobalScore,
		LastActive:  profile.LastUpdated,
		SolvedMap:   solved,
	}
}

// LoadLocal is a local-only read for the first paint before any network work.
// It never contacts Firestore.
func (s *SyncService) LoadLocal() (*storage.LocalProfile, error) {
	if s == nil || s.local == nil {
		return nil, fmt.Errorf("%w: local profile store is required", ErrInvalidInput)
	}
	return s.local.Load()
}
