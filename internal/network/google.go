package network

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ErrGoogleAuth is returned when the Google browser flow or the Firebase IDP
// exchange cannot complete.
var ErrGoogleAuth = errors.New("google sign-in failed")

// ErrBrowserUnavailable is returned when the OAuth URL could not be opened.
var ErrBrowserUnavailable = errors.New("unable to open the browser")

// GoogleConfig configures the browser-based Google sign-in flow.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// OpenBrowser is optional. When nil, the platform default browser is used.
	OpenBrowser func(string) error
}

// GoogleUser is the Firebase identity produced by a Google sign-in. The ID
// token is a bearer credential and must stay in memory only. The long-lived
// refresh token may be persisted encrypted by the caller.
type GoogleUser struct {
	UID          string
	IDToken      string
	RefreshToken string
	Email        string
	DisplayName  string
	PhotoURL     string
	IsNewUser    bool
}

type googleCallbackResult struct {
	code string
	err  error
}

type googleTokenResponse struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
}

type googleTokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type signInWithIDPRequest struct {
	PostBody          string `json:"postBody"`
	RequestURI        string `json:"requestUri"`
	ReturnSecureToken bool   `json:"returnSecureToken"`
}

type signInWithIDPResponse struct {
	LocalID      string `json:"localId"`
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	Email        string `json:"email"`
	DisplayName  string `json:"displayName"`
	PhotoURL     string `json:"photoUrl"`
	IsNewUser    bool   `json:"isNewUser"`
}

// SignInWithGoogle runs a desktop OAuth 2.0 authorization-code flow with PKCE
// against a loopback redirect, then exchanges the Google ID token for a
// Firebase identity. The loopback listener binds 127.0.0.1 only, the state is
// random, and the ID token stays in memory. The caller may persist the
// returned Firebase refresh token encrypted on the device.
func (s *AuthService) SignInWithGoogle(ctx context.Context, cfg GoogleConfig) (GoogleUser, error) {
	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		return GoogleUser{}, fmt.Errorf("%w: GOOGLE_OAUTH_CLIENT_ID is empty", ErrInvalidInput)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	verifier, challenge, err := pkcePair()
	if err != nil {
		return GoogleUser{}, fmt.Errorf("%w: generate PKCE pair: %v", ErrGoogleAuth, err)
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("%w: generate state: %v", ErrGoogleAuth, err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return GoogleUser{}, fmt.Errorf("%w: start loopback listener: %v", ErrGoogleAuth, err)
	}
	defer listener.Close()

	redirectURI := "http://" + listener.Addr().String() + "/oauth2callback"
	results := make(chan googleCallbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if rawErr := query.Get("error"); rawErr != "" {
			writeCallbackPage(w, "Sign-in was cancelled. You can close this tab and return to OpenLeet.")
			desc := query.Get("error_description")
			if desc == "" {
				desc = rawErr
			}
			select {
			case results <- googleCallbackResult{err: fmt.Errorf("%w: %s", ErrGoogleAuth, desc)}:
			default:
			}
			return
		}
		if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(state)) != 1 {
			writeCallbackPage(w, "The sign-in state check failed. Close this tab and try again.")
			select {
			case results <- googleCallbackResult{err: fmt.Errorf("%w: state mismatch", ErrGoogleAuth)}:
			default:
			}
			return
		}
		code := query.Get("code")
		if code == "" {
			writeCallbackPage(w, "No authorization code was returned. Close this tab and try again.")
			select {
			case results <- googleCallbackResult{err: fmt.Errorf("%w: missing authorization code", ErrGoogleAuth)}:
			default:
			}
			return
		}
		writeCallbackPage(w, "Signed in. You can close this tab and return to OpenLeet.")
		select {
		case results <- googleCallbackResult{code: code}:
		default:
		}
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", "openid email profile")
	values.Set("state", state)
	values.Set("code_challenge", challenge)
	values.Set("code_challenge_method", "S256")
	values.Set("prompt", "select_account")
	authURL := s.opts.googleAuthURL + "?" + values.Encode()

	open := cfg.OpenBrowser
	if open == nil {
		open = OpenBrowser
	}
	if err := open(authURL); err != nil {
		return GoogleUser{}, fmt.Errorf("%w: %v (open this URL manually: %s)", ErrBrowserUnavailable, err, authURL)
	}

	waitCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
	}

	select {
	case <-waitCtx.Done():
		return GoogleUser{}, fmt.Errorf("%w: timed out waiting for the browser", ErrGoogleAuth)
	case result := <-results:
		if result.err != nil {
			return GoogleUser{}, result.err
		}
		return s.exchangeGoogleCode(waitCtx, cfg, result.code, verifier, redirectURI)
	}
}

