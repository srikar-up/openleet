package network

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"openleet/internal/storage"
)

type memoryProfileStore struct {
	mu      sync.Mutex
	profile *storage.LocalProfile
	loadErr error
	saveErr error
}

func (m *memoryProfileStore) Load() (*storage.LocalProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.profile == nil {
		return nil, storage.ErrProfileNotFound
	}
	return m.profile, nil
}

func (m *memoryProfileStore) Save(profile *storage.LocalProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.profile = profile
	return nil
}

func newSyncTestService(t *testing.T, handler http.HandlerFunc, local LocalProfileStore) *SyncService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	fs, err := NewFirestoreService(testProjectID, WithFirestoreBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create FirestoreService: %v", err)
	}
	syncSvc, err := NewSyncService(fs, local)
	if err != nil {
		t.Fatalf("failed to create SyncService: %v", err)
	}
	return syncSvc
}

const remoteProfileDoc = `{
	"fields": {
		"uid": {"stringValue": "user123"},
		"user": {"stringValue": "remote-user"},
		"score": {"integerValue": "10"},
		"updated": {"integerValue": "200"},
		"solved": {
			"mapValue": {
				"fields": {"1": {"integerValue": "95"}}
			}
		}
	}
}`

func TestSyncLoadRemoteWinsAndCachesLocal(t *testing.T) {
	local := &memoryProfileStore{}
	svc := newSyncTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(remoteProfileDoc))
	}, local)

	out, err := svc.LoadProfile(context.Background(), testIDToken, testUID)
	if err != nil {
		t.Fatalf("unexpected LoadProfile error: %v", err)
	}
	if out.Source != ProfileSourceRemote || out.RemoteErr != nil || out.NeedsPush {
		t.Fatalf("unexpected load source: %+v", out)
	}
	if out.Profile.Username != "remote-user" || out.Profile.GlobalScore != 10 || out.Profile.Solved["1"] != 95 {
		t.Fatalf("unexpected remote profile: %+v", out.Profile)
	}
	if local.profile == nil || local.profile.Username != "remote-user" {
		t.Fatal("expected the remote profile to be cached locally")
	}
}

func TestSyncLoadFallsBackToLocal(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{name: "expired token", statusCode: http.StatusUnauthorized, wantErr: ErrUnauthorized},
		{name: "service unavailable", statusCode: http.StatusServiceUnavailable, wantErr: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			local := &memoryProfileStore{profile: &storage.LocalProfile{
				Username:    "local-user",
				GlobalScore: 42,
				LastUpdated: 100,
				Solved:      map[string]int{"2": 80},
			}}
			svc := newSyncTestService(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`{"error":{"code":500,"message":"unavailable"}}`))
			}, local)

			out, err := svc.LoadProfile(context.Background(), testIDToken, testUID)
			if err != nil {
				t.Fatalf("unexpected LoadProfile error: %v", err)
			}
			if out.Source != ProfileSourceLocal || out.RemoteErr == nil || !out.NeedsPush {
				t.Fatalf("expected a local fallback with a remote warning, got %+v", out)
			}
			if tc.wantErr != nil && !errors.Is(out.RemoteErr, tc.wantErr) {
				t.Fatalf("expected remote error %v, got %v", tc.wantErr, out.RemoteErr)
			}
			if out.Profile.Username != "local-user" || out.Profile.GlobalScore != 42 {
				t.Fatalf("unexpected local profile: %+v", out.Profile)
			}
		})
	}
}

func TestSyncLoadKeepsNewerLocalProfile(t *testing.T) {
	local := &memoryProfileStore{profile: &storage.LocalProfile{
		Username:    "newer-local",
		GlobalScore: 99,
		LastUpdated: 300,
		Solved:      map[string]int{"3": 100},
	}}
	svc := newSyncTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(remoteProfileDoc))
	}, local)

	out, err := svc.LoadProfile(context.Background(), testIDToken, testUID)
	if err != nil {
		t.Fatalf("unexpected LoadProfile error: %v", err)
	}
	if out.Source != ProfileSourceLocal || out.RemoteErr != nil || !out.NeedsPush {
		t.Fatalf("expected the newer local profile to win, got %+v", out)
	}
	if out.Profile.Username != "newer-local" {
		t.Fatalf("expected local profile, got %+v", out.Profile)
	}
	if local.profile.Username != "newer-local" {
		t.Fatal("an older remote document must not overwrite newer local progress")
	}
}

