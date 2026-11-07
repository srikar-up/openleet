package runner

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var versionRegex = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

// Detector discovers available host language runtimes.
type Detector struct {
	mu         sync.RWMutex
	candidates map[string][]string
}

// NewDetector returns a platform-aware language runtime detector.
func NewDetector() *Detector {
	return &Detector{
		candidates: map[string][]string{
			"python": pythonCandidates(),
			"go":     {"go"},
		},
	}
}

func pythonCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "python3", "py"}
	}
	return []string{"python3", "python"}
}

// SetExecutable prepends a user-configured executable for one language. A
// blank path is ignored so the platform discovery list remains the fallback.
func (d *Detector) SetExecutable(language, path string) {
	if d == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.candidates == nil {
		d.candidates = make(map[string][]string)
	}
	existing := d.candidates[lang]
	next := make([]string, 0, len(existing)+1)
	seen := make(map[string]bool, len(existing)+1)
	for _, candidate := range append([]string{path}, existing...) {
		key := strings.ToLower(candidate)
		if candidate == "" || seen[key] {
			continue
		}
		seen[key] = true
		next = append(next, candidate)
	}
	d.candidates[lang] = next
}

// DetectAll probes all registered languages and returns their descriptors.
func (d *Detector) DetectAll() []Runtime {
	d.mu.RLock()
	languages := make([]string, 0, len(d.candidates))
	for lang := range d.candidates {
		languages = append(languages, lang)
	}
	d.mu.RUnlock()

	results := make([]Runtime, 0, len(languages))
	for _, lang := range languages {
		results = append(results, d.Detect(lang))
	}
	return results
}

// Detect probes a specific language and returns its runtime information.
func (d *Detector) Detect(language string) Runtime {
	langLower := strings.ToLower(language)
	rt := Runtime{Language: langLower}
	if d == nil {
		return rt
	}

	d.mu.RLock()
	names, ok := d.candidates[langLower]
	d.mu.RUnlock()
	if !ok {
		return rt
	}

	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}

		rt.Available = true
		rt.ExecPath = path
		rt.RawVersion = fetchVersion(path, langLower)
		rt.Version = parseVersion(rt.RawVersion)
		return rt
	}

	return rt
}

// IsAvailable returns true if the specified runtime is available in PATH.
func (d *Detector) IsAvailable(language string) bool {
	return d.Detect(language).Available
}

// DetectPython is a convenience shortcut for Python runtime detection.
func (d *Detector) DetectPython() Runtime {
	return d.Detect("python")
}

// DetectGo is a convenience shortcut for Go runtime detection.
func (d *Detector) DetectGo() Runtime {
	return d.Detect("go")
}

func fetchVersion(execPath, language string) string {
	args := []string{"--version"}
	if language == "go" {
		args = []string{"version"}
	}

	cmd := exec.Command(execPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseVersion(raw string) string {
	if raw == "" {
		return ""
	}
	match := versionRegex.FindString(raw)
	if match == "" {
		return raw
	}
	return match
}

// FormatSummary returns a formatted overview of runtime availability.
func (d *Detector) FormatSummary() string {
	var b strings.Builder
	for _, rt := range d.DetectAll() {
		status := "✗ not found"
		if rt.Available {
			status = fmt.Sprintf("✓ %s (%s)", rt.ExecPath, rt.Version)
		}
		fmt.Fprintf(&b, "  %-8s %s\n", rt.Language, status)
	}
	return b.String()
}
