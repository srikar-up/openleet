package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/config"
	"openleet/internal/network"
	"openleet/internal/storage"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/toast"
)

// setPersistentEnv pins both config and data paths for a test install.
func setPersistentEnv(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("XDG_DATA_HOME", dir)
	}
}

// newPersistentAppAt constructs a fresh AppModel against a fixed config and
// data directory so two launches share the same encrypted session vault.
func newPersistentAppAt(t *testing.T, dir string) AppModel {
	t.Helper()
	setPersistentEnv(t, dir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	mgr, err := storage.NewManager(cfg.DataDir())
	if err != nil {
		t.Fatalf("storage manager: %v", err)
	}
	app := NewAppModel(cfg, mgr)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
	return m.(AppModel)
}

func testStoredSession(uid string) storedAuthSession {
	return storedAuthSession{
		RefreshToken: "refresh-token-" + uid,
		UID:          uid,
		Email:        "user@example.com",
		DisplayName:  "neo",
		PublicName:   "neo",
		Plan:         "Free",
		SignedInAt:   time.Unix(1700000000, 0).UTC(),
	}
}

// collectBatchMsgs runs a command tree and returns every produced message. It
// is only used with commands that do not schedule timers.
func collectBatchMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, child := range batch {
			out = append(out, collectBatchMsgs(child)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func findSessionPersistMsg(msgs []tea.Msg) (sessionPersistResultMsg, bool) {
	for _, msg := range msgs {
		if persist, ok := msg.(sessionPersistResultMsg); ok {
			return persist, true
		}
	}
	return sessionPersistResultMsg{}, false
}

func TestAuthSessionStoreEncryptsRoundTripsAndClears(t *testing.T) {
	path := filepath.Join(t.TempDir(), authSessionFileName)
	store, err := newAuthSessionStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	want := testStoredSession("uid-store")
	if err := store.Save(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vault: %v", err)
	}
	if strings.Contains(string(raw), want.RefreshToken) || strings.Contains(string(raw), want.Email) {
		t.Fatal("session vault contains plaintext identity data")
	}

	reopened, err := newAuthSessionStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	got, err := reopened.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Fatalf("loaded session = %#v, want %#v", got, want)
	}

	if err := reopened.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := reopened.Load(); !errors.Is(err, ErrNoStoredSession) {
		t.Fatalf("load after clear = %v, want ErrNoStoredSession", err)
	}
}

func TestAuthSessionStoreRejectsSessionWithoutRefreshToken(t *testing.T) {
	store, err := newAuthSessionStore(filepath.Join(t.TempDir(), authSessionFileName))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save(storedAuthSession{UID: "uid"}); err == nil {
		t.Fatal("expected a session without a refresh token to be rejected")
	}
}

func TestSignInPersistsSessionAndLogoutClearsIt(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	app, _ := newPersistentRoutingTestApp(t)
	if app.sessionStore == nil {
		t.Fatal("persistent app should have a session store")
	}
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts:signInWithPassword" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"localId":"uid-persist",
			"idToken":"id-token-secret",
			"refreshToken":"refresh-token-secret",
			"displayName":"neo"
		}`))
	}))
	t.Cleanup(server.Close)
	app.authBaseURL = server.URL

	m, authCmd := app.Update(settings.AuthRequestMsg{
		Mode:     settings.AuthSignIn,
		Email:    "user@example.com",
		Password: "secret123",
	})
	app = m.(AppModel)
	if authCmd == nil {
		t.Fatal("sign-in returned no command")
	}
	authResult, ok := authCmd().(accountAuthResultMsg)
	if !ok || authResult.err != nil {
		t.Fatalf("sign-in result = %#v", authResult)
	}
	m, follow := app.Update(authResult)
	app = m.(AppModel)
	if app.account == nil || app.account.RefreshToken != "refresh-token-secret" {
		t.Fatalf("session = %#v", app.account)
	}

	msgs := collectBatchMsgs(follow)
	persist, ok := findSessionPersistMsg(msgs)
	if !ok || persist.action != "save" || persist.err != nil {
		t.Fatalf("persist result = %#v (found=%v)", persist, ok)
	}
	stored, err := app.sessionStore.Load()
	if err != nil {
		t.Fatalf("load stored session: %v", err)
	}
	if stored.RefreshToken != "refresh-token-secret" || stored.Email != "user@example.com" {
		t.Fatalf("stored session = %#v", stored)
	}

	configJSON, err := json.Marshal(app.config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if strings.Contains(string(configJSON), "refresh-token-secret") || strings.Contains(string(configJSON), "id-token-secret") {
		t.Fatal("credentials leaked into config.json")
	}

	m, logoutCmd := app.Update(settings.ActionMsg{Action: "account_logout"})
	app = m.(AppModel)
	if app.account != nil {
		t.Fatal("logout did not clear the in-memory account")
	}
	clearMsg, ok := findSessionPersistMsg(collectBatchMsgs(logoutCmd))
	if !ok || clearMsg.action != "clear" || clearMsg.err != nil {
		t.Fatalf("clear result = %#v (found=%v)", clearMsg, ok)
	}
	if _, err := app.sessionStore.Load(); !errors.Is(err, ErrNoStoredSession) {
		t.Fatalf("stored session after logout = %v, want ErrNoStoredSession", err)
	}
}

func TestStartupRestoresRememberedSession(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	app, _ := newPersistentRoutingTestApp(t)
	if app.sessionStore == nil {
		t.Fatal("persistent app should have a session store")
	}
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")

	want := testStoredSession("uid-remembered")
	if err := app.sessionStore.Save(want); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	var sawRefreshForm bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("refresh method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse refresh form: %v", err)
		}
		if r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != want.RefreshToken {
			t.Errorf("refresh form = %#v", r.PostForm)
		}
		sawRefreshForm = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"access-token",
			"expires_in":"3600",
			"token_type":"Bearer",
			"refresh_token":"rotated-refresh-token",
			"id_token":"fresh-id-token",
			"user_id":"uid-remembered"
		}`))
	}))
	t.Cleanup(server.Close)
	app.secureTokenURL = server.URL

	loadCmd := app.restoreSessionCmd()
	if loadCmd == nil {
		t.Fatal("restoreSessionCmd returned nil")
	}
	loaded, ok := loadCmd().(storedSessionLoadedMsg)
	if !ok || loaded.err != nil {
		t.Fatalf("loaded session = %#v", loaded)
	}
	m, refreshCmd := app.Update(loaded)
	app = m.(AppModel)
	if app.account == nil || app.account.UID != want.UID {
		t.Fatalf("cached account = %#v", app.account)
	}
	if app.account.IDToken != "" {
		t.Fatal("cached restore must not invent an ID token")
	}
	if !strings.Contains(app.View(), want.Email) {
		t.Fatal("remembered identity is not painted before the network refresh")
	}
	if refreshCmd == nil {
		t.Fatal("restore did not schedule a token refresh")
	}
	refreshMsg, ok := refreshCmd().(sessionRefreshResultMsg)
	if !ok || refreshMsg.err != nil || refreshMsg.session == nil {
		t.Fatalf("refresh result = %#v", refreshMsg)
	}
	if !sawRefreshForm {
		t.Fatal("refresh endpoint was not called")
	}

	m, follow := app.Update(refreshMsg)
	app = m.(AppModel)
	if app.account == nil || app.account.IDToken != "fresh-id-token" || app.account.RefreshToken != "rotated-refresh-token" {
		t.Fatalf("restored account = %#v", app.account)
	}
	if !strings.Contains(app.View(), want.Email) {
		t.Fatal("restored account is not rendered")
	}

	persist, ok := findSessionPersistMsg(collectBatchMsgs(follow))
	if !ok || persist.err != nil {
		t.Fatalf("refresh did not persist the rotated token: %#v (found=%v)", persist, ok)
	}
	stored, err := app.sessionStore.Load()
	if err != nil {
		t.Fatalf("load rotated session: %v", err)
	}
	if stored.RefreshToken != "rotated-refresh-token" || stored.UID != want.UID {
		t.Fatalf("rotated stored session = %#v", stored)
	}
}

