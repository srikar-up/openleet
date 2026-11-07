package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/commands"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/toast"
)

func TestNotificationBurstCapsVisibleAndKeepsStatusBarClear(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.fireNotificationTest()

	visible := app.notifier.Visible()
	if len(visible) != 3 {
		t.Fatalf("visible = %d, want 3", len(visible))
	}
	if visible[2].Kind != toast.Warning {
		t.Fatalf("newest visible = %#v, want warning", visible[2])
	}

	view := app.View()
	plain := ansi.Strip(view)
	for _, title := range []string{"Notifications online", "Settings saved", "Connection is slow"} {
		if !strings.Contains(plain, title) {
			t.Fatalf("missing visible toast %q", title)
		}
	}
	if strings.Contains(plain, "Sync failed") {
		t.Fatal("queued error toast leaked into the frame")
	}
	lines := strings.Split(plain, "\n")
	status := lines[len(lines)-1]
	for _, title := range []string{"Notifications online", "Settings saved", "Connection is slow"} {
		if strings.Contains(status, title) {
			t.Fatalf("toast %q painted inside the status bar: %q", title, status)
		}
	}
}

func TestToastStackAnchorsBottomRightAtAllWidths(t *testing.T) {
	for _, width := range []int{100, 120, 160} {
		app := testTopBarApp(theme.Resolve("openleet"), width, nil)
		app.notify(toast.Info, "Width check", "A message wide enough to wrap once on small screens.", toast.Options{})
		view := app.View()
		for _, line := range strings.Split(view, "\n") {
			if got := ansi.StringWidth(line); got != width {
				t.Fatalf("width %d: rendered line is %d cells", width, got)
			}
		}
		hits := app.toastHits()
		if len(hits) == 0 {
			t.Fatalf("width %d: no toast hits", width)
		}
		var body *toast.Hit
		for i := range hits {
			if !hits[i].Action {
				body = &hits[i]
				break
			}
		}
		if body == nil {
			t.Fatalf("width %d: no body hit", width)
		}
		if body.X1 != width-3 {
			t.Fatalf("width %d: toast right edge = %d", width, body.X1)
		}
		if body.Y1 != app.height-2 {
			t.Fatalf("width %d: toast bottom = %d, want %d", width, body.Y1, app.height-2)
		}
		if got := ansi.StringWidth(app.renderTopBar()); got != width {
			t.Fatalf("width %d: top bar is %d cells", width, got)
		}
	}
}

