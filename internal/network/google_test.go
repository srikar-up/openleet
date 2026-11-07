package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignInWithGoogleExchangesCodeAndFirebaseToken(t *testing.T) {
	var (
		tokenForm url.Values
		idpBody   signInWithIDPRequest
	)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		tokenForm, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id_token":"google-id-token","access_token":"google-access-token"}`))
	}))
	defer tokenServer.Close()

	firebaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts:signInWithIdp" {
			t.Errorf("unexpected Firebase path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &idpBody); err != nil {
			t.Errorf("decode idp body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"localId":"google-uid",
			"idToken":"firebase-id-token",
			"email":"gopher@example.com",
			"displayName":"Gopher",
			"photoUrl":"https://example.com/gopher.png",
			"isNewUser":true
		}`))
	}))
	defer firebaseServer.Close()

	client := &http.Client{}
	svc, err := NewAuthService("test-key",
		WithAuthBaseURL(firebaseServer.URL),
		WithGoogleAuthURL("https://accounts.example.test/o/oauth2/v2/auth"),
		WithGoogleTokenURL(tokenServer.URL),
		WithHTTPClient(client),
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	var redirectURI, state, challenge string
	opener := func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		q := u.Query()
		if q.Get("client_id") != "google-client-id" {
			t.Errorf("client_id = %q", q.Get("client_id"))
		}
		if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
			t.Errorf("missing PKCE challenge: %v", q)
		}
		state = q.Get("state")
		challenge = q.Get("code_challenge")
		redirectURI = q.Get("redirect_uri")
		callback := redirectURI + "?code=test-code&state=" + url.QueryEscape(state)
		resp, err := http.Get(callback)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("callback status %d", resp.StatusCode)
		}
		return nil
	}

	user, err := svc.SignInWithGoogle(context.Background(), GoogleConfig{
		ClientID:     "google-client-id",
		ClientSecret: "optional-secret",
		OpenBrowser:  opener,
	})
	if err != nil {
		t.Fatalf("SignInWithGoogle: %v", err)
	}
	if user.UID != "google-uid" || user.IDToken != "firebase-id-token" || user.Email != "gopher@example.com" || user.DisplayName != "Gopher" || !user.IsNewUser {
		t.Fatalf("unexpected google user: %+v", user)
	}
	if state == "" || challenge == "" || redirectURI == "" {
		t.Fatalf("missing OAuth parameters: state=%q challenge=%q redirect=%q", state, challenge, redirectURI)
	}
	if tokenForm.Get("code") != "test-code" {
		t.Fatalf("token code = %q", tokenForm.Get("code"))
	}
	if tokenForm.Get("code_verifier") == "" {
		t.Fatal("token exchange omitted the PKCE verifier")
	}
	if tokenForm.Get("redirect_uri") != redirectURI {
		t.Fatalf("token redirect_uri = %q, want %q", tokenForm.Get("redirect_uri"), redirectURI)
	}
	if tokenForm.Get("client_secret") != "optional-secret" {
		t.Fatalf("client_secret = %q", tokenForm.Get("client_secret"))
	}
	if !strings.Contains(idpBody.PostBody, "id_token=google-id-token") || !strings.Contains(idpBody.PostBody, "providerId=google.com") {
		t.Fatalf("postBody = %q", idpBody.PostBody)
	}
	if idpBody.RequestURI != redirectURI {
		t.Fatalf("requestUri = %q, want %q", idpBody.RequestURI, redirectURI)
	}
}

func TestSignInWithGoogleRejectsStateMismatch(t *testing.T) {
	svc, err := NewAuthService("test-key",
		WithAuthBaseURL("http://127.0.0.1:1"),
		WithGoogleAuthURL("https://accounts.example.test/auth"),
		WithGoogleTokenURL("http://127.0.0.1:1/token"),
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	opener := func(raw string) error {
		u, _ := url.Parse(raw)
		redirect := u.Query().Get("redirect_uri")
		resp, err := http.Get(redirect + "?code=test-code&state=wrong-state")
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	_, err = svc.SignInWithGoogle(context.Background(), GoogleConfig{
		ClientID:    "google-client-id",
		OpenBrowser: opener,
	})
	if !errors.Is(err, ErrGoogleAuth) {
		t.Fatalf("error = %v, want ErrGoogleAuth", err)
	}
}

func TestSignInWithGoogleHandlesCancellation(t *testing.T) {
	svc, err := NewAuthService("test-key",
		WithAuthBaseURL("http://127.0.0.1:1"),
		WithGoogleAuthURL("https://accounts.example.test/auth"),
		WithGoogleTokenURL("http://127.0.0.1:1/token"),
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	opener := func(raw string) error {
		u, _ := url.Parse(raw)
		redirect := u.Query().Get("redirect_uri")
		state := u.Query().Get("state")
		resp, err := http.Get(redirect + "?error=access_denied&error_description=cancelled&state=" + url.QueryEscape(state))
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	_, err = svc.SignInWithGoogle(context.Background(), GoogleConfig{
		ClientID:    "google-client-id",
		OpenBrowser: opener,
	})
	if !errors.Is(err, ErrGoogleAuth) || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("error = %v, want cancelled Google auth error", err)
	}
}

func TestSignInWithGoogleBrowserFailure(t *testing.T) {
	svc, err := NewAuthService("test-key",
		WithGoogleAuthURL("https://accounts.example.test/auth"),
		WithGoogleTokenURL("http://127.0.0.1:1/token"),
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	_, err = svc.SignInWithGoogle(context.Background(), GoogleConfig{
		ClientID:    "google-client-id",
		OpenBrowser: func(string) error { return errors.New("no display") },
	})
	if !errors.Is(err, ErrBrowserUnavailable) {
		t.Fatalf("error = %v, want ErrBrowserUnavailable", err)
	}
	if !strings.Contains(err.Error(), "accounts.example.test") {
		t.Fatal("browser failure should include the manual OAuth URL")
	}
}

func TestSignInWithGoogleRequiresClientID(t *testing.T) {
	svc, err := NewAuthService("test-key")
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	_, err = svc.SignInWithGoogle(context.Background(), GoogleConfig{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestSignInWithGoogleTimesOut(t *testing.T) {
	svc, err := NewAuthService("test-key",
		WithGoogleAuthURL("https://accounts.example.test/auth"),
		WithGoogleTokenURL("http://127.0.0.1:1/token"),
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = svc.SignInWithGoogle(ctx, GoogleConfig{
		ClientID:    "google-client-id",
		OpenBrowser: func(string) error { return nil },
	})
	if !errors.Is(err, ErrGoogleAuth) || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestSignInWithGoogleRejectsTokenWithoutIDToken(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"only-access"}`))
	}))
	defer tokenServer.Close()
	svc, err := NewAuthService("test-key",
		WithGoogleAuthURL("https://accounts.example.test/auth"),
		WithGoogleTokenURL(tokenServer.URL),
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	opener := func(raw string) error {
		u, _ := url.Parse(raw)
		redirect := u.Query().Get("redirect_uri")
		state := u.Query().Get("state")
		resp, err := http.Get(redirect + "?code=test-code&state=" + url.QueryEscape(state))
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	_, err = svc.SignInWithGoogle(context.Background(), GoogleConfig{
		ClientID:    "google-client-id",
		OpenBrowser: opener,
	})
	if !errors.Is(err, ErrGoogleAuth) || !strings.Contains(err.Error(), "id_token") {
		t.Fatalf("error = %v, want missing id_token", err)
	}
}
