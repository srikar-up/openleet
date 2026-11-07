package network

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthSignUpSuccess(t *testing.T) {
	var handled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts:signUp" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if key := r.URL.Query().Get("key"); key != "test-api-key" {
			t.Errorf("expected key 'test-api-key', got %q", key)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json Content-Type, got %q", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var req signUpRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request JSON: %v", err)
		}

		if req.Email != "user@example.com" {
			t.Errorf("unexpected email: %s", req.Email)
		}
		if req.Password != "secretpass123" {
			t.Errorf("unexpected password: %s", req.Password)
		}
		if req.DisplayName != "gopher" {
			t.Errorf("unexpected displayName: %s", req.DisplayName)
		}
		if !req.ReturnSecureToken {
			t.Errorf("expected returnSecureToken to be true")
		}

		handled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"localId": "user-uid-12345",
			"idToken": "jwt-token-abc",
			"email": "user@example.com",
			"displayName": "gopher"
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	uid, err := svc.SignUp(context.Background(), "user@example.com", "secretpass123", "gopher")
	if err != nil {
		t.Fatalf("unexpected SignUp error: %v", err)
	}
	if !handled {
		t.Fatal("expected server to handle request")
	}
	if uid != "user-uid-12345" {
		t.Errorf("expected uid 'user-uid-12345', got %q", uid)
	}
}

func TestAuthSignInSuccess(t *testing.T) {
	var handled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts:signInWithPassword" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if key := r.URL.Query().Get("key"); key != "test-api-key" {
			t.Errorf("expected key 'test-api-key', got %q", key)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		var req signInRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request JSON: %v", err)
		}

		if req.Email != "user@example.com" {
			t.Errorf("unexpected email: %s", req.Email)
		}
		if req.Password != "secretpass123" {
			t.Errorf("unexpected password: %s", req.Password)
		}
		if !req.ReturnSecureToken {
			t.Errorf("expected returnSecureToken to be true")
		}

		handled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"localId": "user-uid-67890",
			"idToken": "jwt-token-xyz",
			"email": "user@example.com"
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	uid, token, err := svc.SignIn(context.Background(), "user@example.com", "secretpass123")
	if err != nil {
		t.Fatalf("unexpected SignIn error: %v", err)
	}
	if !handled {
		t.Fatal("expected server to handle request")
	}
	if uid != "user-uid-67890" {
		t.Errorf("expected uid 'user-uid-67890', got %q", uid)
	}
	if token != "jwt-token-xyz" {
		t.Errorf("expected token 'jwt-token-xyz', got %q", token)
	}
}

func TestAuthSignUpEmailExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": 400,
				"message": "EMAIL_EXISTS"
			}
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	_, err = svc.SignUp(context.Background(), "existing@example.com", "secretpass123", "gopher")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrEmailExists) {
		t.Errorf("expected errors.Is(err, ErrEmailExists) to be true, got: %v", err)
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected errors.As to *AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "EMAIL_EXISTS" {
		t.Errorf("expected authErr.Code 'EMAIL_EXISTS', got %q", authErr.Code)
	}
	if authErr.StatusCode != 400 {
		t.Errorf("expected authErr.StatusCode 400, got %d", authErr.StatusCode)
	}
}

func TestAuthSignInInvalidPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": 400,
				"message": "INVALID_PASSWORD"
			}
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	_, _, err = svc.SignIn(context.Background(), "user@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected errors.Is(err, ErrInvalidCredentials) to be true, got: %v", err)
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected errors.As to *AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "INVALID_PASSWORD" {
		t.Errorf("expected authErr.Code 'INVALID_PASSWORD', got %q", authErr.Code)
	}
	if authErr.StatusCode != 400 {
		t.Errorf("expected authErr.StatusCode 400, got %d", authErr.StatusCode)
	}
}

