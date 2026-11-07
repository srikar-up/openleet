package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testProjectID = "test-proj"
	testUID       = "user123"
	testIDToken   = "test-id-token"
)

func newTestFirestore(t *testing.T, handler http.HandlerFunc) *FirestoreService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	svc, err := NewFirestoreService(testProjectID, WithFirestoreBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create FirestoreService: %v", err)
	}
	return svc
}

func TestFirestorePullProfile(t *testing.T) {
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/projects/%s/databases/(default)/documents/users/%s", testProjectID, testUID)
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer "+testIDToken {
			t.Errorf("expected bearer ID token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"fields": {
				"uid": {"stringValue": "user123"},
				"user": {"stringValue": "gopher"},
				"score": {"integerValue": "175"},
				"updated": {"integerValue": "1700000000"},
				"solved": {
					"mapValue": {
						"fields": {
							"1": {"integerValue": "95"},
							"17": {"integerValue": "80"}
						}
					}
				}
			}
		}`))
	})

	data, err := svc.PullProfile(context.Background(), testIDToken, testUID)
	if err != nil {
		t.Fatalf("unexpected PullProfile error: %v", err)
	}
	if data.UID != testUID || data.Username != "gopher" || data.GlobalScore != 175 || data.LastActive != 1700000000 {
		t.Fatalf("unexpected profile: %+v", data)
	}
	if data.SolvedMap["1"] != 95 || data.SolvedMap["17"] != 80 {
		t.Fatalf("unexpected solved map: %+v", data.SolvedMap)
	}
}

func TestFirestorePullNotFound(t *testing.T) {
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Document not found."}}`))
	})

	_, err := svc.PullProfile(context.Background(), testIDToken, "missing-user")
	if !errors.Is(err, ErrRemoteProfileNotFound) {
		t.Fatalf("expected ErrRemoteProfileNotFound, got: %v", err)
	}
}

func TestFirestorePullMalformed(t *testing.T) {
	cases := []struct {
		name         string
		responseJSON string
	}{
		{
			name:         "score is a string",
			responseJSON: `{"fields":{"uid":{"stringValue":"user123"},"score":{"stringValue":"100"}}}`,
		},
		{
			name:         "score is not numeric",
			responseJSON: `{"fields":{"uid":{"stringValue":"user123"},"score":{"integerValue":"abc"}}}`,
		},
		{
			name:         "uid differs from path",
			responseJSON: `{"fields":{"uid":{"stringValue":"other-user"}}}`,
		},
		{
			name:         "solved value is not numeric",
			responseJSON: `{"fields":{"uid":{"stringValue":"user123"},"solved":{"mapValue":{"fields":{"1":{"integerValue":"abc"}}}}}}`,
		},
		{
			name:         "negative score",
			responseJSON: `{"fields":{"uid":{"stringValue":"user123"},"score":{"integerValue":"-5"}}}`,
		},
		{
			name:         "negative solved value",
			responseJSON: `{"fields":{"uid":{"stringValue":"user123"},"solved":{"mapValue":{"fields":{"1":{"integerValue":"-1"}}}}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.responseJSON))
			})
			_, err := svc.PullProfile(context.Background(), testIDToken, testUID)
			if !errors.Is(err, ErrMalformedResponse) {
				t.Fatalf("expected ErrMalformedResponse, got: %v", err)
			}
		})
	}
}

func TestFirestorePullMissingFields(t *testing.T) {
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":{}}`))
	})

	data, err := svc.PullProfile(context.Background(), testIDToken, testUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.UID != testUID || data.Username != "" || data.GlobalScore != 0 || data.LastActive != 0 {
		t.Fatalf("unexpected zero profile: %+v", data)
	}
	if data.SolvedMap == nil {
		t.Fatal("expected non-nil SolvedMap")
	}
}

func TestFirestoreAuthFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		sentinel   error
	}{
		{
			name:       "expired id token",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"code":401,"message":"Missing or invalid authentication token."}}`,
			sentinel:   ErrUnauthorized,
		},
		{
			name:       "rules deny access",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"code":403,"message":"Missing or insufficient permissions."}}`,
			sentinel:   ErrPermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			})

			_, err := svc.PullProfile(context.Background(), testIDToken, testUID)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("expected %v, got: %v", tc.sentinel, err)
			}
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("expected the underlying *HTTPError to remain inspectable, got %T", err)
			}
			if httpErr.StatusCode != tc.statusCode {
				t.Fatalf("expected status %d, got %d", tc.statusCode, httpErr.StatusCode)
			}
			if !strings.Contains(httpErr.Error(), "permission") && !strings.Contains(httpErr.Error(), "authentication") {
				t.Fatalf("expected a descriptive remote message, got %q", httpErr.Error())
			}
		})
	}
}

