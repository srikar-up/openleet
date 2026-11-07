package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultTimeout is the default duration for outbound network requests.
const DefaultTimeout = 15 * time.Second

const maxResponseBytes = 1 << 20

type options struct {
	client           *http.Client
	timeout          time.Duration
	authBaseURL      string
	firestoreBaseURL string
	googleAuthURL    string
	googleTokenURL   string
	secureTokenURL   string
}

// Option configures network client options.
type Option func(*options)

// WithHTTPClient configures a custom http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		if client != nil {
			o.client = client
		}
	}
}

// WithTimeout configures a custom timeout. Values <= 0 are ignored.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		if timeout > 0 {
			o.timeout = timeout
		}
	}
}

// WithAuthBaseURL configures the base URL for Firebase Auth REST endpoints.
func WithAuthBaseURL(baseURL string) Option {
	return func(o *options) {
		if baseURL != "" {
			o.authBaseURL = baseURL
		}
	}
}

// WithFirestoreBaseURL configures the base URL for Firestore REST endpoints.
func WithFirestoreBaseURL(baseURL string) Option {
	return func(o *options) {
		if baseURL != "" {
			o.firestoreBaseURL = baseURL
		}
	}
}

// WithGoogleAuthURL configures the Google OAuth authorization endpoint.
func WithGoogleAuthURL(rawURL string) Option {
	return func(o *options) {
		if rawURL != "" {
			o.googleAuthURL = rawURL
		}
	}
}

// WithGoogleTokenURL configures the Google OAuth token endpoint.
func WithGoogleTokenURL(rawURL string) Option {
	return func(o *options) {
		if rawURL != "" {
			o.googleTokenURL = rawURL
		}
	}
}

// WithSecureTokenURL configures the Firebase refresh-token exchange endpoint.
func WithSecureTokenURL(rawURL string) Option {
	return func(o *options) {
		if rawURL != "" {
			o.secureTokenURL = rawURL
		}
	}
}

func defaultOptions() options {
	return options{
		timeout:          DefaultTimeout,
		authBaseURL:      "https://identitytoolkit.googleapis.com/v1",
		firestoreBaseURL: "https://firestore.googleapis.com/v1",
		googleAuthURL:    "https://accounts.google.com/o/oauth2/v2/auth",
		googleTokenURL:   "https://oauth2.googleapis.com/token",
		secureTokenURL:   "https://securetoken.googleapis.com/v1/token",
	}
}

func (o options) resolveClient() *http.Client {
	if o.client != nil {
		return o.client
	}
	return &http.Client{
		Timeout: o.timeout,
	}
}

func (o options) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, o.timeout)
}

// HTTPError represents an unsuccessful HTTP status response.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       []byte
}

// Error formats the status code and text plus a short sanitized API message
// parsed from the response body. The raw body is never printed.
func (e *HTTPError) Error() string {
	base := fmt.Sprintf("http error: status %d", e.StatusCode)
	if e.Status != "" {
		base = fmt.Sprintf("%s (%s)", base, e.Status)
	}
	if msg := e.remoteMessage(); msg != "" {
		return base + ": " + msg
	}
	return base
}

// remoteMessage extracts a short message from the standard Google API or
// OAuth2 error envelopes without exposing unrelated response body content.
func (e *HTTPError) remoteMessage() string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(e.Body, &raw); err != nil {
		return ""
	}
	errRaw, ok := raw["error"]
	if !ok {
		return ""
	}
	var apiErr struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(errRaw, &apiErr); err == nil && apiErr.Message != "" {
		return truncateRemoteMessage(apiErr.Message)
	}
	var code string
	if err := json.Unmarshal(errRaw, &code); err == nil && code != "" {
		var description string
		if descRaw, ok := raw["error_description"]; ok {
			_ = json.Unmarshal(descRaw, &description)
		}
		if description != "" {
			return truncateRemoteMessage(code + ": " + description)
		}
		return truncateRemoteMessage(code)
	}
	return ""
}

func truncateRemoteMessage(message string) string {
	const maxRunes = 300
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	return string(runes[:maxRunes]) + "..."
}

// ErrorBody returns the trimmed body string for error diagnostics.
func (e *HTTPError) ErrorBody() string {
	return strings.TrimSpace(string(e.Body))
}

// doJSON executes an HTTP request with JSON encoding/decoding and bounds checking.
func doJSON(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string, payload, out interface{}) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("%w: response body exceeds %d bytes", ErrMalformedResponse, maxResponseBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
		}
	}

	if out != nil {
		trimmed := bytes.TrimSpace(body)
		if len(trimmed) == 0 {
			return fmt.Errorf("%w: empty response body", ErrMalformedResponse)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("%w: failed to parse response JSON: %v", ErrMalformedResponse, err)
		}
	}

	return nil
}
