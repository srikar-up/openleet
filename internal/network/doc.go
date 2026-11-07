// Package network implements OpenLeet's optional online layer: Firebase
// Authentication over the Identity Toolkit REST API and per-user profile sync
// over the Firestore REST API.
//
// Configuration is read from a local .env file by LoadConfig, with process
// environment variables taking precedence. A template lives at .env.example in
// the repository root. The package uses only the Go standard library, so it
// does not pull the gRPC Firestore client into the TUI binary.
//
// Authentication uses the public Firebase Web API key. SignUp and SignIn return
// the Firebase UID; SignIn also returns the ephemeral ID token. Passwords are
// sent only to Google over HTTPS and are never logged or stored locally. The ID
// token belongs in memory only; it expires after about an hour and must never
// be written into the local save file.
//
// Firestore uses one document per user:
//
//	users/{uid}
//
// PullProfile is a single authenticated document read at startup. PushProfile
// is a single PATCH with an explicit update mask over the five schema fields.
// There is no read-before-write, transaction, or batch overhead, which is the
// free-tier minimum for this single-document model. The solved map is fully
// replaced on every push so removed entries disappear.
//
// There is no service account and no Admin SDK path in this package. Every
// PullProfile and PushProfile call takes the signed-in user's ID token and sends
// it as Authorization: Bearer <token>. Firestore Security Rules are therefore
// the authority: deploy the rules in firestore.rules so users/{uid} is readable
// and writable only by request.auth.uid == uid. A leaked FIREBASE_API_KEY is a
// public client identifier, not an admin credential; still restrict it in the
// Google Cloud console to the Identity Toolkit API and the intended platforms.
//
// Network calls never mutate local state. The encrypted LocalProfile remains
// readable offline through internal/storage; callers should load it first and
// fall back to it whenever a network call returns an error, including
// ErrUnauthorized. SyncService packages that policy: LoadLocal is a local-only
// first paint, LoadProfile returns the local profile with RemoteErr and
// NeedsPush set when Firestore is unavailable, and SaveProfile always writes
// the encrypted local file before attempting the remote push. Neither the API
// key nor an ID token is persisted.
//
// Every method derives a request context from the caller's context. When the
// caller has no deadline, DefaultTimeout (15s) is applied, so an offline host
// cannot block the Bubble Tea loop indefinitely. Errors are typed and wrap
// sentinel values such as ErrInvalidCredentials, ErrRateLimited,
// ErrUnauthorized, ErrPermissionDenied, and ErrRemoteProfileNotFound; ErrorMsg
// is safe to return directly from a tea.Cmd.
package network