func TestFirestorePushAuthFailure(t *testing.T) {
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"Missing or invalid authentication token."}}`))
	})

	err := svc.PushProfile(context.Background(), testIDToken, &FirestoreUserData{UID: testUID})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestFirestoreRequiresIDToken(t *testing.T) {
	var calls int64
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
	})

	for _, token := range []string{"", "   ", "line\nbreak"} {
		_, err := svc.PullProfile(context.Background(), token, testUID)
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("token %q: expected ErrUnauthorized, got %v", token, err)
		}
		if err := svc.PushProfile(context.Background(), token, &FirestoreUserData{UID: testUID}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("token %q: expected PushProfile ErrUnauthorized, got %v", token, err)
		}
	}

	if count := atomic.LoadInt64(&calls); count != 0 {
		t.Fatalf("expected no HTTP calls for invalid tokens, got %d", count)
	}
}

func TestFirestorePushProfile(t *testing.T) {
	type capturedRequest struct {
		method   string
		path     string
		rawQuery string
		auth     string
		body     firestoreDocumentBody
	}

	var (
		mu       sync.Mutex
		requests []capturedRequest
	)

	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		var body firestoreDocumentBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		mu.Lock()
		requests = append(requests, capturedRequest{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			auth:     r.Header.Get("Authorization"),
			body:     body,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	data := &FirestoreUserData{
		UID:         testUID,
		Username:    "alice",
		GlobalScore: 100,
		SolvedMap: map[string]int{
			"1":  95,
			"17": 80,
		},
	}

	beforePush := time.Now().Unix()
	if err := svc.PushProfile(context.Background(), testIDToken, data); err != nil {
		t.Fatalf("first PushProfile failed: %v", err)
	}

	// A smaller map must replace, not merge, the solved map.
	data.SolvedMap = map[string]int{"42": 50}
	if err := svc.PushProfile(context.Background(), testIDToken, data); err != nil {
		t.Fatalf("second PushProfile failed: %v", err)
	}

	mu.Lock()
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	req1, req2 := requests[0], requests[1]
	mu.Unlock()

	for i, req := range []capturedRequest{req1, req2} {
		if req.method != http.MethodPatch {
			t.Errorf("request %d: expected PATCH, got %s", i+1, req.method)
		}
		if req.auth != "Bearer "+testIDToken {
			t.Errorf("request %d: expected bearer ID token, got %q", i+1, req.auth)
		}
		expectedPath := fmt.Sprintf("/projects/%s/databases/(default)/documents/users/%s", testProjectID, testUID)
		if req.path != expectedPath {
			t.Errorf("request %d: expected path %s, got %s", i+1, expectedPath, req.path)
		}
		for _, field := range []string{"uid", "user", "score", "updated", "solved"} {
			if !strings.Contains(req.rawQuery, "updateMask.fieldPaths="+field) {
				t.Errorf("request %d: update mask missing %q: %s", i+1, field, req.rawQuery)
			}
		}
	}

	if req1.body.Fields["uid"].StringValue == nil || *req1.body.Fields["uid"].StringValue != testUID {
		t.Error("expected uid string field")
	}
	if req1.body.Fields["user"].StringValue == nil || *req1.body.Fields["user"].StringValue != "alice" {
		t.Error("expected username string field")
	}
	if req1.body.Fields["score"].IntegerValue == nil || *req1.body.Fields["score"].IntegerValue != "100" {
		t.Error("expected score encoded as a base-10 string")
	}
	if req1.body.Fields["solved"].MapValue == nil {
		t.Fatal("expected solved map field")
	}
	if got := req1.body.Fields["solved"].MapValue.Fields["1"].IntegerValue; got == nil || *got != "95" {
		t.Error("expected solved[1] encoded as 95")
	}
	if data.LastActive < beforePush {
		t.Fatalf("expected LastActive to be stamped, got %d", data.LastActive)
	}

	if req2.body.Fields["solved"].MapValue == nil {
		t.Fatal("expected second solved map field")
	}
	if _, ok := req2.body.Fields["solved"].MapValue.Fields["17"]; ok {
		t.Fatal("expected the second push to replace the solved map, not merge it")
	}
	if got := req2.body.Fields["solved"].MapValue.Fields["42"].IntegerValue; got == nil || *got != "50" {
		t.Error("expected solved[42] encoded as 50")
	}
}

func TestFirestorePushRejectsInvalidUID(t *testing.T) {
	var calls int64
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusOK)
	})

	invalidUIDs := []string{
		"",
		"  x",
		"a/b",
		".",
		"..",
		strings.Repeat("a", 1501),
	}
	for _, uid := range invalidUIDs {
		err := svc.PushProfile(context.Background(), testIDToken, &FirestoreUserData{UID: uid})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("uid %q: expected ErrInvalidInput, got %v", uid, err)
		}
	}

	if count := atomic.LoadInt64(&calls); count != 0 {
		t.Fatalf("expected no HTTP calls for invalid UIDs, got %d", count)
	}
}

func TestFirestorePushRejectsInvalidData(t *testing.T) {
	var calls int64
	svc := newTestFirestore(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name string
		data *FirestoreUserData
	}{
		{name: "nil profile"},
		{name: "negative score", data: &FirestoreUserData{UID: testUID, GlobalScore: -1}},
		{name: "empty solved key", data: &FirestoreUserData{UID: testUID, SolvedMap: map[string]int{"": 10}}},
		{name: "whitespace solved key", data: &FirestoreUserData{UID: testUID, SolvedMap: map[string]int{"   ": 10}}},
		{name: "negative solved value", data: &FirestoreUserData{UID: testUID, SolvedMap: map[string]int{"1": -10}}},
	}

	for _, tc := range cases {
		if err := svc.PushProfile(context.Background(), testIDToken, tc.data); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: expected ErrInvalidInput, got %v", tc.name, err)
		}
	}

	if count := atomic.LoadInt64(&calls); count != 0 {
		t.Fatalf("expected no HTTP calls for invalid data, got %d", count)
	}
}

func TestFirestoreTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		server.Close()
	})

	svc, err := NewFirestoreService(testProjectID, WithFirestoreBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	_, err = svc.PullProfile(ctx, testIDToken, testUID)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got: %v", err)
	}
}

func TestNewFirestoreServiceValidation(t *testing.T) {
	if _, err := NewFirestoreService("   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty project, got %v", err)
	}
	if _, err := NewFirestoreService(testProjectID, WithFirestoreBaseURL("://bad")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad base URL, got %v", err)
	}
}
