// Package identity owns the encrypted per-device local identity and the stable
// mapping between Firebase UIDs and opaque local profile IDs.
package identity

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"openleet/internal/secrets"
)

const (
	// FileName is the encrypted identity store in the data root.
	FileName = "identity.enc"
	// UsersDirName contains one opaque writable namespace per local profile.
	UsersDirName = "users"
	// MigrationMarker is a short plaintext marker containing only an opaque
	// local ID. It lets an interrupted legacy copy resume without exposing
	// account data or credentials.
	MigrationMarker = ".legacy-migration"
)

var (
	// ErrIdentityNotFound means this device has no identity file yet.
	ErrIdentityNotFound = errors.New("local identity not found")
	// ErrProfileOwned means an already-claimed profile cannot be rebound.
	ErrProfileOwned = errors.New("local profile is already owned")
)

// Profile is one opaque local storage namespace.
type Profile struct {
	LocalID   string    `json:"local_id"`
	OwnerUID  string    `json:"owner_uid,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Identity is the encrypted device state. It never contains tokens, email
// addresses, or passwords.
type Identity struct {
	GuestID   string             `json:"guest_id"`
	Profiles  map[string]Profile `json:"profiles"`
	ByUID     map[string]string  `json:"by_uid"`
	CreatedAt time.Time          `json:"created_at"`
}

// Resolve returns the local profile mapped to a Firebase UID.
func (i Identity) Resolve(uid string) (Profile, bool) {
	localID, ok := i.ByUID[strings.TrimSpace(uid)]
	if !ok {
		return Profile{}, false
	}
	profile, ok := i.Profiles[localID]
	return profile, ok
}

// Guest returns the current unowned guest profile.
func (i Identity) Guest() (Profile, bool) {
	profile, ok := i.Profiles[i.GuestID]
	if !ok || strings.TrimSpace(profile.OwnerUID) != "" {
		return Profile{}, false
	}
	return profile, true
}

func normalize(i *Identity) {
	if i.Profiles == nil {
		i.Profiles = make(map[string]Profile)
	}
	if i.ByUID == nil {
		i.ByUID = make(map[string]string)
	}
	if i.CreatedAt.IsZero() {
		i.CreatedAt = time.Now().UTC()
	}
	for uid, localID := range i.ByUID {
		if _, ok := i.Profiles[localID]; !ok {
			delete(i.ByUID, uid)
		}
	}
}

func validLocalID(localID string) bool {
	if !strings.HasPrefix(localID, "u_") || len(localID) < 4 || len(localID) > 64 {
		return false
	}
	for _, r := range localID {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func newProfile(ownerUID string) (Profile, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Profile{}, fmt.Errorf("generate local profile id: %w", err)
	}
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:]))
	return Profile{
		LocalID:   "u_" + encoded,
		OwnerUID:  strings.TrimSpace(ownerUID),
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (i *Identity) createProfile(ownerUID string) (Profile, error) {
	profile, err := newProfile(ownerUID)
	if err != nil {
		return Profile{}, err
	}
	i.Profiles[profile.LocalID] = profile
	return profile, nil
}

// Store encrypts identity.enc with the existing per-OS-user vault key.
type Store struct {
	mu    sync.Mutex
	vault *secrets.Store
}

// NewStore opens or prepares an identity store at path.
func NewStore(path string) (*Store, error) {
	vault, err := secrets.NewStore(path)
	if err != nil {
		return nil, err
	}
	return &Store{vault: vault}, nil
}

// Path returns the encrypted identity file location.
func (s *Store) Path() string {
	if s == nil || s.vault == nil {
		return ""
	}
	return s.vault.Path()
}

func (s *Store) loadLocked() (Identity, error) {
	if s == nil || s.vault == nil {
		return Identity{}, ErrIdentityNotFound
	}
	raw, err := s.vault.Get("local.identity")
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return Identity{}, ErrIdentityNotFound
		}
		return Identity{}, err
	}
	var id Identity
	if err := json.Unmarshal([]byte(raw), &id); err != nil {
		return Identity{}, fmt.Errorf("decode local identity: %w", err)
	}
	normalize(&id)
	if strings.TrimSpace(id.GuestID) == "" {
		return Identity{}, fmt.Errorf("local identity is missing its guest profile")
	}
	return id, nil
}

func (s *Store) saveLocked(id Identity) error {
	normalize(&id)
	raw, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("encode local identity: %w", err)
	}
	// identity.enc is a dedicated single-secret vault. Replace avoids reading a
	// corrupt prior copy while still refusing to create an identity implicitly.
	return s.vault.Replace("local.identity", string(raw))
}

// Load returns the encrypted identity or ErrIdentityNotFound.
func (s *Store) Load() (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Save atomically replaces the encrypted identity.
func (s *Store) Save(id Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(id)
}

// EnsureGuest creates or repairs the unowned guest profile.
func (s *Store) EnsureGuest() (Identity, Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.loadLocked()
	if errors.Is(err, ErrIdentityNotFound) {
		id = Identity{}
		normalize(&id)
	} else if err != nil {
		return Identity{}, Profile{}, err
	}
	guest, ok := id.Guest()
	if !ok {
		profile, err := id.createProfile("")
		if err != nil {
			return Identity{}, Profile{}, err
		}
		id.GuestID = profile.LocalID
		guest = profile
		if err := s.saveLocked(id); err != nil {
			return Identity{}, Profile{}, err
		}
	}
	return id, guest, nil
}

// Resolve returns the profile mapped to uid.
func (s *Store) Resolve(uid string) (Profile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.loadLocked()
	if err != nil {
		return Profile{}, false, err
	}
	profile, ok := id.Resolve(uid)
	return profile, ok, nil
}

// ClaimGuest claims the unowned guest profile for uid and rotates in a fresh
// unowned guest profile. It returns ErrProfileOwned if the guest is claimed.
func (s *Store) ClaimGuest(uid string) (Identity, Profile, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return Identity{}, Profile{}, fmt.Errorf("firebase uid is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.loadLocked()
	if err != nil {
		return Identity{}, Profile{}, err
	}
	if existing, ok := id.Resolve(uid); ok {
		return id, existing, nil
	}
	guest, ok := id.Guest()
	if !ok {
		return Identity{}, Profile{}, ErrProfileOwned
	}
	guest.OwnerUID = uid
	id.Profiles[guest.LocalID] = guest
	id.ByUID[uid] = guest.LocalID
	fresh, err := id.createProfile("")
	if err != nil {
		return Identity{}, Profile{}, err
	}
	id.GuestID = fresh.LocalID
	if err := s.saveLocked(id); err != nil {
		return Identity{}, Profile{}, err
	}
	return id, guest, nil
}

// CreateProfile creates a fresh unowned-by-default profile for uid.
func (s *Store) CreateProfile(uid string) (Identity, Profile, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return Identity{}, Profile{}, fmt.Errorf("firebase uid is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.loadLocked()
	if errors.Is(err, ErrIdentityNotFound) {
		id = Identity{}
		normalize(&id)
	} else if err != nil {
		return Identity{}, Profile{}, err
	}
	if existing, ok := id.Resolve(uid); ok {
		return id, existing, nil
	}
	profile, err := id.createProfile(uid)
	if err != nil {
		return Identity{}, Profile{}, err
	}
	id.ByUID[uid] = profile.LocalID
	if _, ok := id.Guest(); !ok {
		guest, err := id.createProfile("")
		if err != nil {
			return Identity{}, Profile{}, err
		}
		id.GuestID = guest.LocalID
	}
	if err := s.saveLocked(id); err != nil {
		return Identity{}, Profile{}, err
	}
	return id, profile, nil
}

// Open resolves the identity for this launch, performing one-time legacy
// migration and binding a remembered Firebase UID when present.
func Open(root, rememberedUID string) (*Store, Identity, Profile, bool, error) {
	if strings.TrimSpace(root) == "" {
		return nil, Identity{}, Profile{}, false, fmt.Errorf("data directory is required")
	}
	store, err := NewStore(filepath.Join(root, FileName))
	if err != nil {
		return nil, Identity{}, Profile{}, false, err
	}
	id, err := store.Load()
	migrated := false
	if errors.Is(err, ErrIdentityNotFound) {
		id, migrated, err = migrateLegacy(store, root)
		if err != nil {
			return store, Identity{}, Profile{}, false, err
		}
	} else if err != nil {
		return store, Identity{}, Profile{}, false, err
	} else {
		// A completed migration may leave only its marker if the process was
		// interrupted after identity.enc was committed.
		_ = os.Remove(filepath.Join(root, UsersDirName, MigrationMarker))
	}

	rememberedUID = strings.TrimSpace(rememberedUID)
	var profile Profile
	if rememberedUID != "" {
		if existing, ok := id.Resolve(rememberedUID); ok {
			profile = existing
		} else if guest, ok := id.Guest(); ok {
			id, profile, err = store.ClaimGuest(rememberedUID)
			if err != nil {
				return store, Identity{}, Profile{}, migrated, err
			}
			_ = guest
		} else {
			id, profile, err = store.CreateProfile(rememberedUID)
			if err != nil {
				return store, Identity{}, Profile{}, migrated, err
			}
		}
	} else {
		id, profile, err = store.EnsureGuest()
		if err != nil {
			return store, Identity{}, Profile{}, migrated, err
		}
	}
	if err := os.MkdirAll(filepath.Join(root, UsersDirName, profile.LocalID), 0700); err != nil {
		return store, Identity{}, Profile{}, migrated, err
	}
	return store, id, profile, migrated, nil
}

// usersPath returns the scoped namespace directory for a local ID.
func usersPath(root, localID string) string {
	return filepath.Join(root, UsersDirName, localID)
}
