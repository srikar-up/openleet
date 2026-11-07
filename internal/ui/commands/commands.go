package commands

import (
	"fmt"
	"os"
	"strings"
)

// Context identifies which full-screen view is currently active.
type Context int

const (
	ContextHome Context = iota
	ContextSolve
	ContextRoadmap
	ContextStats
	ContextThemes
	ContextSettings
)

// String returns the canonical name of the context.
func (c Context) String() string {
	switch c {
	case ContextHome:
		return "home"
	case ContextSolve:
		return "solve"
	case ContextRoadmap:
		return "roadmap"
	case ContextStats:
		return "stats"
	case ContextThemes:
		return "themes"
	case ContextSettings:
		return "settings"
	default:
		return fmt.Sprintf("context(%d)", int(c))
	}
}

// Action identifies the semantic operation triggered by a command.
type Action string

const (
	ActionPickProblem   Action = "pick_problem" // displayed as /solve
	ActionRoadmap       Action = "roadmap"
	ActionGoogleLogin   Action = "google_login"
	ActionRandom        Action = "random"
	ActionFilter        Action = "filter"
	ActionTopic         Action = "topic"
	ActionThemes        Action = "themes"
	ActionLang          Action = "lang"
	ActionConnect       Action = "connect"
	ActionModel         Action = "model"
	ActionStats         Action = "stats"
	ActionSettings      Action = "settings"
	ActionThemeAudit    Action = "theme_audit"
	ActionNotifications Action = "notifications"
	ActionNotifyTest    Action = "notify_test"
	ActionUpdate        Action = "update"
	ActionQuit          Action = "quit"
	ActionRun           Action = "run"
	ActionSave          Action = "save"
	ActionCoach         Action = "coach"
	ActionHint          Action = "hint"
	ActionReset         Action = "reset"
)

// Command defines a user-invocable slash command.
type Command struct {
	Name        string // includes leading slash, e.g. "/themes"
	Aliases     []string
	Description string
	Action      Action
	Navigates   bool // true for commands that change the page (/settings, /roadmap)
}

var (
	cmdSolve = Command{
		Name:        "/solve",
		Aliases:     []string{"/pick"},
		Description: "Choose a problem (F2 to solve)",
		Action:      ActionPickProblem,
		Navigates:   false,
	}
	cmdRoadmap = Command{
		Name:        "/roadmap",
		Aliases:     []string{"/map"},
		Description: "Open the problem roadmap",
		Action:      ActionRoadmap,
		Navigates:   true,
	}
	cmdLogin = Command{
		Name:        "/login",
		Aliases:     []string{"/google", "/signin"},
		Description: "Sign in with Google in your browser",
		Action:      ActionGoogleLogin,
		Navigates:   false,
	}
	cmdRandom = Command{
		Name:        "/random",
		Aliases:     []string{"/rand"},
		Description: "Pick a random unsolved problem",
		Action:      ActionRandom,
		Navigates:   false,
	}
	cmdFilter = Command{
		Name:        "/filter",
		Description: "Filter by difficulty",
		Action:      ActionFilter,
		Navigates:   false,
	}
	cmdTopic = Command{
		Name:        "/topic",
		Description: "Filter by topic",
		Action:      ActionTopic,
		Navigates:   false,
	}
	cmdThemes = Command{
		Name:        "/themes",
		Aliases:     []string{"/theme"},
		Description: "Switch colour theme",
		Action:      ActionThemes,
		Navigates:   false,
	}
	cmdLang = Command{
		Name:        "/lang",
		Aliases:     []string{"/language"},
		Description: "Switch language",
		Action:      ActionLang,
		Navigates:   false,
	}
	cmdConnect = Command{
		Name:        "/connect",
		Description: "Connect an AI provider",
		Action:      ActionConnect,
		Navigates:   false,
	}
	cmdModel = Command{
		Name:        "/model",
		Description: "Switch the Coach model",
		Action:      ActionModel,
		Navigates:   false,
	}
	cmdStats = Command{
		Name:        "/stats",
		Description: "Streak, solved, last 7 days",
		Action:      ActionStats,
		Navigates:   false,
	}
	cmdSettings = Command{
		Name:        "/settings",
		Aliases:     []string{"/prefs", "/config"},
		Description: "Open full settings",
		Action:      ActionSettings,
		Navigates:   true,
	}
	cmdThemeAudit = Command{
		Name:        "/theme-audit",
		Description: "Audit semantic colors across every theme",
		Action:      ActionThemeAudit,
		Navigates:   false,
	}
	cmdNotifications = Command{
		Name:        "/notifications",
		Description: "Show recent notifications",
		Action:      ActionNotifications,
		Navigates:   false,
	}
	cmdNotifyTest = Command{
		Name:        "/notify-test",
		Description: "Fire one notification of each kind (debug)",
		Action:      ActionNotifyTest,
		Navigates:   false,
	}
	cmdUpdate = Command{
		Name:        "/update",
		Description: "Show update status",
		Action:      ActionUpdate,
		Navigates:   false,
	}
	cmdQuit = Command{
		Name:        "/quit",
		Aliases:     []string{"/exit"},
		Description: "Exit OpenLeet",
		Action:      ActionQuit,
		Navigates:   false,
	}
	cmdRun = Command{
		Name:        "/run",
		Description: "Run tests (ctrl+r)",
		Action:      ActionRun,
		Navigates:   false,
	}
	cmdSave = Command{
		Name:        "/save",
		Description: "Save solution (ctrl+s)",
		Action:      ActionSave,
		Navigates:   false,
	}
	cmdCoach = Command{
		Name:        "/coach",
		Description: "Open Socratic Coach",
		Action:      ActionCoach,
		Navigates:   false,
	}
	cmdHint = Command{
		Name:        "/hint",
		Description: "Show the Hint tab",
		Action:      ActionHint,
		Navigates:   false,
	}
	cmdReset = Command{
		Name:        "/reset",
		Description: "Reset code buffer",
		Action:      ActionReset,
		Navigates:   false,
	}
)

