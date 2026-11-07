package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.enc")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store, path
}

func TestStoreRoundTripAndMasking(t *testing.T) {
	store, path := newTestStore(t)
	const key = "sk-ant-api03-super-secret-a3f9"
	if err := store.Set("Anthropic", key); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Get("anthropic")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != key {
		t.Fatalf("Get = %q, want original key", got)
	}
	if !store.Has("anthropic") {
		t.Fatal("Has = false after Set")
	}
	if masked := store.Masked("anthropic"); masked != "sk-••••••••a3f9" {
		t.Fatalf("Masked = %q", masked)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret") || strings.Contains(string(raw), key) {
		t.Fatal("vault file contains plaintext")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Fatalf("vault mode = %o, want 0600", perm)
		}
	}

	if err := store.Delete("anthropic"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Has("anthropic") {
		t.Fatal("Has = true after Delete")
	}
	if _, err := store.Get("anthropic"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get after delete = %v, want ErrSecretNotFound", err)
	}
}

func TestMaskKeyNeverLeaksMiddle(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"short":               "••••••••",
		"12345678":            "••••••••",
		"sk-1234567890abcdef": "sk-••••••••cdef",
		"xai-1234567890a3f9":  "••••••••a3f9",
	}
	for input, want := range cases {
		if got := MaskKey(input); got != want {
			t.Errorf("MaskKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTamperedVaultIsRejected(t *testing.T) {
	store, path := newTestStore(t)
	if err := store.Set("openai", "sk-1234567890a3f9"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Get("openai"); !errors.Is(err, ErrVaultTampered) {
		t.Fatalf("Get tampered = %v, want ErrVaultTampered", err)
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Set("openai", "   "); err == nil {
		t.Fatal("expected empty key to be rejected")
	}
	if err := store.Set("", "sk-1234"); err == nil {
		t.Fatal("expected empty provider to be rejected")
	}
}