func TestStartupKeepsRememberedIdentityWhenRefreshIsOffline(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	app, _ := newPersistentRoutingTestApp(t)
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")

	want := testStoredSession("uid-offline")
	if err := app.sessionStore.Save(want); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	app.secureTokenURL = "http://127.0.0.1:1"

	loaded := app.restoreSessionCmd()().(storedSessionLoadedMsg)
	m, refreshCmd := app.Update(loaded)
	app = m.(AppModel)
	refreshMsg := refreshCmd().(sessionRefreshResultMsg)
	if refreshMsg.err == nil {
		t.Fatal("expected the unreachable refresh endpoint to fail")
	}
	m, _ = app.Update(refreshMsg)
	app = m.(AppModel)

	if app.account == nil || app.account.UID != want.UID {
		t.Fatal("offline startup discarded the remembered identity")
	}
	if !strings.Contains(app.View(), want.Email) {
		t.Fatal("offline startup did not render the remembered identity")
	}
	if note := app.accountInfo().SyncNote; !strings.Contains(note, "refresh pending") {
		t.Fatalf("sync note = %q, want a refresh-pending hint", note)
	}
	if app.account.RefreshToken != want.RefreshToken {
		t.Fatal("offline refresh discarded the saved refresh token")
	}
	if _, err := os.Stat(app.sessionStore.vault.Path()); err != nil {
		t.Fatalf("offline refresh removed the session vault: %v", err)
	}
}

