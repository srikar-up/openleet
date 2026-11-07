package ui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/coach"
	"openleet/internal/config"
	"openleet/internal/eventbus"
	"openleet/internal/model"
	"openleet/internal/network"
	"openleet/internal/runner"
	"openleet/internal/storage"
	"openleet/internal/ui/home"
	"openleet/internal/ui/mapview"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/stats"
	"openleet/internal/ui/theme"
	"openleet/internal/ui/toast"
	"openleet/internal/ui/uikit"
	"openleet/internal/ui/workspace"
)

// ViewMode represents the currently active screen.
type ViewMode int

const (
	ViewHome     ViewMode = iota // OpenCode-style minimalist landing / main screen
	ViewRoadmap                  // Problem Roadmap / Level Map
	ViewSolve                    // Problem Workspace / Solver
	ViewStats                    // Activity dashboard
	ViewSettings                 // Schema-driven settings
)

// statsBreadcrumbWidth is the clickable width of the "openleet / stats" header
// title, measured in terminal cells rather than bytes.
var statsBreadcrumbWidth = lipgloss.Width("openleet / stats")

// hitTest resolves a click column against a set of tab ranges.
func hitTest(hits []tabHit, x int) (ViewMode, bool) {
	for _, hit := range hits {
		if x >= hit.start && x <= hit.end {
			return hit.view, true
		}
	}
	return ViewHome, false
}

// AppModel is the root Elm model managing overall application state and routing.
type AppModel struct {
	config       *config.Config
	storage      storage.Manager
	executor     *runner.Executor
	theme        Theme
	palette      theme.Palette
	keyMap       KeyMap
	homeView     home.Model
	mapView      mapview.Model
	workspace    workspace.Model
	statsView    stats.Model
	settingsView settings.Model

	// Dimensions (dynamic, from tea.WindowSizeMsg)
	width  int
	height int

	// Routing & selection
	currentView    ViewMode
	settingsReturn ViewMode
	quick          quickState
	notifier       *toast.Notifier
	problems       []*model.Problem
	selectedIndex  int
	userState      *model.UserState

	// Account dropdown and update status live at the chrome layer.
	accountMenuOpen  bool
	accountMenuIndex int
	updateVersion    string
	toastHoverID     int
	quitToastID      int

	// Top-bar focus and hover state for the Settings button and account chip.
	topBarFocus   topBarTarget
	topBarHover   topBarTarget
	topBarPressed topBarTarget

	// account is the active Firebase session. The ID token stays in memory;
	// the long-lived refresh token may be stored encrypted on this device.
	account          *accountSession
	authBaseURL      string // test override for Identity Toolkit
	firestoreBaseURL string // test override for Firestore
	googleAuthURL    string // test override for Google OAuth authorize
	googleTokenURL   string // test override for Google OAuth token
	secureTokenURL   string // test override for Firebase refresh-token exchange
	openBrowser      func(string) error

	identity      *identityState
	localID       string
	profileStore  *storage.ProfileStore
	sessionStore  *authSessionStore
	syncMetaStore *storage.SyncMetaStore
	syncMeta      *storage.SyncMeta
	lastSynced    time.Time
	syncNote      string

	// leaderboard is the minimal public ranking cache. It is populated only
	// while a Firebase session exists and never includes code or email data.
	leaderboard        []network.LeaderboardEntry
	leaderboardLoaded  bool
	leaderboardLoading bool
	leaderboardErr     string

	// Runner state
	isRunning  bool
	lastResult *model.ExecutionResult

	// Status & error notifications
	statusMsg string
	err       error
	quitArmed bool

	// bus is the optional asynchronous event pipeline. When nil the app still
	// works: every producer falls back to plain tea.Cmd delivery.
	bus *eventbus.Hub

	// streams owns coach token pumps. It is a pointer so copies of the value
	// model all observe the same goroutines and cancellation context.
	streams *coachStreams

	// coachMgr is the shared Socratic Coach manager handed to the workspace.
	// Providers are swapped in place (never recreated) when settings change.
	coachMgr *coach.Manager
}

// NewAppModel constructs an initialized AppModel with injected config and storage.
func NewAppModel(cfg *config.Config, storageMgr storage.Manager) AppModel {
	if cfg != nil {
		settings.EnsureDefaults(cfg)
	}
	detector := runner.NewDetector()
	exec := runner.NewExecutor(detector, time.Duration(cfg.ExecutionTimeout)*time.Second)

	localProvider, cloudProvider := buildCoachProviders(cfg)
	coachMgr := coach.NewManager(localProvider, cloudProvider)
	coachMgr.SetGenerationOptions(coachGenerationOptions(cfg))

	emptyProblems := make([]*model.Problem, 0)
	userState := model.NewUserState()
	pal := paletteFromConfig(cfg)

	sessionStore := newAuthSessionStoreForConfig(cfg)
	idState := openIdentityState(cfg, storageMgr, sessionStore)
	scopedStorage := storageMgr
	localID := ""
	if idState != nil {
		if idState.Err != nil {
			scopedStorage, localID = idState.recoveryManager()
		} else {
			mgr, err := storage.ForUser(idState.Root, idState.Profile.LocalID)
			if err != nil {
				idState.Err = err
				scopedStorage, localID = idState.recoveryManager()
			} else {
				scopedStorage = mgr
				localID = idState.Profile.LocalID
			}
		}
	}

	hm := home.New(cfg, emptyProblems, userState, 100, 30)
	hm.SetPalette(pal)
	mv := mapview.New(emptyProblems, userState, 100, 28)
	mv.SetTheme(mapview.FromPalette(pal))
	ws := workspace.New(nil, cfg.DefaultLanguage, scopedStorage, exec, coachMgr, 100, 28)
	ws.SetScope(localID)
	ws.SetEditorOptions(editorOptionsFromConfig(cfg))
	ws.SetTheme(workspace.FromPalette(pal))
	sv := stats.New(emptyProblems, userState, 100, 30)
	sv.SetPalette(pal)

	settingsView := settings.New(cfg, pal, 100, 30)
	notifier := toast.New()

	userDir := ""
	if idState != nil && localID != "" {
		userDir = storage.UserDir(idState.Root, localID)
	} else if cfg.DataDir() != "" {
		userDir = cfg.DataDir()
	}

	var profileStore *storage.ProfileStore
	if userDir != "" {
		profileStore, _ = storage.NewProfileStoreIn(userDir)
	}

	var syncMetaStore *storage.SyncMetaStore
	var syncMeta *storage.SyncMeta
	if userDir != "" {
		syncMetaStore, _ = storage.NewSyncMetaStoreIn(userDir)
		if syncMetaStore != nil {
			syncMeta, _ = syncMetaStore.Load()
		}
	}

	return AppModel{
		config:         cfg,
		storage:        scopedStorage,
		identity:       idState,
		localID:        localID,
		executor:       exec,
		theme:          themeFromPalette(pal),
		palette:        pal,
		keyMap:         DefaultKeyMap(),
		homeView:       hm,
		mapView:        mv,
		workspace:      ws,
		statsView:      sv,
		settingsView:   settingsView,
		notifier:       notifier,
		profileStore:   profileStore,
		sessionStore:   sessionStore,
		syncMetaStore:  syncMetaStore,
		syncMeta:       syncMeta,
		width:          100, // safe initial fallback before WindowSizeMsg
		height:         30,
		currentView:    ViewHome,
		settingsReturn: ViewHome,
		problems:       emptyProblems,
		selectedIndex:  0,
		userState:      userState,
		streams:        newCoachStreams(),
		coachMgr:       coachMgr,
		openBrowser:    network.OpenBrowser,
	}
}

// paletteFromConfig resolves the active palette and applies the user's accent
// override. Invalid accent values fall back to the theme accent.
func paletteFromConfig(cfg *config.Config) theme.Palette {
	pal := theme.Default()
	if cfg != nil {
		pal = theme.Resolve(cfg.Theme)
	}
	if cfg == nil {
		return pal
	}
	accent := settings.Value(cfg, settings.Field{ID: "appearance.accent", Default: "theme"})
	return theme.ApplyAccent(pal, accent)
}

// editorOptionsFromConfig maps the Editor settings into the live editor.
func editorOptionsFromConfig(cfg *config.Config) workspace.EditorOptions {
	opts := workspace.DefaultEditorOptions()
	if cfg == nil {
		return opts
	}
	if raw := settings.Value(cfg, settings.Field{ID: "editor.tab_width", Default: "4"}); raw != "" {
		if width, err := strconv.Atoi(raw); err == nil {
			opts.TabWidth = width
		}
	}
	opts.ShowLineNumbers = settings.Value(cfg, settings.Field{ID: "editor.line_numbers", Default: "on"}) == "on"
	opts.AutoIndent = settings.Value(cfg, settings.Field{ID: "editor.auto_indent", Default: "on"}) == "on"
	opts.BracketPairing = settings.Value(cfg, settings.Field{ID: "editor.bracket_pairing", Default: "on"}) == "on"
	opts.FormatOnSave = settings.Value(cfg, settings.Field{ID: "editor.format_on_save", Default: "on"}) == "on"
	return opts
}

// applyRuntimeSettings pushes live Settings values into the components that
// were constructed at startup. It is called after a save or when Settings
// closes, so editor/runner changes do not require an app restart.
func (m *AppModel) applyRuntimeSettings() {
	if m == nil || m.config == nil {
		return
	}
	m.applyPaletteColors(paletteFromConfig(m.config))
	m.workspace.SetEditorOptions(editorOptionsFromConfig(m.config))
	m.applyCoachOptions()

	if lang := settings.Value(m.config, settings.Field{ID: "editor.default_language", Default: "python"}); lang != "" {
		m.config.DefaultLanguage = lang
		m.homeView.SetActiveLanguage(lang)
	}

	if m.executor != nil {
		detector := runner.NewDetector()
		if path := settings.Value(m.config, settings.Field{ID: "runner.python_path", Default: "python"}); path != "" {
			detector.SetExecutable("python", path)
		}
		if path := settings.Value(m.config, settings.Field{ID: "runner.go_path", Default: "go"}); path != "" {
			detector.SetExecutable("go", path)
		}
		m.executor.SetDetector(detector)
		timeout := m.config.ExecutionTimeout
		if timeout <= 0 {
			timeout = 5
		}
		m.executor.SetTimeout(time.Duration(timeout) * time.Second)
	}
}

