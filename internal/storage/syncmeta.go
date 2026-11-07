package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SyncMetaFileName is the standard filename for the sync metadata store.
const SyncMetaFileName = "sync-meta.json"

const maxSyncMetaFileSize = 64 * 1024 // 64 KiB

var (
	// ErrSyncMetaNotFound is returned when attempting to load sync metadata from a non-existent file.
	ErrSyncMetaNotFound = errors.New("sync metadata not found")
	// ErrInvalidSyncMeta is returned when sync metadata fails validation or is malformed.
	ErrInvalidSyncMeta = errors.New("invalid sync metadata")
)

// SyncMeta records the last successful sync state. It must never contain auth tokens or passwords.
type SyncMeta struct {
	UID        string `json:"uid"`
	LastSynced int64  `json:"last_synced"`
	Note       string `json:"note"`
}

// Validate verifies sync metadata fields without mutating them.
func (m *SyncMeta) Validate() error {
	if m == nil {
		return fmt.Errorf("%w: nil sync metadata", ErrInvalidSyncMeta)
	}
	if strings.TrimSpace(m.UID) == "" {
		return fmt.Errorf("%w: empty or whitespace uid", ErrInvalidSyncMeta)
	}
	if len(m.UID) > 256 {
		return fmt.Errorf("%w: uid exceeds 256 bytes (%d)", ErrInvalidSyncMeta, len(m.UID))
	}
	if m.LastSynced <= 0 {
		return fmt.Errorf("%w: last synced timestamp must be positive (%d)", ErrInvalidSyncMeta, m.LastSynced)
	}
	if len(m.Note) > 4096 {
		return fmt.Errorf("%w: note exceeds 4096 bytes (%d)", ErrInvalidSyncMeta, len(m.Note))
	}
	return nil
}

// SyncMetaStore provides atomic, unencrypted disk persistence for SyncMeta.
type SyncMetaStore struct {
	mu   sync.Mutex
	path string
}

// NewSyncMetaStore initializes a SyncMetaStore with path trimming, cleaning, and directory creation.
func NewSyncMetaStore(path string) (*SyncMetaStore, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("sync meta store path cannot be empty")
	}
	cleaned := filepath.Clean(trimmed)
	parent := filepath.Dir(cleaned)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sync meta store directory %s: %w", parent, err)
	}
	return &SyncMetaStore{
		path: cleaned,
	}, nil
}

// NewSyncMetaStoreIn initializes a SyncMetaStore inside dataDir using SyncMetaFileName.
func NewSyncMetaStoreIn(dataDir string) (*SyncMetaStore, error) {
	return NewSyncMetaStore(filepath.Join(dataDir, SyncMetaFileName))
}

// Path returns the destination path of the sync metadata file.
func (s *SyncMetaStore) Path() string {
	return s.path
}

// Load reads and parses SyncMeta from disk.
func (s *SyncMetaStore) Load() (*SyncMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSyncMetaNotFound, s.path)
		}
		return nil, fmt.Errorf("failed to stat sync meta at %s: %w", s.path, err)
	}

	if info.Size() > int64(maxSyncMetaFileSize) {
		return nil, fmt.Errorf("%w: sync meta file exceeds the %d-byte limit (%d bytes)", ErrInvalidSyncMeta, maxSyncMetaFileSize, info.Size())
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSyncMetaNotFound, s.path)
		}
		return nil, fmt.Errorf("failed to read sync meta at %s: %w", s.path, err)
	}

	if len(raw) > maxSyncMetaFileSize {
		return nil, fmt.Errorf("%w: sync meta file exceeds the %d-byte limit (%d bytes)", ErrInvalidSyncMeta, maxSyncMetaFileSize, len(raw))
	}

	var meta SyncMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("%w: failed to parse sync meta json: %v", ErrInvalidSyncMeta, err)
	}

	if err := meta.Validate(); err != nil {
		return nil, err
	}

	return &meta, nil
}

// Save atomically writes SyncMeta to disk with mode 0600.
func (s *SyncMetaStore) Save(meta *SyncMeta) error {
	if meta == nil {
		return fmt.Errorf("%w: cannot save nil sync meta", ErrInvalidSyncMeta)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := meta.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal sync meta: %v", ErrInvalidSyncMeta, err)
	}

	if len(data) > maxSyncMetaFileSize {
		return fmt.Errorf("%w: sync meta payload exceeds %d bytes (%d)", ErrInvalidSyncMeta, maxSyncMetaFileSize, len(data))
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".sync-meta-*.tmp")
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

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write sync meta to %s: %w", tmpPath, err)
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
