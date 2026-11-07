package network

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnv(t *testing.T) {
	// Tests UTF-8 BOM, blank lines, comments, export prefix, single quotes,
	// double quotes with \n and \", duplicate key last wins, and values containing '#'.
	raw := "\xef\xbb\xbf" + `
# Leading comment followed by blank lines

export REGULAR_KEY=simple_value
export SINGLE_QUOTED='literal $value with # inside'
DOUBLE_QUOTED="first line\nsecond line with \"nested quotes\" and \\backslash"
DUPLICATE_KEY=first_value
DUPLICATE_KEY=second_value
HASH_VALUE=https://example.com/api#section

# Trailing comment
`

	parsed, err := parseEnv([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error parsing env: %v", err)
	}

	if got := parsed["REGULAR_KEY"]; got != "simple_value" {
		t.Errorf("REGULAR_KEY: expected %q, got %q", "simple_value", got)
	}

	expectedSingle := "literal $value with # inside"
	if got := parsed["SINGLE_QUOTED"]; got != expectedSingle {
		t.Errorf("SINGLE_QUOTED: expected %q, got %q", expectedSingle, got)
	}

	expectedDouble := "first line\nsecond line with \"nested quotes\" and \\backslash"
	if got := parsed["DOUBLE_QUOTED"]; got != expectedDouble {
		t.Errorf("DOUBLE_QUOTED: expected %q, got %q", expectedDouble, got)
	}

	if got := parsed["DUPLICATE_KEY"]; got != "second_value" {
		t.Errorf("DUPLICATE_KEY: expected %q, got %q", "second_value", got)
	}

	expectedHash := "https://example.com/api#section"
	if got := parsed["HASH_VALUE"]; got != expectedHash {
		t.Errorf("HASH_VALUE: expected %q, got %q", expectedHash, got)
	}
}

func TestParseEnvRejectsBadLines(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectLine string
	}{
		{
			name:       "missing equals delimiter",
			input:      "VALID=one\nMALFORMED_LINE\nOTHER=two",
			expectLine: "line 2",
		},
		{
			name:       "key starting with digit",
			input:      "1INVALID=value",
			expectLine: "line 1",
		},
		{
			name:       "key with invalid character",
			input:      "VALID=yes\nINVALID-KEY=val",
			expectLine: "line 2",
		},
		{
			name:       "unbalanced double quote",
			input:      "BAD_DOUBLE=\"unclosed string",
			expectLine: "line 1",
		},
		{
			name:       "unbalanced single quote",
			input:      "BAD_SINGLE='unclosed string",
			expectLine: "line 1",
		},
		{
			name:       "single quote in unquoted value",
			input:      "GOOD=1\nBAD=unbalanced'quote",
			expectLine: "line 2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseEnv([]byte(tc.input))
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.expectLine) {
				t.Errorf("expected error to mention %q, got %q", tc.expectLine, err.Error())
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := `
FIREBASE_PROJECT_ID=project-from-file
FIREBASE_API_KEY=key-from-file
LOCAL_SAVE_FILE_PATH=./storage/../storage/profile.enc
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	t.Setenv(EnvProjectID, "")
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvLocalSavePath, "")

	cfg, err := LoadConfig(envPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.FirebaseProjectID != "project-from-file" {
		t.Errorf("expected FirebaseProjectID %q, got %q", "project-from-file", cfg.FirebaseProjectID)
	}
	if cfg.FirebaseAPIKey != "key-from-file" {
		t.Errorf("expected FirebaseAPIKey %q, got %q", "key-from-file", cfg.FirebaseAPIKey)
	}
	expectedCleanedPath := filepath.Clean("./storage/../storage/profile.enc")
	if cfg.LocalSaveFilePath != expectedCleanedPath {
		t.Errorf("expected cleaned LocalSaveFilePath %q, got %q", expectedCleanedPath, cfg.LocalSaveFilePath)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := `
FIREBASE_PROJECT_ID=project-from-file
FIREBASE_API_KEY=key-from-file
LOCAL_SAVE_FILE_PATH=save-from-file
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	t.Setenv(EnvProjectID, "project-from-env")
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvLocalSavePath, "")

	cfg, err := LoadConfig(envPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.FirebaseProjectID != "project-from-env" {
		t.Errorf("expected env override %q, got %q", "project-from-env", cfg.FirebaseProjectID)
	}
	if cfg.FirebaseAPIKey != "key-from-file" {
		t.Errorf("expected file value %q, got %q", "key-from-file", cfg.FirebaseAPIKey)
	}
}

func TestLoadAuthConfigNeedsOnlyAPIKey(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "does_not_exist.env")

	t.Setenv(EnvProjectID, "")
	t.Setenv(EnvAPIKey, "auth-only-key")
	t.Setenv(EnvLocalSavePath, "")

	cfg, err := LoadAuthConfig(nonexistentPath)
	if err != nil {
		t.Fatalf("unexpected auth config error: %v", err)
	}
	if cfg.FirebaseAPIKey != "auth-only-key" {
		t.Fatalf("API key = %q", cfg.FirebaseAPIKey)
	}

	t.Setenv(EnvAPIKey, "")
	if _, err := LoadAuthConfig(nonexistentPath); err == nil {
		t.Fatal("expected a missing API key error")
	}
}

func TestLoadSyncConfigNeedsProjectAndAPIKey(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "does_not_exist.env")

	t.Setenv(EnvProjectID, "sync-project")
	t.Setenv(EnvAPIKey, "sync-key")
	t.Setenv(EnvLocalSavePath, "")

	cfg, err := LoadSyncConfig(nonexistentPath)
	if err != nil {
		t.Fatalf("unexpected sync config error: %v", err)
	}
	if cfg.FirebaseProjectID != "sync-project" || cfg.FirebaseAPIKey != "sync-key" {
		t.Fatalf("sync config = %#v", cfg)
	}
	t.Setenv(EnvProjectID, "")
	if _, err := LoadSyncConfig(nonexistentPath); err == nil {
		t.Fatal("expected a missing project ID error")
	}
}