var allCommands = []Command{
	cmdSolve,
	cmdRoadmap,
	cmdLogin,
	cmdRandom,
	cmdFilter,
	cmdTopic,
	cmdThemes,
	cmdLang,
	cmdConnect,
	cmdModel,
	cmdStats,
	cmdSettings,
	cmdThemeAudit,
	cmdNotifications,
	cmdNotifyTest,
	cmdUpdate,
	cmdQuit,
	cmdRun,
	cmdSave,
	cmdCoach,
	cmdHint,
	cmdReset,
}

// Contexts returns all supported contexts in authoring order.
func Contexts() []Context {
	return []Context{
		ContextHome,
		ContextSolve,
		ContextRoadmap,
		ContextStats,
		ContextThemes,
		ContextSettings,
	}
}

// CommandsFor returns the exact command list available for the specified context.
func CommandsFor(ctx Context) []Command {
	var list []Command
	switch ctx {
	case ContextHome:
		list = []Command{
			cmdSolve,
			cmdRoadmap,
			cmdLogin,
			cmdRandom,
			cmdFilter,
			cmdTopic,
			cmdThemes,
			cmdLang,
			cmdConnect,
			cmdStats,
			cmdSettings,
			cmdQuit,
		}
	case ContextSolve:
		list = []Command{
			cmdRun,
			cmdSave,
			cmdCoach,
			cmdHint,
			cmdReset,
			cmdRoadmap,
			cmdLogin,
			cmdLang,
			cmdThemes,
			cmdModel,
			cmdConnect,
			cmdSettings,
			cmdQuit,
		}
	case ContextRoadmap:
		list = []Command{
			cmdRandom,
			cmdLogin,
			cmdFilter,
			cmdTopic,
			cmdThemes,
			cmdLang,
			cmdConnect,
			cmdStats,
			cmdSettings,
			cmdQuit,
		}
	case ContextStats:
		list = []Command{
			cmdRoadmap,
			cmdLogin,
			cmdThemes,
			cmdLang,
			cmdConnect,
			cmdStats,
			cmdSettings,
			cmdQuit,
		}
	case ContextThemes:
		list = []Command{
			cmdThemes,
			cmdLang,
			cmdLogin,
			cmdSettings,
			cmdQuit,
		}
	case ContextSettings:
		list = []Command{
			cmdThemes,
			cmdLang,
			cmdConnect,
			cmdModel,
			cmdLogin,
			cmdStats,
			cmdQuit,
		}
	default:
		return nil
	}

	list = append(list, cmdThemeAudit, cmdNotifications, cmdUpdate)
	if NotifyTestEnabled() {
		list = append(list, cmdNotifyTest)
	}
	out := make([]Command, len(list))
	copy(out, list)
	return out
}

// NotifyTestEnabled reports whether the debug-only notification test command
// is available. It is the same flag the theme audit uses for extra warnings.
func NotifyTestEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("OPENLEET_DEBUG")))
	return raw == "1" || raw == "true" || raw == "yes"
}

// All returns every unique action across all contexts in stable order.
func All() []Command {
	out := make([]Command, len(allCommands))
	copy(out, allCommands)
	return out
}
