package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// FirestoreUserData represents user profile and problem-solving progress in Firestore.
type FirestoreUserData struct {
	UID         string         `firestore:"uid"`
	Username    string         `firestore:"user"`
	GlobalScore int            `firestore:"score"`
	LastActive  int64          `firestore:"updated"`
	SolvedMap   map[string]int `firestore:"solved"`
}

// FirestoreService performs direct Firestore REST calls with the signed-in
// user's Firebase ID token. It never holds a service account key and therefore
// cannot bypass Firestore Security Rules: the database decides whether the
// caller may read or write users/{uid}.
type FirestoreService struct {
	projectID string
	client    *http.Client
	opts      options
}

// NewFirestoreService initializes a zero-trust Firestore client. The
// constructor performs no network I/O and reads no credential files; callers
// pass a fresh ID token to each PullProfile or PushProfile call.
func NewFirestoreService(projectID string, opts ...Option) (*FirestoreService, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project ID cannot be empty", ErrInvalidInput)
	}

	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	o.firestoreBaseURL = strings.TrimRight(o.firestoreBaseURL, "/")
	u, err := url.Parse(o.firestoreBaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: invalid firestore base URL %q", ErrInvalidInput, o.firestoreBaseURL)
	}

	return &FirestoreService{
		projectID: projectID,
		client:    o.resolveClient(),
		opts:      o,
	}, nil
}

// validateIDToken rejects empty or header-injection-shaped bearer values. The
// token itself is never logged or written to disk.
func validateIDToken(idToken string) error {
	if strings.TrimSpace(idToken) == "" {
		return fmt.Errorf("%w: Firebase ID token is empty", ErrUnauthorized)
	}
	if strings.ContainsAny(idToken, "\r\n") {
		return fmt.Errorf("%w: Firebase ID token contains invalid characters", ErrUnauthorized)
	}
	return nil
}

func bearerHeaders(idToken string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(idToken),
	}
}

// wrapFirestoreError maps authentication and authorization failures to
// sentinels the TUI can use to prompt for re-authentication or show a rules
// error, while retaining the underlying HTTPError for diagnostics.
func wrapFirestoreError(op, uid string, err error) error {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("%s users/%s: %w: %w", op, uid, ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%s users/%s: %w: %w", op, uid, ErrPermissionDenied, err)
		}
	}
	return fmt.Errorf("%s users/%s: %w", op, uid, err)
}

func validateUID(uid string) error {
	if uid == "" {
		return fmt.Errorf("%w: uid cannot be empty", ErrInvalidInput)
	}
	if strings.TrimSpace(uid) == "" {
		return fmt.Errorf("%w: uid cannot be only whitespace", ErrInvalidInput)
	}
	if uid != strings.TrimSpace(uid) {
		return fmt.Errorf("%w: uid cannot have leading or trailing whitespace", ErrInvalidInput)
	}
	if uid == "." || uid == ".." {
		return fmt.Errorf("%w: uid %q is reserved", ErrInvalidInput, uid)
	}
	if strings.Contains(uid, "/") {
		return fmt.Errorf("%w: uid %q cannot contain '/'", ErrInvalidInput, uid)
	}
	if len(uid) > 1500 {
		return fmt.Errorf("%w: uid exceeds maximum length of 1500 bytes", ErrInvalidInput)
	}
	return nil
}

func (s *FirestoreService) documentURL(uid string) string {
	return fmt.Sprintf("%s/projects/%s/databases/(default)/documents/users/%s",
		s.opts.firestoreBaseURL,
		url.PathEscape(s.projectID),
		url.PathEscape(uid),
	)
}

// profileUpdateMask lists every schema field so a PATCH replaces the whole
// profile. In particular, replacing the solved map drops entries that are no
// longer present. Firestore charges one document write regardless of the mask.
func profileUpdateMask() url.Values {
	mask := url.Values{}
	for _, field := range []string{"uid", "user", "score", "updated", "solved"} {
		mask.Add("updateMask.fieldPaths", field)
	}
	return mask
}

type firestoreDocument struct {
	Name   string                    `json:"name"`
	Fields map[string]firestoreValue `json:"fields"`
}

type firestoreDocumentBody struct {
	Fields map[string]firestoreValue `json:"fields"`
}

type firestoreValue struct {
	StringValue  *string            `json:"stringValue,omitempty"`
	IntegerValue *string            `json:"integerValue,omitempty"`
	MapValue     *firestoreMapValue `json:"mapValue,omitempty"`
}

type firestoreMapValue struct {
	Fields map[string]firestoreValue `json:"fields"`
}

