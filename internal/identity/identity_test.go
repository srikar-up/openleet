package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuestIdentityIsStableAcrossLaunches(t *testing.T) {
	root := t.TempDir()
	store, id, guest, migrated, err := Open(root, "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if migrated {
		t.Fatal("fresh root must not report a migration")
	}
	if guest.OwnerUID != "" {
		t.Fatalf("guest owner = %q, want empty", guest.OwnerUID)
	}
	if guest.LocalID != id.GuestID {
		t.Fatalf("guest id = %q, identity guest = %q", guest.LocalID, id.GuestID)
	}
	if !strings.HasPrefix(guest.LocalID, "u_") {
		t.Fatalf("guest id is not opaque: %q", guest.LocalID)
	}
	userDir := filepath.Join(root, UsersDirName, guest.LocalID)
	if info, err := os.Stat(userDir); err != nil || !info.IsDir() {
		t.Fatalf("guest namespace missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, FileName)); err != nil {
		t.Fatalf("identity.enc missing: %v", err)
	}

	_, next, nextGuest, migrated, err := Open(root, "")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if migrated {
		t.Fatal("second launch must not migrate")
	}
	if nextGuest.LocalID != guest.LocalID || next.GuestID != id.GuestID {
		t.Fatalf("guest identity changed: %q -> %q", guest.LocalID, nextGuest.LocalID)
	}
	if store.Path() != filepath.Join(root, FileName) {
		t.Fatalf("store path = %q", store.Path())
	}
}

func TestGuestClaimRotatesGuestAndKeepsOriginalProfile(t *testing.T) {
	root := t.TempDir()
	store, id, guest, _, err := Open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	next, claimed, err := store.ClaimGuest("uid-a")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.LocalID != guest.LocalID || claimed.OwnerUID != "uid-a" {
		t.Fatalf("claimed = %#v, want %#v", claimed, guest)
	}
	if next.GuestID == guest.LocalID {
		t.Fatal("claim did not rotate the guest profile")
	}
	if next.ByUID["uid-a"] != guest.LocalID {
		t.Fatalf("mapping = %#v", next.ByUID)
	}
	if resolved, ok := next.Resolve("uid-a"); !ok || resolved.LocalID != guest.LocalID {
		t.Fatalf("resolve = %#v, ok=%v", resolved, ok)
	}
	again, againProfile, err := store.ClaimGuest("uid-a")
	if err != nil {
		t.Fatalf("re-claim existing account: %v", err)
	}
	if againProfile.LocalID != guest.LocalID || again.GuestID != next.GuestID {
		t.Fatal("re-claiming an existing account rotated the guest again")
	}
	_ = id
}

func TestSecondAccountGetsADistinctProfile(t *testing.T) {
	root := t.TempDir()
	store, _, guest, _, err := Open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	next, a, err := store.ClaimGuest("uid-a")
	if err != nil {
		t.Fatal(err)
	}
	next, b, err := store.CreateProfile("uid-b")
	if err != nil {
		t.Fatal(err)
	}
	if b.LocalID == a.LocalID || b.LocalID == guest.LocalID {
		t.Fatalf("second account reused a namespace: B=%q A=%q guest=%q", b.LocalID, a.LocalID, guest.LocalID)
	}
	if next.ByUID["uid-a"] != a.LocalID || next.ByUID["uid-b"] != b.LocalID {
		t.Fatalf("mapping = %#v", next.ByUID)
	}
	if got, ok := next.Resolve("uid-a"); !ok || got.OwnerUID != "uid-a" {
		t.Fatalf("A resolve = %#v, ok=%v", got, ok)
	}
}

func TestCorruptIdentityIsReportedAndNotOverwritten(t *testing.T) {
	root := t.TempDir()
	if _, _, _, _, err := Open(root, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, FileName)
	corrupt := []byte("OLSK\x01not-a-valid-vault")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := Open(root, ""); err == nil {
		t.Fatal("corrupt identity should fail to open")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(corrupt) {
		t.Fatal("corrupt identity file was silently replaced")
	}
}

func TestLegacyMigrationCopiesBindsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	legacy := []byte(`{"solved_problems":[{"problem_id":"001","language":"python","optimal_solution":true}]}`)
	if err := os.WriteFile(filepath.Join(root, "user_state.json"), legacy, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workspace_layout.json"), []byte(`{"description_width":40}`), 0644); err != nil {
		t.Fatal(err)
	}

	store, id, profile, migrated, err := Open(root, "uid-legacy")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected one-time legacy migration")
	}
	if profile.OwnerUID != "uid-legacy" {
		t.Fatalf("migrated owner = %q", profile.OwnerUID)
	}
	if id.ByUID["uid-legacy"] != profile.LocalID {
		t.Fatalf("mapping = %#v", id.ByUID)
	}
	userDir := filepath.Join(root, UsersDirName, profile.LocalID)
	got, err := os.ReadFile(filepath.Join(userDir, "user_state.json"))
	if err != nil {
		t.Fatalf("migrated state: %v", err)
	}
	if string(got) != string(legacy) {
		t.Fatalf("migrated bytes = %q", got)
	}
	if _, err := os.Stat(filepath.Join(userDir, "workspace_layout.json")); err != nil {
		t.Fatalf("layout not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "user_state.json")); err != nil {
		t.Fatalf("legacy files must be kept: %v", err)
	}

	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "uid-legacy") || strings.Contains(string(raw), "solved_problems") {
		t.Fatal("identity vault leaked plaintext identity or progress")
	}

	_, next, nextProfile, migratedAgain, err := Open(root, "uid-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if migratedAgain {
		t.Fatal("legacy migration ran twice")
	}
	if nextProfile.LocalID != profile.LocalID || next.ByUID["uid-legacy"] != profile.LocalID {
		t.Fatalf("second launch changed the migrated profile: %#v", nextProfile)
	}
}

func TestLegacyMigrationWithoutSessionBecomesGuest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "user_state.json"), []byte(`{"total_points":7}`), 0644); err != nil {
		t.Fatal(err)
	}
	_, id, profile, migrated, err := Open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Fatal("expected migration")
	}
	if profile.OwnerUID != "" || id.GuestID != profile.LocalID {
		t.Fatalf("legacy profile should be the guest: %#v", profile)
	}
}
