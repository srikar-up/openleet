package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"openleet/internal/model"
	"openleet/internal/network"
	"openleet/internal/storage"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/toast"
)

func TestProfileFromStateCompressesSolvedSet(t *testing.T) {
	state := &model.UserState{
		TotalPoints:    25,
		LastSolvedDate: time.Unix(1700000000, 0),
		SolvedProblems: []model.SolvedProblem{
			{ProblemID: "1", OptimalSolution: true},
			{ProblemID: "2", OptimalSolution: false},
		},
		CodeBuffers: map[string]map[string]string{"1": {"python": "print(1)"}},
	}
	profile := profileFromState(state, "neo")
	if profile.Username != "neo" || profile.GlobalScore != 25 || profile.LastUpdated != 1700000000 {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.Solved["1"] != 15 || profile.Solved["2"] != 10 {
		t.Fatalf("solved = %#v", profile.Solved)
	}
}

func TestMergeRemoteProfileKeepsLocalDetails(t *testing.T) {
	state := &model.UserState{
		TotalPoints:    10,
		LastSolvedDate: time.Unix(100, 0),
		SolvedProblems: []model.SolvedProblem{
			{ProblemID: "1", Attempts: 4, Language: "go", LastCode: "draft"},
		},
		CodeBuffers: map[string]map[string]string{"1": {"go": "draft"}},
	}
	remote := &storage.LocalProfile{
		GlobalScore: 50,
		LastUpdated: 200,
		Solved:      map[string]int{"1": 99, "2": 20},
	}
	merged, added := mergeRemoteProfile(state, remote)
	if added != 1 || len(merged.SolvedProblems) != 2 {
		t.Fatalf("added = %d, solved = %#v", added, merged.SolvedProblems)
	}
	if merged.TotalPoints != 50 {
		t.Fatalf("score = %d, want 50", merged.TotalPoints)
	}
	local := merged.GetSolvedProblem("1")
	if local == nil || local.Attempts != 4 || local.Language != "go" || local.LastCode != "draft" {
		t.Fatalf("local record was overwritten: %#v", local)
	}
	if got := merged.GetCodeBuffer("1", "go"); got != "draft" {
		t.Fatalf("code buffer = %q", got)
	}
	if merged.SolvedProblems[1].ProblemID != "2" || !merged.SolvedProblems[1].OptimalSolution {
		t.Fatalf("remote problem not merged: %#v", merged.SolvedProblems[1])
	}
	if _, again := mergeRemoteProfile(merged, remote); again != 0 {
		t.Fatalf("second merge added %d duplicates", again)
	}
}

func newSyncTestApp(t *testing.T) (AppModel, *model.UserState) {
	t.Helper()
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	t.Setenv("FIREBASE_PROJECT_ID", "test-proj")
	app, _ := newPersistentRoutingTestApp(t)
	state := &model.UserState{
		TotalPoints:    10,
		LastSolvedDate: time.Unix(1, 0),
		SolvedProblems: []model.SolvedProblem{
			{ProblemID: "1", SolvedAt: time.Unix(1, 0), Attempts: 2},
		},
		CodeBuffers: map[string]map[string]string{"1": {"python": "print(1)"}},
	}
	app.userState = state
	app.account = &accountSession{
		UID: "uid-1", Email: "user@example.com", DisplayName: "neo",
		IDToken: "id-token-secret", Plan: "Free", SignedInAt: time.Now(),
	}
	app.settingsView.SetAccount(app.accountInfo())
	app.StartAt(ViewSettings)
	app.OpenSettingsSection(string(settings.SectionAccount))
	return app, state
}

func TestSyncNowPullsMergesAndPushes(t *testing.T) {
	app, _ := newSyncTestApp(t)
	var (
		mu     sync.Mutex
		pushes []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"fields":{
				"uid":{"stringValue":"uid-1"},
				"user":{"stringValue":"neo"},
				"score":{"integerValue":"100"},
				"updated":{"integerValue":"%d"},
				"solved":{"mapValue":{"fields":{"2":{"integerValue":"100"}}}}
			}}`, time.Now().Unix())))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			pushes = append(pushes, string(body))
			mu.Unlock()
			_, _ = w.Write([]byte(`{"fields":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	app.firestoreBaseURL = server.URL

	m, cmd := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatal("sync returned no command")
	}
	result, ok := cmd().(profileSyncResultMsg)
	if !ok {
		t.Fatalf("sync produced %T", result)
	}
	if result.err != nil {
		t.Fatalf("sync: %v", result.err)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if len(app.userState.SolvedProblems) != 2 {
		t.Fatalf("merged state = %#v", app.userState.SolvedProblems)
	}
	if app.userState.TotalPoints != 100 {
		t.Fatalf("points = %d, want 100", app.userState.TotalPoints)
	}
	if got := app.userState.GetCodeBuffer("1", "python"); got != "print(1)" {
		t.Fatalf("local code buffer lost: %q", got)
	}
	if app.lastSynced.IsZero() {
		t.Fatal("lastSynced was not updated")
	}
	view := app.View()
	if !strings.Contains(view, "Synced 2 problems") || !strings.Contains(view, "100 points") {
		t.Fatal("sync status is not rendered in the Account page")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(pushes) != 1 {
		t.Fatalf("pushes = %d, want 1", len(pushes))
	}
	for _, id := range []string{`"1"`, `"2"`} {
		if !strings.Contains(pushes[0], id) {
			t.Fatalf("push body missing %s: %s", id, pushes[0])
		}
	}
	if strings.Contains(pushes[0], "id-token-secret") || strings.Contains(pushes[0], "print(1)") {
		t.Fatal("push body leaked token or code buffer")
	}
}

func TestSyncNowFirstUploadWhenRemoteMissing(t *testing.T) {
	app, _ := newSyncTestApp(t)
	var patched int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Document not found."}}`))
		case http.MethodPatch:
			patched++
			_, _ = w.Write([]byte(`{"fields":{}}`))
		}
	}))
	t.Cleanup(server.Close)
	app.firestoreBaseURL = server.URL

	m, cmd := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	result := cmd().(profileSyncResultMsg)
	if result.err != nil {
		t.Fatalf("first upload: %v", result.err)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if patched != 1 {
		t.Fatalf("patched %d times, want 1", patched)
	}
	if _, err := os.Stat(filepath.Join(app.userDataDir(), storage.ProfileFileName)); err != nil {
		t.Fatalf("encrypted profile was not written: %v", err)
	}
}