// PullProfile fetches one user document with the caller's Firebase ID token.
// The token is ephemeral; callers should pass a freshly signed-in token and
// re-authenticate when ErrUnauthorized is returned.
func (s *FirestoreService) PullProfile(ctx context.Context, idToken string, uid string) (*FirestoreUserData, error) {
	if err := validateIDToken(idToken); err != nil {
		return nil, err
	}
	if err := validateUID(uid); err != nil {
		return nil, err
	}

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	endpoint := s.documentURL(uid)
	headers := bearerHeaders(idToken)

	var doc firestoreDocument
	if err := doJSON(reqCtx, s.client, http.MethodGet, endpoint, headers, nil, &doc); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: users/%s", ErrRemoteProfileNotFound, uid)
		}
		return nil, wrapFirestoreError("firestore pull", uid, err)
	}

	data := &FirestoreUserData{
		UID:       uid,
		SolvedMap: make(map[string]int),
	}

	if doc.Fields != nil {
		if v, ok := doc.Fields["uid"]; ok {
			if v.StringValue == nil {
				return nil, fmt.Errorf("%w: field 'uid' has invalid type", ErrMalformedResponse)
			}
			if *v.StringValue != "" {
				if *v.StringValue != uid {
					return nil, fmt.Errorf("%w: document uid %q differs from requested uid %q", ErrMalformedResponse, *v.StringValue, uid)
				}
				data.UID = *v.StringValue
			}
		}

		if v, ok := doc.Fields["user"]; ok {
			if v.StringValue == nil {
				return nil, fmt.Errorf("%w: field 'user' has invalid type", ErrMalformedResponse)
			}
			data.Username = *v.StringValue
		}

		if v, ok := doc.Fields["score"]; ok {
			if v.IntegerValue == nil {
				return nil, fmt.Errorf("%w: field 'score' has invalid type", ErrMalformedResponse)
			}
			parsed, err := strconv.ParseInt(*v.IntegerValue, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: field 'score' has unparsable integer %q: %v", ErrMalformedResponse, *v.IntegerValue, err)
			}
			if parsed < 0 {
				return nil, fmt.Errorf("%w: field 'score' cannot be negative (%d)", ErrMalformedResponse, parsed)
			}
			if int64(int(parsed)) != parsed {
				return nil, fmt.Errorf("%w: field 'score' overflows int on this platform", ErrMalformedResponse)
			}
			data.GlobalScore = int(parsed)
		}

		if v, ok := doc.Fields["updated"]; ok {
			if v.IntegerValue == nil {
				return nil, fmt.Errorf("%w: field 'updated' has invalid type", ErrMalformedResponse)
			}
			parsed, err := strconv.ParseInt(*v.IntegerValue, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: field 'updated' has unparsable integer %q: %v", ErrMalformedResponse, *v.IntegerValue, err)
			}
			data.LastActive = parsed
		}

		if v, ok := doc.Fields["solved"]; ok {
			if v.MapValue == nil {
				return nil, fmt.Errorf("%w: field 'solved' has invalid type", ErrMalformedResponse)
			}
			if v.MapValue.Fields != nil {
				for k, item := range v.MapValue.Fields {
					if item.IntegerValue == nil {
						return nil, fmt.Errorf("%w: field 'solved.%s' has invalid type", ErrMalformedResponse, k)
					}
					parsed, err := strconv.ParseInt(*item.IntegerValue, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("%w: field 'solved.%s' has unparsable integer %q: %v", ErrMalformedResponse, k, *item.IntegerValue, err)
					}
					if parsed < 0 {
						return nil, fmt.Errorf("%w: field 'solved.%s' cannot be negative (%d)", ErrMalformedResponse, k, parsed)
					}
					if int64(int(parsed)) != parsed {
						return nil, fmt.Errorf("%w: field 'solved.%s' overflows int on this platform", ErrMalformedResponse, k)
					}
					data.SolvedMap[k] = int(parsed)
				}
			}
		}
	}

	return data, nil
}

// PushProfile writes the user's profile document to Firestore using a PATCH request.
// This is exactly one Firestore document write, no read-before-write and no transaction,
// which is the free-tier minimum for this single-document model.
// An explicit updateMask lists every field, so deleted solved entries disappear from
// remote storage instead of being merged back in.
func (s *FirestoreService) PushProfile(ctx context.Context, idToken string, data *FirestoreUserData) error {
	if err := validateIDToken(idToken); err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("%w: profile data cannot be nil", ErrInvalidInput)
	}
	if err := validateUID(data.UID); err != nil {
		return err
	}
	if data.GlobalScore < 0 {
		return fmt.Errorf("%w: global score cannot be negative (%d)", ErrInvalidInput, data.GlobalScore)
	}
	if data.SolvedMap == nil {
		data.SolvedMap = make(map[string]int)
	}
	for k, v := range data.SolvedMap {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("%w: solved map key cannot be empty", ErrInvalidInput)
		}
		if v < 0 {
			return fmt.Errorf("%w: solved map value for key %q cannot be negative (%d)", ErrInvalidInput, k, v)
		}
	}

	data.LastActive = time.Now().Unix()

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	uidVal := data.UID
	userVal := data.Username
	scoreStr := strconv.FormatInt(int64(data.GlobalScore), 10)
	updatedStr := strconv.FormatInt(data.LastActive, 10)

	solvedFields := make(map[string]firestoreValue, len(data.SolvedMap))
	for k, v := range data.SolvedMap {
		vStr := strconv.FormatInt(int64(v), 10)
		solvedFields[k] = firestoreValue{
			IntegerValue: &vStr,
		}
	}

	body := firestoreDocumentBody{
		Fields: map[string]firestoreValue{
			"uid":     {StringValue: &uidVal},
			"user":    {StringValue: &userVal},
			"score":   {IntegerValue: &scoreStr},
			"updated": {IntegerValue: &updatedStr},
			"solved":  {MapValue: &firestoreMapValue{Fields: solvedFields}},
		},
	}

	endpoint := s.documentURL(data.UID) + "?" + profileUpdateMask().Encode()
	headers := bearerHeaders(idToken)

	if err := doJSON(reqCtx, s.client, http.MethodPatch, endpoint, headers, body, nil); err != nil {
		return wrapFirestoreError("firestore push", data.UID, err)
	}

	return nil
}
