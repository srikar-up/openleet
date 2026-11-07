package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"openleet/internal/config"
	"openleet/internal/secrets"
)

const (
	// authSessionFileName is the encrypted session vault stored beside the
	// settings and provider-key vault.
	authSessionFileName = "session.enc"
	authSessionVaultKey = "firebase.session"
)

// ErrNoStoredSession reports that this device has no remembered login.
var ErrNoStoredSession = errors.New("no stored auth session")

// storedAuthSession is the encrypted on-disk identity. RefreshToken is the only
// long-lived credential; ID tokens are never written here.
type storedAuthSession struct {
	RefreshToken string    `json:"refresh_token"`
	UID          string    `json:"uid"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	PublicName   string    `json:"public_name"`
	Plan         string    `json:"plan"`
	SignedInAt   time.Time `json:"signed_in_at"`
}

// authSessionStore encrypts one remembered Firebase session with the same
// per-device key used by the provider-secret vault.
type authSessionStore struct {
	vault *secrets.Store
}

func newAuthSessionStore(path string) (*authSessionStore, error) {
	vault, err := secrets.NewStore(path)
	if err != nil {
		return nil, err
	}
	return &authSessionStore{vault: vault}, nil
}

// newAuthSessionStoreForConfig returns nil when the config directory is not
// available. A missing store degrades to the old memory-only behaviour rather
// than blocking sign-in.
func newAuthSessionStoreForConfig(cfg *config.Config) *authSessionStore {
	if cfg == nil || strings.TrimSpace(cfg.ConfigDir()) == "" {
		return nil
	}
	store, err := newAuthSessionStore(filepath.Join(cfg.ConfigDir(), authSessionFileName))
	if err != nil {
		return nil
	}
	return store
}

// Save writes the remembered session. It refuses sessions without a refresh
// token because those cannot be restored after the ID token expires.
func (s *authSessionStore) Save(session storedAuthSession) error {
	if s == nil || s.vault == nil {
		return errors.New("session store is unavailable")
	}
	if strings.TrimSpace(session.RefreshToken) == "" {
		return errors.New("session has no refresh token")
	}
	if strings.TrimSpace(session.UID) == "" {
		return errors.New("session has no user id")
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode auth session: %w", err)
	}
	// session.enc is a dedicated single-secret vault. Replace rather than Set
	// so an explicit sign-in can recover from a corrupt or unreadable prior
	// file without touching the provider-key vault.
	return s.vault.Replace(authSessionVaultKey, string(raw))
}

// Load returns the remembered session or ErrNoStoredSession.
func (s *authSessionStore) Load() (storedAuthSession, error) {
	if s == nil || s.vault == nil {
		return storedAuthSession{}, ErrNoStoredSession
	}
	raw, err := s.vault.Get(authSessionVaultKey)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return storedAuthSession{}, ErrNoStoredSession
		}
		return storedAuthSession{}, err
	}
	var session storedAuthSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return storedAuthSession{}, fmt.Errorf("decode auth session: %w", err)
	}
	if strings.TrimSpace(session.RefreshToken) == "" || strings.TrimSpace(session.UID) == "" {
		return storedAuthSession{}, ErrNoStoredSession
	}
	return session, nil
}

// Clear removes the remembered session. It deletes the dedicated session
// vault outright so a corrupted file can be replaced by the next sign-in.
func (s *authSessionStore) Clear() error {
	if s == nil || s.vault == nil {
		return nil
	}
	if err := os.Remove(s.vault.Path()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