func TestAuthWeakPasswordLocal(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	// Password with fewer than 6 bytes must be rejected locally before any HTTP invocation.
	_, err = svc.SignUp(context.Background(), "user@example.com", "12345", "gopher")
	if err == nil {
		t.Fatal("expected error for weak password, got nil")
	}

	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("expected errors.Is(err, ErrWeakPassword), got: %v", err)
	}

	if calls := atomic.LoadInt32(&callCount); calls != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", calls)
	}
}

func TestAuthMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	_, err = svc.SignUp(context.Background(), "user@example.com", "secretpass123", "gopher")
	if !errors.Is(err, ErrMalformedResponse) {
		t.Errorf("SignUp: expected ErrMalformedResponse, got: %v", err)
	}

	_, _, err = svc.SignIn(context.Background(), "user@example.com", "secretpass123")
	if !errors.Is(err, ErrMalformedResponse) {
		t.Errorf("SignIn: expected ErrMalformedResponse, got: %v", err)
	}
}

func TestAuthTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	_, err = svc.SignUp(ctx, "user@example.com", "secretpass123", "gopher")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected errors.Is(err, context.DeadlineExceeded) to be true, got: %v", err)
	}
}

func TestAuthInvalidAPIKeyConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": 400,
				"message": "API_KEY_INVALID"
			}
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("bad-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}

	_, err = svc.SignUp(context.Background(), "user@example.com", "secretpass123", "gopher")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrAuthConfig) {
		t.Errorf("expected errors.Is(err, ErrAuthConfig) to be true, got: %v", err)
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected errors.As to *AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "API_KEY_INVALID" {
		t.Errorf("expected Code 'API_KEY_INVALID', got %q", authErr.Code)
	}
}

func TestAuthComplexErrorMapping(t *testing.T) {
	tests := []struct {
		name             string
		firebaseMessage  string
		expectedSentinel error
		expectedCode     string
	}{
		{
			name:             "weak password with description",
			firebaseMessage:  "WEAK_PASSWORD : Password should be at least 6 characters",
			expectedSentinel: ErrWeakPassword,
			expectedCode:     "WEAK_PASSWORD",
		},
		{
			name:             "rate limited",
			firebaseMessage:  "TOO_MANY_ATTEMPTS_TRY_LATER",
			expectedSentinel: ErrRateLimited,
			expectedCode:     "TOO_MANY_ATTEMPTS_TRY_LATER",
		},
		{
			name:             "email not found maps to invalid credentials",
			firebaseMessage:  "EMAIL_NOT_FOUND",
			expectedSentinel: ErrInvalidCredentials,
			expectedCode:     "EMAIL_NOT_FOUND",
		},
		{
			name:             "project not found maps to auth config",
			firebaseMessage:  "PROJECT_NOT_FOUND",
			expectedSentinel: ErrAuthConfig,
			expectedCode:     "PROJECT_NOT_FOUND",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				resp := map[string]interface{}{
					"error": map[string]interface{}{
						"code":    400,
						"message": tc.firebaseMessage,
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			svc, err := NewAuthService("test-key", WithAuthBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewAuthService failed: %v", err)
			}

			_, _, err = svc.SignIn(context.Background(), "user@example.com", "anypass")
			if !errors.Is(err, tc.expectedSentinel) {
				t.Errorf("expected sentinel %v, got: %v", tc.expectedSentinel, err)
			}

			var authErr *AuthError
			if !errors.As(err, &authErr) {
				t.Fatalf("expected *AuthError via errors.As, got: %v", err)
			}
			if authErr.Code != tc.expectedCode {
				t.Errorf("expected Code %q, got %q", tc.expectedCode, authErr.Code)
			}
		})
	}
}

func TestErrorMsg(t *testing.T) {
	// Formatting with Op and Err
	msg1 := ErrorMsg{Op: "network.login", Err: errors.New("connection failed")}
	if got := msg1.Error(); got != "network.login: connection failed" {
		t.Errorf("unexpected formatted error: %s", got)
	}
	if !errors.Is(msg1.Unwrap(), msg1.Err) {
		t.Errorf("Unwrap mismatch")
	}

	// Safe when Err is nil
	msg2 := ErrorMsg{Op: "network.offline", Err: nil}
	if got := msg2.Error(); got != "network.offline" {
		t.Errorf("unexpected formatted error when Err is nil: %s", got)
	}

	// Empty Op with Err
	msg3 := ErrorMsg{Op: "", Err: errors.New("raw error")}
	if got := msg3.Error(); got != "raw error" {
		t.Errorf("unexpected formatted error when Op is empty: %s", got)
	}
}

func TestIsNetworkError(t *testing.T) {
	if IsNetworkError(nil) {
		t.Error("expected false for nil error")
	}
	if !IsNetworkError(context.DeadlineExceeded) {
		t.Error("expected true for context.DeadlineExceeded")
	}
	if !IsNetworkError(context.Canceled) {
		t.Error("expected true for context.Canceled")
	}
	if !IsNetworkError(&net.DNSError{Err: "no such host"}) {
		t.Error("expected true for net.Error")
	}
	if IsNetworkError(errors.New("generic domain error")) {
		t.Error("expected false for generic error")
	}
}

func TestAuthSignInWithProfileReturnsDisplayName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"localId": "user-uid-67890",
			"idToken": "jwt-token-xyz",
			"refreshToken": "refresh-token-abc",
			"displayName": "gopher"
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}
	result, err := svc.SignInWithProfile(context.Background(), "user@example.com", "secretpass123")
	if err != nil {
		t.Fatalf("unexpected SignInWithProfile error: %v", err)
	}
	if result.UID != "user-uid-67890" || result.IDToken != "jwt-token-xyz" || result.DisplayName != "gopher" {
		t.Fatalf("unexpected sign-in result: %+v", result)
	}
	if result.RefreshToken != "refresh-token-abc" {
		t.Fatalf("refresh token = %q, want refresh-token-abc", result.RefreshToken)
	}
}