func TestLoadConfigEnvOnly(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "does_not_exist.env")

	t.Setenv(EnvProjectID, "env-project-only")
	t.Setenv(EnvAPIKey, "env-api-key-only")
	t.Setenv(EnvLocalSavePath, "env-save-path-only")

	cfg, err := LoadConfig(nonexistentPath)
	if err != nil {
		t.Fatalf("unexpected error loading config solely from env: %v", err)
	}

	if cfg.FirebaseProjectID != "env-project-only" {
		t.Errorf("FirebaseProjectID mismatch: got %q", cfg.FirebaseProjectID)
	}
	if cfg.FirebaseAPIKey != "env-api-key-only" {
		t.Errorf("FirebaseAPIKey mismatch: got %q", cfg.FirebaseAPIKey)
	}
	if cfg.LocalSaveFilePath != "env-save-path-only" {
		t.Errorf("LocalSaveFilePath mismatch: got %q", cfg.LocalSaveFilePath)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "nonexistent.env")

	t.Setenv(EnvProjectID, "")
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvLocalSavePath, "")

	cfg, err := LoadConfig(nonexistentPath)
	if cfg != nil {
		t.Fatalf("expected nil config, got %+v", cfg)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *ConfigError via errors.As, got %T: %v", err, err)
	}

	if cfgErr.Path != nonexistentPath {
		t.Errorf("expected ConfigError.Path %q, got %q", nonexistentPath, cfgErr.Path)
	}

	expectedMissing := map[string]bool{
		EnvProjectID:     true,
		EnvAPIKey:        true,
		EnvLocalSavePath: true,
	}

	if len(cfgErr.Missing) != len(expectedMissing) {
		t.Errorf("expected %d missing keys, got %d: %v", len(expectedMissing), len(cfgErr.Missing), cfgErr.Missing)
	}
	for _, k := range cfgErr.Missing {
		if !expectedMissing[k] {
			t.Errorf("unexpected missing key %q", k)
		}
	}

	errMsg := cfgErr.Error()
	if !strings.Contains(errMsg, "network config") {
		t.Errorf("expected error message to contain %q, got %q", "network config", errMsg)
	}
}

func TestLoadGoogleOAuthConfig(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := `
FIREBASE_API_KEY=key-from-file
GOOGLE_OAUTH_CLIENT_ID=client-from-file
GOOGLE_OAUTH_CLIENT_SECRET=secret-from-file
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvGoogleClientID, "")
	t.Setenv(EnvGoogleClientSecret, "")

	cfg, err := LoadAuthConfig(envPath)
	if err != nil {
		t.Fatalf("LoadAuthConfig: %v", err)
	}
	if cfg.GoogleClientID != "client-from-file" || cfg.GoogleClientSecret != "secret-from-file" {
		t.Fatalf("google oauth config = %+v", cfg)
	}
}
