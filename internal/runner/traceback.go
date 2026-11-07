package runner

import (
	"regexp"
	"strconv"
	"strings"
)

// pythonFrameLine matches the file/line prefix of a Python traceback frame.
var pythonFrameLine = regexp.MustCompile(`(File "[^"]*", line )(\d+)`)

const userSolutionBegin = "USER SOLUTION BEGIN"

// userCodeStartLine returns the 1-based line in the generated harness where the
// user's code begins. Tracebacks from the combined file point at harness lines,
// so this offset is what makes an error report match the editor buffer.
func userCodeStartLine(source string) int {
	idx := strings.Index(source, userSolutionBegin)
	if idx < 0 {
		return 0
	}
	return strings.Count(source[:idx], "\n") + 2
}

// remapPythonTraceback rewrites only the frames that fall inside the user code
// range. Harness frames keep their original line numbers, which avoids turning
// an internal helper line into a bogus user line.
func remapPythonTraceback(text string, startLine, lineCount int) string {
	if text == "" || startLine <= 0 || lineCount <= 0 {
		return text
	}
	endLine := startLine + lineCount - 1
	return pythonFrameLine.ReplaceAllStringFunc(text, func(match string) string {
		parts := pythonFrameLine.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		line, err := strconv.Atoi(parts[2])
		if err != nil || line < startLine || line > endLine {
			return match
		}
		return parts[1] + strconv.Itoa(line-startLine+1)
	})
}
