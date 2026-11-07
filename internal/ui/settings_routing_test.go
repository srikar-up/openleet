package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/config"
	"openleet/internal/model"
	"openleet/internal/storage"
	"openleet/internal/ui/home"
	"openleet/internal/ui/settings"
)

func newRoutingTestApp(t *testing.T) AppModel {
	t.Helper()
	dir := t.TempDir()
	mgr, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("storage manager: %v", err)
	}
	app := NewAppModel(config.DefaultConfig(), mgr)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
	return m.(AppModel)
}

// newPersistentRoutingTestApp gives the app real config and data directories so
// Settings data actions can be exercised end to end.
func newPersistentRoutingTestApp(t *testing.T) (AppModel, string) {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("XDG_DATA_HOME", dir)
	}
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
	model := m.(AppModel)
	return model, model.userDataDir()
}

func runDataAction(t *testing.T, app AppModel, action string) (AppModel, dataActionResultMsg) {
	t.Helper()
	m, cmd := app.Update(settings.ActionMsg{Action: action})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatalf("%s returned no command", action)
	}
	msg, ok := cmd().(dataActionResultMsg)
	if !ok {
		t.Fatalf("%s produced %T, want dataActionResultMsg", action, cmd())
	}
	m, _ = app.Update(msg)
	return m.(AppModel), msg
}

func TestSettingsShortcutOpensAndEscRestoresView(t *testing.T) {
	app := newRoutingTestApp(t)

	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyF1}) // roadmap
	app = m.(AppModel)
	if app.currentView != ViewRoadmap {
		t.Fatalf("currentView = %v, want roadmap", app.currentView)
	}

	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyF5})
	app = m.(AppModel)
	if app.currentView != ViewSettings {
		t.Fatalf("F5 currentView = %v, want settings", app.currentView)
	}
	if app.settingsReturn != ViewRoadmap {
		t.Fatalf("settingsReturn = %v, want roadmap", app.settingsReturn)
	}
	if !strings.Contains(app.View(), "SETTINGS") {
		t.Fatal("settings frame is missing its title")
	}

	m, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatal("esc in settings should dispatch a close message")
	}
	m, _ = app.Update(cmd())
	app = m.(AppModel)
	if app.currentView != ViewRoadmap {
		t.Fatalf("currentView after close = %v, want roadmap", app.currentView)
	}
}

func TestPaletteDeepLinkOpensSettingsSection(t *testing.T) {
	app := newRoutingTestApp(t)

	m, _ := app.Update(home.PaletteSelectionMsg{Action: home.ActionSettings, Argument: "theme"})
	app = m.(AppModel)
	if app.currentView != ViewSettings {
		t.Fatalf("currentView = %v, want settings", app.currentView)
	}
	if got := app.settingsView.ActiveSection(); got != settings.SectionAppearance {
		t.Fatalf("active section = %q, want appearance", got)
	}
	if !strings.Contains(app.View(), "[ SETTINGS · APPEARANCE ]") {
		t.Fatal("deep link did not render the appearance canvas")
	}
}

func TestSettingsThemePreviewRepaintsWithoutSaving(t *testing.T) {
	app := newRoutingTestApp(t)
	original := app.config.Theme

	m, _ := app.Update(settings.PreviewThemeMsg{Name: "dracula"})
	app = m.(AppModel)
	if app.palette.Name != "dracula" {
		t.Fatalf("palette = %q, want dracula", app.palette.Name)
	}
	if app.config.Theme != original {
		t.Fatalf("preview persisted theme %q, want %q", app.config.Theme, original)
	}

	m, _ = app.Update(settings.RevertThemeMsg{Name: original})
	app = m.(AppModel)
	if app.palette.Name != original {
		t.Fatalf("palette after revert = %q, want %q", app.palette.Name, original)
	}
}

