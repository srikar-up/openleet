package storage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProfileFileName is the standard filename for the encrypted profile store.
const ProfileFileName = "profile.enc"

const (
	profileMagic      = "OLPR"
	profileVersion    = byte(0x02)
	profileHeaderSize = len(profileMagic) + 1
	profileNonceSize  = 12
	profileTagSize    = 16
	profileMinSize    = profileHeaderSize + profileNonceSize + profileTagSize

	// maxProfilePlaintext bounds the JSON payload before encryption. The file
	// limit adds the fixed header, nonce, and GCM tag so a profile that Save
	// accepts can always be loaded again.
	maxProfilePlaintext = 8 * 1024 * 1024
	maxProfileFileSize  = maxProfilePlaintext + profileHeaderSize + profileNonceSize + profileTagSize
)

var (
	// ErrDataTampered is returned when decrypted data, header, nonce, or tag integrity check fails.
	ErrDataTampered = errors.New("data tampering detected")
	// ErrProfileNotFound is returned when attempting to load a profile from a non-existent file.
	ErrProfileNotFound = errors.New("profile not found")
	// ErrInvalidProfile is returned when profile fields fail validation during creation or save.
	ErrInvalidProfile = errors.New("invalid profile")
	// ErrProfileCorrupt is returned when profile payload is unparseable or fails post-decryption validation.
	ErrProfileCorrupt = errors.New("profile data is corrupt")
	// ErrProfileKeyUnavailable is returned when the OS user identity cannot be
	// read well enough to derive the per-device local encryption key.
	ErrProfileKeyUnavailable = errors.New("local profile key unavailable")
)

// deviceKeySalt version-separates the local key derivation. It is a public
// domain separator, not a secret. The actual key material comes from the OS
// user profile (username plus UID/SID or home directory) so two OS users on
// the same machine cannot read each other's profile file.
const deviceKeySalt = "openleet.local.profile.aes-256-gcm.v2"

// DeriveDeviceKey returns the AES-256 key derived from the current OS user and
// host. Other local stores (for example the provider-secret vault) use the same
// per-device material so no key is compiled into the binary.
func DeriveDeviceKey() ([]byte, error) {
	return deriveProfileKey()
}

// deriveProfileKey derives a 32-byte AES-256 key from the current OS user and
// host. The host name keeps two machines with the same username/UID from
// deriving the same local file key.
func deriveProfileKey() ([]byte, error) {
	username, uid, machine, err := deviceIdentity()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProfileKeyUnavailable, err)
	}
	return deriveProfileKeyFrom(username, uid, machine), nil
}

// deriveProfileKeyFrom is the deterministic part of the derivation. It is
// separated from deviceIdentity so tests can prove that different OS users or
// machines receive different keys.
func deriveProfileKeyFrom(username, uid, machine string) []byte {
	h := sha256.New()
	h.Write([]byte(deviceKeySalt))
	// NUL separators make the identity tuple unambiguous. Neither a POSIX
	// username nor a Windows account name can contain NUL.
	h.Write([]byte{0})
	h.Write([]byte(username))
	h.Write([]byte{0})
	h.Write([]byte(uid))
	h.Write([]byte{0})
	h.Write([]byte(machine))
	return h.Sum(nil)
}

// deviceIdentity returns a stable local identity. os/user.Current is the
// primary source; the environment, home directory, and host name are fallbacks
// for static or CGO-disabled builds where os/user may be unavailable.
func deviceIdentity() (string, string, string, error) {
	machine := ""
	if host, err := os.Hostname(); err == nil {
		machine = strings.TrimSpace(host)
	}

	if current, err := user.Current(); err == nil {
		username := strings.TrimSpace(current.Username)
		if username != "" {
			return username, strings.TrimSpace(current.Uid), machine, nil
		}
	}

	username := strings.TrimSpace(os.Getenv("USERNAME"))
	if username == "" {
		username = strings.TrimSpace(os.Getenv("USER"))
	}
	home, _ := os.UserHomeDir()
	if username != "" {
		return username, strings.TrimSpace(home), machine, nil
	}
	if strings.TrimSpace(home) != "" {
		return strings.TrimSpace(home), "", machine, nil
	}
	if machine != "" {
		return machine, "", machine, nil
	}
	return "", "", "", errors.New("no stable OS user identity found")
}

