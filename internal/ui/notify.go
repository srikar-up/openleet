package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/ui/settings"
	"openleet/internal/ui/toast"
)

// The root model speaks in toast kinds. The service owns the queue, history,
// and timers; screens never draw their own transient messages.
type toastKind = toast.Kind

const (
	toastInfo = toast.Info
	toastPass = toast.Success
	toastWarn = toast.Warning
	toastFail = toast.Error
)

const notificationTickInterval = 200 * time.Millisecond

type notificationTickMsg struct{}

func notificationTickCmd() tea.Cmd {
	return tea.Tick(notificationTickInterval, func(time.Time) tea.Msg {
		return notificationTickMsg{}
	})
}

func (m *AppModel) ensureNotifier() *toast.Notifier {
	if m.notifier == nil {
		m.notifier = toast.New()
	}
	return m.notifier
}

// notify is the single entry point for every transient message. It returns the
// tick command only when this call started the timer loop.
func (m *AppModel) notify(kind toastKind, title, message string, opts toast.Options) tea.Cmd {
	n := m.ensureNotifier()
	now := time.Now()
	n.SetNarrow(m.width > 0 && m.width < 100, now)
	n.Push(kind, title, message, opts, now)
	if n.StartTick() {
		return notificationTickCmd()
	}
	return nil
}

// toast preserves the older one-line call shape while everything routes
// through the notifier.
func (m *AppModel) toast(text string) tea.Cmd {
	return m.notify(toastInfo, text, "", toast.Options{})
}

func (m *AppModel) toastWithKind(kind toastKind, text string) tea.Cmd {
	return m.notify(kind, text, "", toast.Options{})
}

func (m AppModel) toastHost() toast.Host {
	return toast.Host{
		Width:   m.width,
		Height:  m.height,
		Palette: m.palette,
		Unicode: m.iconsUnicode(),
	}
}

func (m AppModel) toastHits() []toast.Hit {
	if m.notifier == nil || m.width <= 0 || m.height <= 0 {
		return nil
	}
	return m.toastHost().Hits(m.notifier.Visible())
}

// toastHitAt prefers an action span over the surrounding card body.
func (m AppModel) toastHitAt(x, y int) (toast.Hit, bool) {
	var body toast.Hit
	haveBody := false
	for _, hit := range m.toastHits() {
		if x < hit.X0 || x > hit.X1 || y < hit.Y0 || y > hit.Y1 {
			continue
		}
		if hit.Action {
			return hit, true
		}
		body, haveBody = hit, true
	}
	return body, haveBody
}

func (m AppModel) visibleToastID(title string) int {
	if m.notifier == nil {
		return 0
	}
	for _, visible := range m.notifier.Visible() {
		if visible.Title == title {
			return visible.ID
		}
	}
	return 0
}

func (m AppModel) toastAction(id int) string {
	if m.notifier == nil {
		return ""
	}
	for _, visible := range m.notifier.Visible() {
		if visible.ID == id {
			return visible.ActionID
		}
	}
	return ""
}

// handleToastAction runs the action attached to a toast. The caller dismisses
// the toast; actions may open a screen or start a retry.
func (m AppModel) handleToastAction(action string) (AppModel, tea.Cmd) {
	switch action {
	case "notifications":
		return m.openNotifications()
	case "update":
		return m.openUpdateDialog()
	case "sync", "retry_sync":
		return m, func() tea.Msg { return settings.ActionMsg{Action: "account_sync"} }
	case "retry_leaderboard":
		return m, m.publishLeaderboardCmd()
	case "settings", "login":
		return m.openSettings("account"), nil
	default:
		return m, nil
	}
}
