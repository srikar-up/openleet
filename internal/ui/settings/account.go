package settings

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/uikit"
)

// AccountInfo is the display-only account snapshot. ID tokens never enter it.
type AccountInfo struct {
	SignedIn    bool
	Remembered  bool
	UID         string
	Email       string
	DisplayName string
	Plan        string
	SyncEnabled bool
	SyncNote    string
	LastSynced  time.Time
	SolvedCount int
	TotalPoints int
}

// AuthMode selects the email/password endpoint used by the form.
type AuthMode int

const (
	AuthSignIn AuthMode = iota
	AuthSignUp
)

// AuthRequestMsg asks the app to authenticate with Firebase Identity Toolkit.
// The password and token remain in memory only.
type AuthRequestMsg struct {
	Mode     AuthMode
	Email    string
	Username string
	Password string
}

// AuthResultMsg reports the authentication result back to Settings.
type AuthResultMsg struct {
	Info    AccountInfo
	Message string
	Err     error
}

// GoogleAuthRequestMsg asks the app to start the browser-based Google flow.
// The flow itself runs inside a tea.Cmd; the Settings model only shows status.
type GoogleAuthRequestMsg struct{}

const (
	authFocusEmail = iota
	authFocusUsername
	authFocusPassword
)

// AuthForm is the modal email/password form shown by the Account section.
type AuthForm struct {
	Mode     AuthMode
	Focus    int
	Email    textinput.Model
	Username textinput.Model
	Password textinput.Model
	Err      string
	Busy     bool
}

func newAuthForm(mode AuthMode) *AuthForm {
	email := textinput.New()
	email.Prompt = ""
	email.Placeholder = "you@example.com"
	email.CharLimit = 254
	email.Width = 32

	username := textinput.New()
	username.Prompt = ""
	username.Placeholder = "display name"
	username.CharLimit = 64
	username.Width = 32

	password := textinput.New()
	password.Prompt = ""
	password.Placeholder = "password"
	password.CharLimit = 128
	password.Width = 32
	password.EchoMode = textinput.EchoPassword

	form := &AuthForm{Mode: mode, Focus: authFocusEmail, Email: email, Username: username, Password: password}
	form.focus()
	return form
}

func (f *AuthForm) focus() {
	f.Email.Blur()
	f.Username.Blur()
	f.Password.Blur()
	switch f.Focus {
	case authFocusUsername:
		f.Username.Focus()
	case authFocusPassword:
		f.Password.Focus()
	default:
		f.Email.Focus()
	}
}

func (f *AuthForm) nextFocus(delta int) {
	order := []int{authFocusEmail, authFocusPassword}
	if f.Mode == AuthSignUp {
		order = []int{authFocusEmail, authFocusUsername, authFocusPassword}
	}
	position := 0
	for i, focus := range order {
		if focus == f.Focus {
			position = i
			break
		}
	}
	position = (position + delta + len(order)) % len(order)
	f.Focus = order[position]
	f.focus()
}

// SetAccount updates the signed-in profile card.
func (m *Model) SetAccount(info AccountInfo) {
	m.account = info
}

// SetGoogleBusy toggles the browser-wait state shown on the signed-out card.
func (m *Model) SetGoogleBusy(busy bool) {
	m.googleBusy = busy
}

// GoogleBusy reports whether the browser flow is waiting for a callback.
func (m Model) GoogleBusy() bool { return m.googleBusy }

func (m *Model) openAuthForm() {
	m.openAuthFormMode(AuthSignIn)
}

func (m *Model) openAuthFormMode(mode AuthMode) {
	m.auth = newAuthForm(mode)
	m.status = "Sign in with Firebase email/password. The session is remembered on this device."
}

