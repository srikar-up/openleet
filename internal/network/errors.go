package network

import (
	"context"
	"errors"
	"fmt"
	"net"
)

var (
	// ErrInvalidInput is returned when an input argument is missing or invalid.
	ErrInvalidInput = errors.New("invalid input")
	// ErrMalformedResponse is returned when a remote server response cannot be parsed or violates schema expectations.
	ErrMalformedResponse = errors.New("malformed remote response")
	// ErrRemoteProfileNotFound is returned when a remote profile does not exist for the user.
	ErrRemoteProfileNotFound = errors.New("remote profile not found")
	// ErrUnauthorized is returned when the Firebase ID token is missing, expired,
	// or rejected by Firestore. The UI should ask the user to sign in again.
	ErrUnauthorized = errors.New("firebase authentication required")
	// ErrSessionExpired is returned when a stored Firebase refresh token is
	// expired, revoked, or belongs to a disabled user. The UI should clear the
	// saved session and ask for a fresh sign-in.
	ErrSessionExpired = errors.New("firebase session expired")
	// ErrPermissionDenied is returned when Firestore rules reject an otherwise
	// authenticated request. This is a deployment/rules problem, not a login one.
	ErrPermissionDenied = errors.New("firestore permission denied")
)

// ErrorMsg encapsulates an operation and error for propagation through
// Bubble Tea pipelines without importing bubbletea.
type ErrorMsg struct {
	Op  string
	Err error
}

// Error formats the operation and underlying error message.
// It is safe to call when Err is nil.
func (e ErrorMsg) Error() string {
	if e.Err == nil {
		return e.Op
	}
	if e.Op != "" {
		return fmt.Sprintf("%s: %s", e.Op, e.Err.Error())
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e ErrorMsg) Unwrap() error {
	return e.Err
}

// IsNetworkError reports whether err represents a transient network transport failure,
// timeout, or cancellation. It serves as a UI hint for offline/retry handling.
// It returns false when err is nil.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}
