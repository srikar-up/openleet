package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetConfigDir returns the platform-specific configuration directory.
// Windows: %LOCALAPPDATA%\openleet
// POSIX: ~/.config/openleet
func GetConfigDir() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "openleet")
		}
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "AppData", "Local", "openleet")
		}
		return filepath.Join(".", ".openleet")
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "openleet")
	}

	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".config", "openleet")
	}
	return filepath.Join(".", ".openleet")
}

// GetDataDir returns the platform-specific data directory.
// Windows: %LOCALAPPDATA%\openleet
// POSIX: ~/.local/share/openleet (XDG_DATA_HOME) or fallback to ~/.config/openleet
func GetDataDir() string {
	if runtime.GOOS == "windows" {
		return GetConfigDir()
	}

	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "openleet")
	}

	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".local", "share", "openleet")
	}
	return filepath.Join(".", ".openleet", "data")
}

// GetProblemsDir returns the directory where problems are stored.
func GetProblemsDir() string {
	return filepath.Join(GetDataDir(), "problems")
}