func (m Model) submitAuth() (Model, tea.Cmd) {
	form := m.auth
	if form == nil {
		return m, nil
	}
	email := strings.TrimSpace(form.Email.Value())
	password := form.Password.Value()
	username := strings.TrimSpace(form.Username.Value())
	switch {
	case email == "":
		form.Err = "Email is required"
		return m, nil
	case password == "":
		form.Err = "Password is required"
		return m, nil
	case form.Mode == AuthSignUp && username == "":
		form.Err = "Display name is required for sign up"
		return m, nil
	case form.Mode == AuthSignUp && len(password) < 6:
		form.Err = "Password must be at least 6 characters"
		return m, nil
	}
	form.Busy = true
	form.Err = ""
	mode := form.Mode
	return m, func() tea.Msg {
		return AuthRequestMsg{Mode: mode, Email: email, Username: username, Password: password}
	}
}

func (m Model) updateAuth(msg tea.KeyMsg) (Model, tea.Cmd) {
	form := m.auth
	if form == nil {
		return m, nil
	}
	if form.Busy {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.auth = nil
		m.status = "Continuing offline; the local profile remains active."
		return m, nil
	case "tab":
		form.nextFocus(1)
		return m, textinput.Blink
	case "shift+tab":
		form.nextFocus(-1)
		return m, textinput.Blink
	case "ctrl+t":
		if form.Mode == AuthSignIn {
			form.Mode = AuthSignUp
			form.Focus = authFocusEmail
		} else {
			form.Mode = AuthSignIn
			form.Focus = authFocusEmail
		}
		form.Err = ""
		form.focus()
		return m, textinput.Blink
	case "enter":
		return m.submitAuth()
	}

	var cmd tea.Cmd
	switch form.Focus {
	case authFocusUsername:
		form.Username, cmd = form.Username.Update(msg)
	case authFocusPassword:
		form.Password, cmd = form.Password.Update(msg)
	default:
		form.Email, cmd = form.Email.Update(msg)
	}
	return m, cmd
}

func (m Model) authFormWidth() int {
	width := min(64, m.width-4)
	if width < 34 {
		width = 34
	}
	return width
}

func (m Model) renderAuthForm() string {
	form := m.auth
	if form == nil {
		return ""
	}
	panelW := m.authFormWidth()
	contentW := panelW - 6
	labelW := 10
	inputW := contentW - labelW
	if inputW < 8 {
		inputW = 8
	}
	title := "SIGN IN"
	button := "[ SIGN IN ]"
	if form.Mode == AuthSignUp {
		title = "CREATE ACCOUNT"
		button = "[ CREATE ACCOUNT ]"
	}

	row := func(label string, input textinput.Model, focused bool) string {
		labelStyle := lipgloss.NewStyle().Foreground(m.pal.Muted)
		if focused {
			labelStyle = labelStyle.Bold(true).Foreground(m.pal.Accent)
		}
		input.Width = inputW
		value := uikit.FitExact(input.View(), inputW)
		return labelStyle.Render(uikit.FitExact(label, labelW)) + value
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent).Render(title),
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render("Firebase email/password · remembered on this device"),
		"",
		row("Email", form.Email, form.Focus == authFocusEmail),
	}
	if form.Mode == AuthSignUp {
		lines = append(lines, row("Display", form.Username, form.Focus == authFocusUsername))
	}
	lines = append(lines, row("Password", form.Password, form.Focus == authFocusPassword))
	if form.Err != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.pal.Danger).
			Render(uikit.Truncate(form.Err, contentW, "…")))
	}
	lines = append(lines,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent).Render(button),
		lipgloss.NewStyle().Foreground(m.pal.Muted).
			Render("tab field  enter submit  ctrl+t sign in/up  esc offline"),
	)
	if form.Busy {
		lines[len(lines)-2] = lipgloss.NewStyle().Foreground(m.pal.Warning).Render("Authenticating…")
	}
	return lipgloss.NewStyle().
		Width(panelW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.pal.Accent).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderAuthOverlay(base string) string {
	if m.auth == nil {
		return base
	}
	return centerPanelOver(base, m.renderAuthForm(), m.width)
}