func TestSettingsActionResetsSchema(t *testing.T) {
	app, _ := newPersistentRoutingTestApp(t)
	app.config.Settings["general.confirm_quit"] = "on"
	app.config.Theme = "dracula"

	app, result := runDataAction(t, app, "reset_settings")
	if result.err != nil {
		t.Fatalf("reset settings: %v", result.err)
	}
	if app.config.Settings["general.confirm_quit"] != "off" {
		t.Fatalf("confirm_quit = %q, want off", app.config.Settings["general.confirm_quit"])
	}
	if app.config.Theme != "openleet" {
		t.Fatalf("theme = %q, want openleet", app.config.Theme)
	}
	if got := app.settingsView.ActiveSection(); got != settings.SectionData {
		t.Fatalf("settings section after reset = %q, want data", got)
	}
}

func TestDataExportAndImportRoundTrip(t *testing.T) {
	app, dataDir := newPersistentRoutingTestApp(t)
	state := model.NewUserState()
	state.MarkSolved("1", "python", "print(1)")
	if err := app.storage.SaveUserState(state); err != nil {
		t.Fatal(err)
	}

	app, result := runDataAction(t, app, "data_export")
	if result.err != nil {
		t.Fatalf("export: %v", result.err)
	}
	for _, name := range []string{"openleet-progress.json", "openleet-progress.csv"} {
		path := filepath.Join(dataDir, name)
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("%s not written: %v", path, err)
		}
	}

	if err := app.storage.SaveUserState(model.NewUserState()); err != nil {
		t.Fatal(err)
	}
	app, result = runDataAction(t, app, "data_import")
	if result.err != nil {
		t.Fatalf("import: %v", result.err)
	}
	loaded, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SolvedProblems) != 1 || loaded.SolvedProblems[0].ProblemID != "1" {
		t.Fatalf("imported state = %#v", loaded.SolvedProblems)
	}
	if app.userState == nil || len(app.userState.SolvedProblems) != 1 {
		t.Fatal("imported state was not adopted by the app")
	}
}

func TestDataResetRoadmapKeepsCodeBuffers(t *testing.T) {
	app, _ := newPersistentRoutingTestApp(t)
	state := model.NewUserState()
	state.MarkSolved("1", "python", "print(1)")
	state.SetCodeBuffer("1", "python", "draft")
	if err := app.storage.SaveUserState(state); err != nil {
		t.Fatal(err)
	}

	app, result := runDataAction(t, app, "data_reset_roadmap")
	if result.err != nil {
		t.Fatalf("reset roadmap: %v", result.err)
	}
	loaded, err := app.storage.LoadUserState()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SolvedProblems) != 0 || loaded.TotalPoints != 0 {
		t.Fatalf("progress not reset: %#v", loaded)
	}
	if got := loaded.GetCodeBuffer("1", "python"); got != "draft" {
		t.Fatalf("code buffer = %q, want draft", got)
	}
}

func TestDataDeleteLocalRemovesProgress(t *testing.T) {
	app, dataDir := newPersistentRoutingTestApp(t)
	state := model.NewUserState()
	state.MarkSolved("1", "python", "print(1)")
	if err := app.storage.SaveUserState(state); err != nil {
		t.Fatal(err)
	}

	app, result := runDataAction(t, app, "data_delete_local")
	if result.err != nil {
		t.Fatalf("delete local: %v", result.err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "user_state.json")); !os.IsNotExist(err) {
		t.Fatalf("user_state.json still present: %v", err)
	}
	if app.userState == nil || len(app.userState.SolvedProblems) != 0 {
		t.Fatal("app did not adopt an empty state")
	}
}

