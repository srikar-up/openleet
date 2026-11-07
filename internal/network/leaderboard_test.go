package network

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
)

func TestSanitizeLeaderboardName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "collapses whitespace", in: "  neo\n\tanderson  ", want: "neo anderson"},
		{name: "drops control characters", in: "ne\x00o", want: "neo"},
		{name: "caps runes", in: strings.Repeat("a", 40), want: strings.Repeat("a", LeaderboardNameMaxRunes)},
		{name: "empty becomes anonymous", in: " \n\t ", want: "Anonymous"},
		{name: "keeps unicode", in: "李雷", want: "李雷"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeLeaderboardName(tc.in); got != tc.want {
				t.Fatalf("SanitizeLeaderboardName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPublishLeaderboardSendsOnlyMinimalFields(t *testing.T) {
	var (
		method  string
		path    string
		auth    string
		mask    []string
		payload struct {
			Fields map[string]struct {
				StringValue  *string `json:"stringValue"`
				IntegerValue *string `json:"integerValue"`
			} `json:"fields"`
		}
	)
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		mask = r.URL.Query()["updateMask.fieldPaths"]
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":{}}`))
	})

	entry := LeaderboardEntry{
		UID:    testUID,
		Name:   "  neo\nanderson ",
		Score:  175,
		Solved: 12,
	}
	if err := svc.PublishLeaderboard(context.Background(), testIDToken, entry); err != nil {
		t.Fatalf("PublishLeaderboard: %v", err)
	}

	if method != http.MethodPatch {
		t.Fatalf("method = %s, want PATCH", method)
	}
	wantPath := "/projects/" + testProjectID + "/databases/(default)/documents/leaderboard/" + testUID
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
	if auth != "Bearer "+testIDToken {
		t.Fatalf("authorization = %q", auth)
	}
	sort.Strings(mask)
	wantMask := []string{"name", "score", "solved", "updated"}
	if strings.Join(mask, ",") != strings.Join(wantMask, ",") {
		t.Fatalf("update mask = %v, want %v", mask, wantMask)
	}

	fields := payload.Fields
	if len(fields) != 4 {
		t.Fatalf("fields = %v, want exactly name/score/solved/updated", fields)
	}
	for _, forbidden := range []string{"uid", "email", "solvedMap", "code", "user"} {
		if _, ok := fields[forbidden]; ok {
			t.Fatalf("leaderboard body must not contain %q", forbidden)
		}
	}
	if fields["name"].StringValue == nil || *fields["name"].StringValue != "neo anderson" {
		t.Fatalf("name = %v, want sanitized neo anderson", fields["name"].StringValue)
	}
	if fields["score"].IntegerValue == nil || *fields["score"].IntegerValue != "175" {
		t.Fatalf("score = %v, want 175", fields["score"].IntegerValue)
	}
	if fields["solved"].IntegerValue == nil || *fields["solved"].IntegerValue != "12" {
		t.Fatalf("solved = %v, want 12", fields["solved"].IntegerValue)
	}
	if fields["updated"].IntegerValue == nil || *fields["updated"].IntegerValue == "0" || *fields["updated"].IntegerValue == "" {
		t.Fatalf("updated = %v, want a stamped unix timestamp", fields["updated"].IntegerValue)
	}
}