func TestSignedOutChipOpensAccountMenu(t *testing.T) {
	app := newRoutingTestApp(t)
	app.currentView = ViewSolve
	hits := app.topBarHits()
	var account topBarHit
	found := false
	for _, hit := range hits {
		if hit.target == topBarTargetAccount {
			account, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("account chip hit target missing")
	}
	if plain := ansi.Strip(app.renderTopBar()); !strings.Contains(plain, "Log in") {
		t.Fatalf("signed-out chip is missing Log in: %q", plain)
	}

	m, _ := app.Update(tea.MouseMsg{
		X:      account.start,
		Y:      0,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	next := m.(AppModel)
	if !next.accountMenuOpen {
		t.Fatal("clicking the chip did not open the dropdown")
	}
	panel, _ := next.accountMenuPanel()
	if !strings.Contains(ansi.Strip(panel), "Log in") || !strings.Contains(ansi.Strip(panel), "Continue offline") {
		t.Fatalf("signed-out dropdown is missing actions: %q", ansi.Strip(panel))
	}

	for _, hit := range next.accountMenuHits() {
		if hit.Action != "login" {
			continue
		}
		m, _ := next.Update(tea.MouseMsg{
			X:      hit.Start + 1,
			Y:      hit.Y,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		})
		opened := m.(AppModel)
		if opened.currentView != ViewSettings {
			t.Fatalf("Log in opened view %v", opened.currentView)
		}
		if got := opened.settingsView.ActiveSection(); got != settings.SectionAccount {
			t.Fatalf("Log in opened Settings section %q", got)
		}
		return
	}
	t.Fatal("Log in row not found")
}

func TestSignedInAccountMenuShowsPlanAndLogoutToasts(t *testing.T) {
	app := newRoutingTestApp(t)
	app.account = &accountSession{UID: "uid-1", Email: "user@example.com", Plan: "Free", IDToken: "token"}
	app.currentView = ViewSolve
	app = app.openAccountMenu()
	panel, _ := app.accountMenuPanel()
	plain := ansi.Strip(panel)
	for _, want := range []string{"user@example.com", "Plan  Free", "Sync now", "Account settings", "Log out"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("signed-in dropdown missing %q: %q", want, plain)
		}
	}

	for _, hit := range app.accountMenuHits() {
		if hit.Action != "logout" {
			continue
		}
		m, cmd := app.Update(tea.MouseMsg{
			X:      hit.Start + 1,
			Y:      hit.Y,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		})
		next := m.(AppModel)
		if next.accountMenuOpen {
			t.Fatal("logout did not close the dropdown")
		}
		if cmd == nil {
			t.Fatal("logout did not dispatch the account action")
		}
		msg := cmd()
		final, _ := next.Update(msg)
		finalApp := final.(AppModel)
		if finalApp.account != nil {
			t.Fatal("logout left the account signed in")
		}
		if !hasVisibleToast(finalApp, toast.Info, "Signed out") {
			t.Fatalf("logout toast missing: %#v", visibleToasts(finalApp))
		}
		return
	}
	t.Fatal("Log out row not found")
}

func TestToastClickDismissAndAction(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.notify(toast.Info, "Action toast", "Press the hint", toast.Options{
		ActionHint: "enter settings",
		ActionID:   "settings",
	})
	actionHit := toast.Hit{}
	foundAction := false
	for _, hit := range app.toastHits() {
		if hit.Action {
			actionHit, foundAction = hit, true
			break
		}
	}
	if !foundAction {
		t.Fatal("action hit missing")
	}
	m, _ := app.Update(tea.MouseMsg{
		X:      actionHit.X0,
		Y:      actionHit.Y0,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	next := m.(AppModel)
	if len(next.notifier.Visible()) != 0 {
		t.Fatal("clicking the action did not dismiss the toast")
	}
	if next.currentView != ViewSettings {
		t.Fatalf("toast action opened view %v, want Settings", next.currentView)
	}

	app = testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.notify(toast.Success, "Dismiss me", "", toast.Options{})
	var body toast.Hit
	for _, hit := range app.toastHits() {
		if !hit.Action {
			body = hit
			break
		}
	}
	m, _ = app.Update(tea.MouseMsg{
		X:      body.X0 + 2,
		Y:      body.Y0 + 2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	next = m.(AppModel)
	if len(next.notifier.Visible()) != 0 {
		t.Fatal("clicking the toast body did not dismiss it")
	}
	if next.currentView != ViewSolve {
		t.Fatalf("toast body click changed view to %v", next.currentView)
	}
}

func TestEscDismissesNewestToastBeforeLeavingScreen(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.notify(toast.Info, "Dismiss with esc", "", toast.Options{})
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := m.(AppModel)
	if next.currentView != ViewSolve {
		t.Fatalf("first esc changed view to %v", next.currentView)
	}
	if len(next.notifier.Visible()) != 0 {
		t.Fatal("first esc did not dismiss the toast")
	}
	m, _ = next.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := m.(AppModel).currentView; got != ViewRoadmap {
		t.Fatalf("second esc changed view to %v, want roadmap", got)
	}
}

func TestCtrlNOpensNotificationHistory(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.notify(toast.Success, "Saved solution", "buffer.py", toast.Options{})
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	next := m.(AppModel)
	if !next.quick.dialogOpen {
		t.Fatal("ctrl+n did not open the notification history")
	}
	panel := ansi.Strip(next.quick.dialog.Panel())
	if !strings.Contains(panel, "NOTIFICATIONS") || !strings.Contains(panel, "Saved solution") {
		t.Fatalf("history panel missing content: %q", panel)
	}
}

func TestToastHostHiddenBehindModal(t *testing.T) {
	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	app.notify(toast.Info, "Hidden toast", "", toast.Options{})
	app = app.openCommandPalette()
	plain := ansi.Strip(app.View())
	if strings.Contains(plain, "Hidden toast") {
		t.Fatal("toast painted over an open command palette")
	}
}

func TestNotifyTestIsDebugOnly(t *testing.T) {
	t.Setenv("OPENLEET_DEBUG", "")
	for _, cmd := range commands.CommandsFor(commands.ContextHome) {
		if cmd.Name == "/notify-test" {
			t.Fatal("/notify-test should be hidden without the debug flag")
		}
	}
	t.Setenv("OPENLEET_DEBUG", "1")
	found := false
	for _, cmd := range commands.CommandsFor(commands.ContextHome) {
		if cmd.Name == "/notify-test" {
			found = true
		}
	}
	if !found {
		t.Fatal("/notify-test missing with OPENLEET_DEBUG=1")
	}

	app := testTopBarApp(theme.Resolve("openleet"), 160, nil)
	next, _ := app.handleNotifyTest("")
	if got := len(next.notifier.Visible()); got != 3 {
		t.Fatalf("visible after notify-test = %d, want 3", got)
	}
	if got := len(next.notifier.History()); got != 5 {
		t.Fatalf("history after notify-test = %d, want 5", got)
	}
}
