package ui

import (
	"os"
	"runtime"
	"strings"
)

// detectUnicodeSupport is a best-effort terminal capability probe. Terminals do
// not expose font coverage, so the check combines the locale, TERM, and known
// Unicode-capable hosts. The appearance.icons setting (auto / on / off) can
// override the result: "on" forces glyphs, "off" keeps every label textual.
func detectUnicodeSupport() bool {
	if strings.TrimSpace(os.Getenv("OPENLEET_NO_UNICODE")) != "" {
		return false
	}
	term := strings.ToLower(strings.TrimSpace(os.Getenv("TERM")))
	if term == "dumb" {
		return false
	}
	locale := strings.ToLower(strings.Join([]string{
		os.Getenv("LC_ALL"),
		os.Getenv("LC_CTYPE"),
		os.Getenv("LANG"),
	}, " "))
	if strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8") {
		return true
	}
	if runtime.GOOS == "windows" {
		// Windows Terminal and modern conhost ship Unicode-capable fonts by
		// default; the legacy console is covered by the icons setting.
		return true
	}
	return term != ""
}
