package ui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/config"
	"openleet/internal/identity"
	"openleet/internal/model"
	"openleet/internal/storage"
	"openleet/internal/ui/toast"
)

// identityState is the per-process view of identity.enc plus the active local
// namespace. It contains no tokens or email addresses.
type identityState struct {
	Root     string
	Store    *identity.Store
	Data     identity.Identity
	Profile  identity.Profile
	Migrated bool
	Err      error
}

func storageRoot(cfg *config.Config, mgr storage.Manager) string {
	if cfg != nil && strings.TrimSpace(cfg.DataDir()) != "" {
		return cfg.DataDir()
	}
	if provider, ok := mgr.(interface{ RootDir() string }); ok {
		return provider.RootDir()
	}
	return ""
}

func openIdentityState(cfg *config.Config, mgr storage.Manager, sessionStore *authSessionStore) *identityState {
	root := storageRoot(cfg, mgr)
	if root == "" {
		return nil
	}
	rememberedUID := ""
	if sessionStore != nil {
		if stored, err := sessionStore.Load(); err == nil {
			rememberedUID = stored.UID
		}
	}
	store, data, profile, migrated, err := identity.Open(root, rememberedUID)
	if err != nil {
		return &identityState{Root: root, Store: store, Err: err}
	}
	return &identityState{
		Root:     root,
		Store:    store,
		Data:     data,
		Profile:  profile,
		Migrated: migrated,
	}
}

// recoveryManager keeps a corrupt identity from falling back to another user's
// legacy or scoped data. The error is surfaced separately.
func (s *identityState) recoveryManager() (storage.Manager, string) {
	if s == nil || s.Root == "" {
		return nil, ""
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, ""
	}
	localID := "u_recovery_" + hex.EncodeToString(raw[:])
	mgr, err := storage.ForUser(s.Root, localID)
	if err != nil {
		return nil, ""
	}
	return mgr, localID
}

// userDataDir is the current profile's writable directory.
func (m AppModel) userDataDir() string {
	if m.identity != nil && m.localID != "" {
		return storage.UserDir(m.identity.Root, m.localID)
	}
	if m.config != nil {
		return m.config.DataDir()
	}
	return ""
}

// switchProfile replaces every per-user store and clears visible user data
// before loading the new namespace.
func (m AppModel) switchProfile(profile identity.Profile, data identity.Identity) (AppModel, tea.Cmd) {
	if m.identity == nil {
		return m, nil
	}
	if profile.LocalID == "" {
		return m, nil
	}
	if profile.LocalID == m.localID {
		m.identity.Data = data
		m.identity.Profile = profile
		return m, nil
	}
	mgr, err := storage.ForUser(m.identity.Root, profile.LocalID)
	if err != nil {
		m.identity.Err = err
		return m, m.notify(toast.Error, "Local profile unavailable", err.Error(), toast.Options{})
	}
	dir := storage.UserDir(m.identity.Root, profile.LocalID)
	profileStore, profileErr := storage.NewProfileStoreIn(dir)
	if profileErr != nil {
		m.identity.Err = profileErr
		return m, m.notify(toast.Error, "Local profile unavailable", profileErr.Error(), toast.Options{})
	}
	syncStore, syncErr := storage.NewSyncMetaStoreIn(dir)
	if syncErr != nil {
		m.identity.Err = syncErr
		return m, m.notify(toast.Error, "Local profile unavailable", syncErr.Error(), toast.Options{})
	}

	m.storage = mgr
	m.localID = profile.LocalID
	m.identity.Data = data
	m.identity.Profile = profile
	m.identity.Err = nil
	m.profileStore = profileStore
	m.syncMetaStore = syncStore
	m.syncMeta, _ = syncStore.Load()
	m.lastSynced = time.Time{}
	m.syncNote = ""
	m.leaderboard = nil
	m.leaderboardLoaded = false
	m.leaderboardLoading = false
	m.leaderboardErr = ""

	empty := model.NewUserState()
	m.userState = empty
	m.homeView.SetUserState(empty)
	m.mapView.SetUserState(empty)
	m.statsView.SetUserState(empty)

	workspaceCmd := m.workspace.SetStorage(mgr, profile.LocalID)
	return m, tea.Batch(m.loadUserStateCmd(), workspaceCmd)
}

// switchForLogin resolves uid to its profile. An unowned guest is claimed only
// when it is the currently active profile; otherwise a new namespace is made.
func (m AppModel) switchForLogin(uid string) (AppModel, tea.Cmd, error) {
	if m.identity == nil || m.identity.Store == nil {
		return m, nil, nil
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return m, nil, fmt.Errorf("firebase account has no uid")
	}
	if existing, ok := m.identity.Data.Resolve(uid); ok {
		next, cmd := m.switchProfile(existing, m.identity.Data)
		return next, cmd, nil
	}
	var (
		data    identity.Identity
		profile identity.Profile
		err     error
	)
	if guest, ok := m.identity.Data.Guest(); ok && guest.LocalID == m.identity.Profile.LocalID {
		data, profile, err = m.identity.Store.ClaimGuest(uid)
	} else {
		data, profile, err = m.identity.Store.CreateProfile(uid)
	}
	if err != nil {
		return m, nil, err
	}
	next, cmd := m.switchProfile(profile, data)
	return next, cmd, nil
}

// switchToGuest enters an unowned guest namespace. It never falls back into an
// owned account profile.
func (m AppModel) switchToGuest() (AppModel, tea.Cmd, error) {
	if m.identity == nil || m.identity.Store == nil {
		return m, nil, nil
	}
	data, profile, err := m.identity.Store.EnsureGuest()
	if err != nil {
		return m, nil, err
	}
	next, cmd := m.switchProfile(profile, data)
	return next, cmd, nil
}
