package runner

import (
	"strings"
	"testing"
)

func TestUserCodeStartLine(t *testing.T) {
	source := "line one\nline two\n# USER SOLUTION BEGIN\ncode\n"
	if got := userCodeStartLine(source); got != 4 {
		t.Fatalf("expected user code to start at line 4, got %d", got)
	}
}

func TestRemapPythonTracebackUserFrames(t *testing.T) {
	text := `Traceback (most recent call last):
  File "/tmp/openleet-run-x/solution.py", line 20, in <module>
    _fn(**_inp)
  File "/tmp/openleet-run-x/solution.py", line 25, in twoSum
    raise ValueError("boom")
ValueError: boom
`
	// User code occupies lines 15..32 of the harness; frame lines 20 and 25 are
	// editor lines 6 and 11. A harness frame outside that range stays put.
	text += "  File \"/tmp/openleet-run-x/solution.py\", line 40, in <module>\n"
	got := remapPythonTraceback(text, 15, 18)

	for _, want := range []string{"line 6, in <module>", "line 11, in twoSum", "line 40, in <module>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in remapped traceback:\n%s", want, got)
		}
	}
	if strings.Contains(got, "line 25, in twoSum") {
		t.Fatalf("user frame was not remapped:\n%s", got)
	}
}
