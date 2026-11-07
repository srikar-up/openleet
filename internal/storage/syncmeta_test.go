package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSyncMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSyncMetaStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create sync meta store: %v", err)
	}

	if !strings.HasSuffix(filepath.ToSlash(store.Path()), "/"+SyncMetaFileName) {
		t.Fatalf("expected store path to end with %q, got %s", SyncMetaFileName, store.Path())
	}

	meta := &SyncMeta{
		UID:        "user-abc-123",
		LastSynced: 1710000000,
		Note:       "Pushed local progress · 5 problems (local wins)",
	}

	if err := store.Save(meta); err != nil {
		t.Fatalf("failed to save sync meta: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(store.Path())
		if err != nil {
			t.Fatalf("failed to stat sync meta file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("expected file permissions 0600, got %o", info.Mode().Perm())
		}
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load sync meta: %v", err)
	}

	if loaded.UID != meta.UID {
		t.Fatalf("expected UID %q, got %q", meta.UID, loaded.UID)
	}
	if loaded.LastSynced != meta.LastSynced {
		t.Fatalf("expected LastSynced %d, got %d", meta.LastSynced, loaded.LastSynced)
	}
	if loaded.Note != meta.Note {
		t.Fatalf("expected Note %q, got %q", meta.Note, loaded.Note)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sync-meta-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("leftover temp file found: %s", entry.Name())
		}
	}
}

func TestSyncMetaNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSyncMetaStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create sync meta store: %v", err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected error loading nonexistent sync meta, got nil")
	}
	if !errors.Is(err, ErrSyncMetaNotFound) {
		t.Fatalf("expected ErrSyncMetaNotFound, got %v", err)
	}
}

func TestSyncMetaInvalidSavePreservesExisting(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSyncMetaStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create sync meta store: %v", err)
	}

	valid := &SyncMeta{
		UID:        "valid-uid",
		LastSynced: 1234567,
		Note:       "initial valid note",
	}
	if err := store.Save(valid); err != nil {
		t.Fatalf("failed to save valid meta: %v", err)
	}

	// Attempt saving nil meta
	if err := store.Save(nil); err == nil {
		t.Fatal("expected error saving nil meta, got nil")
	} else if !errors.Is(err, ErrInvalidSyncMeta) {
		t.Fatalf("expected ErrInvalidSyncMeta for nil meta, got: %v", err)
	}

	// Attempt saving invalid meta (empty UID)
	invalid := &SyncMeta{
		UID:        "   ",
		LastSynced: 1234568,
		Note:       "invalid note",
	}
	if err := store.Save(invalid); err == nil {
		t.Fatal("expected error saving invalid meta, got nil")
	} else if !errors.Is(err, ErrInvalidSyncMeta) {
		t.Fatalf("expected ErrInvalidSyncMeta, got %v", err)
	}

	// Check that previous file was not replaced or corrupted
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load meta after failed save: %v", err)
	}
	if loaded.UID != "valid-uid" || loaded.LastSynced != 1234567 || loaded.Note != "initial valid note" {
		t.Fatalf("existing file was altered: %+v", loaded)
	}

	// Ensure no temp files remained
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sync-meta-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("leftover temp file found: %s", entry.Name())
		}
	}
}

func TestSyncMetaValidation(t *testing.T) {
	var nilMeta *SyncMeta
	if err := nilMeta.Validate(); !errors.Is(err, ErrInvalidSyncMeta) {
		t.Fatalf("expected ErrInvalidSyncMeta for nil, got %v", err)
	}

	cases := []struct {
		name string
		meta SyncMeta
	}{
		{"empty UID", SyncMeta{UID: "", LastSynced: 100}},
		{"whitespace UID", SyncMeta{UID: "   \t\n", LastSynced: 100}},
		{"UID too long", SyncMeta{UID: strings.Repeat("a", 257), LastSynced: 100}},
		{"LastSynced zero", SyncMeta{UID: "uid", LastSynced: 0}},
		{"LastSynced negative", SyncMeta{UID: "uid", LastSynced: -10}},
		{"Note too long", SyncMeta{UID: "uid", LastSynced: 100, Note: strings.Repeat("x", 4097)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.meta.Validate()
			if !errors.Is(err, ErrInvalidSyncMeta) {
				t.Fatalf("expected ErrInvalidSyncMeta, got %v", err)
			}
		})
	}
}

func TestSyncMetaNewStoreEmptyPath(t *testing.T) {
	_, err := NewSyncMetaStore("   ")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestSyncMetaLoadCorruptOrOversized(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSyncMetaStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Corrupt JSON
	if err := os.WriteFile(store.Path(), []byte("{not-json"), 0600); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}
	_, err = store.Load()
	if !errors.Is(err, ErrInvalidSyncMeta) {
		t.Fatalf("expected ErrInvalidSyncMeta for corrupt json, got %v", err)
	}

	// Oversized file (> 64 KiB)
	oversized := make([]byte, 65*1024)
	if err := os.WriteFile(store.Path(), oversized, 0600); err != nil {
		t.Fatalf("failed to write oversized file: %v", err)
	}
	_, err = store.Load()
	if !errors.Is(err, ErrInvalidSyncMeta) {
		t.Fatalf("expected ErrInvalidSyncMeta for oversized file, got %v", err)
	}
}
