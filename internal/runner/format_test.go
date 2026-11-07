package runner

import (
	"context"
	"strings"
	"testing"
)

func TestFormatGoSource(t *testing.T) {
	formatted, err := FormatCode(context.Background(), "go", "package main\nfunc main(){\nx:=1\n_ = x\n}\n")
	if err != nil {
		t.Fatalf("unexpected go format error: %v", err)
	}
	if !strings.Contains(formatted, "\tx := 1") {
		t.Fatalf("expected gofmt indentation, got:\n%s", formatted)
	}
	if !strings.HasSuffix(formatted, "\n") {
		t.Fatal("expected formatted Go to end with a newline")
	}
}

func TestTidyPython(t *testing.T) {
	input := "\tdef solve():\n\t\treturn 1   \n\n\n\n"
	got := tidyPython(input)
	want := "def solve():\n    return 1\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTidyPythonDedentsFarRightCode(t *testing.T) {
	input := "        class Solution:\n            def solve(self):\n                return 1\n"
	got := tidyPython(input)
	want := "class Solution:\n    def solve(self):\n        return 1\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTidyPythonMapsUnevenIndentToFourSpaces(t *testing.T) {
	input := "def solve():\n   if True:\n       return 1\n"
	got := tidyPython(input)
	want := "def solve():\n    if True:\n        return 1\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTidyPythonCommentDoesNotInflateBlock(t *testing.T) {
	input := "def solve():\n    x = 1\n  # misaligned comment\n    return x\n"
	got := tidyPython(input)
	want := "def solve():\n    x = 1\n    # misaligned comment\n    return x\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTidyPythonNormalisesIndent(t *testing.T) {
	input := "class Solution:\n  def solve(self):\n    return 1\n"
	got := tidyPython(input)
	want := "class Solution:\n    def solve(self):\n        return 1\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTidyPythonKeepsTripleQuotedContent(t *testing.T) {
	input := "def f():\n    \"\"\"\n  keep me\n    \"\"\"\n    return 1\n"
	got := tidyPython(input)
	if !strings.Contains(got, "\n  keep me\n") {
		t.Fatalf("expected triple-quoted indentation to be preserved, got %q", got)
	}
}

func TestFormatUnsupportedLanguage(t *testing.T) {
	if _, err := FormatCode(context.Background(), "rust", "fn main() {}"); err == nil {
		t.Fatal("expected an error for an unsupported formatter")
	}
}