func TestStartupClearsRejectedRememberedSession(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	app, _ := newPersistentRoutingTestApp(t)
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")

	if err := app.sessionStore.Save(testStoredSession("uid-revoked")); err != nil {
		t.Fatalf("seed session: %v", err)
	}
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
		t.Fatalf("refresh error = %v, want ErrSessionExpired", refreshMsg.err)
	}
	m, clearCmd := app.Update(refreshMsg)
	app = m.(AppModel)
	if app.account != nil {
		t.Fatal("revoked session should clear the in-memory account")
	}
	if !strings.Contains(app.View(), "Signed out") {
		t.Fatal("revoked session should return the account page to signed out")
	}
	if clearMsg, ok := findSessionPersistMsg(collectBatchMsgs(clearCmd)); !ok || clearMsg.action != "clear" || clearMsg.err != nil {
		t.Fatalf("clear result = %#v (found=%v)", clearMsg, ok)
	}
	if _, err := app.sessionStore.Load(); !errors.Is(err, ErrNoStoredSession) {
		t.Fatalf("stored session after rejection = %v, want ErrNoStoredSession", err)
	}
}

func TestLoginSurvivesNewAppModelLaunch(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	dir := t.TempDir()
	app := newPersistentAppAt(t, dir)
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts:signInWithPassword" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"localId":"uid-new-launch",
			"idToken":"id-token-secret",
			"refreshToken":"refresh-token-new-launch",
			"displayName":"neo"
		}`))
	}))
	t.Cleanup(server.Close)
	app.authBaseURL = server.URL

	m, authCmd := app.Update(settings.AuthRequestMsg{
		Mode:     settings.AuthSignIn,
		Email:    "user@example.com",
		Password: "secret123",
	})
	app = m.(AppModel)
	result := authCmd().(accountAuthResultMsg)
	if result.err != nil {
		t.Fatalf("sign-in: %v", result.err)
	}
	m, follow := app.Update(result)
	app = m.(AppModel)
	collectBatchMsgs(follow)

	vaultPath := app.sessionStore.vault.Path()
	if _, err := os.Stat(vaultPath); err != nil {
		t.Fatalf("sign-in did not write session vault: %v", err)
	}
	// A normal quit must not remove the remembered session.
	_, quitCmd := app.requestQuit()
	if quitCmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg for a normal quit")
	}
	if _, err := os.Stat(vaultPath); err != nil {
		t.Fatalf("normal quit removed the session vault: %v", err)
	}

	// Simulate the next process launch against the same directories.
	next := newPersistentAppAt(t, dir)
	next.StartAt(ViewSettings)
	next.OpenSettingsSection(string(settings.SectionAccount))
	loadCmd := next.restoreSessionCmd()
	if loadCmd == nil {
		t.Fatal("new AppModel has no restore command")
	}
	loaded, ok := loadCmd().(storedSessionLoadedMsg)
	if !ok || loaded.err != nil {
		t.Fatalf("restore on new launch = %#v", loaded)
	}
	m, refreshCmd := next.Update(loaded)
	restored := m.(AppModel)
	if restored.account == nil {
		t.Fatal("new launch did not restore the remembered identity")
	}
	if restored.account.UID != "uid-new-launch" || restored.account.Email != "user@example.com" {
		t.Fatalf("restored identity = %#v", restored.account)
	}
	if restored.account.IDToken != "" {
		t.Fatal("remembered identity restored an ID token from disk")
	}
	if refreshCmd == nil {
		t.Fatal("restored identity did not schedule a background token refresh")
	}
	if !strings.Contains(restored.View(), "user@example.com") {
		t.Fatal("restored identity is not visible on the new launch")
	}

	raw, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("read vault: %v", err)
	}
	if strings.Contains(string(raw), "id-token-secret") {
		t.Fatal("ID token was written to the session vault")
	}
}

func TestTransientVaultErrorKeepsSessionFile(t *testing.T) {
	dir := t.TempDir()
	app := newPersistentAppAt(t, dir)
	if err := app.sessionStore.Save(testStoredSession("uid-transient")); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	vaultPath := app.sessionStore.vault.Path()
	if err := os.WriteFile(vaultPath, []byte("OLSK\x01truncated"), 0600); err != nil {
		t.Fatalf("corrupt vault: %v", err)
	}
	_, loadErr := app.sessionStore.Load()
	if loadErr == nil {
		t.Fatal("expected the corrupted vault to fail to load")
	}

	m, cmd := app.Update(storedSessionLoadedMsg{err: loadErr, attempt: 1})
	next := m.(AppModel)
	if next.account != nil {
		t.Fatal("a failed vault read must not invent an account")
	}
	if _, err := os.Stat(vaultPath); err != nil {
		t.Fatalf("transient vault error deleted the session file: %v", err)
	}
	if !hasVisibleToast(next, toast.Warning, "Remembered session unavailable") {
		t.Fatalf("missing vault-error warning: %#v", visibleToasts(next))
	}
	// Running the returned command must not include a destructive clear.
	for _, msg := range collectBatchMsgs(cmd) {
		if clear, ok := msg.(sessionPersistResultMsg); ok && clear.action == "clear" {
			t.Fatal("transient vault error scheduled a session clear")
		}
	}
	if _, err := os.Stat(vaultPath); err != nil {
		t.Fatalf("session file disappeared after the error command ran: %v", err)
	}
}

func TestAuthSessionStoreRepairsCorruptVaultOnExplicitSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), authSessionFileName)
	store, err := newAuthSessionStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := os.WriteFile(path, []byte("OLSK\x01truncated"), 0600); err != nil {
		t.Fatalf("corrupt vault: %v", err)
	}
	want := testStoredSession("uid-repaired")
	if err := store.Save(want); err != nil {
		t.Fatalf("explicit sign-in could not replace a corrupt vault: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load repaired store: %v", err)
	}
	if got.UID != want.UID || got.RefreshToken != want.RefreshToken {
		t.Fatalf("repaired session = %#v", got)
	}
}