func (s *AuthService) exchangeGoogleCode(ctx context.Context, cfg GoogleConfig, code, verifier, redirectURI string) (GoogleUser, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", strings.TrimSpace(cfg.ClientID))
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", verifier)
	if secret := strings.TrimSpace(cfg.ClientSecret); secret != "" {
		form.Set("client_secret", secret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.opts.googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return GoogleUser{}, fmt.Errorf("%w: build token request: %v", ErrGoogleAuth, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("%w: token exchange: %v", ErrGoogleAuth, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return GoogleUser{}, fmt.Errorf("%w: read token response: %v", ErrGoogleAuth, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr googleTokenError
		_ = json.Unmarshal(body, &apiErr)
		message := strings.TrimSpace(apiErr.ErrorDescription)
		if message == "" {
			message = strings.TrimSpace(apiErr.Error)
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return GoogleUser{}, fmt.Errorf("%w: token exchange: %s", ErrGoogleAuth, message)
	}

	var token googleTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return GoogleUser{}, fmt.Errorf("%w: decode token response: %v", ErrGoogleAuth, err)
	}
	if strings.TrimSpace(token.IDToken) == "" {
		return GoogleUser{}, fmt.Errorf("%w: Google did not return an id_token", ErrGoogleAuth)
	}
	return s.signInWithGoogleIDToken(ctx, token.IDToken, redirectURI)
}

func (s *AuthService) signInWithGoogleIDToken(ctx context.Context, idToken, requestURI string) (GoogleUser, error) {
	endpoint := fmt.Sprintf("%s/accounts:signInWithIdp?key=%s", s.opts.authBaseURL, url.QueryEscape(s.apiKey))
	payload := signInWithIDPRequest{
		PostBody:          "id_token=" + url.QueryEscape(idToken) + "&providerId=google.com",
		RequestURI:        requestURI,
		ReturnSecureToken: true,
	}

	var resp signInWithIDPResponse
	if err := doJSON(ctx, s.client, http.MethodPost, endpoint, nil, payload, &resp); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			return GoogleUser{}, parseAuthHTTPError(httpErr)
		}
		if errors.Is(err, ErrMalformedResponse) {
			return GoogleUser{}, err
		}
		return GoogleUser{}, fmt.Errorf("firebase auth accounts:signInWithIdp: %w", err)
	}
	if resp.LocalID == "" || resp.IDToken == "" {
		return GoogleUser{}, fmt.Errorf("%w: missing localId or idToken in signInWithIdp response", ErrMalformedResponse)
	}
	return GoogleUser{
		UID:          resp.LocalID,
		IDToken:      resp.IDToken,
		RefreshToken: strings.TrimSpace(resp.RefreshToken),
		Email:        strings.TrimSpace(resp.Email),
		DisplayName:  strings.TrimSpace(resp.DisplayName),
		PhotoURL:     strings.TrimSpace(resp.PhotoURL),
		IsNewUser:    resp.IsNewUser,
	}, nil
}

func pkcePair() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomURLSafe(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func writeCallbackPage(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>OpenLeet sign-in</title></head><body style=\"font-family:system-ui,sans-serif;background:#111;color:#eee;display:grid;place-items:center;height:100vh;margin:0\"><p>"+html.EscapeString(message)+"</p></body></html>")
}

// OpenBrowser opens the platform default browser without a shell.
func OpenBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		switch {
		case hasExecutable("xdg-open"):
			cmd = exec.Command("xdg-open", target)
		case hasExecutable("wslview"):
			cmd = exec.Command("wslview", target)
		case hasExecutable("cmd.exe"):
			cmd = exec.Command("cmd.exe", "/c", "start", "", target)
		default:
			return fmt.Errorf("%w: no xdg-open, wslview, or cmd.exe found", ErrBrowserUnavailable)
		}
	}
	if cmd == nil {
		return ErrBrowserUnavailable
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func hasExecutable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