// requestQuit honours the general.confirm_quit preference. The first quit
// request arms the prompt and routes it through the notifier; a second request
// exits.
func (m AppModel) requestQuit() (tea.Model, tea.Cmd) {
	confirm := m.config != nil &&
		settings.Value(m.config, settings.Field{ID: "general.confirm_quit", Default: "off"}) == "on"
	if !confirm || m.quitArmed {
		return m, tea.Quit
	}
	m.quitArmed = true
	m.quitToastID = m.visibleToastID("Quit OpenLeet?")
	cmd := m.notify(toast.Warning, "Quit OpenLeet?", "Press q or ctrl+c again · any key cancels", toast.Options{Sticky: true})
	if m.quitToastID == 0 {
		m.quitToastID = m.visibleToastID("Quit OpenLeet?")
	}
	return m, cmd
}

// AttachBus wires the asynchronous event pipeline into the model. Call this
// before tea.Program.Run so the pump has somewhere to send from the first frame.
func (m *AppModel) AttachBus(hub *eventbus.Hub) {
	m.bus = hub
	if m.streams != nil {
		m.streams.attach(hub)
	}
}

// Close stops background stream pumps. It is safe to call more than once.
func (m *AppModel) Close() {
	if m.streams != nil {
		m.streams.close()
	}
}

// applyPaletteColors repaints every section for a palette without touching the
// config file. Settings uses it for live theme previews.
func (m *AppModel) applyPaletteColors(pal theme.Palette) {
	if m.config != nil {
		pal = theme.ApplyAccent(pal, settings.Value(m.config, settings.Field{ID: "appearance.accent", Default: "theme"}))
	}
	m.palette = pal
	m.theme = themeFromPalette(pal)
	m.homeView.SetPalette(pal)
	m.mapView.SetTheme(mapview.FromPalette(pal))
	m.workspace.SetTheme(workspace.FromPalette(pal))
	m.statsView.SetPalette(pal)
	m.settingsView.SetPalette(pal)
	m.quick.palette.SetPalette(pal)
	m.quick.dialog.SetPalette(pal)
}

// applyPalette switches the whole application to a named theme and persists it.
// Theme is one setting shared by Settings > Appearance and the /themes quick
// action, so both the legacy top-level field and the schema key are written.
func (m *AppModel) applyPalette(name string) {
	pal, ok := theme.Get(name)
	if !ok {
		pal = theme.Default()
	}
	m.applyPaletteColors(pal)
	m.settingsView.SetActiveTheme(pal.Name)

	if m.config != nil {
		m.config.Theme = pal.Name
		if m.config.Settings == nil {
			m.config.Settings = make(map[string]string)
		}
		m.config.Settings["appearance.theme"] = pal.Name
		if err := m.config.Save(); err != nil {
			m.err = err
		}
	}
}

// openSettings switches to Settings and remembers which screen should receive
// focus again on Esc.
func (m AppModel) openSettings(argument string) AppModel {
	if m.currentView != ViewSettings {
		m.settingsReturn = m.currentView
	}
	m.settingsView.SetSize(m.width, m.height)
	m.settingsView.SetActiveTheme(m.palette.Name)
	if strings.TrimSpace(argument) == "" {
		// Reopening Settings from another screen must land where the user
		// left off; only an explicit section deep link scrolls the page.
		m.settingsView.Restore()
	} else {
		m.settingsView.OpenSection(settings.ResolveSection(argument))
	}
	if m.account != nil {
		m.settingsView.SetAccount(m.accountInfo())
	}
	m.currentView = ViewSettings
	return m
}

// coachReady reports whether the Socratic Coach can run right now: the
// selected provider has a saved key (or is local Ollama), or the legacy
// OPENCODE_API_KEY environment fallback is present in auto mode only. An
// explicit provider choice never silently falls back to the environment key.
func (m AppModel) coachReady() bool {
	if m.settingsView.CoachReady() {
		return true
	}
	if settings.CoachProviderID(m.config) != "" {
		return false
	}
	return strings.TrimSpace(os.Getenv("OPENCODE_API_KEY")) != ""
}

// applyCoachOptions pushes the persisted AI generation settings into the shared
// coach manager. The manager is swapped by pointer, so the workspace sees the
// update without recreating either model.
func (m *AppModel) applyCoachOptions() {
	if m.coachMgr == nil {
		return
	}
	m.coachMgr.SetGenerationOptions(coachGenerationOptions(m.config))
}

// authConfigPath prefers a project-local .env, then the user config directory,
// then the process working directory. Environment variables always win inside
// network.LoadAuthConfig.
func (m AppModel) authConfigPath() string {
	candidates := make([]string, 0, 2)
	if m.config != nil && m.config.ConfigDir() != "" {
		candidates = append(candidates, filepath.Join(m.config.ConfigDir(), ".env"))
	}
	candidates = append(candidates, ".env")
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ".env"
}

