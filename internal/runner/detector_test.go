package runner

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDetectorGo(t *testing.T) {
	d := NewDetector()
	rt := d.DetectGo()
	if !rt.Available {
		t.Fatal("expected Go to be available in test environment")
	}
	if rt.Version == "" {
		t.Fatal("expected non-empty Go version string")
	}
	if !d.IsAvailable("go") {
		t.Fatal("expected IsAvailable('go') to be true")
	}
}

func TestDetectorPython(t *testing.T) {
	d := NewDetector()
	rt := d.DetectPython()
	if !rt.Available {
		t.Skip("python not available in environment, skipping")
	}
	if rt.Version == "" {
		t.Fatal("expected non-empty Python version string")
	}
	if !strings.HasPrefix(rt.Version, "3.") && !strings.HasPrefix(rt.Version, "2.") {
		t.Fatalf("unexpected python version format: %s", rt.Version)
	}
}

func TestDetectorFormatSummary(t *testing.T) {
	d := NewDetector()
	summary := d.FormatSummary()
	if !strings.Contains(summary, "go") {
		t.Fatal("expected summary to contain 'go'")
	}
}

func TestDetectorSetExecutableOverridesCandidate(t *testing.T) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not installed on this host")
	}
	d := NewDetector()
	d.SetExecutable("go", "  "+goPath+"  ")

	d.mu.RLock()
	first := d.candidates["go"][0]
	d.mu.RUnlock()
	if first != goPath {
		t.Fatalf("first go candidate = %q, want %q", first, goPath)
	}

	rt := d.Detect("go")
	if !rt.Available {
		t.Fatal("expected the override executable to be detected")
	}
	if rt.ExecPath != goPath {
		t.Fatalf("ExecPath = %q, want the override %q", rt.ExecPath, goPath)
	}
}

func TestDetectorSetExecutableIgnoresBlankPath(t *testing.T) {
	d := NewDetector()
	before := append([]string(nil), d.candidates["python"]...)
	d.SetExecutable("python", "   ")
	after := d.candidates["python"]
	if len(after) != len(before) {
		t.Fatalf("blank path changed candidates: %v -> %v", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("blank path reordered candidates: %v -> %v", before, after)
		}
	}
}
