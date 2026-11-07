// Package secrets stores provider API keys outside the plain-text settings
// file. It uses AES-256-GCM with the per-OS-user key derived by
// internal/storage, and writes mode 0600. This is the portable fallback when a
// platform keychain integration is unavailable.
package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"openleet/internal/storage"
)

const (
	magic          = "OLSK"
	version        = byte(1)
	headerSize     = len(magic) + 1
	nonceSize      = 12
	tagSize        = 16
	minSize        = headerSize + nonceSize + tagSize
	maxVaultSize   = 1 << 20
	secretFileMode = 0600
)

var (
	// ErrSecretNotFound is returned when a provider has no stored key.
	ErrSecretNotFound = errors.New("provider secret not found")
	// ErrVaultTampered is returned when the vault fails authentication.
	ErrVaultTampered = errors.New("secret vault tampering detected")
	// ErrVaultCorrupt is returned when the authenticated vault is not valid JSON.
	ErrVaultCorrupt = errors.New("secret vault is corrupt")
)

// Store persists provider API keys in one authenticated encrypted file.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore creates the parent directory and returns a vault handle. No file is
// created until the first Set call.
func NewStore(path string) (*Store, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("secret vault path cannot be empty")
	}
	cleaned := filepath.Clean(trimmed)
	if err := os.MkdirAll(filepath.Dir(cleaned), 0700); err != nil {
		return nil, fmt.Errorf("create secret vault directory: %w", err)
	}
	return &Store{path: cleaned}, nil
}

// Path returns the vault location.
func (s *Store) Path() string { return s.path }

// Set stores or replaces one provider key. Empty keys are rejected.
func (s *Store) Set(provider, key string) error {
	provider = normalizeProvider(provider)
	key = strings.TrimSpace(key)
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	if key == "" {
		return fmt.Errorf("secret key is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	values, err := s.load()
	if err != nil {
		return err
	}
	values[provider] = key
	return s.save(values)
}

// Replace writes one provider value without reading the existing vault. It is
// for dedicated single-secret vaults that must recover even when the prior
// file is unreadable or corrupt. General provider key stores should use Set.
func (s *Store) Replace(provider, key string) error {
	provider = normalizeProvider(provider)
	key = strings.TrimSpace(key)
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	if key == "" {
		return fmt.Errorf("secret key is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(map[string]string{provider: key})
}

// Get returns the full provider key. Callers must not log or render it.
func (s *Store) Get(provider string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, err := s.load()
	if err != nil {
		return "", err
	}
	key, ok := values[normalizeProvider(provider)]
	if !ok || key == "" {
		return "", ErrSecretNotFound
	}
	return key, nil
}

// Has reports whether a provider key exists.
func (s *Store) Has(provider string) bool {
	_, err := s.Get(provider)
	return err == nil
}

// Delete removes one provider key. Missing keys are not an error.
func (s *Store) Delete(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, err := s.load()
	if err != nil {
		return err
	}
	delete(values, normalizeProvider(provider))
	return s.save(values)
}

// Masked returns a safe display form such as "sk-••••••••a3f9".
func (s *Store) Masked(provider string) string {
	key, err := s.Get(provider)
	if err != nil {
		return ""
	}
	return MaskKey(key)
}

// MaskKey masks a key for UI display without exposing its middle. It shows a
// short recognizable prefix and the final four characters only.
func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) <= 8 {
		return "••••••••"
	}
	prefix := ""
	if strings.HasPrefix(key, "sk-") {
		prefix = "sk-"
	}
	suffix := string(runes[len(runes)-4:])
	return prefix + "••••••••" + suffix
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func (s *Store) load() (map[string]string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("read secret vault: %w", err)
	}
	if len(data) > maxVaultSize {
		return nil, fmt.Errorf("%w: vault is too large", ErrVaultTampered)
	}
	plaintext, err := decrypt(data)
	if err != nil {
		return nil, err
	}
	var values map[string]string
	if err := json.Unmarshal(plaintext, &values); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVaultCorrupt, err)
	}
	if values == nil {
		values = make(map[string]string)
	}
	return values, nil
}

func (s *Store) save(values map[string]string) error {
	plaintext, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode secret vault: %w", err)
	}
	raw, err := encrypt(plaintext)
	if err != nil {
		return err
	}
	return writeAtomic(s.path, raw)
}

func header() []byte {
	h := make([]byte, 0, headerSize)
	h = append(h, magic...)
	h = append(h, version)
	return h
}

func encrypt(plaintext []byte) ([]byte, error) {
	key, err := storage.DeriveDeviceKey()
	if err != nil {
		return nil, fmt.Errorf("derive device key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create vault cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create vault gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate vault nonce: %w", err)
	}
	h := header()
	ciphertext := gcm.Seal(nil, nonce, plaintext, h)
	out := make([]byte, 0, len(h)+len(nonce)+len(ciphertext))
	out = append(out, h...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func decrypt(raw []byte) ([]byte, error) {
	if len(raw) < minSize {
		return nil, fmt.Errorf("%w: vault is truncated", ErrVaultTampered)
	}
	h := header()
	if !bytes.Equal(raw[:len(h)], h) {
		return nil, fmt.Errorf("%w: vault header mismatch", ErrVaultTampered)
	}
	key, err := storage.DeriveDeviceKey()
	if err != nil {
		return nil, fmt.Errorf("derive device key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create vault cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create vault gcm: %w", err)
	}
	nonce := raw[len(h) : len(h)+nonceSize]
	ciphertext := raw[len(h)+nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, h)
	if err != nil {
		return nil, fmt.Errorf("%w: authentication failed", ErrVaultTampered)
	}
	return plaintext, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create vault directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("create vault temp file: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(secretFileMode); err != nil {
		return fmt.Errorf("chmod vault temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write vault temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync vault temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close vault temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace vault: %w", err)
	}
	ok = true
	return nil
}