func TestSyncUnauthorizedKeepsSessionWithoutRefreshToken(t *testing.T) {
	app, _ := newSyncTestApp(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"Missing or invalid authentication token."}}`))
	}))
	t.Cleanup(server.Close)
	app.firestoreBaseURL = server.URL

	m, cmd := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	result := cmd().(profileSyncResultMsg)
	if !errors.Is(result.err, network.ErrUnauthorized) {
		t.Fatalf("sync error = %v, want ErrUnauthorized", result.err)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if app.account == nil {
		t.Fatal("an unauthorized sync must not clear the remembered account")
	}
	if !strings.Contains(app.View(), "user@example.com") {
		t.Fatal("the account page should still show the remembered identity")
	}
	if !hasVisibleToast(app, toast.Warning, "Sync needs re-authentication") {
		t.Fatalf("missing re-auth warning: %#v", visibleToasts(app))
	}
}

func TestSyncNowLocalPolicySkipsPullAndPushesLocal(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.config.Settings["sync.policy"] = "local"

	var (
		mu         sync.Mutex
		getCount   int
		patchCount int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			getCount++
			_, _ = w.Write([]byte(fmt.Sprintf(`{"fields":{
				"uid":{"stringValue":"uid-1"},
				"user":{"stringValue":"neo"},
				"score":{"integerValue":"100"},
				"updated":{"integerValue":"%d"},
				"solved":{"mapValue":{"fields":{"2":{"integerValue":"100"}}}}
			}}`, time.Now().Unix())))
		case http.MethodPatch:
			patchCount++
			_, _ = w.Write([]byte(`{"fields":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	app.firestoreBaseURL = server.URL

	m, cmd := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatal("sync returned no command")
	}
	result, ok := cmd().(profileSyncResultMsg)
	if !ok {
		t.Fatalf("sync produced %T", result)
	}
	if result.err != nil {
		t.Fatalf("sync: %v", result.err)
	}
	if result.at.IsZero() {
		t.Fatal("expected non-zero result.at")
	}

	m, _ = app.Update(result)
	app = m.(AppModel)

	mu.Lock()
	gotGet := getCount
	gotPatch := patchCount
	mu.Unlock()

	if gotGet != 0 {
		t.Fatalf("GET count = %d, want 0", gotGet)
	}
	if gotPatch != 1 {
		t.Fatalf("PATCH count = %d, want 1", gotPatch)
	}
	if !app.userState.HasSolved("1") || app.userState.HasSolved("2") || len(app.userState.SolvedProblems) != 1 {
		t.Fatalf("userState = %#v, want only solved '1'", app.userState.SolvedProblems)
	}
	if app.lastSynced.IsZero() {
		t.Fatal("lastSynced was not updated")
	}
	view := ansi.Strip(app.View())
	if !strings.Contains(view, "Firestore synced") || strings.Contains(view, "sync needs attention") {
		t.Fatal("local-policy success should render as a healthy sync, not an error")
	}

	metaPath := filepath.Join(app.userDataDir(), storage.SyncMetaFileName)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read sync metadata file: %v", err)
	}
	var meta storage.SyncMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal sync metadata: %v", err)
	}
	if meta.UID != "uid-1" {
		t.Fatalf("meta UID = %q, want uid-1", meta.UID)
	}
	if meta.LastSynced <= 0 {
		t.Fatalf("meta LastSynced = %d, want > 0", meta.LastSynced)
	}
	if strings.Contains(string(raw), "id-token-secret") {
		t.Fatal("sync-meta.json leaked id-token-secret")
	}
}

