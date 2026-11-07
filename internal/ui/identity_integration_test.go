package ui

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openleet/internal/config"
	"openleet/internal/model"
	"openleet/internal/network"
	"openleet/internal/storage"
	"openleet/internal/ui/settings"
)

func identityTestInstall(t *testing.T) (string, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	setPersistentEnv(t, dir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return dir, cfg
}

func loginTestAccount(t *testing.T, app AppModel, uid, email string) AppModel {
	t.Helper()
	m, cmd := app.Update(accountAuthResultMsg{session: &accountSession{
		UID:          uid,
		Email:        email,
		DisplayName:  "neo",
		PublicName:   "neo",
		IDToken:      "id-token-secret",
		RefreshToken: "refresh-token-" + uid,
		Plan:         "Free",
		SignedInAt:   time.Now(),
	}})
	app = m.(AppModel)
	collectBatchMsgs(cmd)
	return app
}

func saveTestProgress(t *testing.T, app AppModel, marker, buffer string) {
	t.Helper()
	state := model.NewUserState()
	state.MarkSolved("001", "python", buffer)
	state.SetCodeBuffer("001", "python", buffer)
	if err := app.storage.SaveUserState(state); err != nil {
		t.Fatalf("save %s state: %v", marker, err)
	}
}

func TestFreshIdentityGuestSurvivesRelaunch(t *testing.T) {
	dir := t.TempDir()
	app := newPersistentAppAt(t, dir)
	if app.identity == nil || app.identity.Err != nil {
		t.Fatalf("identity = %#v", app.identity)
	}
	if app.identity.Profile.OwnerUID != "" {
		t.Fatalf("fresh profile owner = %q", app.identity.Profile.OwnerUID)
	}
	guestID := app.localID
	saveTestProgress(t, app, "guest", "guest_buffer")
	if _, err := os.Stat(app.userDataDir()); err != nil {
		t.Fatalf("guest namespace missing: %v", err)
	}

	_, quit := app.requestQuit()
	if quit == nil {
		t.Fatal("expected normal quit command")
	}
	if app.sessionStore != nil {
		if _, err := os.Stat(app.sessionStore.vault.Path()); !os.IsNotExist(err) {
			t.Fatalf("guest launch should not write session.enc: %v", err)
		}
	}

	next := newPersistentAppAt(t, dir)
	if next.localID != guestID {
		t.Fatalf("guest id changed: %q -> %q", guestID, next.localID)
	}
	state, err := next.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if !state.HasSolved("001") || state.GetCodeBuffer("001", "python") != "guest_buffer" {
		t.Fatalf("guest progress did not survive: %#v", state)
	}
}

func TestGuestLoginClaimsProfileAndRelaunchRestores(t *testing.T) {
	dir := t.TempDir()
	app := newPersistentAppAt(t, dir)
	guestID := app.localID
	saveTestProgress(t, app, "guest", "offline_buffer")

	app = loginTestAccount(t, app, "uid-a", "a@example.com")
	if app.localID != guestID {
		t.Fatalf("guest claim changed namespace: %q -> %q", guestID, app.localID)
	}
	if app.identity.Profile.OwnerUID != "uid-a" {
		t.Fatalf("profile owner = %q", app.identity.Profile.OwnerUID)
	}
	if app.identity.Data.ByUID["uid-a"] != guestID {
		t.Fatalf("mapping = %#v", app.identity.Data.ByUID)
	}
	if app.identity.Data.GuestID == guestID {
		t.Fatal("claim did not rotate in a fresh guest profile")
	}
	state, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if !state.HasSolved("001") || state.GetCodeBuffer("001", "python") != "offline_buffer" {
		t.Fatal("guest progress was not preserved on claim")
	}
	if stored, err := app.sessionStore.Load(); err != nil || stored.UID != "uid-a" {
		t.Fatalf("session = %#v, err=%v", stored, err)
	}

	next := newPersistentAppAt(t, dir)
	if next.localID != guestID {
		t.Fatalf("relaunch namespace = %q, want %q", next.localID, guestID)
	}
	if next.identity.Profile.OwnerUID != "uid-a" {
		t.Fatalf("relaunch owner = %q", next.identity.Profile.OwnerUID)
	}
	loaded, err := next.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasSolved("001") {
		t.Fatal("A progress did not restore on relaunch")
	}
}

func TestSecondAccountGetsFreshNamespaceAndLogoutReturnsGuest(t *testing.T) {
	dir := t.TempDir()
	app := newPersistentAppAt(t, dir)
	app = loginTestAccount(t, app, "uid-a", "a@example.com")
	aID := app.localID
	saveTestProgress(t, app, "A", "a_buffer")
	aProfile := storage.NewLocalProfile("neo-a")
	aProfile.SetSolved("001", 100)
	if err := app.profileStore.Save(aProfile); err != nil {
		t.Fatal(err)
	}
	if err := app.syncMetaStore.Save(&storage.SyncMeta{UID: "uid-a", LastSynced: 100, Note: "A sync"}); err != nil {
		t.Fatal(err)
	}

	app = loginTestAccount(t, app, "uid-b", "b@example.com")
	bID := app.localID
	if bID == aID {
		t.Fatal("account B reused account A's namespace")
	}
	if app.identity.Data.ByUID["uid-a"] != aID || app.identity.Data.ByUID["uid-b"] != bID {
		t.Fatalf("mapping = %#v", app.identity.Data.ByUID)
	}
	bState, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if len(bState.SolvedProblems) != 0 || len(bState.CodeBuffers) != 0 {
		t.Fatal("account B inherited A's progress")
	}
	if _, err := app.profileStore.Load(); !errors.Is(err, storage.ErrProfileNotFound) {
		t.Fatalf("account B inherited A's profile: %v", err)
	}
	saveTestProgress(t, app, "B", "b_buffer")
	bProfile := storage.NewLocalProfile("neo-b")
	bProfile.SetSolved("001", 25)
	if err := app.profileStore.Save(bProfile); err != nil {
		t.Fatal(err)
	}
	if err := app.syncMetaStore.Save(&storage.SyncMeta{UID: "uid-b", LastSynced: 200, Note: "B sync"}); err != nil {
		t.Fatal(err)
	}

	m, logoutCmd := app.Update(settings.ActionMsg{Action: "account_logout"})
	app = m.(AppModel)
	collectBatchMsgs(logoutCmd)
	if app.account != nil {
		t.Fatal("logout kept the in-memory account")
	}
	if app.localID == aID || app.localID == bID {
		t.Fatalf("logout entered an owned namespace %q", app.localID)
	}
	if app.identity.Profile.OwnerUID != "" {
		t.Fatalf("logout guest is owned by %q", app.identity.Profile.OwnerUID)
	}
	if _, err := app.sessionStore.Load(); !errors.Is(err, ErrNoStoredSession) {
		t.Fatalf("logout session = %v", err)
	}
	guestState, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if len(guestState.SolvedProblems) != 0 {
		t.Fatal("logout guest can read account progress")
	}

	aStore, err := storage.ForUser(app.identity.Root, aID)
	if err != nil {
		t.Fatal(err)
	}
	aState, err := aStore.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if !aState.HasSolved("001") || aState.GetCodeBuffer("001", "python") != "a_buffer" {
		t.Fatal("account A's data was changed by B or logout")
	}
	aStoreProfile, _ := storage.NewProfileStoreIn(storage.UserDir(app.identity.Root, aID))
	aSaved, err := aStoreProfile.Load()
	if err != nil {
		t.Fatal(err)
	}
	if score, _ := aSaved.ScoreFor("001"); score != 100 {
		t.Fatalf("A profile score = %d", score)
	}
	aStoreSync, _ := storage.NewSyncMetaStoreIn(storage.UserDir(app.identity.Root, aID))
	aMeta, err := aStoreSync.Load()
	if err != nil {
		t.Fatal(err)
	}
	if aMeta.UID != "uid-a" || aMeta.Note != "A sync" {
		t.Fatalf("A sync meta = %#v", aMeta)
	}

	app = loginTestAccount(t, app, "uid-a", "a@example.com")
	if app.localID != aID {
		t.Fatalf("A re-login namespace = %q, want %q", app.localID, aID)
	}
	restored, err := app.profileStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if score, _ := restored.ScoreFor("001"); score != 100 {
		t.Fatalf("A profile did not restore: %#v", restored)
	}
	restoredState, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if restoredState.GetCodeBuffer("001", "python") != "a_buffer" {
		t.Fatal("A code buffer did not restore")
	}
}

func TestLegacyInstallMigratesOnceAndBindsRememberedUID(t *testing.T) {
	dir, cfg := identityTestInstall(t)
	legacy := model.NewUserState()
	legacy.MarkSolved("001", "python", "legacy_buffer")
	legacy.SetCodeBuffer("001", "python", "legacy_buffer")
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir(), "user_state.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	sessionStore, err := newAuthSessionStore(filepath.Join(cfg.ConfigDir(), authSessionFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := sessionStore.Save(testStoredSession("uid-legacy")); err != nil {
		t.Fatal(err)
	}

	app := newPersistentAppAt(t, dir)
	if !app.identity.Migrated {
		t.Fatal("legacy install did not migrate")
	}
	if app.identity.Profile.OwnerUID != "uid-legacy" {
		t.Fatalf("migrated owner = %q", app.identity.Profile.OwnerUID)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir(), "user_state.json")); err != nil {
		t.Fatalf("legacy source was deleted: %v", err)
	}
	migrated, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if !migrated.HasSolved("001") || migrated.GetCodeBuffer("001", "python") != "legacy_buffer" {
		t.Fatal("legacy progress was not copied")
	}
	profileCount := len(app.identity.Data.Profiles)

	next := newPersistentAppAt(t, dir)
	if next.identity.Migrated {
		t.Fatal("second launch migrated again")
	}
	if next.localID != app.localID {
		t.Fatalf("second launch namespace = %q, want %q", next.localID, app.localID)
	}
	if len(next.identity.Data.Profiles) != profileCount {
		t.Fatalf("profile count grew from %d to %d", profileCount, len(next.identity.Data.Profiles))
	}
}