func TestDataBackupCreatesSnapshot(t *testing.T) {
	app, dataDir := newPersistentRoutingTestApp(t)
	state := model.NewUserState()
	state.MarkSolved("1", "python", "print(1)")
	if err := app.storage.SaveUserState(state); err != nil {
		t.Fatal(err)
	}

	app, result := runDataAction(t, app, "data_backup")
	if result.err != nil {
		t.Fatalf("backup: %v", result.err)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "backups"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("backup directory missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "backups", entries[0].Name(), "user_state.json")); err != nil {
		t.Fatalf("backup snapshot missing user state: %v", err)
	}
}

func newAuthTestServer(t *testing.T, calls *[]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "accounts:signUp"):
			*calls = append(*calls, "signUp")
			_, _ = w.Write([]byte(`{"localId":"uid-new","email":"new@example.com"}`))
		case strings.Contains(r.URL.Path, "accounts:signInWithPassword"):
			*calls = append(*calls, "signIn")
			_, _ = w.Write([]byte(`{"localId":"uid-new","idToken":"id-token-secret"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAccountSignInStoresSessionOnly(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	var calls []string
	server := newAuthTestServer(t, &calls)

	app := newRoutingTestApp(t)
	app.authBaseURL = server.URL
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")

	m, cmd := app.Update(settings.AuthRequestMsg{
		Mode:     settings.AuthSignIn,
		Email:    "user@example.com",
		Password: "secret123",
	})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatal("sign-in returned no command")
	}
	result, ok := cmd().(accountAuthResultMsg)
	if !ok {
		t.Fatalf("sign-in produced %T", result)
	}
	if result.err != nil {
		t.Fatalf("sign-in: %v", result.err)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if app.account == nil || app.account.UID != "uid-new" || app.account.IDToken != "id-token-secret" {
		t.Fatalf("session = %#v", app.account)
	}
	if app.account.PublicName != "Anonymous" {
		t.Fatalf("public name = %q, want Anonymous when Firebase has no display name", app.account.PublicName)
	}
	view := app.View()
	if !strings.Contains(view, "user@example.com") || !strings.Contains(view, "PLAN  Free") {
		t.Fatal("signed-in profile is not rendered")
	}

	raw, err := json.Marshal(app.config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "id-token-secret") || strings.Contains(string(raw), "secret123") {
		t.Fatal("credentials leaked into config")
	}

	m, cmd = app.Update(settings.ActionMsg{Action: "account_logout"})
	app = m.(AppModel)
	if cmd != nil {
		_, _ = app.Update(cmd())
	}
	if app.account != nil {
		t.Fatal("logout did not clear the session")
	}
	if !strings.Contains(app.View(), "Signed out") {
		t.Fatal("signed-out state is not rendered")
	}
}

func TestAccountSignUpThenAutoSignIn(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	var calls []string
	server := newAuthTestServer(t, &calls)

	app := newRoutingTestApp(t)
	app.authBaseURL = server.URL
	m, cmd := app.Update(settings.AuthRequestMsg{
		Mode:     settings.AuthSignUp,
		Email:    "new@example.com",
		Username: "neo",
		Password: "secret123",
	})
	app = m.(AppModel)
	result, ok := cmd().(accountAuthResultMsg)
	if !ok || result.err != nil {
		t.Fatalf("sign-up result = %#v", result)
	}
	if len(calls) != 2 || calls[0] != "signUp" || calls[1] != "signIn" {
		t.Fatalf("calls = %#v, want signUp then signIn", calls)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if app.account == nil || app.account.DisplayName != "neo" {
		t.Fatalf("session = %#v", app.account)
	}
}

func TestAccountSignInErrorKeepsFormOpen(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-key")
	app := newRoutingTestApp(t)
	app.authBaseURL = "http://127.0.0.1:1"
	app.StartAt(ViewSettings)
	app.OpenSettingsSection("account")
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open form
	app = m.(AppModel)

	m, cmd := app.Update(settings.AuthRequestMsg{Mode: settings.AuthSignIn, Email: "a@b.c", Password: "secret"})
	app = m.(AppModel)
	result := cmd().(accountAuthResultMsg)
	if result.err == nil {
		t.Fatal("expected the unreachable auth endpoint to fail")
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if app.account != nil {
		t.Fatal("failed auth must not create a session")
	}
	if !strings.Contains(app.View(), "SIGN IN") {
		t.Fatal("failed auth should keep the form visible")
	}
}
