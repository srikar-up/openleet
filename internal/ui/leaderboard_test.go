package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"openleet/internal/network"
	"openleet/internal/ui/settings"
)

func TestPublishLeaderboardCmdIsNilWhenSignedOut(t *testing.T) {
	app, _ := newPersistentRoutingTestApp(t)
	app.account = nil
	if cmd := app.publishLeaderboardCmd(); cmd != nil {
		t.Fatal("signed-out app must not publish a leaderboard entry")
	}
}

func TestPublishLeaderboardCmdSendsMinimalPayload(t *testing.T) {
	app, _ := newSyncTestApp(t)
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":{}}`))
	}))
	defer server.Close()
	app.firestoreBaseURL = server.URL

	cmd := app.publishLeaderboardCmd()
	if cmd == nil {
		t.Fatal("signed-in app should publish a leaderboard entry")
	}
	result, ok := cmd().(leaderboardWriteResultMsg)
	if !ok {
		t.Fatalf("unexpected message type %T", cmd())
	}
	if result.err != nil {
		t.Fatalf("publish failed: %v", result.err)
	}

	var payload struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.Fields) != 4 {
		t.Fatalf("leaderboard fields = %v, want exactly four", payload.Fields)
	}
	for _, want := range []string{"name", "score", "solved", "updated"} {
		if _, ok := payload.Fields[want]; !ok {
			t.Fatalf("missing leaderboard field %q", want)
		}
	}
	for _, forbidden := range []string{"email", "uid", "code", "last_code", "solved_map"} {
		if _, ok := payload.Fields[forbidden]; ok {
			t.Fatalf("leaderboard payload must not contain %q", forbidden)
		}
	}
	if strings.Contains(string(body), "user@example.com") {
		t.Fatal("leaderboard payload leaked the account email")
	}
}

func TestLeaderboardResultUpdatesStats(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.statsView.SetSize(120, 40)
	app.leaderboardLoading = true
	app.refreshLeaderboardView()

	updated, _ := app.Update(leaderboardResultMsg{entries: []network.LeaderboardEntry{
		{UID: "uid-2", Name: "ada", Score: 20, Solved: 2},
		{UID: "uid-1", Name: "neo", Score: 10, Solved: 1},
	}})
	app = updated.(AppModel)
	if app.leaderboardLoading {
		t.Fatal("leaderboard result must clear the loading flag")
	}
	if !app.leaderboardLoaded || len(app.leaderboard) != 2 {
		t.Fatalf("leaderboard cache = %+v, loaded=%v", app.leaderboard, app.leaderboardLoaded)
	}
	view := app.statsView.View()
	for _, want := range []string{"LEADERBOARD", "#1 ada 20", "#2 neo 10"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in the stats view\n%s", want, view)
		}
	}
}

func TestEnterStatsLoadsLeaderboardOnlyWhenSignedIn(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.currentView = ViewHome
	app.leaderboardLoaded = false
	next, cmd := app.enterStats()
	if next.currentView != ViewStats {
		t.Fatalf("view = %v, want stats", next.currentView)
	}
	if !next.leaderboardLoading || cmd == nil {
		t.Fatal("a signed-in user should trigger one leaderboard read on first visit")
	}

	app.account = nil
	app.leaderboardLoaded = false
	next, cmd = app.enterStats()
	if next.leaderboardLoading || cmd != nil {
		t.Fatal("a signed-out user must not start a leaderboard read")
	}
}

func TestLeaderboardEntryUsesPublicNameNotEmailPrefix(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.account.DisplayName = "user"
	app.account.PublicName = ""
	entry, ok := app.leaderboardEntry()
	if !ok {
		t.Fatal("expected a leaderboard entry while signed in")
	}
	if entry.Name != "Anonymous" {
		t.Fatalf("name = %q, want Anonymous when no public name is set", entry.Name)
	}

	app.account.PublicName = "neo"
	entry, _ = app.leaderboardEntry()
	if entry.Name != "neo" {
		t.Fatalf("name = %q, want neo", entry.Name)
	}
}

func TestLeaderboardPublicNameComesFromSignUpNotEmail(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	var calls []string
	server := newAuthTestServer(t, &calls)

	app := newRoutingTestApp(t)
	app.authBaseURL = server.URL
	m, cmd := app.Update(settings.AuthRequestMsg{
		Mode:     settings.AuthSignUp,
		Email:    "private@example.com",
		Username: "neo",
		Password: "secret123",
	})
	app = m.(AppModel)
	result, ok := cmd().(accountAuthResultMsg)
	if !ok || result.err != nil {
		t.Fatalf("sign-up result = %#v", result)
	}
	m, _ = app.Update(result)
	app = m.(AppModel)
	if app.account == nil || app.account.PublicName != "neo" {
		t.Fatalf("public name = %#v, want neo", app.account)
	}
	if strings.Contains(app.account.PublicName, "private") {
		t.Fatal("public name leaked the email prefix")
	}
}

func TestLeaderboardWriteResultKeepsReadCache(t *testing.T) {
	app, _ := newSyncTestApp(t)
	app.statsView.SetSize(120, 40)
	app.leaderboard = []network.LeaderboardEntry{
		{UID: "uid-1", Name: "neo", Score: 10, Solved: 1},
	}
	app.leaderboardLoaded = true
	app.refreshLeaderboardView()

	updated, _ := app.Update(leaderboardWriteResultMsg{})
	app = updated.(AppModel)
	if len(app.leaderboard) != 1 || !app.leaderboardLoaded {
		t.Fatalf("write result cleared the read cache: %+v", app.leaderboard)
	}
	if view := app.statsView.View(); !strings.Contains(view, "neo") {
		t.Fatal("write result replaced the rendered ranking")
	}
}
