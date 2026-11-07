package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	// ErrEmailExists is returned when trying to sign up with an existing email.
	ErrEmailExists = errors.New("email already registered")
	// ErrInvalidCredentials is returned when email or password is incorrect.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrWeakPassword is returned when the password does not meet requirements.
	ErrWeakPassword = errors.New("password does not meet requirements")
	// ErrRateLimited is returned when Firebase temporarily blocks requests due to too many attempts.
	ErrRateLimited = errors.New("too many attempts, try again later")
	// ErrAuthConfig is returned when the Firebase project or API key configuration is invalid.
	ErrAuthConfig = errors.New("firebase authentication is misconfigured")
)

// AuthError represents an error returned by the Firebase Authentication REST API.
type AuthError struct {
	Code       string
	Message    string
	StatusCode int
	sentinel   error
}

// Error formats the Firebase authentication error.
func (e *AuthError) Error() string {
	if e.Message != "" {
		if e.Code != "" {
			return fmt.Sprintf("firebase auth: %s (%s)", e.Message, e.Code)
		}
		return fmt.Sprintf("firebase auth: %s", e.Message)
	}
	if e.Code != "" {
		return fmt.Sprintf("firebase auth: status %d (%s)", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("firebase auth: status %d", e.StatusCode)
}

// Unwrap returns the underlying sentinel error for errors.Is matching.
func (e *AuthError) Unwrap() error {
	return e.sentinel
}

// Is reports whether this AuthError matches target error.
func (e *AuthError) Is(target error) bool {
	return e.sentinel != nil && errors.Is(e.sentinel, target)
}

// AuthService handles authentication with Firebase Identity Toolkit REST endpoints.
type AuthService struct {
	apiKey string
	opts   options
	client *http.Client
}

// NewAuthService creates and initializes a new AuthService.
func NewAuthService(apiKey string, opts ...Option) (*AuthService, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return nil, fmt.Errorf("%w: API key cannot be empty", ErrInvalidInput)
	}

	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	o.authBaseURL = strings.TrimRight(o.authBaseURL, "/")
	u, err := url.Parse(o.authBaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: invalid auth base URL %q", ErrInvalidInput, o.authBaseURL)
	}

	return &AuthService{
		apiKey: key,
		opts:   o,
		client: o.resolveClient(),
	}, nil
}

type signUpRequest struct {
	Email             string `json:"email"`
	Password          string `json:"password"`
	ReturnSecureToken bool   `json:"returnSecureToken"`
	DisplayName       string `json:"displayName"`
}

type signUpResponse struct {
	LocalID     string `json:"localId"`
	IDToken     string `json:"idToken"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type signInRequest struct {
	Email             string `json:"email"`
	Password          string `json:"password"`
	ReturnSecureToken bool   `json:"returnSecureToken"`
}

type signInResponse struct {
	LocalID      string `json:"localId"`
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	DisplayName  string `json:"displayName"`
}

// SignInResult is the ephemeral result of a password sign-in. DisplayName is
// the public profile name Firebase stored at sign-up; it is never derived from
// the email address. RefreshToken is the only long-lived credential and may be
// persisted encrypted by the caller.
type SignInResult struct {
	UID          string
	IDToken      string
	RefreshToken string
	DisplayName  string
}

type firebaseErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type refreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    string `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	UserID       string `json:"user_id"`
	ProjectID    string `json:"project_id"`
}

type secureTokenErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func parseAuthHTTPError(httpErr *HTTPError) *AuthError {
	var env firebaseErrorResponse
	if err := json.Unmarshal(httpErr.Body, &env); err != nil || strings.TrimSpace(env.Error.Message) == "" {
		return &AuthError{
			StatusCode: httpErr.StatusCode,
			Message:    "authentication failed",
		}
	}

	rawMsg := strings.TrimSpace(env.Error.Message)
	code := rawMsg
	if idx := strings.Index(rawMsg, ":"); idx != -1 {
		code = strings.TrimSpace(rawMsg[:idx])
	}

	var sentinel error
	switch code {
	case "EMAIL_EXISTS":
		sentinel = ErrEmailExists
	case "EMAIL_NOT_FOUND", "INVALID_PASSWORD", "INVALID_LOGIN_CREDENTIALS", "INVALID_EMAIL", "MISSING_PASSWORD":
		sentinel = ErrInvalidCredentials
	case "WEAK_PASSWORD":
		sentinel = ErrWeakPassword
	case "TOO_MANY_ATTEMPTS_TRY_LATER":
		sentinel = ErrRateLimited
	case "API_KEY_INVALID", "PROJECT_NOT_FOUND", "OPERATION_NOT_ALLOWED", "APP_NOT_FOUND":
		sentinel = ErrAuthConfig
	default:
		sentinel = nil
	}

	return &AuthError{
		Code:       code,
		Message:    rawMsg,
		StatusCode: httpErr.StatusCode,
		sentinel:   sentinel,
	}
}

// SignUp registers a new user with Firebase Identity Toolkit.
// Note: displayName is sent here on a best-effort basis for Firebase Auth.
// We do not issue a secondary updateProfile request; the Firestore profile
// document is the authoritative source of truth for usernames.
func (s *AuthService) SignUp(ctx context.Context, email, password, username string) (string, error) {
	email = strings.TrimSpace(email)
	username = strings.TrimSpace(username)

	if email == "" {
		return "", fmt.Errorf("%w: email cannot be empty", ErrInvalidInput)
	}
	if password == "" {
		return "", fmt.Errorf("%w: password cannot be empty", ErrInvalidInput)
	}
	if username == "" {
		return "", fmt.Errorf("%w: username cannot be empty", ErrInvalidInput)
	}
	if len(password) < 6 {
		return "", fmt.Errorf("%w: password must be at least 6 characters", ErrWeakPassword)
	}

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	endpoint := fmt.Sprintf("%s/accounts:signUp?key=%s", s.opts.authBaseURL, url.QueryEscape(s.apiKey))
	payload := signUpRequest{
		Email:             email,
		Password:          password,
		ReturnSecureToken: true,
		DisplayName:       username,
	}

	var resp signUpResponse
	if err := doJSON(reqCtx, s.client, http.MethodPost, endpoint, nil, payload, &resp); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			return "", parseAuthHTTPError(httpErr)
		}
		if errors.Is(err, ErrMalformedResponse) {
			return "", err
		}
		return "", fmt.Errorf("firebase auth accounts:signUp: %w", err)
	}

	if resp.LocalID == "" {
		return "", fmt.Errorf("%w: missing localId in signUp response", ErrMalformedResponse)
	}

	return resp.LocalID, nil
}