func (m Model) renderAccount(width int) string {
	accountField, _ := fieldByID("account.panel")
	accountFocused := m.isSelected(accountField)

	var accountTitle string
	var accountLines []string
	var syncLines []string

	if !m.account.SignedIn {
		accountTitle = "Account"
		googleLine := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent).
			Render("[ Continue with Google ]") +
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("   g")
		if m.googleBusy {
			googleLine = lipgloss.NewStyle().Foreground(m.pal.Warning).Render("Waiting for Google sign-in in your browser…")
		}
		accountLines = []string{
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("Signed out · offline profile only"),
			"",
			googleLine,
			lipgloss.NewStyle().Foreground(m.pal.Text).Render("[ Sign in with email ]") +
				lipgloss.NewStyle().Foreground(m.pal.Muted).Render("   e"),
			lipgloss.NewStyle().Foreground(m.pal.Text).Render("[ Create account ]") +
				lipgloss.NewStyle().Foreground(m.pal.Muted).Render("   u"),
			"",
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("Google opens your browser; no password is typed into OpenLeet."),
		}
		syncLines = []string{
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("Sync is optional · this device stays signed in until logout"),
		}
	} else {
		accountTitle = "Profile & sessions"
		name := m.account.DisplayName
		if name == "" {
			name = m.account.Email
		}
		plan := m.account.Plan
		if plan == "" {
			plan = "Free"
		}
		rememberLine := "L logs out · this session ends when OpenLeet closes"
		if m.account.Remembered {
			rememberLine = "L logs out · refresh token encrypted on this device"
		}
		accountLines = []string{
			lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent).Render("◈ " + uikit.Truncate(name, width-10, "…")),
			lipgloss.NewStyle().Foreground(m.pal.Text).Render(uikit.Truncate(m.account.Email, width-4, "…")),
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("PLAN  " + plan),
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("UID   " + uikit.Truncate(m.account.UID, width-10, "…")),
			lipgloss.NewStyle().Foreground(m.pal.Success).Render("● this device · active"),
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render(rememberLine),
		}

		last := "never"
		if !m.account.LastSynced.IsZero() {
			last = m.account.LastSynced.Format("2006-01-02 15:04")
		}
		note := m.account.SyncNote
		if note == "" {
			policy := "newest wins"
			if ConflictPolicy(m.cfg) == "local" {
				policy = "local wins"
			}
			note = "never synced · " + policy
		}
		statusColor := m.pal.Muted
		statusText := "● not synced yet"
		if !SyncEnabled(m.cfg) {
			statusColor = m.pal.Warning
			statusText = "● cloud sync off"
		} else {
			if !m.account.LastSynced.IsZero() {
				statusColor = m.pal.Success
				statusText = "● Firestore synced"
			}
			if strings.HasPrefix(note, "Sync failed") || strings.Contains(note, "(pull warning)") || strings.Contains(note, "push failed") {
				statusColor = m.pal.Danger
				statusText = "● sync needs attention"
			}
		}
		syncLines = []string{
			lipgloss.NewStyle().Foreground(statusColor).Render(statusText),
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render("LAST   " + last),
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render(
				fmt.Sprintf("LOCAL  %d solved · %d points", m.account.SolvedCount, m.account.TotalPoints)),
			lipgloss.NewStyle().Foreground(m.pal.Text).Render("[ Sync now ]   s sync · l log out"),
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render(note),
		}
	}

	syncEnabled, _ := fieldByID("sync.enabled")
	syncPolicy, _ := fieldByID("sync.policy")
	syncLines = append(syncLines,
		m.renderFieldRow(syncEnabled, width-4, m.isSelected(syncEnabled)),
		m.renderFieldRow(syncPolicy, width-4, m.isSelected(syncPolicy)),
	)
	for _, field := range []Field{syncEnabled, syncPolicy} {
		if m.isSelected(field) && field.Help != "" {
			syncLines = append(syncLines, m.renderHelp(field.Help, width-4))
			break
		}
	}

	syncFocused := m.isSelected(syncEnabled) || m.isSelected(syncPolicy)

	accountCard := m.renderCard(accountTitle, accountLines, width, accountFocused)
	syncCard := m.renderCard("Sync", syncLines, width, syncFocused)
	return accountCard + "\n\n" + syncCard
}
