package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	// LeaderboardCollection is the Firestore collection that holds the public
	// ranking record. Its document ID is the Firebase UID, but the UID is not
	// stored as a field inside the document.
	LeaderboardCollection = "leaderboard"
	// LeaderboardNameMaxRunes bounds the only free-text field in the ranking
	// document.
	LeaderboardNameMaxRunes = 32
	// DefaultLeaderboardLimit is the number of rows the Stats page requests.
	DefaultLeaderboardLimit = 10
	// MaxLeaderboardLimit caps a single list request.
	MaxLeaderboardLimit = 100
)

// LeaderboardEntry is the entire public ranking record. It deliberately holds
// no email, no solution code, and no per-problem solved map. Firestore stores
// one document per signed-in user at leaderboard/{uid}; UID is the document ID,
// not a body field.
type LeaderboardEntry struct {
	UID     string
	Name    string
	Score   int
	Solved  int
	Updated int64
}

// SanitizeLeaderboardName collapses whitespace, removes control characters,
// and caps the result at LeaderboardNameMaxRunes runes. An empty value becomes
// "Anonymous" so a ranking row never exposes an email address by accident.
func SanitizeLeaderboardName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	var b strings.Builder
	count := 0
	for _, r := range name {
		if unicode.IsControl(r) {
			continue
		}
		if count >= LeaderboardNameMaxRunes {
			break
		}
		b.WriteRune(r)
		count++
	}
	cleaned := strings.TrimSpace(b.String())
	if cleaned == "" {
		return "Anonymous"
	}
	return cleaned
}

func validateLeaderboardEntry(entry LeaderboardEntry) error {
	if err := validateUID(entry.UID); err != nil {
		return err
	}
	if entry.Score < 0 {
		return fmt.Errorf("%w: leaderboard score cannot be negative (%d)", ErrInvalidInput, entry.Score)
	}
	if entry.Score > 100_000_000 {
		return fmt.Errorf("%w: leaderboard score exceeds the sanity bound (%d)", ErrInvalidInput, entry.Score)
	}
	if entry.Solved < 0 {
		return fmt.Errorf("%w: leaderboard solved count cannot be negative (%d)", ErrInvalidInput, entry.Solved)
	}
	if entry.Solved > 1_000_000 {
		return fmt.Errorf("%w: leaderboard solved count exceeds the sanity bound (%d)", ErrInvalidInput, entry.Solved)
	}
	if entry.Updated < 0 {
		return fmt.Errorf("%w: leaderboard timestamp cannot be negative (%d)", ErrInvalidInput, entry.Updated)
	}
	return nil
}

func (s *FirestoreService) leaderboardCollectionURL() string {
	return fmt.Sprintf("%s/projects/%s/databases/(default)/documents/%s",
		s.opts.firestoreBaseURL,
		url.PathEscape(s.projectID),
		LeaderboardCollection,
	)
}

func (s *FirestoreService) leaderboardDocumentURL(uid string) string {
	return s.leaderboardCollectionURL() + "/" + url.PathEscape(uid)
}

func leaderboardUpdateMask() url.Values {
	mask := url.Values{}
	for _, field := range []string{"name", "score", "solved", "updated"} {
		mask.Add("updateMask.fieldPaths", field)
	}
	return mask
}