func TestSyncRemoteSnapshotPreservesTimestamp(t *testing.T) {
	store, err := storage.NewProfileStoreIn(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create local profile store: %v", err)
	}
	svc := newSyncTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(remoteProfileDoc))
	}, store)

	if _, err := svc.LoadProfile(context.Background(), testIDToken, testUID); err != nil {
		t.Fatalf("unexpected LoadProfile error: %v", err)
	}
	cached, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load cached profile: %v", err)
	}
	if cached.LastUpdated != 200 {
		t.Fatalf("expected cached LastUpdated 200 from Firestore, got %d", cached.LastUpdated)
	}
}

func TestSyncSavePersistsLocalBeforeRemote(t *testing.T) {
	local := &memoryProfileStore{}
	svc := newSyncTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"write failed"}}`))
	}, local)

	profile := &storage.LocalProfile{
		Username:    "offline-user",
		GlobalScore: 7,
		LastUpdated: 150,
		Solved:      map[string]int{"4": 70},
	}
	if err := svc.SaveProfile(context.Background(), testIDToken, testUID, profile); err == nil {
		t.Fatal("expected the remote failure to be reported")
	}
	if local.profile == nil || local.profile.Username != "offline-user" {
		t.Fatal("expected the profile to be saved locally before the remote push")
	}
}

func TestSyncSaveMapsRemoteDocument(t *testing.T) {
	local := &memoryProfileStore{}
	var captured firestoreDocumentBody
	svc := newSyncTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testIDToken {
			t.Errorf("expected bearer token, got %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}, local)

	profile := &storage.LocalProfile{
		Username:    "alice",
		GlobalScore: 12,
		Solved:      map[string]int{"7": 88},
	}
	if err := svc.SaveProfile(context.Background(), testIDToken, testUID, profile); err != nil {
		t.Fatalf("unexpected SaveProfile error: %v", err)
	}
	if captured.Fields["uid"].StringValue == nil || *captured.Fields["uid"].StringValue != testUID {
		t.Fatal("expected the remote UID to come from the authenticated user")
	}
	if captured.Fields["user"].StringValue == nil || *captured.Fields["user"].StringValue != "alice" {
		t.Fatal("expected the username to be mapped")
	}
	if got := captured.Fields["solved"].MapValue.Fields["7"].IntegerValue; got == nil || *got != "88" {
		t.Fatal("expected the solved map to be mapped")
	}
}

func TestSyncLoadLocalWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewProfileStoreIn(dir)
	if err != nil {
		t.Fatalf("failed to create local profile store: %v", err)
	}
	profile := storage.NewLocalProfile("offline")
	profile.GlobalScore = 5
	if err := store.Save(profile); err != nil {
		t.Fatalf("failed to save local profile: %v", err)
	}

	fs, err := NewFirestoreService(testProjectID, WithFirestoreBaseURL("http://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("failed to create FirestoreService: %v", err)
	}
	svc, err := NewSyncService(fs, store)
	if err != nil {
		t.Fatalf("failed to create SyncService: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	out, err := svc.LoadProfile(ctx, testIDToken, testUID)
	if err != nil {
		t.Fatalf("expected local fallback, got error: %v", err)
	}
	if out.Source != ProfileSourceLocal || out.RemoteErr == nil {
		t.Fatalf("expected local fallback, got %+v", out)
	}
	if out.Profile.Username != "offline" || out.Profile.GlobalScore != 5 {
		t.Fatalf("unexpected local profile: %+v", out.Profile)
	}
}

func TestNewSyncServiceValidation(t *testing.T) {
	if _, err := NewSyncService(nil, &memoryProfileStore{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil remote, got %v", err)
	}
	fs, err := NewFirestoreService(testProjectID)
	if err != nil {
		t.Fatalf("failed to create FirestoreService: %v", err)
	}
	if _, err := NewSyncService(fs, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil local store, got %v", err)
	}
}
