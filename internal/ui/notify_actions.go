package ui

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/ui/commands"
	"openleet/internal/ui/quickdialog"
	"openleet/internal/ui/toast"
)

// openNotifications shows the bounded history newest-first.
func (m AppModel) openNotifications() (AppModel, tea.Cmd) {
	items := make([]quickdialog.Item, 0, 20)
	if m.notifier != nil {
		for _, entry := range m.notifier.History() {
			label := entry.CreatedAt.Format("15:04:05") + "  " + strings.ToUpper(notificationKindLabel(entry.Kind))
			value := entry.Title
			if entry.Message != "" {
				value += " · " + entry.Message
			}
			if entry.Count > 1 {
				value += fmt.Sprintf("  x%d", entry.Count)
			}
			items = append(items, quickdialog.Item{
				ID:    fmt.Sprintf("%d", entry.ID),
				Label: label,
				Value: value,
			})
		}
	}
	if len(items) == 0 {
		items = append(items, quickdialog.Item{
			ID:    "empty",
			Label: "NOTIFICATIONS",
			Value: "No notifications yet",
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "NOTIFICATIONS", ReadOnly: true, Hint: "esc close"},
		[]quickdialog.Group{{Title: "RECENT", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionNotifications
	return m, nil
}

func notificationKindLabel(kind toast.Kind) string {
	switch kind {
	case toast.Success:
		return "success"
	case toast.Warning:
		return "warning"
	case toast.Error:
		return "error"
	default:
		return "info"
	}
}

// openUpdateDialog is the destination for an update toast action. It never
// pretends an updater exists: it reports the current build and whatever update
// version the notifier was told about.
func (m AppModel) openUpdateDialog() (AppModel, tea.Cmd) {
	status := "No update detected in this session"
	if m.updateVersion != "" {
		status = m.updateVersion + " available"
	}
	items := []quickdialog.Item{
		{ID: "current", Label: "CURRENT BUILD", Value: buildVersion()},
		{ID: "status", Label: "STATUS", Value: status},
	}
	if m.updateVersion != "" {
		items = append(items, quickdialog.Item{
			ID:    "next",
			Label: "NEXT",
			Value: "Install the newer build, then restart OpenLeet",
		})
	}
	m.quick.dialog = quickdialog.NewList(
		quickdialog.Options{Title: "UPDATE", ReadOnly: true, Hint: "esc close"},
		[]quickdialog.Group{{Title: "RELEASE", Items: items}},
		m.palette,
		m.width,
		m.height,
	)
	m.quick.dialogOpen = true
	m.quick.paletteOpen = false
	m.quick.action = commands.ActionUpdate
	return m, nil
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// notifyUpdateAvailable is the hook an update checker should call. The debug
// test command uses it with v0.5.0.
func (m *AppModel) notifyUpdateAvailable(version string) tea.Cmd {
	if strings.TrimSpace(version) == "" {
		return nil
	}
	m.updateVersion = version
	return m.notify(toast.Info, version+" available", "A newer OpenLeet build is ready.",
		toast.Options{ActionHint: "enter to update", ActionID: "update", Duration: 8 * time.Second})
}

// fireNotificationTest pushes one of every kind plus an update notice. The
// queue intentionally exceeds the visible cap.
func (m *AppModel) fireNotificationTest() tea.Cmd {
	now := time.Now()
	n := m.ensureNotifier()
	n.SetNarrow(m.width > 0 && m.width < 100, now)
	cmds := make([]tea.Cmd, 0, 5)
	cmds = append(cmds, m.notify(toast.Info, "Notifications online", "Newest messages appear above the status bar.", toast.Options{}))
	cmds = append(cmds, m.notify(toast.Success, "Settings saved", "Your preferences were written to disk.", toast.Options{}))
	cmds = append(cmds, m.notify(toast.Warning, "Connection is slow", "The coach is retrying in the background.", toast.Options{}))
	cmds = append(cmds, m.notify(toast.Error, "Sync failed", "The saved session could not reach Firestore.", toast.Options{
		ActionHint: "enter retry",
		ActionID:   "retry_sync",
	}))
	cmds = append(cmds, m.notifyUpdateAvailable("v0.5.0"))
	return tea.Batch(cmds...)
}