func wrapLeaderboardError(op string, err error) error {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("%s: %w: %w", op, ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%s: %w: %w", op, ErrPermissionDenied, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// documentUID extracts the final path segment of a Firestore document name.
// The list API returns names such as
// "projects/p/databases/(default)/documents/leaderboard/{uid}".
func documentUID(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndexByte(name, '/'); idx >= 0 {
		name = name[idx+1:]
	}
	if decoded, err := url.PathUnescape(name); err == nil {
		return decoded
	}
	return name
}

func leaderboardIntField(field string, value firestoreValue) (int64, error) {
	if value.IntegerValue == nil {
		return 0, fmt.Errorf("%w: leaderboard field %q has invalid type", ErrMalformedResponse, field)
	}
	parsed, err := strconv.ParseInt(*value.IntegerValue, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: leaderboard field %q has unparsable integer %q", ErrMalformedResponse, field, *value.IntegerValue)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%w: leaderboard field %q cannot be negative (%d)", ErrMalformedResponse, field, parsed)
	}
	return parsed, nil
}

// PublishLeaderboard writes the caller's single ranking document. It is a
// PATCH with an explicit update mask over the four minimal fields. The caller's
// ID token is the authority; Firestore rules require the document ID to match
// request.auth.uid.
func (s *FirestoreService) PublishLeaderboard(ctx context.Context, idToken string, entry LeaderboardEntry) error {
	if err := validateIDToken(idToken); err != nil {
		return err
	}
	entry.Name = SanitizeLeaderboardName(entry.Name)
	if err := validateLeaderboardEntry(entry); err != nil {
		return err
	}
	if entry.Updated <= 0 {
		entry.Updated = time.Now().Unix()
	}

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	name := entry.Name
	score := strconv.FormatInt(int64(entry.Score), 10)
	solved := strconv.FormatInt(int64(entry.Solved), 10)
	updated := strconv.FormatInt(entry.Updated, 10)

	body := firestoreDocumentBody{
		Fields: map[string]firestoreValue{
			"name":    {StringValue: &name},
			"score":   {IntegerValue: &score},
			"solved":  {IntegerValue: &solved},
			"updated": {IntegerValue: &updated},
		},
	}

	endpoint := s.leaderboardDocumentURL(entry.UID) + "?" + leaderboardUpdateMask().Encode()
	if err := doJSON(reqCtx, s.client, http.MethodPatch, endpoint, bearerHeaders(idToken), body, nil); err != nil {
		return wrapLeaderboardError("firestore leaderboard push", err)
	}
	return nil
}

// TopLeaderboard reads at most limit ranking documents ordered by score. The
// result is sorted locally as well so ties are deterministic even when the
// server returns an arbitrary order within one score.
func (s *FirestoreService) TopLeaderboard(ctx context.Context, idToken string, limit int) ([]LeaderboardEntry, error) {
	if err := validateIDToken(idToken); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultLeaderboardLimit
	}
	if limit > MaxLeaderboardLimit {
		limit = MaxLeaderboardLimit
	}

	reqCtx, cancel := s.opts.requestContext(ctx)
	defer cancel()

	query := url.Values{}
	query.Set("pageSize", strconv.Itoa(limit))
	query.Set("orderBy", "score desc")
	endpoint := s.leaderboardCollectionURL() + "?" + strings.ReplaceAll(query.Encode(), "+", "%20")

	var payload struct {
		Documents []firestoreDocument `json:"documents"`
	}
	if err := doJSON(reqCtx, s.client, http.MethodGet, endpoint, bearerHeaders(idToken), nil, &payload); err != nil {
		return nil, wrapLeaderboardError("firestore leaderboard list", err)
	}

	entries := make([]LeaderboardEntry, 0, len(payload.Documents))
	for _, doc := range payload.Documents {
		uid := documentUID(doc.Name)
		if uid == "" {
			continue
		}
		entry := LeaderboardEntry{UID: uid, Name: "Anonymous"}
		if value, ok := doc.Fields["name"]; ok {
			if value.StringValue == nil {
				return nil, fmt.Errorf("%w: leaderboard field 'name' has invalid type", ErrMalformedResponse)
			}
			entry.Name = SanitizeLeaderboardName(*value.StringValue)
		}
		if value, ok := doc.Fields["score"]; ok {
			parsed, err := leaderboardIntField("score", value)
			if err != nil {
				return nil, err
			}
			if int64(int(parsed)) != parsed {
				return nil, fmt.Errorf("%w: leaderboard field 'score' overflows int on this platform", ErrMalformedResponse)
			}
			entry.Score = int(parsed)
		}
		if value, ok := doc.Fields["solved"]; ok {
			parsed, err := leaderboardIntField("solved", value)
			if err != nil {
				return nil, err
			}
			if int64(int(parsed)) != parsed {
				return nil, fmt.Errorf("%w: leaderboard field 'solved' overflows int on this platform", ErrMalformedResponse)
			}
			entry.Solved = int(parsed)
		}
		if value, ok := doc.Fields["updated"]; ok {
			parsed, err := leaderboardIntField("updated", value)
			if err != nil {
				return nil, err
			}
			entry.Updated = parsed
		}
		entries = append(entries, entry)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		if entries[i].Solved != entries[j].Solved {
			return entries[i].Solved > entries[j].Solved
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].UID < entries[j].UID
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}
