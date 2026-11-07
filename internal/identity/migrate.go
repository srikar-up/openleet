package identity

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var legacyUserFiles = []string{
	"user_state.json",
	"profile.enc",
	"workspace_layout.json",
	"sync-meta.json",
}

// migrateLegacy copies the old global per-user files into one opaque profile.
// The source files are never removed. A small marker with only the opaque local
// ID makes an interrupted copy resumable and prevents a second migration.
func migrateLegacy(store *Store, root string) (Identity, bool, error) {
	usersDir := filepath.Join(root, UsersDirName)
	markerPath := filepath.Join(usersDir, MigrationMarker)

	localID := ""
	if raw, err := os.ReadFile(markerPath); err == nil {
		localID = strings.TrimSpace(string(raw))
		if !validLocalID(localID) {
			return Identity{}, false, fmt.Errorf("migration marker contains an invalid local id")
		}
	} else if !os.IsNotExist(err) {
		return Identity{}, false, fmt.Errorf("read migration marker: %w", err)
	}

	usersExisted := false
	if _, err := os.Stat(usersDir); err != nil {
		if !os.IsNotExist(err) {
			return Identity{}, false, fmt.Errorf("stat users directory: %w", err)
		}
		if !legacyFilesPresent(root) {
			return Identity{}, false, nil
		}
		if err := os.MkdirAll(usersDir, 0700); err != nil {
			return Identity{}, false, fmt.Errorf("create users directory: %w", err)
		}
	} else {
		usersExisted = true
	}
	if localID == "" {
		// An existing users/ tree without our marker already belongs to an
		// identity created by another path. Never guess at legacy ownership.
		if usersExisted || !legacyFilesPresent(root) {
			return Identity{}, false, nil
		}
		profile, err := newProfile("")
		if err != nil {
			return Identity{}, false, err
		}
		localID = profile.LocalID
	}
	if err := writeMarker(markerPath, localID); err != nil {
		return Identity{}, false, err
	}

	target := usersPath(root, localID)
	if err := os.MkdirAll(target, 0700); err != nil {
		return Identity{}, false, fmt.Errorf("create legacy profile directory: %w", err)
	}

	copied := 0
	for _, name := range legacyUserFiles {
		source := filepath.Join(root, name)
		info, err := os.Stat(source)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Identity{}, false, fmt.Errorf("stat legacy %s: %w", name, err)
		}
		if info.IsDir() {
			continue
		}
		if err := copyVerified(source, filepath.Join(target, name)); err != nil {
			return Identity{}, false, err
		}
		copied++
	}
	if copied == 0 {
		return Identity{}, false, nil
	}

	profile := Profile{LocalID: localID, CreatedAt: time.Now().UTC()}
	id := Identity{
		GuestID:   localID,
		Profiles:  map[string]Profile{localID: profile},
		ByUID:     make(map[string]string),
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Save(id); err != nil {
		return Identity{}, false, fmt.Errorf("save migrated identity: %w", err)
	}
	_ = os.Remove(markerPath)
	return id, true, nil
}

func legacyFilesPresent(root string) bool {
	for _, name := range legacyUserFiles {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func writeMarker(path, localID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create migration marker directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".migration-*.tmp")
	if err != nil {
		return fmt.Errorf("create migration marker: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err := tmp.WriteString(localID); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("install migration marker: %w", err)
	}
	ok = true
	return nil
}

func copyVerified(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".legacy-*.tmp")
	if err != nil {
		return fmt.Errorf("create legacy copy: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open legacy %s: %w", source, err)
	}
	sourceHash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, sourceHash), sourceFile); err != nil {
		_ = sourceFile.Close()
		return fmt.Errorf("copy legacy %s: %w", source, err)
	}
	if err := sourceFile.Close(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("install legacy copy %s: %w", target, err)
	}
	ok = true

	targetFile, err := os.Open(target)
	if err != nil {
		return fmt.Errorf("verify legacy copy %s: %w", target, err)
	}
	targetHash := sha256.New()
	if _, err := io.Copy(targetHash, targetFile); err != nil {
		_ = targetFile.Close()
		return fmt.Errorf("verify legacy copy %s: %w", target, err)
	}
	if err := targetFile.Close(); err != nil {
		return err
	}
	if !equalHash(sourceHash.Sum(nil), targetHash.Sum(nil)) {
		return fmt.Errorf("legacy copy verification failed for %s", target)
	}
	return nil
}

func equalHash(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
