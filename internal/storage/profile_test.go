package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestProfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	p := NewLocalProfile("ZaphodBeeblebrox42")
	p.GlobalScore = 420
	p.SetSolved("001", 95)
	p.SetSolved("017", 80)

	if err := store.Save(p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	if !store.Exists() {
		t.Fatal("expected store to exist after save")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load profile: %v", err)
	}

	if loaded.Username != "ZaphodBeeblebrox42" {
		t.Fatalf("expected username ZaphodBeeblebrox42, got %q", loaded.Username)
	}
	if loaded.GlobalScore != 420 {
		t.Fatalf("expected global score 420, got %d", loaded.GlobalScore)
	}
	if loaded.SolvedCount() != 2 {
		t.Fatalf("expected 2 solved problems, got %d", loaded.SolvedCount())
	}
	if s, ok := loaded.ScoreFor("001"); !ok || s != 95 {
		t.Fatalf("expected score 95 for 001, got %d, %v", s, ok)
	}
	if s, ok := loaded.ScoreFor("017"); !ok || s != 80 {
		t.Fatalf("expected score 80 for 017, got %d, %v", s, ok)
	}
	if loaded.LastUpdated <= 0 {
		t.Fatalf("expected LastUpdated > 0, got %d", loaded.LastUpdated)
	}
}

func TestProfileFreshNonce(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	p := NewLocalProfile("ArthurDent")
	p.GlobalScore = 100
	if err := store.Save(p); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	raw1, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read raw profile after first save: %v", err)
	}

	p.GlobalScore = 200
	if err := store.Save(p); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	raw2, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read raw profile after second save: %v", err)
	}

	if bytes.Equal(raw1, raw2) {
		t.Fatal("expected raw bytes to differ across saves due to fresh random nonce")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load profile after second save: %v", err)
	}
	if loaded.GlobalScore != 200 {
		t.Fatalf("expected latest global score 200, got %d", loaded.GlobalScore)
	}
}

func TestProfileTamperDetection(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	p := NewLocalProfile("FordPrefect")
	p.GlobalScore = 42
	p.SetSolved("042", 100)
	if err := store.Save(p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	original, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read original file: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(orig []byte) []byte
	}{
		{
			name: "flip last byte",
			mutate: func(orig []byte) []byte {
				m := append([]byte(nil), orig...)
				m[len(m)-1] ^= 0xff
				return m
			},
		},
		{
			name: "flip nonce byte at index 5+3",
			mutate: func(orig []byte) []byte {
				m := append([]byte(nil), orig...)
				m[5+3] ^= 0xff
				return m
			},
		},
		{
			name: "flip magic byte at 0",
			mutate: func(orig []byte) []byte {
				m := append([]byte(nil), orig...)
				m[0] ^= 0xff
				return m
			},
		},
		{
			name: "flip version byte at 4",
			mutate: func(orig []byte) []byte {
				m := append([]byte(nil), orig...)
				m[4] ^= 0xff
				return m
			},
		},
		{
			name: "truncate to len-4",
			mutate: func(orig []byte) []byte {
				return append([]byte(nil), orig[:len(orig)-4]...)
			},
		},
		{
			name: "append one byte",
			mutate: func(orig []byte) []byte {
				return append(append([]byte(nil), orig...), 0xaa)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := tc.mutate(original)
			if err := os.WriteFile(store.Path(), mutated, 0600); err != nil {
				t.Fatalf("failed to write mutated file: %v", err)
			}
			loaded, err := store.Load()
			if err == nil {
				t.Fatalf("expected error from tampered file, got loaded profile: %+v", loaded)
			}
			if !errors.Is(err, ErrDataTampered) {
				t.Fatalf("expected ErrDataTampered, got: %v", err)
			}
		})
	}
}

func TestProfilePlaintextNotLeaked(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	username := "TrillianAstra99"
	p := NewLocalProfile(username)
	p.GlobalScore = 888
	p.SetSolved("100", 99)
	if err := store.Save(p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read raw profile: %v", err)
	}

	forbiddenStrings := []string{
		username,
		"username",
		"global_score",
		"solved",
		"last_updated",
	}

	for _, s := range forbiddenStrings {
		if bytes.Contains(raw, []byte(s)) {
			t.Fatalf("found plaintext leak of %q in encrypted payload", s)
		}
	}

	key, err := deriveProfileKey()
	if err != nil {
		t.Fatalf("failed to derive device key: %v", err)
	}
	if bytes.Contains(raw, key) {
		t.Fatal("found derived encryption key inside raw profile file")
	}
}

func TestProfileNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	if store.Exists() {
		t.Fatal("expected Exists to be false on empty store")
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected error on Load from missing profile")
	}
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got: %v", err)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("expected Delete to return nil for missing file, got: %v", err)
	}
}

func TestProfileInvalid(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	// 1. Save(nil)
	err = store.Save(nil)
	if err == nil || !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile for Save(nil), got: %v", err)
	}
	if store.Exists() {
		t.Fatal("expected store not to exist after failed Save(nil)")
	}

	// 2. Negative GlobalScore
	pNegativeScore := NewLocalProfile("Marvin")
	pNegativeScore.GlobalScore = -1
	err = store.Save(pNegativeScore)
	if err == nil || !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile for negative GlobalScore, got: %v", err)
	}
	if store.Exists() {
		t.Fatal("expected store not to exist after failed save with negative GlobalScore")
	}

	// 3. Negative solved score
	pNegativeSolved := NewLocalProfile("Marvin")
	pNegativeSolved.SetSolved("001", -10)
	err = store.Save(pNegativeSolved)
	if err == nil || !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile for negative solved score, got: %v", err)
	}
	if store.Exists() {
		t.Fatal("expected store not to exist after failed save with negative solved score")
	}

	// 4. Empty problem ID
	pEmptyProblemID := NewLocalProfile("Marvin")
	pEmptyProblemID.SetSolved("", 100)
	err = store.Save(pEmptyProblemID)
	if err == nil || !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile for empty problem ID, got: %v", err)
	}
	if store.Exists() {
		t.Fatal("expected store not to exist after failed save with empty problem ID")
	}

	// 5. Whitespace problem ID
	pWhitespaceProblemID := NewLocalProfile("Marvin")
	pWhitespaceProblemID.SetSolved("   ", 100)
	err = store.Save(pWhitespaceProblemID)
	if err == nil || !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile for whitespace problem ID, got: %v", err)
	}
	if store.Exists() {
		t.Fatal("expected store not to exist after failed save with whitespace problem ID")
	}
}

func TestProfileFailedSaveKeepsPreviousFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	good := NewLocalProfile("ArthurDent")
	good.GlobalScore = 10
	if err := store.Save(good); err != nil {
		t.Fatalf("failed to save valid profile: %v", err)
	}

	bad := NewLocalProfile("ArthurDent")
	bad.GlobalScore = -1
	if err := store.Save(bad); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile, got: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("previous profile should remain loadable: %v", err)
	}
	if loaded.GlobalScore != 10 {
		t.Fatalf("expected previous global score 10, got %d", loaded.GlobalScore)
	}
}

func TestProfileCorruptPayload(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	corruptBytes, err := encryptProfile([]byte("not json"))
	if err != nil {
		t.Fatalf("failed to encrypt corrupt payload: %v", err)
	}

	if err := os.WriteFile(store.Path(), corruptBytes, 0600); err != nil {
		t.Fatalf("failed to write corrupt payload to %s: %v", store.Path(), err)
	}

	loaded, err := store.Load()
	if err == nil {
		t.Fatalf("expected error from non-json payload, got: %+v", loaded)
	}
	if !errors.Is(err, ErrProfileCorrupt) {
		t.Fatalf("expected ErrProfileCorrupt, got: %v", err)
	}
}

func TestProfileDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	p := NewLocalProfile("EddieTheComputer")
	if err := store.Save(p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	if !store.Exists() {
		t.Fatal("expected file to exist before delete")
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("first Delete failed: %v", err)
	}

	if store.Exists() {
		t.Fatal("expected file to be gone after Delete")
	}

	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("expected os.IsNotExist on deleted path, got: %v", err)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("second Delete should return nil (idempotent), got: %v", err)
	}
}

func TestProfileFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions not applicable on windows")
	}

	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	p := NewLocalProfile("Slartibartfast")
	if err := store.Save(p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("failed to stat profile: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected 0600 permissions, got %04o", perm)
	}
}

func TestProfileNoTempFilesLeft(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	p := NewLocalProfile("DeepThought")
	if err := store.Save(p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".profile-*.tmp"))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) > 0 {
		t.Fatalf("expected no temp files left, found: %v", matches)
	}
}

func TestProfileKeyStableAndDeviceScoped(t *testing.T) {
	k1, err := deriveProfileKey()
	if err != nil {
		t.Fatalf("first key derivation failed: %v", err)
	}
	k2, err := deriveProfileKey()
	if err != nil {
		t.Fatalf("second key derivation failed: %v", err)
	}

	if len(k1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(k1))
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("expected the device key to be deterministic across calls")
	}
	zero := make([]byte, 32)
	if bytes.Equal(k1, zero) {
		t.Fatal("key should not be all zero bytes")
	}

	alice := deriveProfileKeyFrom("alice", "1000", "machine-a")
	bob := deriveProfileKeyFrom("bob", "1001", "machine-a")
	otherMachine := deriveProfileKeyFrom("alice", "1000", "machine-b")
	if len(alice) != 32 || len(bob) != 32 || len(otherMachine) != 32 {
		t.Fatal("expected 32-byte scoped keys")
	}
	if bytes.Equal(alice, bob) {
		t.Fatal("different OS profiles must derive different keys")
	}
	if bytes.Equal(alice, otherMachine) {
		t.Fatal("different machines must derive different keys")
	}
}

func TestProfileLoadValidationCorrupt(t *testing.T) {
	dir := t.TempDir()
	store, err := NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create profile store: %v", err)
	}

	// Valid JSON with negative global score
	badJSON := []byte(`{"username":"InvalidUser","global_score":-10,"last_updated":1234,"solved":{}}`)
	encrypted, err := encryptProfile(badJSON)
	if err != nil {
		t.Fatalf("failed to encrypt payload: %v", err)
	}

	if err := os.WriteFile(store.Path(), encrypted, 0600); err != nil {
		t.Fatalf("failed to write encrypted file: %v", err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected error on Load with invalid profile fields")
	}
	if !errors.Is(err, ErrProfileCorrupt) {
		t.Fatalf("expected ErrProfileCorrupt, got: %v", err)
	}
	if errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected load validation error NOT to wrap ErrInvalidProfile, got: %v", err)
	}
}

func TestProfileStorePathAndString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "custom_profile.enc")
	store, err := NewProfileStore(path)
	if err != nil {
		t.Fatalf("failed to initialize store: %v", err)
	}

	if store.Path() != path {
		t.Fatalf("expected path %s, got %s", path, store.Path())
	}

	expectedStr := "profile store " + path
	if store.String() != expectedStr {
		t.Fatalf("expected string %q, got %q", expectedStr, store.String())
	}
}

func TestProfileMethodsAndNilSafety(t *testing.T) {
	var nilProfile *LocalProfile
	if err := nilProfile.Validate(); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected ErrInvalidProfile for nil profile Validate, got %v", err)
	}
	nilProfile.Touch() // should not panic
	nilProfile.SetSolved("001", 10)
	if score, ok := nilProfile.ScoreFor("001"); ok || score != 0 {
		t.Fatalf("expected 0, false for nil ScoreFor, got %d, %v", score, ok)
	}
	if count := nilProfile.SolvedCount(); count != 0 {
		t.Fatalf("expected 0 for nil SolvedCount, got %d", count)
	}

	p := NewLocalProfile("test")
	before := p.LastUpdated
	time.Sleep(10 * time.Millisecond)
	p.Touch()
	if p.LastUpdated < before {
		t.Fatalf("expected LastUpdated >= before, got %d < %d", p.LastUpdated, before)
	}

	p.SetSolved("001", 50)
	if score, ok := p.ScoreFor("001"); !ok || score != 50 {
		t.Fatalf("expected score 50, got %d, %v", score, ok)
	}
	if score, ok := p.ScoreFor("999"); ok || score != 0 {
		t.Fatalf("expected 0, false for missing problem, got %d, %v", score, ok)
	}
	if count := p.SolvedCount(); count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}
