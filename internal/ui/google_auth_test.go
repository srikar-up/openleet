package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"openleet/internal/ui/commands"
	"openleet/internal/ui/toast"
)

func TestLoginQuickActionStartsGoogle(t *testing.T) {
	app := newRoutingTestApp(t)
	next, cmd := app.executeCommand(commands.Command{
		Name:   "/login",
		Action: commands.ActionGoogleLogin,
	}, "")
	if cmd == nil {
		t.Fatal("/login should dispatch a Google auth command")
	}
	if !hasVisibleToast(next, toast.Info, "Opening Google") {
		t.Fatalf("toasts = %#v, want opening hint", visibleToasts(next))
	}
	if !next.settingsView.GoogleBusy() {
		t.Fatal("/login should mark the Settings account card busy")
	}
}

func TestLoginQuickActionWhenAlreadySignedIn(t *testing.T) {
	app := newRoutingTestApp(t)
	app.account = &accountSession{UID: "uid-1", Email: "neo@example.com", IDToken: "token"}
	next, _ := app.executeCommand(commands.Command{
		Name:   "/login",
		Action: commands.ActionGoogleLogin,
	}, "")
	if !hasVisibleToast(next, toast.Info, "neo@example.com") {
		t.Fatalf("toasts = %#v, want current account", visibleToasts(next))
	}
	if next.settingsView.GoogleBusy() {
		t.Fatal("signed-in /login must not mark the account card busy")
	}
}

func TestGoogleAuthCmdRequiresClientID(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	app := newRoutingTestApp(t)
	cmd := app.googleAuthCmd()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	result, ok := cmd().(accountAuthResultMsg)
	if !ok {
		t.Fatalf("unexpected message %T", cmd())
	}
	if result.err == nil || !strings.Contains(result.err.Error(), "GOOGLE_OAUTH_CLIENT_ID") {
		t.Fatalf("error = %v, want missing client ID", result.err)
	}
}

func TestGoogleAuthCmdMapsSession(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "google-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id_token":"google-id-token"}`))
	}))
	defer tokenServer.Close()

	firebaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts:signInWithIdp" {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body["postBody"].(string), "providerId=google.com") {
			t.Errorf("postBody = %v", body["postBody"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"localId":"google-uid",
			"idToken":"firebase-id-token",
			"email":"gopher@example.com",
			"displayName":"Gopher"
		}`))
	}))
	defer firebaseServer.Close()

	app := newRoutingTestApp(t)
	app.authBaseURL = firebaseServer.URL
	app.googleAuthURL = "https://accounts.example.test/auth"
	app.googleTokenURL = tokenServer.URL
	app.openBrowser = func(raw string) error {
		parsed, err := url.Parse(raw)
		if err != nil {
			return err
		}
		redirect := parsed.Query().Get("redirect_uri")
		state := parsed.Query().Get("state")
		resp, err := http.Get(redirect + "?code=test-code&state=" + url.QueryEscape(state))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}

	result, ok := app.googleAuthCmd()().(accountAuthResultMsg)
	if !ok {
		t.Fatalf("unexpected message %T", result)
	}
	if result.err != nil {
		t.Fatalf("google auth: %v", result.err)
	}
	if result.session == nil || result.session.UID != "google-uid" || result.session.IDToken != "firebase-id-token" {
		t.Fatalf("session = %#v", result.session)
	}
	if result.session.PublicName != "Gopher" || result.session.DisplayName != "Gopher" {
		t.Fatalf("display names = %#v", result.session)
	}

	m, _ := app.Update(result)
	app = m.(AppModel)
	if app.account == nil || app.account.UID != "google-uid" {
		t.Fatal("app did not adopt the Google session")
	}
	if !hasVisibleToast(app, toast.Success, "Signed in") {
		t.Fatalf("toasts = %#v, want signed-in success", visibleToasts(app))
	}
}

func TestGoogleAuthCmdRejectsAuthDomainAsClientID(t *testing.T) {
	t.Setenv("FIREBASE_API_KEY", "test-web-key")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "openleet-cd127.firebaseapp.com")
	app := newRoutingTestApp(t)
	result, ok := app.googleAuthCmd()().(accountAuthResultMsg)
	if !ok {
		t.Fatalf("unexpected message %T", result)
	}
	if result.err == nil || !strings.Contains(result.err.Error(), "auth domain") {
		t.Fatalf("error = %v, want auth-domain guidance", result.err)
	}
}
