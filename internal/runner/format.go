package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	goformat "go/format"
	"os/exec"
	"strings"
	"time"
)

const formatterTimeout = 5 * time.Second

// FormatCode reformats a solution buffer. Go is formatted in-process with
// go/format. Python uses ruff or black when one is installed and falls back to
// a conservative whitespace cleanup so the action always produces a usable
// buffer.
func FormatCode(ctx context.Context, language, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return code, nil
	}

	switch strings.ToLower(strings.TrimSpace(language)) {
	case "python", "py":
		if formatted, ok := formatPython(ctx, code); ok {
			return formatted, nil
		}
		return tidyPython(code), nil
	case "go", "golang":
		formatted, err := goformat.Source([]byte(code))
		if err != nil {
			return "", fmt.Errorf("gofmt: %w", err)
		}
		return string(formatted), nil
	default:
		return "", fmt.Errorf("formatting is not supported for %s", language)
	}
}

type formatterCandidate struct {
	name string
	args []string
}

// formatPython tries the common standalone and module formatters in order of
// speed. It returns ok=false when none is installed.
func formatPython(ctx context.Context, code string) (string, bool) {
	candidates := []formatterCandidate{
		{name: "ruff", args: []string{"format", "--stdin-filename", "solution.py", "-"}},
		{name: "black", args: []string{"-q", "-"}},
	}
	for _, python := range []string{"python", "python3"} {
		candidates = append(candidates,
			formatterCandidate{name: python, args: []string{"-m", "black", "-q", "-"}},
			formatterCandidate{name: python, args: []string{"-m", "ruff", "format", "--stdin-filename", "solution.py", "-"}},
		)
	}
	candidates = append(candidates,
		formatterCandidate{name: "py", args: []string{"-3", "-m", "black", "-q", "-"}},
		formatterCandidate{name: "py", args: []string{"-3", "-m", "ruff", "format", "--stdin-filename", "solution.py", "-"}},
	)

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		formatted, err := runFormatter(ctx, path, candidate.args, code)
		if err == nil {
			return formatted, true
		}
	}
	return "", false
}

func runFormatter(ctx context.Context, path string, args []string, code string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, formatterTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, path, args...)
	cmd.Stdin = strings.NewReader(code)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return "", runCtx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}

	formatted := stdout.String()
	if strings.TrimSpace(formatted) == "" {
		return "", errors.New("formatter returned an empty buffer")
	}
	return formatted, nil
}

// tidyPython applies the safe subset of formatting that needs no external
// tool. It removes a shared left margin, normalises block indentation to four
// spaces per level, and leaves triple-quoted string bodies alone so docstrings
// and embedded text keep their original alignment.
func tidyPython(code string) string {
	code = strings.ReplaceAll(code, "\r\n", "\n")
	code = strings.ReplaceAll(code, "\r", "\n")
	code = strings.ReplaceAll(code, "\t", "    ")

	lines := strings.Split(code, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}

	// Remove a shared left margin first. Code pasted from a block that starts
	// far to the right often has every line shifted by the same number of
	// spaces; treating that margin as a real indent level would preserve the
	// problem instead of beautifying it.
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		width := leadingSpaces(line)
		if minIndent < 0 || width < minIndent {
			minIndent = width
		}
	}
	if minIndent > 0 {
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if leadingSpaces(line) >= minIndent {
				lines[i] = line[minIndent:]
			} else {
				lines[i] = strings.TrimLeft(line, " ")
			}
		}
	}

	stack := []int{0}
	state := pythonStringState{}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}

		startsInString := state.triple != ""
		state = scanPythonLine(line, state)
		if startsInString {
			out = append(out, line)
			continue
		}

		raw := leadingSpaces(line)
		content := line[raw:]
		level, nextStack := pythonIndentLevel(stack, raw)
		if strings.HasPrefix(content, "#") {
			// Comments are allowed at any indentation. Align them with the
			// surrounding block without letting a stray comment push the next
			// real statement one level deeper.
			out = append(out, strings.Repeat(" ", level*4)+content)
			continue
		}
		stack = nextStack
		out = append(out, strings.Repeat(" ", level*4)+content)
	}

	return trimPythonWhitespace(strings.Join(out, "\n"))
}

// pythonStringState tracks a triple-quoted string that continues across lines.
type pythonStringState struct {
	triple string
}

// scanPythonLine advances the triple-quote state across one line. Single-line
// strings and comments are skipped so a '#' inside a string is not mistaken
// for a comment.
func scanPythonLine(line string, state pythonStringState) pythonStringState {
	for i := 0; i < len(line); {
		if state.triple != "" {
			idx := strings.Index(line[i:], state.triple)
			if idx < 0 {
				return state
			}
			i += idx + len(state.triple)
			state.triple = ""
			continue
		}

		switch line[i] {
		case '#':
			return state
		case '"', '\'':
			quote := line[i]
			if i+2 < len(line) && line[i+1] == quote && line[i+2] == quote {
				state.triple = line[i : i+3]
				i += 3
				continue
			}
			i++
			for i < len(line) {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == quote {
					i++
					break
				}
				i++
			}
		default:
			i++
		}
	}
	return state
}

// pythonIndentLevel maps a raw indentation width onto the current block stack.
// Increases create a level; decreases pop back to the nearest enclosing width.
func pythonIndentLevel(stack []int, raw int) (int, []int) {
	next := append([]int(nil), stack...)
	if raw > next[len(next)-1] {
		next = append(next, raw)
	} else if raw < next[len(next)-1] {
		for len(next) > 1 && raw < next[len(next)-1] {
			next = next[:len(next)-1]
		}
		if raw > next[len(next)-1] {
			next = append(next, raw)
		}
	}
	return len(next) - 1, next
}

// trimPythonWhitespace removes trailing whitespace, collapses blank runs, and
// ends the buffer with exactly one newline.
func trimPythonWhitespace(code string) string {
	lines := strings.Split(code, "\n")
	out := make([]string, 0, len(lines))
	blankRun := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 2 {
				continue
			}
			out = append(out, "")
			continue
		}
		blankRun = 0
		out = append(out, line)
	}

	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n") + "\n"
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}