// profileHeader returns the authenticated 5-byte header: the "OLPR" magic and
// version 2 (the first version to use the OS-user-derived key).
func profileHeader() []byte {
	header := make([]byte, 0, profileHeaderSize)
	header = append(header, profileMagic...)
	header = append(header, profileVersion)
	return header
}

// encryptProfile encrypts plaintext with AES-256-GCM using a random nonce and the profile header as AAD.
func encryptProfile(plaintext []byte) ([]byte, error) {
	key, err := deriveProfileKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive profile key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	header := profileHeader()
	ciphertext := aesGCM.Seal(nil, nonce, plaintext, header)

	raw := make([]byte, 0, len(header)+len(nonce)+len(ciphertext))
	raw = append(raw, header...)
	raw = append(raw, nonce...)
	raw = append(raw, ciphertext...)
	return raw, nil
}

// decryptProfile verifies the header, extracts the nonce, and authenticates/decrypts ciphertext.
func decryptProfile(raw []byte) ([]byte, error) {
	if len(raw) < profileMinSize {
		return nil, fmt.Errorf("%w: payload shorter than %d bytes (%d bytes)", ErrDataTampered, profileMinSize, len(raw))
	}

	header := profileHeader()
	if !bytes.Equal(raw[:len(header)], header) {
		return nil, fmt.Errorf("%w: magic or version mismatch", ErrDataTampered)
	}

	key, err := deriveProfileKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive profile key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create cipher: %v", ErrDataTampered, err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create gcm: %v", ErrDataTampered, err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(raw) < len(header)+nonceSize+aesGCM.Overhead() {
		return nil, fmt.Errorf("%w: payload too short for ciphertext", ErrDataTampered)
	}

	nonce := raw[len(header) : len(header)+nonceSize]
	ciphertext := raw[len(header)+nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, header)
	if err != nil {
		return nil, fmt.Errorf("%w: gcm authentication failed: %v", ErrDataTampered, err)
	}

	return plaintext, nil
}

// LocalProfile models the user's local progress and scores.
type LocalProfile struct {
	Username    string         `json:"username"`
	GlobalScore int            `json:"global_score"`
	LastUpdated int64          `json:"last_updated"`
	Solved      map[string]int `json:"solved"`
}

// NewLocalProfile initializes a new LocalProfile with an initialized Solved map and timestamp.
func NewLocalProfile(username string) *LocalProfile {
	return &LocalProfile{
		Username:    username,
		GlobalScore: 0,
		LastUpdated: time.Now().Unix(),
		Solved:      make(map[string]int),
	}
}

// Validate verifies profile invariants without mutating fields.
func (p *LocalProfile) Validate() error {
	if p == nil {
		return fmt.Errorf("%w: nil profile", ErrInvalidProfile)
	}
	if p.GlobalScore < 0 {
		return fmt.Errorf("%w: global score cannot be negative (%d)", ErrInvalidProfile, p.GlobalScore)
	}
	for pid, score := range p.Solved {
		if strings.TrimSpace(pid) == "" {
			return fmt.Errorf("%w: empty or whitespace problem id", ErrInvalidProfile)
		}
		if score < 0 {
			return fmt.Errorf("%w: negative score (%d) for problem %q", ErrInvalidProfile, score, pid)
		}
	}
	return nil
}

// Touch updates LastUpdated to the current Unix timestamp.
func (p *LocalProfile) Touch() {
	if p != nil {
		p.LastUpdated = time.Now().Unix()
	}
}

// SetSolved records or updates the efficiency score for a solved problem ID.
func (p *LocalProfile) SetSolved(problemID string, score int) {
	if p == nil {
		return
	}
	if p.Solved == nil {
		p.Solved = make(map[string]int)
	}
	p.Solved[problemID] = score
}

// ScoreFor returns the efficiency score for the given problem ID if present.
func (p *LocalProfile) ScoreFor(problemID string) (int, bool) {
	if p == nil || p.Solved == nil {
		return 0, false
	}
	score, ok := p.Solved[problemID]
	return score, ok
}