func TestAuthRefreshSessionSuccess(t *testing.T) {
	var sawForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Query().Get("key"); got != "test-api-key" {
			t.Errorf("key = %q, want test-api-key", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		sawForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"access-token",
			"expires_in":"3600",
			"token_type":"Bearer",
			"refresh_token":"rotated-refresh-token",
			"id_token":"fresh-id-token",
			"user_id":"user-uid-123",
			"project_id":"test-project"
		}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithSecureTokenURL(server.URL))
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	result, err := svc.RefreshSession(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if result.UID != "user-uid-123" || result.IDToken != "fresh-id-token" || result.RefreshToken != "rotated-refresh-token" {
		t.Fatalf("result = %#v", result)
	}
	if sawForm.Get("grant_type") != "refresh_token" || sawForm.Get("refresh_token") != "old-refresh-token" {
		t.Fatalf("form = %#v", sawForm)
	}
}

func TestAuthRefreshSessionExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"INVALID_REFRESH_TOKEN","status":"INVALID_ARGUMENT"}}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithSecureTokenURL(server.URL))
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	if _, err := svc.RefreshSession(context.Background(), "revoked-token"); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("error = %v, want ErrSessionExpired", err)
	}
}

func TestAuthSignInWithProfileNeverDerivesNameFromEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"localId":"uid","idToken":"token"}`))
	}))
	defer server.Close()

	svc, err := NewAuthService("test-api-key", WithAuthBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error creating AuthService: %v", err)
	}
	result, err := svc.SignInWithProfile(context.Background(), "private@example.com", "secretpass123")
	if err != nil {
		t.Fatalf("unexpected SignInWithProfile error: %v", err)
	}
	if result.DisplayName != "" {
		t.Fatalf("display name must not be derived from email, got %q", result.DisplayName)
	}
}
