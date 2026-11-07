package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/ui/settings"
	"openleet/internal/ui/toast"
)

func logoutBindingKey(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// ddSignedIn mirrors the real installed state: signed in with a persisted
// session and an owned profile.
func logoutSignedInFixture(t *testing.T) AppModel {
	t.Helper()
	dir, _ := identityTestInstall(t)
	app := newPersistentAppAt(t, dir)
	return loginTestAccount(t, app, "uid-a", "a@example.com")
}

// ddSettle replays every generated message back into Update until the app is
// quiet, so deferred Cmd hops (like ActionMsg emitters) take effect.
func settleLogoutHops(t *testing.T, app AppModel, cmd tea.Cmd) AppModel {
	t.Helper()
	for hops := 0; cmd != nil && hops < 100; hops++ {
		msgs := collectBatchMsgs(cmd)
		nextCmd := tea.Cmd(nil)
		for _, msg := range msgs {
			var next tea.Model
			next, nextCmd = app.Update(msg)
			app = next.(AppModel)
			cmd = nextCmd
		}
	}
	return app
}

func requireLogoutOutcome(t *testing.T, name string, app AppModel) {
	t.Helper()
	accountCleared := app.account == nil
	toastSignedOut := hasVisibleToast(app, toast.Info, "Signed out")
	sessionCleared := true
	if app.sessionStore != nil {
		if _, err := app.sessionStore.Load(); !errors.Is(err, ErrNoStoredSession) {
			sessionCleared = false
		}
	}
	guestSwitched := app.identity != nil && app.identity.Profile.OwnerUID == "" && app.localID != ""
	if pass := accountCleared && toastSignedOut && sessionCleared && guestSwitched; !pass {
		t.Fatalf("RESULT %-30s accountCleared=%v toastSignedOut=%v sessionCleared=%v guestSwitched=%v profile=%#v localID=%q",
			name, accountCleared, toastSignedOut, sessionCleared, guestSwitched, app.identity.Profile, app.localID)
	}
}

// TestLogoutBindingsDeliverAccountLogout pins every user-visible sign-out
// control so the logout path can regress only through visible changes.
func TestLogoutBindingsDeliverAccountLogout(t *testing.T) {
	t.Run("topbar_menu_down_down_enter", func(t *testing.T) {
		app := logoutSignedInFixture(t)
		app = app.openAccountMenu()
		next, cmd := app.Update(logoutBindingKey("down"))
		app = next.(AppModel)
		next, cmd = app.Update(logoutBindingKey("down"))
		app = next.(AppModel)
		next, cmd = app.Update(logoutBindingKey("enter"))
		app = next.(AppModel)
		app = settleLogoutHops(t, app, cmd)
		requireLogoutOutcome(t, "topbar menu enter Log out", app)
	})

	t.Run("topbar_menu_click_logout_row", func(t *testing.T) {
		app := logoutSignedInFixture(t)
		app = app.openAccountMenu()
		for _, hit := range app.accountMenuHits() {
			if hit.Action != "logout" {
				continue
			}
			next, cmd := app.Update(tea.MouseMsg{X: hit.Start + 1, Y: hit.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			app = next.(AppModel)
			app = settleLogoutHops(t, app, cmd)
			requireLogoutOutcome(t, "topbar menu click Log out", app)
			return
		}
		t.Fatal("Log out hit missing")
	})

	t.Run("settings_press_L", func(t *testing.T) {
		app := logoutSignedInFixture(t)
		app = app.openSettings("account")
		next, cmd := app.Update(logoutBindingKey("L"))
		app = next.(AppModel)
		app = settleLogoutHops(t, app, cmd)
		requireLogoutOutcome(t, "settings L", app)
	})

	t.Run("settings_press_l_lowercase", func(t *testing.T) {
		app := logoutSignedInFixture(t)
		app = app.openSettings("account")
		next, cmd := app.Update(logoutBindingKey("l"))
		app = next.(AppModel)
		app = settleLogoutHops(t, app, cmd)
		requireLogoutOutcome(t, "settings l lowercase", app)
	})

	t.Run("settings_sidebar_then_L", func(t *testing.T) {
		app := logoutSignedInFixture(t)
		app = app.openSettings("account")
		next, _ := app.Update(logoutBindingKey("h"))
		app = next.(AppModel)
		next, cmd := app.Update(logoutBindingKey("L"))
		app = next.(AppModel)
		app = settleLogoutHops(t, app, cmd)
		requireLogoutOutcome(t, "settings L (sidebar focus)", app)
	})

	t.Run("settings_direct_action_msg", func(t *testing.T) {
		app := logoutSignedInFixture(t)
		next, cmd := app.Update(settings.ActionMsg{Action: "account_logout"})
		app = next.(AppModel)
		app = settleLogoutHops(t, app, cmd)
		requireLogoutOutcome(t, "direct settings.ActionMsg", app)
	})
}