func TestCorruptIdentityUsesRecoveryWithoutDeletingFile(t *testing.T) {
	dir := t.TempDir()
	app := newPersistentAppAt(t, dir)
	identityPath := app.identity.Store.Path()
	corrupt := []byte("OLSK\x01corrupt-identity")
	if err := os.WriteFile(identityPath, corrupt, 0600); err != nil {
		t.Fatal(err)
	}

	next := newPersistentAppAt(t, dir)
	if next.identity == nil || next.identity.Err == nil {
		t.Fatal("corrupt identity was not reported")
	}
	if !strings.HasPrefix(next.localID, "u_recovery_") {
		t.Fatalf("recovery namespace = %q", next.localID)
	}
	raw, err := os.ReadFile(identityPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(corrupt) {
		t.Fatal("corrupt identity.enc was replaced")
	}
	if strings.Contains(next.userDataDir(), app.localID) {
		t.Fatal("recovery namespace can read the previous profile")
	}
}

func TestGenuineSessionExpiryClearsSessionAndKeepsProfile(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	dir, cfg := identityTestInstall(t)
	sessionStore, err := newAuthSessionStore(filepath.Join(cfg.ConfigDir(), authSessionFileName))
	if err != nil {
		t.Fatal(err)
	}
	want := testStoredSession("uid-expired")
	if err := sessionStore.Save(want); err != nil {
		t.Fatal(err)
	}
	app := newPersistentAppAt(t, dir)
	if app.localID == "" || app.identity.Profile.OwnerUID != "uid-expired" {
		t.Fatalf("startup identity = %#v", app.identity.Profile)
	}
	accountID := app.localID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"INVALID_REFRESH_TOKEN","status":"INVALID_ARGUMENT"}}`))
	}))
	t.Cleanup(server.Close)
	app.secureTokenURL = server.URL

	loaded := app.restoreSessionCmd()().(storedSessionLoadedMsg)
	m, refreshCmd := app.Update(loaded)
	app = m.(AppModel)
	refreshMsg := refreshCmd().(sessionRefreshResultMsg)
	if !errors.Is(refreshMsg.err, network.ErrSessionExpired) {
		t.Fatalf("refresh error = %v", refreshMsg.err)
	}
	m, clearCmd := app.Update(refreshMsg)
	app = m.(AppModel)
	collectBatchMsgs(clearCmd)
	if app.account != nil {
		t.Fatal("expired session kept the account")
	}
	if _, err := app.sessionStore.Load(); !errors.Is(err, ErrNoStoredSession) {
		t.Fatalf("expired session file = %v", err)
	}
	if app.localID == accountID {
		t.Fatal("expired session did not enter the guest namespace")
	}
	if _, err := os.Stat(storage.UserDir(cfg.DataDir(), accountID)); err != nil {
		t.Fatalf("account namespace was deleted: %v", err)
	}
}

func TestNoIDTokenAnywhereUnderDataDir(t *testing.T) {
	dir, _ := identityTestInstall(t)
	app := newPersistentAppAt(t, dir)
	app = loginTestAccount(t, app, "uid-a", "a@example.com")
	app = loginTestAccount(t, app, "uid-b", "b@example.com")

	err := filepath.WalkDir(app.identity.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(raw), "id-token-secret") {
			t.Fatalf("ID token leaked into %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
