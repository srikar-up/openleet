package workspace

import "openleet/internal/model"

// Focus identifies which pane receives keyboard events.
type Focus int

const (
	FocusDescription Focus = iota
	FocusEditor
	FocusConsole
)

// FocusChangedMsg is dispatched when focus transitions to another pane.
type FocusChangedMsg struct {
	Focus Focus
}

// CodeChangedMsg is dispatched when the user edits the code buffer.
type CodeChangedMsg struct {
	Content string
}

// SaveCodeMsg requests persisting the code buffer to storage.
type SaveCodeMsg struct {
	ProblemID string
	Language  string
	Code      string
}

// CodeSavedMsg reports the outcome of a save operation.
type CodeSavedMsg struct {
	ProblemID string
	Language  string
	Err       error
}

// RunTestsMsg dispatches test suite execution against host runner fixtures.
type RunTestsMsg struct {
	ProblemID string
	Language  string
	Code      string
}

// CodeFormattedMsg carries the outcome of a beautify pass.
type CodeFormattedMsg struct {
	ProblemID string
	Language  string
	Code      string
	Err       error
}

// CustomTestRequestedMsg asks the workspace to run one user-authored fixture
// against the current editor buffer.
type CustomTestRequestedMsg struct {
	Input    interface{}
	Expected interface{}
}

// CustomTestCompletedMsg carries the result of a custom fixture run.
type CustomTestCompletedMsg struct {
	Result *model.ExecutionResult
}

// TestsCompletedMsg carries the completed execution result.
type TestsCompletedMsg struct {
	Result *model.ExecutionResult
	Err    error
}

// ExitWorkspaceMsg signals the parent model to return to the level map.
type ExitWorkspaceMsg struct{}

// ConsoleTab represents which view is active in the bottom-right console.
type ConsoleTab int

const (
	TabTestResults ConsoleTab = iota
	TabSocraticCoach
	TabHint
	TabCustom
)

// ConsoleTabChangedMsg is dispatched when switching tabs in the console.
type ConsoleTabChangedMsg struct {
	Tab ConsoleTab
}

// BufferLoadedMsg indicates initial problem code has been loaded from disk.
type BufferLoadedMsg struct {
	ProblemID string
	Content   string
	Language  string
	Scope     string
}

// LanguageChangedMsg reports that the workspace switched its code language.
// The root model persists the choice through config.DefaultLanguage.
type LanguageChangedMsg struct {
	Language string
}

// LanguageSwitchedMsg reports a language change that preserved the previous
// language buffer and loaded the target language's saved buffer when present.
type LanguageSwitchedMsg struct {
	Language  string
	ProblemID string
	Content   string
	HasBuffer bool
}