// SignIn authenticates an existing user with email and password. The returned
// ID token is a bearer credential: keep it in memory only, never write it to
// the local profile, logs, or crash reports. Firebase ID tokens expire after
// roughly one hour; callers should prompt for a fresh sign-in and retry after
// Firestore returns ErrUnauthorized.
func (s *AuthService) SignIn(ctx context.Context, email, password string) (string, string, error) {
	result, err := s.SignInWithProfile(ctx, email, password)
	return result.UID, result.IDToken, err
}

// SignInWithProfile is SignIn plus the public display name Firebase stored at
// sign-up. The name is only used for the optional leaderboard; it is never
// derived from the email address.
func (s *AuthService) SignInWithProfile(ctx context.Context, email, password string) (SignInResult, error) {
	email = strings.TrimSpace(email)

	if email == "" {
		return SignInResult{}, fmt.Errorf("%w: email cannot be empty", ErrInvalidInput)
	}
	if password == "" {
		return SignInResult{}, fmt.Errorf("%w: password cannot be empty", ErrInvalidInput)
	}

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	endpoint := fmt.Sprintf("%s/accounts:signInWithPassword?key=%s", s.opts.authBaseURL, url.QueryEscape(s.apiKey))
	payload := signInRequest{
		Email:             email,
		Password:          password,
		ReturnSecureToken: true,
	}

	var resp signInResponse
	if err := doJSON(reqCtx, s.client, http.MethodPost, endpoint, nil, payload, &resp); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			return SignInResult{}, parseAuthHTTPError(httpErr)
		}
		if errors.Is(err, ErrMalformedResponse) {
			return SignInResult{}, err
		}
		return SignInResult{}, fmt.Errorf("firebase auth accounts:signInWithPassword: %w", err)
	}

	if resp.LocalID == "" || resp.IDToken == "" {
		return SignInResult{}, fmt.Errorf("%w: missing localId or idToken in signIn response", ErrMalformedResponse)
	}

	return SignInResult{
		UID:          resp.LocalID,
		IDToken:      resp.IDToken,
		RefreshToken: strings.TrimSpace(resp.RefreshToken),
		DisplayName:  strings.TrimSpace(resp.DisplayName),
	}, nil
}

// RefreshSession exchanges a long-lived Firebase refresh token for a fresh ID
// token. Firebase rotates refresh tokens, so callers should persist the
// returned RefreshToken when it differs from the one they supplied.
func (s *AuthService) RefreshSession(ctx context.Context, refreshToken string) (SignInResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return SignInResult{}, fmt.Errorf("%w: refresh token cannot be empty", ErrInvalidInput)
	}

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	endpoint := fmt.Sprintf("%s?key=%s", strings.TrimRight(s.opts.secureTokenURL, "/"), url.QueryEscape(s.apiKey))
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return SignInResult{}, fmt.Errorf("firebase securetoken: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return SignInResult{}, fmt.Errorf("firebase securetoken: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SignInResult{}, fmt.Errorf("firebase securetoken: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var env secureTokenErrorResponse
		message := ""
		if err := json.Unmarshal(body, &env); err == nil {
			message = strings.TrimSpace(env.Error.Message)
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		switch message {
		case "TOKEN_EXPIRED", "INVALID_REFRESH_TOKEN", "INVALID_GRANT", "MISSING_REFRESH_TOKEN", "USER_DISABLED", "USER_NOT_FOUND":
			return SignInResult{}, fmt.Errorf("%w: %s", ErrSessionExpired, message)
		}
		return SignInResult{}, &AuthError{
			Code:       message,
			Message:    message,
			StatusCode: resp.StatusCode,
		}
	}

	var out refreshTokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return SignInResult{}, fmt.Errorf("%w: decode securetoken response: %v", ErrMalformedResponse, err)
	}
	if strings.TrimSpace(out.IDToken) == "" || strings.TrimSpace(out.UserID) == "" {
		return SignInResult{}, fmt.Errorf("%w: securetoken response is missing id_token or user_id", ErrMalformedResponse)
	}
	if strings.TrimSpace(out.RefreshToken) == "" {
		out.RefreshToken = refreshToken
	}
	return SignInResult{
		UID:          strings.TrimSpace(out.UserID),
		IDToken:      strings.TrimSpace(out.IDToken),
		RefreshToken: strings.TrimSpace(out.RefreshToken),
	}, nil
}