func TestSyncDisabledActionHasNoNetworkCommand(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.config.Settings["sync.enabled"] = "off"

	m, _ := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	if !strings.Contains(app.syncNote, "Cloud sync is off") {
		t.Fatalf("app.syncNote = %q, want containing 'Cloud sync is off'", app.syncNote)
	}
	if !hasVisibleToast(app, toast.Warning, "Cloud sync is off") {
		t.Fatalf("missing sync-disabled warning: %#v", visibleToasts(app))
	}
	plain := ansi.Strip(app.View())
	if !strings.Contains(plain, "cloud sync off") {
		t.Fatalf("app.View() missing 'cloud sync off': %s", plain)
	}
}

func TestUnauthorizedSyncKeepsRememberedSession(t *testing.T) {
	app, state := newSyncTestApp(t)
	app.account.RefreshToken = "refresh-token"
	app.account.RefreshedAt = time.Now()
	if err := app.sessionStore.Save(storedAuthSession{
		RefreshToken: "refresh-token",
		UID:          "uid-1",
		Email:        "user@example.com",
		SignedInAt:   time.Now(),
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	vaultPath := app.sessionStore.vault.Path()

	m, cmd := app.Update(profileSyncResultMsg{state: state, err: network.ErrUnauthorized})
	next := m.(AppModel)
	if next.account == nil {
		t.Fatal("unauthorized sync cleared the remembered account")
	}
	if _, err := os.Stat(vaultPath); err != nil {
		t.Fatalf("unauthorized sync deleted session.enc: %v", err)
	}
	if !hasVisibleToast(next, toast.Warning, "Sync needs re-authentication") {
		t.Fatalf("missing re-auth warning: %#v", visibleToasts(next))
	}
	for _, msg := range collectBatchMsgs(cmd) {
		if clear, ok := msg.(sessionPersistResultMsg); ok && clear.action == "clear" {
			t.Fatal("unauthorized sync scheduled a session clear")
		}
	}
}

func TestFailedSyncDoesNotPersistSuccessMetadata(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.config.Settings["sync.policy"] = "local"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			t.Error("unexpected GET request on local policy")
			http.Error(w, "unexpected GET", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"internal error"}}`))
	}))
	t.Cleanup(server.Close)
	app.firestoreBaseURL = server.URL

	m, cmd := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatal("expected sync command")
	}
	res := cmd()
	result, ok := res.(profileSyncResultMsg)
	if !ok {
		t.Fatalf("expected profileSyncResultMsg, got %T", res)
	}
	if result.err == nil {
		t.Fatal("expected error from failed sync, got nil")
	}
	if !result.at.IsZero() {
		t.Fatalf("expected result.at to be zero, got %v", result.at)
	}

	m, _ = app.Update(result)
	app = m.(AppModel)

	if !app.lastSynced.IsZero() {
		t.Fatalf("expected app.lastSynced to be zero, got %v", app.lastSynced)
	}

	metaPath := filepath.Join(app.userDataDir(), storage.SyncMetaFileName)
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Fatalf("expected sync-meta.json not to exist, got err: %v", err)
	}
}

func TestSyncMetadataSurvivesRestartForSameUID(t *testing.T) {
	app, _ := newSyncTestApp(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Document not found."}}`))
		case http.MethodPatch:
			_, _ = w.Write([]byte(`{"fields":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	app.firestoreBaseURL = server.URL

	m, cmd := app.Update(settings.ActionMsg{Action: "account_sync"})
	app = m.(AppModel)
	if cmd == nil {
		t.Fatal("expected sync command")
	}
	result, ok := cmd().(profileSyncResultMsg)
	if !ok || result.err != nil {
		t.Fatalf("sync failed: %v", result.err)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)

	savedLastSynced := app.lastSynced
	if savedLastSynced.IsZero() {
		t.Fatal("expected non-zero lastSynced after successful sync")
	}
	dataDir := app.config.DataDir()

	mgr, err := storage.NewManager(dataDir)
	if err != nil {
		t.Fatalf("storage.NewManager: %v", err)
	}
	second := NewAppModel(app.config, mgr)
	secondModel, _ := second.Update(accountAuthResultMsg{
		session: &accountSession{
			UID:         "uid-1",
			Email:       "user@example.com",
			DisplayName: "neo",
			IDToken:     "id-token-secret",
		},
	})
	second = secondModel.(AppModel)

	if second.accountInfo().LastSynced.Unix() != savedLastSynced.Unix() {
		t.Fatalf("second.accountInfo().LastSynced = %v, want %v",
			second.accountInfo().LastSynced.Unix(), savedLastSynced.Unix())
	}
	if second.syncNote == "" {
		t.Fatal("expected second.syncNote to be non-empty")
	}

	third := NewAppModel(app.config, mgr)
	thirdModel, _ := third.Update(accountAuthResultMsg{
		session: &accountSession{
			UID:         "uid-2",
			Email:       "user2@example.com",
			DisplayName: "trinity",
			IDToken:     "id-token-secret-2",
		},
	})
	third = thirdModel.(AppModel)

	if !third.accountInfo().LastSynced.IsZero() {
		t.Fatalf("expected UID uid-2 LastSynced to be zero, got %v", third.accountInfo().LastSynced)
	}
}