// authenticateCmd performs Firebase email/password authentication off the Elm
// loop. The ID token lives only in memory; the refresh token is returned so the
// caller can persist it encrypted for the next launch.
func (m AppModel) authenticateCmd(req settings.AuthRequestMsg) tea.Cmd {
	path := m.authConfigPath()
	baseURL := m.authBaseURL
	return func() tea.Msg {
		cfg, err := network.LoadAuthConfig(path)
		if err != nil {
			return accountAuthResultMsg{err: fmt.Errorf("Firebase auth is not configured: %w", err)}
		}
		opts := []network.Option{}
		if baseURL != "" {
			opts = append(opts, network.WithAuthBaseURL(baseURL))
		}
		auth, err := network.NewAuthService(cfg.FirebaseAPIKey, opts...)
		if err != nil {
			return accountAuthResultMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		if req.Mode == settings.AuthSignUp {
			if _, err := auth.SignUp(ctx, req.Email, req.Password, req.Username); err != nil {
				return accountAuthResultMsg{err: err}
			}
		}
		result, err := auth.SignInWithProfile(ctx, req.Email, req.Password)
		if err != nil {
			return accountAuthResultMsg{err: err}
		}
		display := strings.TrimSpace(req.Username)
		publicName := display
		if display == "" {
			display = strings.TrimSpace(result.DisplayName)
		}
		if publicName == "" {
			publicName = strings.TrimSpace(result.DisplayName)
		}
		if display == "" {
			// Local greeting only. The public leaderboard name must never be
			// derived from the email address.
			if at := strings.IndexByte(req.Email, '@'); at > 0 {
				display = req.Email[:at]
			} else {
				display = req.Email
			}
		}
		if publicName == "" {
			publicName = "Anonymous"
		}
		return accountAuthResultMsg{session: &accountSession{
			UID:          result.UID,
			Email:        strings.TrimSpace(req.Email),
			DisplayName:  display,
			PublicName:   publicName,
			IDToken:      result.IDToken,
			RefreshToken: result.RefreshToken,
			Plan:         "Free",
			SignedInAt:   time.Now(),
		}}
	}
}

// googleAuthCmd runs the loopback Google OAuth flow and exchanges the Google
// ID token for a Firebase session. The browser and local listener live inside
// the returned tea.Cmd, never inside Update.
func (m AppModel) googleAuthCmd() tea.Cmd {
	path := m.authConfigPath()
	authBase := m.authBaseURL
	googleAuth := m.googleAuthURL
	googleToken := m.googleTokenURL
	opener := m.openBrowser
	return func() tea.Msg {
		cfg, err := network.LoadAuthConfig(path)
		if err != nil {
			return accountAuthResultMsg{err: fmt.Errorf("Firebase auth is not configured: %w", err)}
		}
		if strings.TrimSpace(cfg.GoogleClientID) == "" {
			return accountAuthResultMsg{err: errors.New("Google sign-in is not configured; set GOOGLE_OAUTH_CLIENT_ID in .env")}
		}
		if strings.Contains(cfg.GoogleClientID, ".firebaseapp.com") {
			return accountAuthResultMsg{err: errors.New("GOOGLE_OAUTH_CLIENT_ID is the Firebase auth domain, not an OAuth client ID; create a Desktop app OAuth client in Google Cloud Console")}
		}
		opts := []network.Option{}
		if authBase != "" {
			opts = append(opts, network.WithAuthBaseURL(authBase))
		}
		if googleAuth != "" {
			opts = append(opts, network.WithGoogleAuthURL(googleAuth))
		}
		if googleToken != "" {
			opts = append(opts, network.WithGoogleTokenURL(googleToken))
		}
		auth, err := network.NewAuthService(cfg.FirebaseAPIKey, opts...)
		if err != nil {
			return accountAuthResultMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		user, err := auth.SignInWithGoogle(ctx, network.GoogleConfig{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			OpenBrowser:  opener,
		})
		if err != nil {
			return accountAuthResultMsg{err: err}
		}
		display := strings.TrimSpace(user.DisplayName)
		publicName := display
		if display == "" {
			if at := strings.IndexByte(user.Email, '@'); at > 0 {
				display = user.Email[:at]
			} else {
				display = user.Email
			}
		}
		if publicName == "" {
			publicName = "Anonymous"
		}
		return accountAuthResultMsg{session: &accountSession{
			UID:          user.UID,
			Email:        user.Email,
			DisplayName:  display,
			PublicName:   publicName,
			IDToken:      user.IDToken,
			RefreshToken: user.RefreshToken,
			Plan:         "Free",
			SignedInAt:   time.Now(),
		}}
	}
}

// restoreSessionCmd loads the remembered identity without blocking on the
// network. The caller paints the cached identity first, then refreshes the ID
// token.
func (m AppModel) restoreSessionCmd() tea.Cmd {
	store := m.sessionStore
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		session, err := store.Load()
		return storedSessionLoadedMsg{session: session, err: err}
	}
}

// refreshStoredSessionCmd exchanges the saved refresh token for a fresh ID
// token. The refresh token never leaves this command except in memory.
func (m AppModel) refreshStoredSessionCmd(stored storedAuthSession, origin string) tea.Cmd {
	path := m.authConfigPath()
	baseURL := m.authBaseURL
	secureToken := m.secureTokenURL
	return func() tea.Msg {
		cfg, err := network.LoadAuthConfig(path)
		if err != nil {
			return sessionRefreshResultMsg{origin: origin, err: fmt.Errorf("Firebase auth is not configured: %w", err)}
		}
		opts := []network.Option{}
		if baseURL != "" {
			opts = append(opts, network.WithAuthBaseURL(baseURL))
		}
		if secureToken != "" {
			opts = append(opts, network.WithSecureTokenURL(secureToken))
		}
		auth, err := network.NewAuthService(cfg.FirebaseAPIKey, opts...)
		if err != nil {
			return sessionRefreshResultMsg{origin: origin, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		result, err := auth.RefreshSession(ctx, stored.RefreshToken)
		if err != nil {
			return sessionRefreshResultMsg{origin: origin, err: err}
		}
		if result.UID != stored.UID {
			return sessionRefreshResultMsg{origin: origin, err: fmt.Errorf("%w: refreshed user does not match the saved account", network.ErrSessionExpired)}
		}

		display := strings.TrimSpace(stored.DisplayName)
		if display == "" {
			display = emailDisplayName(stored.Email)
		}
		publicName := strings.TrimSpace(stored.PublicName)
		if publicName == "" {
			publicName = "Anonymous"
		}
		plan := strings.TrimSpace(stored.Plan)
		if plan == "" {
			plan = "Free"
		}
		signedInAt := stored.SignedInAt
		if signedInAt.IsZero() {
			signedInAt = time.Now()
		}
		return sessionRefreshResultMsg{origin: origin, session: &accountSession{
			UID:          result.UID,
			Email:        stored.Email,
			DisplayName:  display,
			PublicName:   publicName,
			IDToken:      result.IDToken,
			RefreshToken: result.RefreshToken,
			RefreshedAt:  time.Now(),
			Plan:         plan,
			SignedInAt:   signedInAt,
		}}
	}
}

// emailDisplayName derives the local greeting from an email prefix. It is
// never used as the public leaderboard name.
func emailDisplayName(email string) string {
	if at := strings.IndexByte(email, '@'); at > 0 {
		return email[:at]
	}
	return email
}

// persistSessionCmd writes the current identity to the encrypted session vault.
func (m AppModel) persistSessionCmd() tea.Cmd {
	store := m.sessionStore
	if store == nil || m.account == nil || strings.TrimSpace(m.account.RefreshToken) == "" {
		return nil
	}
	stored := storedAuthSession{
		RefreshToken: m.account.RefreshToken,
		UID:          m.account.UID,
		Email:        m.account.Email,
		DisplayName:  m.account.DisplayName,
		PublicName:   m.account.PublicName,
		Plan:         m.account.Plan,
		SignedInAt:   m.account.SignedInAt,
	}
	return func() tea.Msg {
		return sessionPersistResultMsg{action: "save", err: store.Save(stored)}
	}
}

// clearStoredSessionCmd removes the remembered login after logout or when
// Firebase rejects the refresh token.
func (m AppModel) clearStoredSessionCmd() tea.Cmd {
	store := m.sessionStore
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		return sessionPersistResultMsg{action: "clear", err: store.Clear()}
	}
}

// refreshCurrentSessionCmd refreshes the token for the active account.
func (m AppModel) refreshCurrentSessionCmd(origin string) tea.Cmd {
	if m.account == nil || strings.TrimSpace(m.account.RefreshToken) == "" {
		return nil
	}
	return m.refreshStoredSessionCmd(storedAuthSession{
		RefreshToken: m.account.RefreshToken,
		UID:          m.account.UID,
		Email:        m.account.Email,
		DisplayName:  m.account.DisplayName,
		PublicName:   m.account.PublicName,
		Plan:         m.account.Plan,
		SignedInAt:   m.account.SignedInAt,
	}, origin)
}

// accountSession is the active Firebase identity. The ID token stays in
// memory; RefreshToken may be persisted encrypted on this device.
type accountSession struct {
	UID          string
	Email        string
	DisplayName  string // local greeting; may be an email prefix
	PublicName   string // leaderboard name; never derived from the email
	IDToken      string
	RefreshToken string
	RefreshedAt  time.Time
	Plan         string
	SignedInAt   time.Time
}

func (s *accountSession) info() settings.AccountInfo {
	if s == nil {
		return settings.AccountInfo{}
	}
	return settings.AccountInfo{
		SignedIn:    true,
		Remembered:  strings.TrimSpace(s.RefreshToken) != "",
		UID:         s.UID,
		Email:       s.Email,
		DisplayName: s.DisplayName,
		Plan:        s.Plan,
	}
}

func (m AppModel) accountInfo() settings.AccountInfo {
	info := m.account.info()
	if m.account == nil {
		return info
	}
	info.SyncEnabled = settings.SyncEnabled(m.config)
	info.SyncNote = m.syncNote
	info.LastSynced = m.lastSynced
	if m.userState != nil {
		info.SolvedCount = len(m.userState.SolvedProblems)
		info.TotalPoints = m.userState.TotalPoints
	}
	return info
}

type accountAuthResultMsg struct {
	session *accountSession
	err     error
}

// storedSessionLoadedMsg carries the encrypted identity loaded at startup.
type storedSessionLoadedMsg struct {
	session storedAuthSession
	err     error
	attempt int
}

// sessionRefreshResultMsg reports a refresh-token exchange. Origin identifies
// the caller so a sync that hit an expired token can retry once the token is
// renewed.
type sessionRefreshResultMsg struct {
	session *accountSession
	origin  string
	err     error
}

// sessionPersistResultMsg reports a best-effort encrypted session write.
type sessionPersistResultMsg struct {
	action string
	err    error
}

// dataActionResultMsg carries the outcome of an app-owned file operation back
// to Settings. All I/O runs inside tea.Cmd, never inside Update.
type dataActionResultMsg struct {
	action        string
	detail        string
	err           error
	state         *model.UserState
	settingsReset bool
}

func (m *AppModel) adoptUserState(state *model.UserState) {
	if state == nil {
		return
	}
	m.userState = state
	m.homeView.SetUserState(state)
	m.mapView.SetUserState(state)
	m.statsView.SetUserState(state)
}

// runSettingsAction performs a Settings-initiated data action off the Elm loop.
func (m AppModel) runSettingsAction(action string) tea.Cmd {
	cfg := m.config
	store := m.storage
	return func() tea.Msg {
		result := dataActionResultMsg{action: action}
		dataDir := m.userDataDir()
		if dataDir == "" {
			dataDir = cfg.DataDir()
		}
		switch action {
		case "data_export":
			state, err := store.LoadUserState()
			if err != nil {
				result.err = err
				return result
			}
			jsonPath := filepath.Join(dataDir, "openleet-progress.json")
			raw, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				result.err = err
				return result
			}
			if err := os.WriteFile(jsonPath, raw, 0600); err != nil {
				result.err = err
				return result
			}
			csvPath := filepath.Join(dataDir, "openleet-progress.csv")
			if err := writeProgressCSV(csvPath, state); err != nil {
				result.err = err
				return result
			}
			result.detail = fmt.Sprintf("Exported %d solved problems to %s", len(state.SolvedProblems), jsonPath)
			return result

		case "data_import":
			jsonPath := filepath.Join(dataDir, "openleet-progress.json")
			info, err := os.Stat(jsonPath)
			if err != nil {
				result.err = fmt.Errorf("no export found at %s", jsonPath)
				return result
			}
			if info.Size() > 10<<20 {
				result.err = fmt.Errorf("export is too large")
				return result
			}
			raw, err := os.ReadFile(jsonPath)
			if err != nil {
				result.err = err
				return result
			}
			var state model.UserState
			if err := json.Unmarshal(raw, &state); err != nil {
				result.err = fmt.Errorf("invalid progress JSON: %w", err)
				return result
			}
			if state.CodeBuffers == nil {
				state.CodeBuffers = make(map[string]map[string]string)
			}
			if len(state.SolvedProblems) > 10000 {
				result.err = fmt.Errorf("progress file has too many records")
				return result
			}
			if err := store.SaveUserState(&state); err != nil {
				result.err = err
				return result
			}
			result.state = &state
			result.detail = fmt.Sprintf("Imported %d solved problems from %s", len(state.SolvedProblems), jsonPath)
			return result

		case "data_backup":
			dir := filepath.Join(dataDir, "backups", time.Now().Format("20060102-150405"))
			if err := os.MkdirAll(dir, 0700); err != nil {
				result.err = err
				return result
			}
			sources := []string{
				filepath.Join(dataDir, "user_state.json"),
				filepath.Join(dataDir, "workspace_layout.json"),
				filepath.Join(cfg.ConfigDir(), "config.json"),
			}
			copied := 0
			for _, source := range sources {
				if err := copyFile(source, filepath.Join(dir, filepath.Base(source))); err == nil {
					copied++
				}
			}
			if copied == 0 {
				result.err = fmt.Errorf("nothing to back up in %s", dataDir)
				return result
			}
			result.detail = fmt.Sprintf("Backed up %d files to %s", copied, dir)
			return result

		case "data_clear_cache":
			if err := os.Remove(filepath.Join(dataDir, "workspace_layout.json")); err != nil && !os.IsNotExist(err) {
				result.err = err
				return result
			}
			result.detail = "Workspace layout cache cleared; progress is untouched"
			return result

		case "data_reset_roadmap":
			state, err := store.LoadUserState()
			if err != nil {
				result.err = err
				return result
			}
			state.SolvedProblems = nil
			state.CurrentStreak = 0
			state.LongestStreak = 0
			state.LastSolvedDate = time.Time{}
			state.TotalPoints = 0
			if err := store.SaveUserState(state); err != nil {
				result.err = err
				return result
			}
			result.state = state
			result.detail = "Roadmap progress reset; code buffers kept"
			return result

		case "data_delete_local":
			for _, name := range []string{"user_state.json", "workspace_layout.json", storage.SyncMetaFileName} {
				if err := os.Remove(filepath.Join(dataDir, name)); err != nil && !os.IsNotExist(err) {
					result.err = err
					return result
				}
			}
			result.state = model.NewUserState()
			result.detail = "Local progress, code buffers, and layout deleted"
			return result

		case "reset_settings":
			defaults := config.DefaultConfig()
			cfg.Theme = defaults.Theme
			cfg.DefaultLanguage = defaults.DefaultLanguage
			cfg.OllamaEndpoint = defaults.OllamaEndpoint
			cfg.OllamaModel = defaults.OllamaModel
			cfg.ExecutionTimeout = defaults.ExecutionTimeout
			cfg.ShowLineNumbers = defaults.ShowLineNumbers
			cfg.AutoSave = defaults.AutoSave
			cfg.Settings = nil
			cfg.SettingsVersion = 0
			settings.EnsureDefaults(cfg)
			if err := cfg.Save(); err != nil {
				result.err = err
				return result
			}
			result.settingsReset = true
			result.detail = "All settings reset to schema defaults"
			return result
		}
		result.err = fmt.Errorf("unknown settings action %q", action)
		return result
	}
}

// writeProgressCSV exports solved problems in a spreadsheet-friendly format.
func writeProgressCSV(path string, state *model.UserState) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"problem_id", "solved_at", "attempts", "optimal", "language"}); err != nil {
		return err
	}
	for _, solved := range state.SolvedProblems {
		if err := writer.Write([]string{
			solved.ProblemID,
			solved.SolvedAt.Format(time.RFC3339),
			strconv.Itoa(solved.Attempts),
			strconv.FormatBool(solved.OptimalSolution),
			solved.Language,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// copyFile copies one regular file, creating the destination directory.
func copyFile(source, destination string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	return os.WriteFile(destination, raw, 0600)
}

// OpenSettingsSection jumps directly to a named Settings category. It is used
// by the --settings-section capture flag and palette deep links.
func (m *AppModel) OpenSettingsSection(argument string) {
	m.settingsView.SetSize(m.width, m.height)
	m.settingsView.OpenSection(settings.ResolveSection(argument))
}

// StartAt selects the screen shown on first paint. It is used by the --view
// flag and by headless visual capture so no key sequence is required.
func (m *AppModel) StartAt(view ViewMode) {
	m.currentView = view
	switch view {
	case ViewStats:
		m.statsView.SetSize(m.width, m.height)
		m.refreshLeaderboardView()
	case ViewSettings:
		m.settingsView.SetSize(m.width, m.height)
		m.settingsView.OpenSection(settings.SectionGeneral)
		if m.account != nil {
			m.settingsView.SetAccount(m.accountInfo())
		}
		m.settingsReturn = ViewHome
	}
}

// Init triggers initial asynchronous data loads.
func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.loadUserStateCmd(),
		m.loadProblemsCmd(),
		m.homeView.Init(),
		m.restoreSessionCmd(),
	}
	if m.identity != nil && m.identity.Err != nil {
		cmds = append(cmds, m.notify(toast.Error, "Local identity unavailable",
			"OpenLeet is using an isolated recovery namespace; identity.enc was not replaced.",
			toast.Options{Sticky: true}))
	}
	if m.identity != nil && m.identity.Migrated {
		cmds = append(cmds, m.notify(toast.Success, "Local profile migrated",
			"Legacy progress now lives in an encrypted per-user namespace.", toast.Options{}))
	}
	return tea.Batch(cmds...)
}

func (m AppModel) loadUserStateCmd() tea.Cmd {
	store := m.storage
	localID := m.localID
	return func() tea.Msg {
		state, err := store.LoadUserState()
		return UserStateLoadedMsg{State: state, Err: err, LocalID: localID}
	}
}

func (m AppModel) loadProblemsCmd() tea.Cmd {
	return func() tea.Msg {
		problems, err := m.storage.ListProblems()
		return ProblemListLoadedMsg{Problems: problems, Err: err}
	}
}

func (m AppModel) switchToSolveView(index int) (AppModel, tea.Cmd) {
	if len(m.problems) == 0 || index < 0 || index >= len(m.problems) {
		return m, nil
	}
	m.selectedIndex = index
	p := m.problems[index]
	contentHeight := m.height - 2
	if contentHeight < 4 {
		contentHeight = 4
	}
	m.workspace.SetLanguage(m.config.DefaultLanguage)
	m.workspace.SetProblem(p)
	m.workspace.SetDimensions(m.width, contentHeight)
	m.currentView = ViewSolve
	return m, m.workspace.Init()
}

// ensureSolveView returns the app positioned on the solver, loading the
// selected problem when a different section currently owns the screen.
func (m AppModel) ensureSolveView() (AppModel, tea.Cmd) {
	if m.currentView == ViewSolve && m.workspace.Problem() != nil {
		return m, nil
	}
	return m.switchToSolveView(m.selectedIndex)
}

// enterStats opens the dashboard and lazily fetches the leaderboard the first
// time a signed-in user visits it. Network work stays inside a tea.Cmd.
func (m AppModel) enterStats() (AppModel, tea.Cmd) {
	m.currentView = ViewStats
	m.statsView.SetSize(m.width, m.height)
	m.refreshLeaderboardView()
	if m.account == nil || m.leaderboardLoaded || m.leaderboardLoading {
		return m, nil
	}
	m.leaderboardLoading = true
	m.leaderboardErr = ""
	m.refreshLeaderboardView()
	return m, m.loadLeaderboardCmd()
}

// Update processes incoming Bubble Tea messages.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case eventbus.BatchMsg:
		return m.updateBatch(msg)

	case notificationTickMsg:
		if m.notifier == nil {
			return m, nil
		}
		if m.notifier.Tick(time.Now()) {
			return m, notificationTickCmd()
		}
		m.notifier.FinishTick()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.notifier != nil {
			m.notifier.SetNarrow(msg.Width < 100, time.Now())
		}
		contentHeight := msg.Height - 2
		if contentHeight < 4 {
			contentHeight = 4
		}
		homeHeight := msg.Height - 2
		if homeHeight < 1 {
			homeHeight = 1
		}
		mapHeight := msg.Height - 2
		if mapHeight < 1 {
			mapHeight = 1
		}
		settingsHeight := msg.Height - 2
		if settingsHeight < 1 {
			settingsHeight = 1
		}
		var homeCmd, mapCmd, wsCmd tea.Cmd
		m.homeView, homeCmd = m.homeView.Update(tea.WindowSizeMsg{Width: msg.Width, Height: homeHeight})
		m.mapView, mapCmd = m.mapView.Update(tea.WindowSizeMsg{Width: msg.Width, Height: mapHeight})
		m.workspace, wsCmd = m.workspace.Update(tea.WindowSizeMsg{Width: msg.Width, Height: contentHeight})
		m.statsView, _ = m.statsView.Update(tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.settingsView.SetSize(msg.Width, settingsHeight)
		m.quick.palette.SetSize(msg.Width, msg.Height)
		m.quick.dialog.SetSize(msg.Width, msg.Height)
		return m, tea.Batch(homeCmd, mapCmd, wsCmd)

	case UserStateLoadedMsg:
		if msg.LocalID != "" && msg.LocalID != m.localID {
			// A load from the previous profile finished after an account switch.
			return m, nil
		}
		if msg.Err == nil && msg.State != nil {
			m.userState = msg.State
			// Count this launch as a study session.
			m.userState.Sessions++
			_ = m.storage.SaveUserState(m.userState)
			m.homeView.SetUserState(m.userState)
			m.mapView.SetUserState(m.userState)
			m.statsView.SetUserState(m.userState)
			if m.account != nil {
				m.settingsView.SetAccount(m.accountInfo())
			}
		}
		return m, nil

	case ProblemListLoadedMsg:
		if msg.Err == nil {
			m.problems = msg.Problems
			m.homeView.SetProblems(m.problems)
			m.mapView.SetProblems(m.problems)
			m.statsView.SetProblems(m.problems)
			if len(m.problems) > 0 && m.workspace.Problem() == nil {
				m.workspace.SetLanguage(m.config.DefaultLanguage)
				m.workspace.SetProblem(m.problems[0])
			}
		} else {
			m.err = msg.Err
		}
		return m, nil

	case settings.CloseMsg:
		if m.config != nil {
			_ = m.config.Save()
		}
		previousLanguage := m.workspace.Language()
		nextLanguage := ""
		if m.config != nil {
			nextLanguage = settings.Value(m.config, settings.Field{ID: "editor.default_language", Default: "python"})
		}
		m.applyRuntimeSettings()
		returnView := m.settingsReturn
		var languageCmd tea.Cmd
		var languageNotice tea.Cmd
		if returnView == ViewSolve && nextLanguage != "" && nextLanguage != previousLanguage {
			languageCmd = m.workspace.SwitchLanguage(nextLanguage)
		}
		if nextLanguage != "" && nextLanguage != previousLanguage {
			languageNotice = m.notify(toast.Info, "Language changed", nextLanguage, toast.Options{})
		}
		if returnView == ViewStats && m.account != nil && !m.leaderboardLoaded {
			m.leaderboardLoading = true
			m.leaderboardErr = ""
			m.refreshLeaderboardView()
			languageCmd = tea.Batch(languageCmd, m.loadLeaderboardCmd())
		}
		m.currentView = returnView
		m.settingsReturn = ViewHome
		saved := m.notify(toast.Success, "Settings saved", "Preferences are up to date.", toast.Options{})
		return m, tea.Batch(languageCmd, languageNotice, saved)

	case settings.PreviewThemeMsg:
		if pal, ok := theme.Get(msg.Name); ok {
			m.applyPaletteColors(pal)
		}
		return m, nil

	case settings.ApplyThemeMsg:
		m.applyPalette(msg.Name)
		return m, m.notify(toast.Info, "Theme changed", theme.Resolve(msg.Name).Label, toast.Options{})

	case settings.RevertThemeMsg:
		if pal, ok := theme.Get(msg.Name); ok {
			m.applyPaletteColors(pal)
		}
		return m, nil

	case settings.SaveMsg:
		m.applyRuntimeSettings()
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(msg)
		if msg.Err != nil {
			return m, tea.Batch(cmd, m.notify(toast.Error, "Settings not saved", msg.Err.Error(), toast.Options{}))
		}
		return m, tea.Batch(cmd, m.notify(toast.Success, "Settings saved", "Preferences are up to date.", toast.Options{}))

	case settings.ProviderTestMsg:
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(msg)
		if msg.Err != nil {
			return m, tea.Batch(cmd, m.notify(toast.Error, "Provider test failed", msg.Provider+": "+msg.Err.Error(), toast.Options{}))
		}
		detail := msg.Provider
		if msg.Latency > 0 {
			detail = msg.Provider + " · " + msg.Latency.Round(time.Millisecond).String()
		}
		return m, tea.Batch(cmd, m.notify(toast.Success, "Provider connected", detail, toast.Options{}))

	case settings.DetectMsg:
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(msg)
		return m, cmd

	case settings.AIProviderChangedMsg:
		// The coach manager is shared with the workspace by pointer, so the
		// swap is visible everywhere without recreating either model.
		if m.coachMgr != nil {
			local, cloud := buildCoachProviders(m.config)
			m.coachMgr.SetProviders(local, cloud)
		}
		if m.config != nil {
			// Persist explicit provider/model choices immediately. A config
			// without a directory fails Save, and that is tolerated.
			_ = m.config.Save()
		}
		m.applyRuntimeSettings()
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(settings.ActionResultMsg{Detail: "Socratic Coach provider ready"})
		return m, cmd

	case settings.AuthRequestMsg:
		return m, m.authenticateCmd(msg)

	case settings.GoogleAuthRequestMsg:
		return m, m.googleAuthCmd()

	case accountAuthResultMsg:
		var cmd tea.Cmd
		if msg.err != nil {
			m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{Err: msg.err})
			if m.currentView != ViewSettings {
				return m, tea.Batch(cmd, m.toast("Sign-in failed: "+msg.err.Error()))
			}
			return m, cmd
		}
		var profileCmd tea.Cmd
		if m.identity != nil {
			next, cmd, err := m.switchForLogin(msg.session.UID)
			if err != nil {
				m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{Err: fmt.Errorf("local profile: %w", err)})
				return m, tea.Batch(cmd, m.notify(toast.Error, "Local profile unavailable", err.Error(), toast.Options{}))
			}
			m = next
			profileCmd = cmd
		}
		m.account = msg.session
		m.lastSynced = time.Time{}
		m.leaderboard = nil
		m.leaderboardLoaded = false
		m.leaderboardLoading = false
		m.leaderboardErr = ""
		if settings.SyncEnabled(m.config) {
			m.syncNote = "Ready to sync · manual"
		} else {
			m.syncNote = "Cloud sync is off · progress stays on this device"
		}
		if m.syncMeta != nil && m.syncMeta.UID == msg.session.UID {
			m.lastSynced = time.Unix(m.syncMeta.LastSynced, 0)
			if settings.SyncEnabled(m.config) && m.syncMeta.Note != "" {
				m.syncNote = m.syncMeta.Note
			}
		}
		m.refreshLeaderboardView()
		m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{Info: m.accountInfo()})
		signedIn := m.notify(toast.Success, "Signed in", m.account.Email, toast.Options{})
		return m, tea.Batch(profileCmd, cmd, m.persistSessionCmd(), m.publishLeaderboardCmd(), signedIn)

	case storedSessionLoadedMsg:
		if msg.err != nil {
			if errors.Is(msg.err, ErrNoStoredSession) {
				return m, nil
			}
			// A read or decrypt failure is not proof that the saved login is
			// invalid. Keep the vault and retry once before falling back to the
			// signed-out account page; a later launch may recover cleanly.
			if msg.attempt < 1 && m.sessionStore != nil {
				store := m.sessionStore
				attempt := msg.attempt + 1
				return m, tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
					session, err := store.Load()
					return storedSessionLoadedMsg{session: session, err: err, attempt: attempt}
				})
			}
			m.syncNote = "Remembered session could not be read; the vault was kept."
			var cmd tea.Cmd
			m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{
				Info:    settings.AccountInfo{},
				Message: "Remembered session could not be read; the encrypted vault was kept.",
			})
			notice := m.notify(toast.Warning, "Remembered session unavailable",
				"The encrypted vault was kept. Restart OpenLeet to retry.", toast.Options{})
			return m, tea.Batch(cmd, notice)
		}
		var profileCmd tea.Cmd
		if m.identity != nil {
			next, cmd, err := m.switchForLogin(msg.session.UID)
			if err != nil {
				return m, m.notify(toast.Error, "Local profile unavailable", err.Error(), toast.Options{})
			}
			m = next
			profileCmd = cmd
		}
		// Paint the remembered identity before the network round-trip so a
		// slow or offline refresh never looks like a signed-out device.
		m.account = &accountSession{
			UID:          msg.session.UID,
			Email:        msg.session.Email,
			DisplayName:  msg.session.DisplayName,
			PublicName:   msg.session.PublicName,
			RefreshToken: msg.session.RefreshToken,
			Plan:         msg.session.Plan,
			SignedInAt:   msg.session.SignedInAt,
		}
		m.lastSynced = time.Time{}
		if m.syncMeta != nil && m.syncMeta.UID == msg.session.UID {
			m.lastSynced = time.Unix(m.syncMeta.LastSynced, 0)
		}
		m.syncNote = "Restoring session…"
		m.settingsView.SetAccount(m.accountInfo())
		refreshCmd := m.refreshStoredSessionCmd(msg.session, "startup")
		if profileCmd == nil {
			return m, refreshCmd
		}
		return m, tea.Batch(profileCmd, refreshCmd)

	case sessionRefreshResultMsg:
		if msg.err != nil {
			if errors.Is(msg.err, network.ErrSessionExpired) {
				m.account = nil
				m.lastSynced = time.Time{}
				m.syncNote = ""
				m.leaderboard = nil
				m.leaderboardLoaded = false
				m.leaderboardLoading = false
				m.leaderboardErr = ""
				m.refreshLeaderboardView()
				var cmd tea.Cmd
				m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{
					Info:    settings.AccountInfo{},
					Message: "Saved session expired; sign in again.",
				})
				var guestCmd tea.Cmd
				if m.identity != nil {
					if next, guest, err := m.switchToGuest(); err == nil {
						m = next
						guestCmd = guest
					}
				}
				expired := m.notify(toast.Warning, "Session expired", "Sign in again to resume sync.", toast.Options{})
				return m, tea.Batch(cmd, m.clearStoredSessionCmd(), expired, guestCmd)
			}
			if m.account != nil {
				m.syncNote = "Signed in offline · token refresh pending"
				m.settingsView.SetAccount(m.accountInfo())
			}
			return m, nil
		}

		m.account = msg.session
		if settings.SyncEnabled(m.config) {
			m.syncNote = "Session restored · ready to sync"
		} else {
			m.syncNote = "Cloud sync is off · progress stays on this device"
		}
		if m.syncMeta != nil && m.syncMeta.UID == msg.session.UID {
			m.lastSynced = time.Unix(m.syncMeta.LastSynced, 0)
			if settings.SyncEnabled(m.config) && m.syncMeta.Note != "" {
				m.syncNote = m.syncMeta.Note
			}
		}
		m.refreshLeaderboardView()
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{Info: m.accountInfo()})
		batch := []tea.Cmd{cmd, m.persistSessionCmd()}
		if msg.origin == "sync" {
			// The failed sync already told the user the session expired. Retry
			// it automatically with the renewed token.
			batch = append(batch, m.syncNowCmd())
		} else {
			batch = append(batch, m.publishLeaderboardCmd())
		}
		return m, tea.Batch(batch...)

	case sessionPersistResultMsg:
		if msg.err != nil {
			if msg.action == "clear" {
				return m, m.notify(toast.Warning, "Signed out", "The saved session could not be removed from this device.", toast.Options{})
			}
			return m, m.notify(toast.Error, "Session not saved", "This device could not save the signed-in session.", toast.Options{})
		}
		return m, nil

	case profileSyncResultMsg:
		m.adoptUserState(msg.state)
		if msg.err != nil && errors.Is(msg.err, network.ErrUnauthorized) {
			if m.account != nil && strings.TrimSpace(m.account.RefreshToken) != "" &&
				(m.account.RefreshedAt.IsZero() || time.Since(m.account.RefreshedAt) > 30*time.Second) {
				m.syncNote = "Session expired · refreshing token…"
				m.settingsView.SetAccount(m.accountInfo())
				return m, m.refreshCurrentSessionCmd("sync")
			}
			// An unauthorized Firestore call is not proof that the refresh
			// token was rejected. Keep the remembered identity and let the
			// explicit refresh path decide whether the session is expired.
			m.syncNote = "Sync needs re-authentication · saved session kept"
			if m.account != nil {
				m.settingsView.SetAccount(m.accountInfo())
			}
			var cmd tea.Cmd
			m.settingsView, cmd = m.settingsView.Update(settings.ActionResultMsg{
				Detail: "Sync needs re-authentication; the saved session was kept.",
				Err:    msg.err,
			})
			notice := m.notify(toast.Warning, "Sync needs re-authentication",
				"Your saved session was kept.", toast.Options{})
			return m, tea.Batch(cmd, notice)
		}
		if !msg.at.IsZero() {
			m.lastSynced = msg.at
		}
		if msg.err == nil && !msg.at.IsZero() && m.account != nil {
			m.syncMeta = &storage.SyncMeta{
				UID:        m.account.UID,
				LastSynced: msg.at.Unix(),
				Note:       msg.detail,
			}
		}
		if msg.err != nil {
			m.syncNote = "Sync failed: " + msg.err.Error()
			if msg.detail != "" {
				m.syncNote += " · " + msg.detail
			}
		} else {
			m.syncNote = msg.detail
		}
		if m.account != nil {
			m.settingsView.SetAccount(m.accountInfo())
		}
		detail := msg.detail
		var notice tea.Cmd
		if msg.err != nil {
			if detail == "" {
				detail = msg.err.Error()
			}
			notice = m.notify(toast.Error, "Sync failed", detail, toast.Options{
				ActionHint: "enter retry",
				ActionID:   "retry_sync",
			})
		} else {
			if detail == "" {
				detail = "Progress is up to date."
			}
			notice = m.notify(toast.Success, "Sync complete", detail, toast.Options{})
		}
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(settings.ActionResultMsg{Detail: detail, Err: msg.err})
		return m, tea.Batch(cmd, m.publishLeaderboardCmd(), notice)

	case leaderboardResultMsg:
		m.leaderboard = msg.entries
		m.leaderboardLoading = false
		m.leaderboardLoaded = msg.err == nil
		m.leaderboardErr = ""
		if msg.err != nil {
			m.leaderboardErr = msg.err.Error()
		}
		m.refreshLeaderboardView()
		return m, nil

	case leaderboardWriteResultMsg:
		// Ranking writes are best-effort. Never replace the read cache with a
		// write result; the next explicit refresh will reconcile failures.
		if msg.err != nil {
			m.leaderboardErr = msg.err.Error()
			return m, m.notify(toast.Error, "Leaderboard publish failed", msg.err.Error(), toast.Options{
				ActionHint: "enter retry",
				ActionID:   "retry_leaderboard",
			})
		}
		return m, nil

	case settings.ActionMsg:
		switch msg.Action {
		case "account_logout":
			m.account = nil
			m.lastSynced = time.Time{}
			m.syncNote = ""
			m.leaderboard = nil
			m.leaderboardLoaded = false
			m.leaderboardLoading = false
			m.leaderboardErr = ""
			m.accountMenuOpen = false
			m.refreshLeaderboardView()
			var cmd tea.Cmd
			m.settingsView, cmd = m.settingsView.Update(settings.AuthResultMsg{
				Info:    settings.AccountInfo{},
				Message: "Signed out; the account profile was kept and a guest profile is active.",
			})
			var guestCmd tea.Cmd
			if m.identity != nil {
				if next, guest, err := m.switchToGuest(); err == nil {
					m = next
					guestCmd = guest
				}
			}
			signedOut := m.notify(toast.Info, "Signed out", "Your account profile was kept; a guest profile is active.", toast.Options{})
			return m, tea.Batch(cmd, m.clearStoredSessionCmd(), signedOut, guestCmd)
		case "account_sync":
			if !settings.SyncEnabled(m.config) {
				m.syncNote = "Cloud sync is off · enable it in Sync settings"
				m.settingsView.SetAccount(m.accountInfo())
				var cmd tea.Cmd
				m.settingsView, cmd = m.settingsView.Update(settings.ActionResultMsg{Detail: m.syncNote})
				return m, tea.Batch(cmd, m.notify(toast.Warning, "Cloud sync is off", "Enable it in Settings > Account.", toast.Options{}))
			}
			if m.account != nil && strings.TrimSpace(m.account.IDToken) == "" && strings.TrimSpace(m.account.RefreshToken) != "" {
				m.syncNote = "Refreshing saved session…"
				m.settingsView.SetAccount(m.accountInfo())
				return m, m.refreshCurrentSessionCmd("sync")
			}
			return m, m.syncNowCmd()
		}
		return m, m.runSettingsAction(msg.Action)

	case dataActionResultMsg:
		m.adoptUserState(msg.state)
		if msg.err == nil && msg.action == "data_delete_local" {
			m.syncMeta = nil
			m.lastSynced = time.Time{}
			m.syncNote = ""
		}
		if m.account != nil {
			m.settingsView.SetAccount(m.accountInfo())
		}
		if msg.settingsReset {
			m.applyPaletteColors(theme.Resolve("openleet"))
			m.workspace.SetLanguage(m.config.DefaultLanguage)
			m.homeView.SetActiveLanguage(m.config.DefaultLanguage)
			m.settingsView = settings.New(m.config, m.palette, m.width, m.height)
			m.settingsView.OpenSection(settings.SectionData)
		}
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(settings.ActionResultMsg{
			Action: msg.action,
			Detail: msg.detail,
			Err:    msg.err,
		})
		return m, tea.Batch(cmd, m.publishLeaderboardCmd())

	case home.OpenCommandsMsg:
		return m.openCommandPalette(), nil

	case home.PaletteSelectionMsg:
		switch msg.Action {
		case home.ActionRoadmap:
			m.currentView = ViewRoadmap
			return m, nil

		case home.ActionOpenProblem:
			if msg.Problem != nil {
				for i, p := range m.problems {
					if p.ID == msg.Problem.ID {
						m.selectedIndex = i
						break
					}
				}
			}
			return m.switchToSolveView(m.selectedIndex)

		case home.ActionViewStats:
			return m.enterStats()

		case home.ActionThemes:
			// The theme gallery moved into Settings > Appearance. F4 and the
			// footer hint keep a one-press route straight to it.
			return m.openSettings("appearance"), nil

		case home.ActionSettings:
			return m.openSettings(msg.Argument), nil
		}
		return m, nil

	case mapview.OpenProblemMsg:
		if msg.Problem != nil {
			for i, p := range m.problems {
				if p.ID == msg.Problem.ID {
					m.selectedIndex = i
					break
				}
			}
		}
		return m.switchToSolveView(m.selectedIndex)

	case mapview.ProblemSelectedMsg:
		if msg.Problem != nil {
			for i, p := range m.problems {
				if p.ID == msg.Problem.ID {
					m.selectedIndex = i
					break
				}
			}
		}
		return m, nil

	case workspace.ExitWorkspaceMsg:
		m.currentView = ViewRoadmap
		return m, nil

	case workspace.LanguageChangedMsg:
		m.setDefaultLanguage(msg.Language)
		return m, nil

	case workspace.LanguageSwitchedMsg:
		m.setDefaultLanguage(msg.Language)
		if msg.HasBuffer && msg.ProblemID != "" {
			var cmd tea.Cmd
			m.workspace, cmd = m.workspace.Update(workspace.BufferLoadedMsg{
				ProblemID: msg.ProblemID,
				Content:   msg.Content,
				Language:  msg.Language,
			})
			return m, cmd
		}
		return m, nil

	case workspace.CodeSavedMsg:
		var cmd tea.Cmd
		m.workspace, cmd = m.workspace.Update(msg)
		if msg.Err != nil {
			return m, tea.Batch(cmd, m.notify(toast.Error, "Save failed", msg.Err.Error(), toast.Options{}))
		}
		return m, tea.Batch(cmd, m.notify(toast.Success, "Saved", "Your solution is on disk.", toast.Options{}))

	case workspace.BufferLoadedMsg, workspace.CodeFormattedMsg,
		workspace.CustomTestRequestedMsg, workspace.CustomTestCompletedMsg:
		// These messages run off the UI goroutine and complete even when the
		// user has moved to another section. Route them back into the workspace
		// so the editor can adopt a persisted buffer, install a beautified
		// buffer, and resolve custom fixtures.
		var cmd tea.Cmd
		m.workspace, cmd = m.workspace.Update(msg)
		return m, cmd

	case coach.CoachStreamStartedMsg:
		// When the bus is attached the token channels are drained by a
		// goroutine and re-enter through the pump as batches. Clearing the
		// channels stops the console from scheduling a blocking
		// WaitForNextToken command on the UI goroutine.
		if m.bus != nil && m.streams != nil && msg.Channels.Tokens != nil {
			m.streams.consume(msg.StreamID, msg.Channels)
			msg.Channels = coach.StreamChannels{}
		}
		var cmd tea.Cmd
		m.workspace, cmd = m.workspace.Update(msg)
		return m, cmd

	case coach.CoachTokenMsg, coach.CoachTokenBatchMsg, coach.CoachStreamDoneMsg, coach.CoachStreamErrorMsg:
		var cmd tea.Cmd
		m.workspace, cmd = m.workspace.Update(msg)
		return m, cmd

	case workspace.TestsCompletedMsg:
		var cmd tea.Cmd
		var publish tea.Cmd
		m.workspace, cmd = m.workspace.Update(msg)
		if msg.Result != nil && msg.Result.IsSuccessful() && len(m.problems) > m.selectedIndex {
			p := m.problems[m.selectedIndex]
			lang := m.workspace.Language()
			m.userState.MarkSolved(p.ID, lang, msg.Result.Code)
			m.homeView.SetUserState(m.userState)
			m.mapView.SetUserState(m.userState)
			m.statsView.SetUserState(m.userState)
			_ = m.storage.SaveUserState(m.userState)
			publish = m.publishLeaderboardCmd()
		}
		var notice tea.Cmd
		switch {
		case msg.Err != nil:
			notice = m.notify(toast.Error, "Tests failed", msg.Err.Error(), toast.Options{})
		case msg.Result != nil:
			title := "Tests passed"
			kind := toast.Success
			if !msg.Result.IsSuccessful() {
				title = "Tests failed"
				kind = toast.Warning
			}
			notice = m.notify(kind, title, fmt.Sprintf("%d/%d passed", msg.Result.PassedTests, msg.Result.TotalTests), toast.Options{})
		}
		return m, tea.Batch(cmd, publish, notice)

	case ExecutionCompletedMsg:
		m.isRunning = false
		m.lastResult = msg.Result
		var publish tea.Cmd
		if msg.Result != nil && msg.Result.IsSuccessful() && len(m.problems) > m.selectedIndex {
			p := m.problems[m.selectedIndex]
			lang := m.workspace.Language()
			m.userState.MarkSolved(p.ID, lang, msg.Result.Code)
			m.homeView.SetUserState(m.userState)
			m.mapView.SetUserState(m.userState)
			m.statsView.SetUserState(m.userState)
			_ = m.storage.SaveUserState(m.userState)
			publish = m.publishLeaderboardCmd()
		}
		var notice tea.Cmd
		if msg.Result != nil {
			if msg.Result.IsSuccessful() {
				notice = m.notify(toast.Success, "Run complete", fmt.Sprintf("%d/%d tests passed", msg.Result.PassedTests, msg.Result.TotalTests), toast.Options{})
			} else {
				detail := strings.TrimSpace(msg.Result.StdErr)
				if detail == "" {
					detail = fmt.Sprintf("%d/%d tests passed", msg.Result.PassedTests, msg.Result.TotalTests)
				}
				notice = m.notify(toast.Warning, "Run finished with failures", detail, toast.Options{})
			}
		}
		return m, tea.Batch(publish, notice)

	case tea.MouseMsg:
		if m.accountMenuOpen {
			if handled, next, cmd := m.handleAccountMenuMouse(msg); handled {
				return next, cmd
			}
		}
		if m.quick.paletteOpen {
			var cmd tea.Cmd
			m.quick.palette, cmd = m.quick.palette.Update(msg)
			return m, cmd
		}
		if m.quick.dialogOpen {
			var cmd tea.Cmd
			m.quick.dialog, cmd = m.quick.dialog.Update(msg)
			return m, cmd
		}
		// The toast host owns pointer input over its cards. Hover pauses the
		// expiry timer; a body click dismisses; the action span runs its action.
		if m.notifier != nil {
			switch msg.Action {
			case tea.MouseActionMotion:
				if hit, ok := m.toastHitAt(msg.X, msg.Y); ok {
					if m.toastHoverID != hit.ID {
						now := time.Now()
						if m.toastHoverID != 0 {
							m.notifier.SetHovered(m.toastHoverID, false, now)
						}
						m.toastHoverID = hit.ID
						m.notifier.SetHovered(hit.ID, true, now)
					}
					return m, nil
				}
				if m.toastHoverID != 0 {
					m.notifier.SetHovered(m.toastHoverID, false, time.Now())
					m.toastHoverID = 0
				}
			case tea.MouseActionPress:
				if msg.Button == tea.MouseButtonLeft {
					if hit, ok := m.toastHitAt(msg.X, msg.Y); ok {
						action := m.toastAction(hit.ID)
						m.notifier.Dismiss(hit.ID, time.Now())
						if m.toastHoverID == hit.ID {
							m.toastHoverID = 0
						}
						if hit.Action {
							return m.handleToastAction(action)
						}
						return m, nil
					}
				}
			}
		}
		press := msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft

		// 1. Shared chrome row: the breadcrumb segments and the Settings +
		// account chips live on one hit map across home, roadmap, solve and
		// settings, so the header behaves identically on every screen.
		switch m.currentView {
		case ViewHome, ViewRoadmap, ViewSolve, ViewSettings:
			if msg.Y == 0 {
				switch msg.Action {
				case tea.MouseActionMotion:
					m.topBarHover = topBarTargetNone
					for _, hit := range m.topBarHits() {
						if msg.X >= hit.start && msg.X <= hit.end {
							m.topBarHover = hit.target
							break
						}
					}
					return m, nil
				case tea.MouseActionRelease:
					m.topBarPressed = topBarTargetNone
					return m, nil
				}
				if press {
					for _, hit := range m.topBarHits() {
						if msg.X < hit.start || msg.X > hit.end {
							continue
						}
						m.topBarPressed = hit.target
						m.topBarFocus = hit.target
						switch hit.target {
						case topBarTargetSettings:
							m.topBarPressed = topBarTargetNone
							m.topBarFocus = topBarTargetNone
							return m.openSettings(""), nil
						case topBarTargetAccount:
							m.topBarPressed = topBarTargetNone
							m.topBarFocus = topBarTargetNone
							return m.openAccountMenu(), nil
						case topBarTargetBreadcrumb:
							m.topBarPressed = topBarTargetNone
							m.topBarFocus = topBarTargetNone
							if hit.view == ViewSolve {
								return m.switchToSolveView(m.selectedIndex)
							}
							m.currentView = hit.view
							return m, nil
						}
					}
				}
				return m, nil
			}
		}

		// 2. Delegate to the active view. Views rendered under the chrome pass
		// pointer coordinates shifted by the header and spacer rows.
		switch m.currentView {
		case ViewHome:
			var cmd tea.Cmd
			adjustedMsg := tea.MouseMsg{
				X:      msg.X,
				Y:      msg.Y - 1,
				Button: msg.Button,
				Action: msg.Action,
				Alt:    msg.Alt,
				Ctrl:   msg.Ctrl,
			}
			m.homeView, cmd = m.homeView.Update(adjustedMsg)
			return m, cmd

		case ViewRoadmap:
			var cmd tea.Cmd
			adjustedMsg := tea.MouseMsg{
				X:      msg.X,
				Y:      msg.Y - 1,
				Button: msg.Button,
				Action: msg.Action,
				Alt:    msg.Alt,
				Ctrl:   msg.Ctrl,
			}
			m.mapView, cmd = m.mapView.Update(adjustedMsg)
			return m, cmd

		case ViewSolve:
			var cmd tea.Cmd
			adjustedMsg := tea.MouseMsg{
				X:      msg.X,
				Y:      msg.Y - 1,
				Button: msg.Button,
				Action: msg.Action,
				Alt:    msg.Alt,
				Ctrl:   msg.Ctrl,
			}
			m.workspace, cmd = m.workspace.Update(adjustedMsg)
			return m, cmd

		case ViewStats:
			// Clicking the wordmark row returns to the landing page.
			if press && msg.Y == 0 && msg.X < statsBreadcrumbWidth {
				m.currentView = ViewHome
			}
			return m, nil

		case ViewSettings:
			var cmd tea.Cmd
			adjustedMsg := tea.MouseMsg{
				X:      msg.X,
				Y:      msg.Y - 1,
				Button: msg.Button,
				Action: msg.Action,
				Alt:    msg.Alt,
				Ctrl:   msg.Ctrl,
			}
			m.settingsView, cmd = m.settingsView.Update(adjustedMsg)
			return m, cmd
		}

	case tea.KeyMsg:
		keyStr := msg.String()
		if m.quitArmed && keyStr != "q" && keyStr != "ctrl+c" {
			m.quitArmed = false
			if m.quitToastID != 0 && m.notifier != nil {
				m.notifier.Dismiss(m.quitToastID, time.Now())
			}
			m.quitToastID = 0
		}

		// The account dropdown owns the keyboard while it is open.
		if m.accountMenuOpen {
			return m.accountMenuKey(msg)
		}

		// 0. The command palette and quick dialogs overlay the active section.
		// They own the keyboard while open and never change currentView.
		if m.quick.paletteOpen || m.quick.dialogOpen {
			return m.handleQuickKey(msg)
		}
		// 0a. The roadmap's inline command bar owns the keyboard while open.
		if m.currentView == ViewRoadmap && m.mapView.IsCommandMode() {
			if keyStr == "ctrl+c" {
				return m.requestQuit()
			}
			var cmd tea.Cmd
			m.mapView, cmd = m.mapView.Update(msg)
			return m, cmd
		}
		if keyStr == "ctrl+n" {
			return m.openNotifications()
		}
		if keyStr == "ctrl+p" {
			return m.openCommandPalette(), nil
		}
		if keyStr == "/" && m.currentView == ViewHome {
			return m.openCommandPalette(), nil
		}
		if keyStr == "?" && m.currentView != ViewSolve && m.currentView != ViewSettings {
			return m.openHelpDialog(), nil
		}
		if m.hasChrome() && keyStr == "f6" {
			m.topBarFocus = topBarTargetBreadcrumb
			m.topBarHover = topBarTargetNone
			return m, nil
		}
		if m.hasChrome() && m.topBarFocus != topBarTargetNone {
			switch keyStr {
			case "esc":
				m.topBarFocus = topBarTargetNone
				return m, nil
			case "tab":
				m.topBarFocus = nextTopBarTarget(m.topBarFocus, 1)
				return m, nil
			case "shift+tab":
				m.topBarFocus = nextTopBarTarget(m.topBarFocus, -1)
				return m, nil
			case "left", "h":
				m.topBarFocus = nextTopBarTarget(m.topBarFocus, -1)
				return m, nil
			case "right", "l":
				m.topBarFocus = nextTopBarTarget(m.topBarFocus, 1)
				return m, nil
			case "enter", " ":
				target := m.topBarFocus
				m.topBarFocus = topBarTargetNone
				switch target {
				case topBarTargetBreadcrumb:
					m.currentView = ViewHome
					return m, nil
				case topBarTargetSettings:
					return m.openSettings(""), nil
				case topBarTargetAccount:
					return m.openAccountMenu(), nil
				}
				return m, nil
			default:
				// Any other key leaves the chrome focus and continues to the
				// active view, so global shortcuts are never swallowed.
				m.topBarFocus = topBarTargetNone
			}
		}

		// Esc dismisses the newest toast only when no chrome or dialog owns the
		// keyboard. The next Esc performs the screen's normal back action.
		if keyStr == "esc" && m.topBarFocus == topBarTargetNone &&
			m.currentView != ViewSettings && m.notifier != nil {
			if m.notifier.DismissNewest(time.Now()) {
				return m, nil
			}
		}

		if keyStr == "ctrl+," || keyStr == "f5" {
			if m.currentView == ViewSettings {
				m.topBarFocus = topBarTargetSettings
				m.settingsView.SetStatus("Settings is already open")
				return m, nil
			}
			return m.openSettings(""), nil
		}
		if m.currentView == ViewSettings {
			if keyStr == "ctrl+c" {
				return m.requestQuit()
			}
			var cmd tea.Cmd
			m.settingsView, cmd = m.settingsView.Update(msg)
			return m, cmd
		}

		// 1. Dashboard is reachable from every screen with F3.
		if keyStr == "f3" && m.currentView != ViewStats {
			return m.enterStats()
		}
		if m.currentView == ViewStats {
			switch keyStr {
			case "esc", "f1":
				m.currentView = ViewHome
				return m, nil
			case "q", "ctrl+c":
				return m.requestQuit()
			case "f2":
				return m.switchToSolveView(m.selectedIndex)
			case "r":
				if m.account == nil {
					return m, nil
				}
				m.leaderboardLoading = true
				m.leaderboardLoaded = false
				m.leaderboardErr = ""
				m.refreshLeaderboardView()
				return m, m.loadLeaderboardCmd()
			}
			return m, nil
		}

		// 0b. Settings — which now hosts the theme gallery under Appearance —
		// is reachable from every screen with F4.
		if keyStr == "f4" {
			return m.openSettings("appearance"), nil
		}

		// 1. If currently on Home page, delegate to homeView
		if m.currentView == ViewHome {
			switch keyStr {
			case "ctrl+c":
				return m.requestQuit()
			case "f1":
				m.currentView = ViewRoadmap
				return m, nil
			case "f2":
				return m.switchToSolveView(m.selectedIndex)
			}
			var cmd tea.Cmd
			m.homeView, cmd = m.homeView.Update(msg)
			return m, cmd
		}

		// 2. Roadmap view key handling
		if m.currentView == ViewRoadmap {
			if keyStr == "esc" {
				m.currentView = ViewHome
				return m, nil
			}
			if keyStr == "q" || keyStr == "ctrl+c" {
				return m.requestQuit()
			}
			if keyStr == "f2" || keyStr == "shift+tab" {
				return m.switchToSolveView(m.selectedIndex)
			}
			var cmd tea.Cmd
			m.mapView, cmd = m.mapView.Update(msg)
			return m, cmd
		}

		// 4. Workspace view key handling
		if m.currentView == ViewSolve {
			switch {
			case keyStr == "esc":
				m.currentView = ViewRoadmap
				return m, nil
			case key.Matches(msg, m.keyMap.Next):
				if len(m.problems) > 0 && m.selectedIndex < len(m.problems)-1 {
					return m.switchToSolveView(m.selectedIndex + 1)
				}
				return m, nil
			case key.Matches(msg, m.keyMap.Prev):
				if len(m.problems) > 0 && m.selectedIndex > 0 {
					return m.switchToSolveView(m.selectedIndex - 1)
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.workspace, cmd = m.workspace.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// updateBatch applies a coalesced pump delivery in one Elm update pass. A burst
// of N stream events therefore costs one Update and one View, which is the
// render-side counterpart of the pump's frame cap.
func (m AppModel) updateBatch(batch eventbus.BatchMsg) (tea.Model, tea.Cmd) {
	if len(batch.Events) == 0 {
		return m, nil
	}

	cmds := make([]tea.Cmd, 0, len(batch.Events))
	for _, event := range batch.Events {
		next, cmd := m.Update(event.Msg)
		// Update only ever returns AppModel; a mismatch is a programming error
		// and must fail loudly rather than silently drop stream data.
		m = next.(AppModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View computes and renders the deterministic Neo-TUI frame. All pane geometry
// is derived here from the root canvas, on a value copy, so a frame is always
// laid out for the terminal it is about to be painted into.
func (m AppModel) View() string {
	canvas := newFrame(m.width, m.height)
	if canvas.tooSmall() {
		notice := fmt.Sprintf("Terminal too small (%dx%d)\nMinimum required: 100x30", canvas.Width, canvas.Height)
		placed := lipgloss.Place(canvas.Width, canvas.Height, lipgloss.Center, lipgloss.Center, notice)
		return fitToGrid(placed, canvas.Width, canvas.Height)
	}

	var output string
	switch m.currentView {
	case ViewHome:
		m.homeView.SetDimensions(canvas.Width, canvas.Height-2)
		output = lipgloss.JoinVertical(lipgloss.Left,
			m.renderTopBar(), m.homeView.View(), m.renderFooterBar())
	case ViewRoadmap:
		m.mapView.SetSize(canvas.Width, canvas.Height-2)
		output = lipgloss.JoinVertical(lipgloss.Left,
			m.renderTopBar(), m.mapView.View(), m.renderFooterBar())
	case ViewStats:
		m.statsView.SetSize(canvas.Width, canvas.Height)
		output = m.statsView.View()
	case ViewSettings:
		m.settingsView.SetSize(canvas.Width, canvas.Height-2)
		output = lipgloss.JoinVertical(lipgloss.Left,
			m.renderTopBar(), m.settingsView.View(), m.renderFooterBar())
	default:
		m.workspace.SetDimensions(canvas.Width, canvas.bodyHeight())
		body := m.renderSolveView(canvas.bodyHeight())
		output = lipgloss.JoinVertical(lipgloss.Left,
			m.renderTopBar(), body, m.renderFooterBar())
		// The chrome bars contain nested foreground resets, so the frame must
		// repaint its own background after every reset. Without this, a light
		// palette exposes the host terminal's dark background on the top and
		// bottom rows.
		output = uikit.PaintBackground(output, m.palette.Background)
	}
	output = uikit.PaintBackground(output, m.palette.Background)

	// Hover/focus tooltips are transient: a one-row pill spliced over the first
	// body row, so the header never grows a second line.
	output = m.overlayTopBarTooltip(output)

	// Toasts sit above the screen and below every modal. While a dialog or the
	// account dropdown owns the frame, the host is hidden and the queue waits.
	modalOpen := m.quick.paletteOpen || m.quick.dialogOpen || m.accountMenuOpen
	if !modalOpen && m.notifier != nil {
		output, _ = m.toastHost().Render(output, m.notifier.Visible())
	}
	if m.quick.paletteOpen {
		output = overlayPanel(output, m.quick.palette.Panel(), canvas.Width, canvas.Height, m.palette)
	}
	if m.quick.dialogOpen {
		output = overlayPanel(output, m.quick.dialog.Panel(), canvas.Width, canvas.Height, m.palette)
	}
	if m.accountMenuOpen {
		output = m.overlayAccountMenu(output)
	}

	return fitToGrid(output, canvas.Width, canvas.Height)
}

// fitToGrid forces the frame onto the exact terminal grid: extra rows and cells
// are dropped and short lines are padded, so resizing can never leave stale
// cells behind. Widths are measured in grapheme cells (never bytes), so wide
// characters and emoji are cut cleanly instead of leaving broken UTF-8.
func fitToGrid(output string, maxWidth, maxHeight int) string {
	if maxWidth <= 0 || maxHeight <= 0 {
		return ""
	}
	lines := strings.Split(output, "\n")
	if len(lines) > maxHeight {
		lines = lines[:maxHeight]
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "")
		}
		// Truncating in front of a double-width grapheme can leave one cell
		// unused; repad so the row still covers the exact grid.
		if gap := maxWidth - ansi.StringWidth(line); gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		lines[i] = line
	}
	for len(lines) < maxHeight {
		lines = append(lines, strings.Repeat(" ", maxWidth))
	}
	return strings.Join(lines, "\n")
}

// tabHit is a clickable range on the shared top bar or the roadmap breadcrumb.
type tabHit struct {
	start int
	end   int
	view  ViewMode
}

// topBarTabs renders the primary navigation tabs and reports the inclusive cell
// range each one occupies, so pointer and keyboard navigation stay in sync.
func (m AppModel) topBarTabs() (string, []tabHit) {
	tabs := []struct {
		view ViewMode
		text string
	}{
		{ViewHome, "home"},
		{ViewRoadmap, "roadmap"},
		{ViewSolve, "solve"},
	}

	var b strings.Builder
	hits := make([]tabHit, 0, len(tabs))
	x := 1 // the bar renders with one cell of left padding
	for i, tab := range tabs {
		if i > 0 {
			b.WriteString("    ")
			x += 4
		}
		marker := "○ "
		style := lipgloss.NewStyle().Foreground(m.theme.Muted)
		if m.currentView == tab.view {
			marker = "● "
			style = lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text)
		}
		rendered := style.Render(marker + tab.text)
		b.WriteString(rendered)
		width := lipgloss.Width(rendered)
		hits = append(hits, tabHit{start: x, end: x + width - 1, view: tab.view})
		x += width
	}
	return b.String(), hits
}

// breadcrumb renders the file-path-style location for the current section and
// reports the clickable ranges of its ancestors.
func (m AppModel) breadcrumb() (string, []tabHit) {
	type segment struct {
		view  ViewMode
		label string
	}
	segments := []segment{{ViewHome, "home"}}
	switch m.currentView {
	case ViewRoadmap:
		segments = append(segments, segment{ViewRoadmap, "roadmap"})
	case ViewSolve:
		segments = append(segments, segment{ViewRoadmap, "roadmap"}, segment{ViewSolve, "solve"})
	case ViewStats:
		segments = append(segments, segment{ViewStats, "stats"})
	case ViewSettings:
		segments = append(segments, segment{ViewSettings, "settings"})
	}

	separator := lipgloss.NewStyle().Foreground(m.palette.TextDim).Render("›")
	ancestor := lipgloss.NewStyle().Foreground(m.palette.TextDim)
	current := lipgloss.NewStyle().Bold(true).Foreground(m.palette.Text)

	var b strings.Builder
	hits := make([]tabHit, 0, len(segments))
	x := 1 // one cell of left padding on the app top bar
	for i, seg := range segments {
		if i > 0 {
			b.WriteString(separator)
			x++
		}
		label := seg.label
		style := ancestor
		if i == len(segments)-1 {
			style = current
		}
		rendered := style.Render(label)
		b.WriteString(rendered)
		width := lipgloss.Width(rendered)
		hits = append(hits, tabHit{start: x, end: x + width - 1, view: seg.view})
		x += width
	}

	if m.currentView == ViewSolve && len(m.problems) > m.selectedIndex {
		problem := " (" + m.problems[m.selectedIndex].Title + ")"
		b.WriteString(lipgloss.NewStyle().Foreground(m.palette.Muted).Render(problem))
	}
	return b.String(), hits
}

func (m AppModel) renderTopBar() string {
	bar, _ := m.topBarLayout()
	return bar
}

// bottomHint is one key/label pair of the per-screen footer hint list.
type bottomHint struct {
	key   string
	label string
}

// bottomHints lists the three most useful hints for the current screen. The
// Format is stable everywhere: key in text, label in text_dim, three cells
// between hints. Settings lives in the top bar, so no hint can repeat it.
func (m AppModel) bottomHints() []bottomHint {
	switch m.currentView {
	case ViewHome:
		return []bottomHint{{"ctrl+p", "commands"}, {"F3", "stats"}, {"?", "help"}}
	case ViewRoadmap:
		return []bottomHint{{"F2", "solve"}, {"F3", "stats"}, {"ctrl+p", "commands"}}
	case ViewSolve:
		return []bottomHint{{"ctrl+r", "run"}, {"ctrl+s", "save"}, {"ctrl+p", "commands"}}
	case ViewSettings:
		// Settings owns the context hints (auth, integration, provider, edit);
		// the footer renders them so the screen never adds a second band.
		raw := m.settingsView.StatusHints()
		hints := make([]bottomHint, 0, len(raw))
		for _, hint := range raw {
			key, label, found := strings.Cut(hint, " ")
			if !found {
				key, label = hint, ""
			}
			hints = append(hints, bottomHint{key: key, label: label})
		}
		return hints
	default:
		return []bottomHint{{"F2", "solve"}, {"F3", "stats"}, {"esc", "back"}}
	}
}

// bottomHintsText renders the right-side hint list in the shared format.
func (m AppModel) bottomHintsText() string {
	keyStyle := lipgloss.NewStyle().Foreground(m.palette.Text)
	labelStyle := lipgloss.NewStyle().Foreground(m.palette.TextDim)
	parts := make([]string, 0, len(m.bottomHints()))
	for _, hint := range m.bottomHints() {
		parts = append(parts, keyStyle.Render(hint.key)+" "+labelStyle.Render(hint.label))
	}
	return strings.Join(parts, "   ")
}

func (m AppModel) renderFooterBar() string {
	innerW := m.width - 2
	if innerW < 1 {
		innerW = 1
	}

	// The right hint list has the highest priority and is never clipped.
	right := m.bottomHintsText()
	rightW := lipgloss.Width(right)

	// The cwd takes at most 24 cells and always leaves room for the hints.
	cwdMax := 24
	if limit := innerW - rightW - 2; cwdMax > limit {
		cwdMax = limit
	}
	if cwdMax < 1 {
		cwdMax = 1
	}
	wd := "openleet"
	if dir, err := os.Getwd(); err == nil {
		wd = dir
	}
	left := lipgloss.NewStyle().Foreground(m.palette.TextDim).Render(uikit.Truncate(wd, cwdMax, "…"))
	leftW := lipgloss.Width(left)

	gap := innerW - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	content := uikit.FitExact(left+strings.Repeat(" ", gap)+right, innerW)
	return uikit.PaintBackground(" "+content+" ", m.palette.Background)
}

func (m AppModel) progressBar(maxWidth int) string {
	solvedCount := 0
	if m.userState != nil {
		solvedCount = len(m.userState.SolvedProblems)
	}
	totalCount := len(m.problems)
	pct := 0
	if totalCount > 0 {
		pct = (solvedCount * 100) / totalCount
	}

	label := fmt.Sprintf("%d/%d", solvedCount, totalCount)
	percent := fmt.Sprintf("%d%%", pct)
	barW := maxWidth - lipgloss.Width(label) - lipgloss.Width(percent) - 2
	if barW < 4 {
		return lipgloss.NewStyle().Foreground(m.theme.Text).Render(label + " " + percent)
	}
	filled := barW * pct / 100
	if filled > barW {
		filled = barW
	}
	bar := lipgloss.NewStyle().Foreground(m.theme.Accent).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render(strings.Repeat("░", barW-filled))
	return lipgloss.NewStyle().Foreground(m.theme.Text).Render(label) + " " + bar + " " +
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render(percent)
}

func (m AppModel) renderSolveView(availHeight int) string {
	if len(m.problems) == 0 {
		return lipgloss.Place(m.width, availHeight, lipgloss.Center, lipgloss.Center, "No problem selected.")
	}

	return m.workspace.View()
}