// SolvedCount returns the count of solved problems.
func (p *LocalProfile) SolvedCount() int {
	if p == nil || p.Solved == nil {
		return 0
	}
	return len(p.Solved)
}

// ProfileStore provides atomic, encrypted disk persistence for LocalProfile.
type ProfileStore struct {
	mu   sync.Mutex
	path string
}

// NewProfileStore initializes a ProfileStore with path trimming, cleaning, and directory creation.
func NewProfileStore(path string) (*ProfileStore, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("profile store path cannot be empty")
	}
	cleaned := filepath.Clean(trimmed)
	parent := filepath.Dir(cleaned)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, fmt.Errorf("failed to create profile store directory %s: %w", parent, err)
	}
	return &ProfileStore{
		path: cleaned,
	}, nil
}

// NewProfileStoreIn initializes a ProfileStore inside dataDir using ProfileFileName.
func NewProfileStoreIn(dataDir string) (*ProfileStore, error) {
	return NewProfileStore(filepath.Join(dataDir, ProfileFileName))
}

// Path returns the destination path of the profile file.
func (s *ProfileStore) Path() string {
	return s.path
}

// Exists returns true if the profile file exists on disk.
func (s *ProfileStore) Exists() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Save atomically persists an encrypted LocalProfile with 0600 permissions and
// stamps LastUpdated to the current time.
func (s *ProfileStore) Save(profile *LocalProfile) error {
	return s.save(profile, true)
}

// SavePreservingTimestamp writes an authenticated remote snapshot without
// changing LastUpdated. Sync conflict resolution depends on the remote
// document's timestamp, so cache writes must not make it look newer.
func (s *ProfileStore) SavePreservingTimestamp(profile *LocalProfile) error {
	return s.save(profile, false)
}

func (s *ProfileStore) save(profile *LocalProfile, touch bool) error {
	if profile == nil {
		return fmt.Errorf("%w: cannot save nil profile", ErrInvalidProfile)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := profile.Validate(); err != nil {
		return err
	}
	if touch {
		profile.Touch()
	}

	plaintext, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if len(plaintext) > maxProfilePlaintext {
		return fmt.Errorf("%w: plaintext exceeds 8 MiB limit", ErrInvalidProfile)
	}

	encrypted, err := encryptProfile(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt profile: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".profile-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()

	var success bool
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(0600); err != nil {
		return fmt.Errorf("failed to chmod temp file %s: %w", tmpPath, err)
	}

	if _, err := tmpFile.Write(encrypted); err != nil {
		return fmt.Errorf("failed to write encrypted profile to %s: %w", tmpPath, err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file %s: %w", tmpPath, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmpPath, s.path, err)
	}

	success = true
	return nil
}

// Load reads and decrypts the LocalProfile from disk.
func (s *ProfileStore) Load() (*LocalProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, s.path)
		}
		return nil, fmt.Errorf("failed to stat profile at %s: %w", s.path, err)
	}

	if info.Size() > int64(maxProfileFileSize) {
		return nil, fmt.Errorf("%w: profile file exceeds the %d-byte limit (%d bytes)", ErrDataTampered, maxProfileFileSize, info.Size())
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, s.path)
		}
		return nil, fmt.Errorf("failed to read profile at %s: %w", s.path, err)
	}

	if len(raw) > maxProfileFileSize {
		return nil, fmt.Errorf("%w: profile file exceeds the %d-byte limit (%d bytes)", ErrDataTampered, maxProfileFileSize, len(raw))
	}

	plaintext, err := decryptProfile(raw)
	if err != nil {
		return nil, err
	}

	var profile LocalProfile
	if err := json.Unmarshal(plaintext, &profile); err != nil {
		return nil, fmt.Errorf("%w: failed to parse profile json: %v", ErrProfileCorrupt, err)
	}

	if err := profile.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProfileCorrupt, err)
	}

	if profile.Solved == nil {
		profile.Solved = make(map[string]int)
	}

	return &profile, nil
}

// Delete removes the profile file from disk. If missing, Delete returns nil for an idempotent reset.
func (s *ProfileStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete profile at %s: %w", s.path, err)
	}
	return nil
}

// String returns a human-readable representation of the ProfileStore.
func (s *ProfileStore) String() string {
	return fmt.Sprintf("profile store %s", s.path)
}