func TestPublishLeaderboardRejectsInvalidInput(t *testing.T) {
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid input")
	})
	cases := []struct {
		name  string
		token string
		entry LeaderboardEntry
		want  error
	}{
		{name: "empty token", token: "", entry: LeaderboardEntry{UID: testUID}, want: ErrUnauthorized},
		{name: "bad uid", token: testIDToken, entry: LeaderboardEntry{UID: "../x", Score: 1}, want: ErrInvalidInput},
		{name: "negative score", token: testIDToken, entry: LeaderboardEntry{UID: testUID, Score: -1}, want: ErrInvalidInput},
		{name: "negative solved", token: testIDToken, entry: LeaderboardEntry{UID: testUID, Solved: -1}, want: ErrInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.PublishLeaderboard(context.Background(), tc.token, tc.entry)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTopLeaderboardReadsAndSorts(t *testing.T) {
	var (
		path     string
		pageSize string
		orderBy  string
		auth     string
	)
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		pageSize = r.URL.Query().Get("pageSize")
		orderBy = r.URL.Query().Get("orderBy")
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"documents":[
			{"name":"projects/test-proj/databases/(default)/documents/leaderboard/uid-b",
			 "fields":{"name":{"stringValue":"beta"},"score":{"integerValue":"10"},"solved":{"integerValue":"2"},"updated":{"integerValue":"100"}}},
			{"name":"projects/test-proj/databases/(default)/documents/leaderboard/uid-c",
			 "fields":{"name":{"stringValue":"gamma"},"score":{"integerValue":"20"},"solved":{"integerValue":"1"},"updated":{"integerValue":"200"}}},
			{"name":"projects/test-proj/databases/(default)/documents/leaderboard/uid-a",
			 "fields":{"name":{"stringValue":"alpha"},"score":{"integerValue":"10"},"solved":{"integerValue":"3"},"updated":{"integerValue":"300"}}}
		]}`))
	})

	entries, err := svc.TopLeaderboard(context.Background(), testIDToken, 5)
	if err != nil {
		t.Fatalf("TopLeaderboard: %v", err)
	}
	wantPath := "/projects/" + testProjectID + "/databases/(default)/documents/leaderboard"
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
	if pageSize != "5" || orderBy != "score desc" {
		t.Fatalf("query pageSize=%q orderBy=%q", pageSize, orderBy)
	}
	if auth != "Bearer "+testIDToken {
		t.Fatalf("authorization = %q", auth)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	wantOrder := []string{"uid-c", "uid-a", "uid-b"}
	for i, want := range wantOrder {
		if entries[i].UID != want {
			t.Fatalf("entry %d = %s, want %s (order %+v)", i, entries[i].UID, want, entries)
		}
	}
	if entries[0].Name != "gamma" || entries[0].Score != 20 || entries[0].Solved != 1 || entries[0].Updated != 200 {
		t.Fatalf("first entry = %+v", entries[0])
	}
}

func TestTopLeaderboardMapsAuthErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrPermissionDenied},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":{"message":"denied"}}`))
			})
			_, err := svc.TopLeaderboard(context.Background(), testIDToken, 5)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTopLeaderboardRejectsMalformedDocuments(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "string score", body: `{"documents":[{"name":"projects/p/databases/(default)/documents/leaderboard/u","fields":{"score":{"stringValue":"10"}}}]}`},
		{name: "bad integer", body: `{"documents":[{"name":"projects/p/databases/(default)/documents/leaderboard/u","fields":{"score":{"integerValue":"abc"}}}]}`},
		{name: "negative solved", body: `{"documents":[{"name":"projects/p/databases/(default)/documents/leaderboard/u","fields":{"solved":{"integerValue":"-1"}}}]}`},
		{name: "name is not string", body: `{"documents":[{"name":"projects/p/databases/(default)/documents/leaderboard/u","fields":{"name":{"integerValue":"1"}}}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			if _, err := svc.TopLeaderboard(context.Background(), testIDToken, 5); !errors.Is(err, ErrMalformedResponse) {
				t.Fatalf("error = %v, want ErrMalformedResponse", err)
			}
		})
	}
}

func TestTopLeaderboardCapsLimit(t *testing.T) {
	var pageSize string
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		pageSize = r.URL.Query().Get("pageSize")
		_, _ = w.Write([]byte(`{"documents":[]}`))
	})
	if _, err := svc.TopLeaderboard(context.Background(), testIDToken, 500); err != nil {
		t.Fatalf("TopLeaderboard: %v", err)
	}
	if pageSize != "100" {
		t.Fatalf("pageSize = %q, want 100", pageSize)
	}
}
