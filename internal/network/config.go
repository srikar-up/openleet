package network

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvProjectID is the environment variable key for the Firebase project ID.
	EnvProjectID = "FIREBASE_PROJECT_ID"
	// EnvAPIKey is the environment variable key for the public Firebase Web API Key.
	EnvAPIKey = "FIREBASE_API_KEY"
	// EnvLocalSavePath is the environment variable key for the local encrypted save file path.
	EnvLocalSavePath = "LOCAL_SAVE_FILE_PATH"
	// EnvGoogleClientID is the OAuth 2.0 client ID for Google browser sign-in.
	EnvGoogleClientID = "GOOGLE_OAUTH_CLIENT_ID"
	// EnvGoogleClientSecret is the optional OAuth client secret. Desktop client
	// secrets are not confidential; PKCE is always used.
	EnvGoogleClientSecret = "GOOGLE_OAUTH_CLIENT_SECRET"
)

// Config holds Firebase credentials and local storage settings for OpenLeet network services.
type Config struct {
	// FirebaseProjectID is the unique project identifier in Google Cloud / Firebase.
	FirebaseProjectID string
	// FirebaseAPIKey is the public client Web API Key used for Firebase Auth REST endpoints.
	FirebaseAPIKey string
	// LocalSaveFilePath specifies the file path where encrypted local profile data is stored.
	// This path is consumed by the encrypted storage layer, not by this package.
	LocalSaveFilePath string
	// GoogleClientID is the public OAuth client ID used for browser sign-in.
	GoogleClientID string
	// GoogleClientSecret is optional for desktop OAuth clients. PKCE is always used.
	GoogleClientSecret string
}

// ConfigError reports errors encountered while loading or validating network configuration.
type ConfigError struct {
	Path    string
	Missing []string
	Err     error
}

// Error formats the configuration error details, including file path, missing keys, and wrapped errors.
func (e ConfigError) Error() string {
	var parts []string
	if e.Path != "" {
		parts = append(parts, fmt.Sprintf("path %s", e.Path))
	}
	if len(e.Missing) > 0 {
		parts = append(parts, fmt.Sprintf(
			"missing required keys: [%s] (copy .env.example to .env or export the variables)",
			strings.Join(e.Missing, ", "),
		))
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	if len(parts) == 0 {
		return "network config: invalid configuration"
	}
	return "network config: " + strings.Join(parts, "; ")
}

// Unwrap returns the underlying error if present.
func (e ConfigError) Unwrap() error {
	return e.Err
}

// unescapeDoubleQuoted unescapes standard escape sequences within double-quoted .env values.
func unescapeDoubleQuoted(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				sb.WriteByte('\\')
				i++
			case '"':
				sb.WriteByte('"')
				i++
			case 'n':
				sb.WriteByte('\n')
				i++
			case 'r':
				sb.WriteByte('\r')
				i++
			case 't':
				sb.WriteByte('\t')
				i++
			default:
				sb.WriteByte(s[i])
			}
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// parseEnv parses raw .env file bytes into a key-value map.
// It strips UTF-8 BOM, skips blank lines and '#' comments, supports an
// optional "export " prefix, unescapes double-quoted values (\\, \", \n, \r, \t),
// treats single-quoted values as literal, and rejects unbalanced quotes or malformed lines.
// Inline comments are NOT special: '#' characters within values are treated as literal.
func parseEnv(data []byte) (map[string]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	lines := strings.Split(string(data), "\n")
	env := make(map[string]string)

	for i, rawLine := range lines {
		lineNum := i + 1
		line := strings.TrimRight(rawLine, "\r")
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			return nil, fmt.Errorf("line %d: missing '=' delimiter", lineNum)
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}

		for j, r := range key {
			if j == 0 && (r >= '0' && r <= '9') {
				return nil, fmt.Errorf("line %d: key %q cannot start with a digit", lineNum, key)
			}
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return nil, fmt.Errorf("line %d: invalid character %q in key %q", lineNum, r, key)
			}
		}

		var parsedVal string
		if strings.HasPrefix(val, "\"") {
			if len(val) < 2 || !strings.HasSuffix(val, "\"") {
				return nil, fmt.Errorf("line %d: unbalanced double quote in value", lineNum)
			}
			parsedVal = unescapeDoubleQuoted(val[1 : len(val)-1])
		} else if strings.HasPrefix(val, "'") {
			if len(val) < 2 || !strings.HasSuffix(val, "'") {
				return nil, fmt.Errorf("line %d: unbalanced single quote in value", lineNum)
			}
			parsedVal = val[1 : len(val)-1]
		} else {
			if strings.Count(val, "\"") == 1 || strings.Count(val, "'") == 1 ||
				strings.HasSuffix(val, "\"") || strings.HasSuffix(val, "'") {
				return nil, fmt.Errorf("line %d: unbalanced quote in value", lineNum)
			}
			parsedVal = val
		}

		env[key] = parsedVal
	}

	return env, nil
}

// LoadConfig loads configuration from the specified .env file path.
// When path is empty, it defaults to ".env". Environment variables take precedence over file values.
func LoadConfig(path string) (*Config, error) {
	return loadConfig(path, true, true)
}

// LoadAuthConfig loads only the Firebase client API key needed for sign-in.
// Firestore project and local-save settings are not required for authentication.
func LoadAuthConfig(path string) (*Config, error) {
	return loadConfig(path, false, false)
}

// LoadSyncConfig loads the Firebase client API key and project ID needed for
// Firestore sync. The local-save path is managed by the TUI's encrypted store,
// so it is optional here.
func LoadSyncConfig(path string) (*Config, error) {
	return loadConfig(path, true, false)
}

func loadConfig(path string, requireProject, requireSave bool) (*Config, error) {
	if path == "" {
		path = ".env"
	}

	fileVals := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, &ConfigError{
				Path: path,
				Err:  err,
			}
		}
	} else {
		parsed, err := parseEnv(data)
		if err != nil {
			return nil, &ConfigError{
				Path: path,
				Err:  err,
			}
		}
		fileVals = parsed
	}

	getVal := func(key string) string {
		envVal := strings.TrimSpace(os.Getenv(key))
		if envVal != "" {
			return envVal
		}
		return strings.TrimSpace(fileVals[key])
	}

	cfg := &Config{
		FirebaseProjectID:  getVal(EnvProjectID),
		FirebaseAPIKey:     getVal(EnvAPIKey),
		LocalSaveFilePath:  getVal(EnvLocalSavePath),
		GoogleClientID:     getVal(EnvGoogleClientID),
		GoogleClientSecret: getVal(EnvGoogleClientSecret),
	}

	if cfg.LocalSaveFilePath != "" {
		cfg.LocalSaveFilePath = filepath.Clean(cfg.LocalSaveFilePath)
	}

	var missing []string
	if requireProject && cfg.FirebaseProjectID == "" {
		missing = append(missing, EnvProjectID)
	}
	if cfg.FirebaseAPIKey == "" {
		missing = append(missing, EnvAPIKey)
	}
	if requireSave && cfg.LocalSaveFilePath == "" {
		missing = append(missing, EnvLocalSavePath)
	}

	if len(missing) > 0 {
		return nil, &ConfigError{
			Path:    path,
			Missing: missing,
		}
	}

	return cfg, nil
}

// LoadDefaultConfig loads network configuration from ".env" in the current working directory.
func LoadDefaultConfig() (*Config, error) {
	return LoadConfig(".env")
}
