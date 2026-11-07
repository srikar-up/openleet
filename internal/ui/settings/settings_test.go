package settings

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/config"
	"openleet/internal/ui/theme"
)

var errTestAuth = errors.New("invalid email or password")

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("XDG_DATA_HOME", dir)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func testModel(t *testing.T, width, height int) Model {
	t.Helper()
	return New(testConfig(t), theme.Default(), width, height)
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// TestAccountCardKeysWorkInAnyFocus pins the Account section's action keys:
// they must dispatch from the sidebar pane too — the one Settings opens with —
// and from the canvas pane, in both letter cases.
func TestAccountCardKeysWorkInAnyFocus(t *testing.T) {
	info := AccountInfo{SignedIn: true, UID: "uid-1", Email: "user@example.com", Plan: "Free"}

	open := func(t *testing.T) (m Model) {
		t.Helper()
		m = testModel(t, 140, 45)
		m.OpenSection(SectionAccount)
		m, _ = m.Update(AuthResultMsg{Info: info})
		m.focus = FocusSidebar // Settings opens with the sidebar pane focused.
		return m
	}

	for _, key := range []rune{'L', 'l'} {
		name := string(key) + " logs out"
		t.Run(name+" from sidebar", func(t *testing.T) {
			m := open(t)
			_, cmd := m.Update(keyRune(key))
			if msg, ok := cmd().(ActionMsg); !ok || msg.Action != "account_logout" {
				t.Fatalf("%s from sidebar focus dispatched %#v", name, cmd())
			}
		})
		t.Run(name+" from canvas", func(t *testing.T) {
			m := open(t)
			m.focus = FocusCanvas
			_, cmd := m.Update(keyRune(key))
			if msg, ok := cmd().(ActionMsg); !ok || msg.Action != "account_logout" {
				t.Fatalf("%s from canvas focus dispatched %#v", name, cmd())
			}
		})
	}

	t.Run("signout_l_still_moves_focus_to_canvas", func(t *testing.T) {
		m := open(t)
		m.account = AccountInfo{} // signed out
		m.focus = FocusSidebar
		next, cmd := m.Update(keyRune('l'))
		m = next
		if cmd != nil {
			t.Fatal("signed-out l must not dispatch an action")
		}
		if m.focus != FocusCanvas {
			t.Fatal("signed-out l must still move focus to the canvas")
		}
	})
}

func fieldIndex(m Model, id string) int {
	for i, field := range m.sectionFields() {
		if field.ID == id {
			return i
		}
	}
	return 0
}

func TestEnsureDefaultsMigratesLegacyFields(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Theme = "dracula"
	cfg.DefaultLanguage = "go"
	cfg.ExecutionTimeout = 9
	cfg.ShowLineNumbers = false
	cfg.SettingsVersion = 1 // config.Load stamps this before settings runs
	cfg.Settings = nil

	EnsureDefaults(cfg)

	want := map[string]string{
		"appearance.theme":        "dracula",
		"editor.default_language": "go",
		"runner.timeout":          "9",
		"editor.line_numbers":     "off",
		"ai.never_send_code":      "on",
	}
	for id, expected := range want {
		if got := cfg.Settings[id]; got != expected {
			t.Errorf("%s = %q, want %q", id, got, expected)
		}
	}
	if cfg.SettingsVersion != SchemaVersion {
		t.Fatalf("SettingsVersion = %d, want %d", cfg.SettingsVersion, SchemaVersion)
	}
}

func TestEnsureDefaultsPreservesExplicitValues(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Theme = "dracula"
	cfg.Settings = map[string]string{"appearance.theme": "opencode"}

	EnsureDefaults(cfg)

	if got := cfg.Settings["appearance.theme"]; got != "opencode" {
		t.Fatalf("explicit theme overwritten: %q", got)
	}
}

func TestIconsSettingUsesAutoMode(t *testing.T) {
	field, ok := fieldByID("appearance.icons")
	if !ok {
		t.Fatal("appearance.icons field is missing")
	}
	if field.Default != "auto" {
		t.Fatalf("default = %q, want auto", field.Default)
	}
	want := []string{"auto", "on", "off"}
	if len(field.Options) != len(want) {
		t.Fatalf("options = %#v, want %v", field.Options, want)
	}
	for i, value := range want {
		if field.Options[i].Value != value {
			t.Fatalf("option %d = %q, want %q", i, field.Options[i].Value, value)
		}
	}
}

func TestResolveSectionDeepLinks(t *testing.T) {
	cases := map[string]SectionID{
		"":                      SectionGeneral,
		"/settings":             SectionGeneral,
		"/settings ai":          SectionAI,
		"/settings theme":       SectionAppearance,
		"/settings keybindings": SectionKeys,
		"models":                SectionAI,
		"unknown":               SectionGeneral,
	}
	for input, want := range cases {
		if got := ResolveSection(input); got != want {
			t.Errorf("ResolveSection(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchFindsFieldAcrossSections(t *testing.T) {
	results := Search("temperature")
	if len(results) != 1 || results[0].ID != "ai.temperature" {
		t.Fatalf("Search(temperature) = %#v", results)
	}
	if got := Search("timeout"); len(got) == 0 {
		t.Fatal("expected runner timeout to be searchable")
	}
	if got := Search("   "); got != nil {
		t.Fatalf("blank search = %#v, want nil", got)
	}
}

func TestViewFillsExactGrid(t *testing.T) {
	sizes := []struct{ width, height int }{
		{100, 30},
		{120, 40},
		{160, 50},
		{200, 60},
	}
	for _, size := range sizes {
		m := testModel(t, size.width, size.height)
		for _, section := range Sections() {
			m.OpenSection(section.ID)
			frame := m.View()
			lines := strings.Split(frame, "\n")
			if len(lines) != size.height {
				t.Fatalf("%s at %dx%d: %d rows, want %d", section.ID, size.width, size.height, len(lines), size.height)
			}
			for i, line := range lines {
				if got := ansi.StringWidth(line); got != size.width {
					t.Fatalf("%s at %dx%d: row %d width %d, want %d", section.ID, size.width, size.height, i, got, size.width)
				}
			}
			plain := ansi.Strip(frame)
			if !strings.Contains(plain, "[ SETTINGS · "+section.Label+" ]") {
				t.Fatalf("%s: missing bracketed canvas title", section.ID)
			}
			if !strings.Contains(plain, "saved") && !strings.Contains(plain, "unsaved") {
				t.Fatalf("%s: missing status indicator", section.ID)
			}
			if !strings.Contains(plain, "/ search settings") {
				t.Fatalf("%s: missing sidebar search", section.ID)
			}
		}
	}
}

func TestSidebarShowsEverySectionAtMinimumSize(t *testing.T) {
	m := testModel(t, 100, 30)
	plain := ansi.Strip(m.View())
	for _, section := range Sections() {
		if !strings.Contains(plain, section.Label) {
			t.Errorf("section %s is not visible at 100x30", section.ID)
		}
	}
	if _, ok := m.sidebarIndexAt(28); !ok {
		t.Fatal("last sidebar row should be clickable")
	}
	if _, ok := m.sidebarIndexAt(29); ok {
		t.Fatal("status bar row must not be clickable")
	}
}

func TestViewNarrowTabLayout(t *testing.T) {
	m := testModel(t, 80, 24)
	m.OpenSection(SectionAppearance)
	frame := m.View()
	lines := strings.Split(frame, "\n")
	if len(lines) != 24 {
		t.Fatalf("%d rows, want 24", len(lines))
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != 80 {
			t.Fatalf("row %d width %d, want 80", i, got)
		}
	}
	if !strings.Contains(ansi.Strip(frame), "[THEME]") {
		t.Fatal("expected the active short tab to be bracketed")
	}
}

func TestNavigationToggleAndSave(t *testing.T) {
	m := testModel(t, 120, 40)
	m.OpenSection(SectionGeneral)
	if m.focus != FocusCanvas {
		t.Fatalf("focus = %v, want canvas", m.focus)
	}
	if m.Dirty() {
		t.Fatal("fresh settings must not be dirty")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if field := m.selectedFieldValue(); field == nil || field.ID != "general.confirm_quit" {
		t.Fatalf("down did not select confirm_quit: %#v", field)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.Dirty() {
		t.Fatal("toggling a setting should mark the model dirty")
	}
	if got := Value(m.Config(), Field{ID: "general.confirm_quit"}); got != "on" {
		t.Fatalf("confirm_quit = %q, want on", got)
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("ctrl+s should return a save command")
	}
	if _, ok := cmd().(SaveMsg); !ok {
		t.Fatal("ctrl+s command did not produce SaveMsg")
	}
	if m.Dirty() {
		t.Fatal("successful save should clear the dirty flag")
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Settings["general.confirm_quit"]; got != "on" {
		t.Fatalf("persisted confirm_quit = %q, want on", got)
	}

	m, _ = m.Update(keyRune('h'))
	if m.focus != FocusSidebar {
		t.Fatal("h should move focus to the sidebar")
	}
	m, _ = m.Update(keyRune('l'))
	if m.focus != FocusCanvas {
		t.Fatal("l should move focus to the canvas")
	}
	if got := strings.Join(m.StatusHints(), "   "); !strings.Contains(got, "j/k move   ctrl+s save   esc back") {
		t.Fatal("tabbed status hints are missing")
	}
	if got := ansi.Strip(m.View()); strings.Contains(got, "j/k move") {
		t.Fatal("settings must not render a second hint band")
	}
}

func TestSidebarClickShowsOnlySelectedSection(t *testing.T) {
	m := testModel(t, 140, 45)
	markers := map[SectionID]string{
		SectionGeneral:    "Startup screen",
		SectionAppearance: "THEME PREVIEW",
		SectionEditor:     "Tab width",
		SectionRunner:     "Python path",
		SectionAI:         "Connect an integration",
		SectionKeys:       "Save buffer",
		SectionAccount:    "Continue with Google",
		SectionData:       "Export progress",
		SectionAbout:      "VERSION",
	}
	for index, section := range Sections() {
		y := 0
		for candidate := 1; candidate < m.height-2; candidate++ {
			if got, ok := m.sidebarIndexAt(candidate); ok && got == index {
				y = candidate
				break
			}
		}
		if y == 0 {
			t.Fatalf("sidebar row for %s is not clickable", section.ID)
		}
		m, _ = m.Update(tea.MouseMsg{
			X:      5,
			Y:      y,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		if got := m.ActiveSection(); got != section.ID {
			t.Fatalf("click row %d selected %q, want %q", y, got, section.ID)
		}
		plain := ansi.Strip(m.View())
		if !strings.Contains(plain, "[ SETTINGS · "+section.Label+" ]") {
			t.Fatalf("%s title does not match the selected section", section.ID)
		}
		if !strings.Contains(plain, markers[section.ID]) {
			t.Fatalf("%s tab is missing its own card marker %q", section.ID, markers[section.ID])
		}
		for other, marker := range markers {
			if other != section.ID && strings.Contains(plain, marker) {
				t.Fatalf("%s tab leaked %s content (%q)", section.ID, other, marker)
			}
		}
	}
}

func TestTabbedCardsKeepRightEdge(t *testing.T) {
	for _, width := range []int{100, 120, 160} {
		m := testModel(t, width, 40)
		for _, section := range Sections() {
			m.OpenSection(section.ID)
			lines := strings.Split(ansi.Strip(m.View()), "\n")
			for row := 1; row < len(lines)-1; row++ {
				trimmed := strings.TrimRight(lines[row], " ")
				if trimmed == "" {
					continue
				}
				runes := []rune(trimmed)
				switch runes[len(runes)-1] {
				case '│', '╮', '╯', '┐', '┘':
				default:
					t.Fatalf("%s at %d cols: right edge clipped on row %d: %q", section.ID, width, row, lines[row])
				}
			}
		}
	}
}

func TestSearchTypingJumpsToField(t *testing.T) {
	m := testModel(t, 140, 45)
	for _, r := range "/temperature" {
		m, _ = m.Update(keyRune(r))
	}
	if !m.searchActive {
		t.Fatal("typing / should focus the search input")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.ActiveSection(); got != SectionAI {
		t.Fatalf("search jump section = %q, want ai", got)
	}
	if field := m.selectedFieldValue(); field == nil || field.ID != "ai.temperature" {
		t.Fatalf("search jump field = %#v, want ai.temperature", field)
	}
	if m.searchActive || m.query != "" {
		t.Fatal("a committed search should close and clear the input")
	}
}

func TestThemePreviewApplyAndRevert(t *testing.T) {
	m := testModel(t, 160, 50)
	m.OpenSection(SectionAppearance)
	original := m.Config().Theme
	if field := m.selectedFieldValue(); field == nil || field.Kind != KindThemeGrid {
		t.Fatalf("theme grid is not the first appearance field: %#v", field)
	}

	previewed, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd == nil {
		t.Fatal("adjusting the theme grid should dispatch a preview")
	}
	msg, ok := cmd().(PreviewThemeMsg)
	if !ok {
		t.Fatalf("preview command produced %T", msg)
	}
	if msg.Name == original {
		t.Fatal("preview should move to a different theme")
	}
	if previewed.Config().Theme != msg.Name {
		t.Fatalf("config theme = %q, want preview %q", previewed.Config().Theme, msg.Name)
	}
	if field := previewed.selectedFieldValue(); field == nil || field.Kind != KindThemeGrid {
		t.Fatal("adjusting the value must not move field selection")
	}

	reverted, cmd := previewed.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("escaping a preview should revert the theme")
	}
	revertMsg, ok := cmd().(RevertThemeMsg)
	if !ok || revertMsg.Name != original {
		t.Fatalf("revert message = %#v, want %q", revertMsg, original)
	}
	if reverted.Config().Theme != original {
		t.Fatalf("theme after revert = %q, want %q", reverted.Config().Theme, original)
	}

	applied, cmd := previewed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the theme grid should apply")
	}
	applyMsg, ok := cmd().(ApplyThemeMsg)
	if !ok || applyMsg.Name != msg.Name {
		t.Fatalf("apply message = %#v, want %q", applyMsg, msg.Name)
	}
	if !applied.Dirty() {
		t.Fatal("applying a theme should mark settings dirty")
	}
}

func TestKeybindingConflictIsRejected(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionKeys)
	m, _ = m.Update(keyRune('j')) // move to Save buffer (ctrl+s)
	if m.captureIdx != 1 {
		t.Fatalf("captureIdx = %d, want 1", m.captureIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.captureKey {
		t.Fatal("enter should start key capture")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if m.conflict == "" {
		t.Fatal("expected a conflict for ctrl+r (Run tests)")
	}
	if _, ok := m.Config().Settings["keys.save"]; ok {
		t.Fatal("conflicting binding must not be persisted")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if m.captureKey || m.conflict != "" {
		t.Fatal("a non-conflicting key should finish capture")
	}
	if got := m.Config().Settings["keys.save"]; got != "ctrl+g" {
		t.Fatalf("keys.save = %q, want ctrl+g", got)
	}
	// A per-row reset removes the override and falls back to the default.
	m, _ = m.Update(keyRune('r'))
	if _, ok := m.Config().Settings["keys.save"]; ok {
		t.Fatal("r should remove the saved override")
	}
}

func TestProviderKeyStoredEncryptedAndMasked(t *testing.T) {
	cfg := testConfig(t)
	m := New(cfg, theme.Default(), 160, 50)
	m.OpenSection(SectionAI)

	m.selectedField = fieldIndex(m, "ai.providers")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open provider detail
	if !m.providerDetail {
		t.Fatal("enter on the provider list should open the detail panel")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // start key entry
	if !m.editing {
		t.Fatal("enter on the key row should start editing")
	}
	const secret = "sk-ant-api03-super-secret-a3f9"
	m.editInput.SetValue(secret)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editing {
		t.Fatal("enter should commit the key")
	}
	masked := m.SecretMasked("anthropic")
	if !strings.HasSuffix(masked, "a3f9") || strings.Contains(masked, "super-secret") {
		t.Fatalf("masked key = %q", masked)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.ConfigDir(), "secrets.enc"))
	if err != nil {
		t.Fatalf("read secret vault: %v", err)
	}
	if strings.Contains(string(raw), "super-secret") {
		t.Fatal("plaintext secret found in vault file")
	}
	cfgRaw, err := os.ReadFile(filepath.Join(cfg.ConfigDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cfgRaw), "super-secret") {
		t.Fatal("secret written to config.json")
	}
	if !strings.Contains(ansi.Strip(m.View()), masked) {
		t.Fatal("masked key is not rendered in the provider card")
	}
}

func TestProviderProbeReportsLatencyAndError(t *testing.T) {
	m := testModel(t, 140, 45)
	m.providerStatus["anthropic"] = ProviderTestMsg{Provider: "anthropic", Latency: 42 * time.Millisecond}
	m, _ = m.Update(ProviderTestMsg{Provider: "anthropic", Latency: 42 * time.Millisecond})
	if !strings.Contains(m.status, "connected") {
		t.Fatalf("status = %q, want connected", m.status)
	}
	m, _ = m.Update(ProviderTestMsg{Provider: "anthropic", Err: os.ErrPermission})
	if !strings.Contains(m.status, "test failed") {
		t.Fatalf("status = %q, want failure", m.status)
	}
}

func TestConfirmRequiresTypedWord(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionData)
	for i := 0; i < 4; i++ { // Export, Import, Backup, Clear cache, Reset roadmap
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if field := m.selectedFieldValue(); field == nil || field.ID != "data.reset_roadmap" {
		t.Fatalf("selected field = %#v, want data.reset_roadmap", field)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirm == nil {
		t.Fatal("enter on reset roadmap should open a confirmation")
	}
	m.confirm.Input.SetValue("nope")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirm == nil {
		t.Fatal("wrong word must not dismiss the confirmation")
	}
	if m.confirm.Err == "" {
		t.Fatal("wrong word should set an inline error")
	}
	m.confirm.Input.SetValue("RESET")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirm != nil {
		t.Fatal("correct word should dismiss the confirmation")
	}
	if cmd == nil {
		t.Fatal("confirmed action should dispatch a command")
	}
	msg, ok := cmd().(ActionMsg)
	if !ok || msg.Action != "data_reset_roadmap" {
		t.Fatalf("action = %#v, want data_reset_roadmap", msg)
	}
}

func TestDeleteLocalRequiresDeleteWord(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionData)
	for i := 0; i < 6; i++ { // through Delete local data
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if field := m.selectedFieldValue(); field == nil || field.ID != "data.delete_local" {
		t.Fatalf("selected field = %#v, want data.delete_local", field)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirm == nil || m.confirm.Word != "DELETE" {
		t.Fatalf("confirmation = %#v, want DELETE", m.confirm)
	}
	m.confirm.Input.SetValue("RESET")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirm == nil {
		t.Fatal("RESET must not confirm a DELETE action")
	}
	m.confirm.Input.SetValue("DELETE")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("confirmed delete should dispatch a command")
	}
	if msg, ok := cmd().(ActionMsg); !ok || msg.Action != "data_delete_local" {
		t.Fatalf("action = %#v, want data_delete_local", msg)
	}
}

func TestAppearanceSelectionStaysInsideTab(t *testing.T) {
	m := testModel(t, 100, 30)
	m.OpenSection(SectionAppearance)
	if field := m.selectedFieldValue(); field == nil || field.Kind != KindThemeGrid {
		t.Fatalf("first appearance field = %#v, want theme grid", field)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if field := m.selectedFieldValue(); field == nil || field.ID != "appearance.accent" {
		t.Fatalf("down from theme grid selected %#v, want appearance.accent", field)
	}
	if got := m.ActiveSection(); got != SectionAppearance {
		t.Fatalf("active section changed to %q after moving rows", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if field := m.selectedFieldValue(); field == nil || field.Kind != KindThemeGrid {
		t.Fatalf("up did not return to the theme grid: %#v", field)
	}
}

func TestSectionSwitchKeysChangeOnlyTheTab(t *testing.T) {
	m := testModel(t, 120, 40)
	m.OpenSection(SectionGeneral)
	m, _ = m.Update(keyRune('>'))
	if got := m.ActiveSection(); got != SectionAppearance {
		t.Fatalf("after > active section = %q, want appearance", got)
	}
	if !strings.Contains(ansi.Strip(m.View()), "[ SETTINGS · APPEARANCE ]") {
		t.Fatal("title did not follow the selected section")
	}
	m, _ = m.Update(keyRune('<'))
	if got := m.ActiveSection(); got != SectionGeneral {
		t.Fatalf("after < active section = %q, want general", got)
	}
}

func TestTabbedCanvasShowsOnlySelectedSection(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAI)
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Connect an integration") {
		t.Fatal("AI tab is missing its own cards")
	}
	if strings.Contains(plain, "Startup screen") || strings.Contains(plain, "Export progress") {
		t.Fatal("AI tab rendered cards from another section")
	}

	m.OpenSection(SectionData)
	plain = ansi.Strip(m.View())
	if !strings.Contains(plain, "Export progress") {
		t.Fatal("Data tab is missing its own cards")
	}
	if strings.Contains(plain, "Connect an integration") {
		t.Fatal("Data tab rendered AI cards")
	}
}

func TestSelectAdjustsLeftAndRight(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionGeneral)
	original := Value(m.Config(), *m.selectedFieldValue())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	afterRight := Value(m.Config(), *m.selectedFieldValue())
	if afterRight == original {
		t.Fatal("right arrow should advance the select value")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if got := Value(m.Config(), *m.selectedFieldValue()); got != original {
		t.Fatalf("left arrow restored %q, want %q", got, original)
	}
}

func TestMouseClickTogglesRow(t *testing.T) {
	m := testModel(t, 100, 30)
	m.OpenSection(SectionGeneral)
	y, ok := m.fieldScreenY("general.confirm_quit")
	if !ok {
		t.Fatal("could not locate the Confirm on quit row")
	}
	x := m.fieldValueColumnX() + 1
	m, _ = m.Update(tea.MouseMsg{
		X:      x,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if field := m.selectedFieldValue(); field == nil || field.ID != "general.confirm_quit" {
		t.Fatalf("click selected %#v, want general.confirm_quit", field)
	}
	if got := Value(m.Config(), Field{ID: "general.confirm_quit"}); got != "on" {
		t.Fatalf("confirm_quit = %q, want on after click", got)
	}
}

func TestAccountAuthFormEmitsRequestAndAdoptsSession(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.auth == nil {
		t.Fatal("enter on the signed-out account panel should open the auth form")
	}
	m.auth.Email.SetValue("user@example.com")
	m.auth.Password.SetValue("secret123")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("submitting the form should dispatch an auth request")
	}
	req, ok := cmd().(AuthRequestMsg)
	if !ok || req.Mode != AuthSignIn || req.Email != "user@example.com" || req.Password != "secret123" {
		t.Fatalf("auth request = %#v", req)
	}
	if !m.auth.Busy {
		t.Fatal("form should be busy while authentication runs")
	}

	m, _ = m.Update(AuthResultMsg{Err: errTestAuth})
	if m.auth == nil || m.auth.Err == "" || m.auth.Busy {
		t.Fatal("failed auth should keep the form open with an inline error")
	}

	info := AccountInfo{SignedIn: true, UID: "uid-1", Email: "user@example.com", DisplayName: "user", Plan: "Free"}
	m, _ = m.Update(AuthResultMsg{Info: info})
	if m.auth != nil {
		t.Fatal("successful auth should close the form")
	}
	if !m.account.SignedIn || m.account.Email != "user@example.com" {
		t.Fatalf("account = %#v", m.account)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "user@example.com") || !strings.Contains(view, "PLAN  Free") {
		t.Fatal("signed-in profile card is missing identity details")
	}

	m, cmd = m.Update(keyRune('L'))
	if cmd == nil {
		t.Fatal("L should request a logout")
	}
	if msg, ok := cmd().(ActionMsg); !ok || msg.Action != "account_logout" {
		t.Fatalf("logout action = %#v", msg)
	}
}

func TestAccountSignUpFormValidatesAndToggles(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.auth == nil || m.auth.Mode != AuthSignUp {
		t.Fatal("ctrl+t should switch the form to sign-up mode")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.auth.Focus != authFocusUsername {
		t.Fatalf("focus = %d, want username", m.auth.Focus)
	}
	m.auth.Email.SetValue("new@example.com")
	m.auth.Username.SetValue("newuser")
	m.auth.Password.SetValue("123")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.auth.Err == "" || m.auth.Busy {
		t.Fatal("short sign-up password should be rejected inline")
	}
	m.auth.Password.SetValue("secret123")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("valid sign-up should dispatch a request")
	}
	if req, ok := cmd().(AuthRequestMsg); !ok || req.Mode != AuthSignUp || req.Username != "newuser" {
		t.Fatalf("sign-up request = %#v", req)
	}
}

func TestAccountAuthOverlayFillsExactGrid(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {160, 50}} {
		m := testModel(t, size.width, size.height)
		m.OpenSection(SectionAccount)
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		frame := m.View()
		lines := strings.Split(frame, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d: %d rows", size.width, size.height, len(lines))
		}
		for i, line := range lines {
			if got := ansi.StringWidth(line); got != size.width {
				t.Fatalf("%dx%d row %d width %d", size.width, size.height, i, got)
			}
		}
		if !strings.Contains(ansi.Strip(frame), "SIGN IN") {
			t.Fatal("auth overlay is missing its title")
		}
	}
}

func TestSignedInAccountShowsSyncCard(t *testing.T) {
	m := testModel(t, 160, 50)
	m.OpenSection(SectionAccount)
	now := time.Now().Add(-time.Hour)
	m.SetAccount(AccountInfo{
		SignedIn: true, UID: "uid-1", Email: "user@example.com", DisplayName: "neo",
		Plan: "Free", SyncEnabled: true, SyncNote: "Synced 4 problems · 1 new",
		LastSynced: now, SolvedCount: 4, TotalPoints: 40,
	})
	plain := ansi.Strip(m.View())
	for _, want := range []string{
		"PROFILE", "SYNC", "Sync now", "Synced 4 problems", "4 solved", "40 points",
		"Cloud sync", "Conflict policy", "Newest wins",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("signed-in account view missing %q", want)
		}
	}
	m, cmd := m.Update(keyRune('s'))
	if cmd == nil {
		t.Fatal("s should request a sync")
	}
	if msg, ok := cmd().(ActionMsg); !ok || msg.Action != "account_sync" {
		t.Fatalf("sync action = %#v", msg)
	}
}

func TestSignedInAccountFitsMinimumSize(t *testing.T) {
	m := testModel(t, 100, 30)
	m.OpenSection(SectionAccount)
	m.SetAccount(AccountInfo{
		SignedIn: true, UID: "uid-1", Email: "user@example.com", DisplayName: "neo",
		Plan: "Free", SyncEnabled: true, SyncNote: "never synced · newest wins",
		SolvedCount: 2, TotalPoints: 20,
	})
	plain := ansi.Strip(m.View())
	for _, want := range []string{"PROFILE & SESSIONS", "SYNC", "Sync now", "2 solved"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("100x30 signed-in view missing %q", want)
		}
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 30 {
		t.Fatalf("%d rows, want 30", len(lines))
	}
}

func TestThemeHitTestMatchesRenderedRows(t *testing.T) {
	m := testModel(t, 160, 50)
	m.OpenSection(SectionAppearance)
	cols, cardW := themeGridLayout(m.sectionWidth())
	originX, originY := m.themeGridOrigin()
	for index := 0; index < len(m.themes); index++ {
		row := index / cols
		col := index % cols
		x := originX + col*(cardW+themeCardGap) + 1
		y := originY + row*themeCardHeight + 2
		got, ok := m.themeAt(x, y)
		if !ok || got != index {
			t.Fatalf("themeAt(%d,%d) = %d,%v want %d", x, y, got, ok, index)
		}
	}
}

func TestProviderHitTestMatchesRenderedRows(t *testing.T) {
	m := testModel(t, 160, 50)
	m.OpenSection(SectionAI)
	originY := m.providerListOriginY()
	x := m.canvasOriginX() + 4
	for index := 0; index < len(m.providers); index++ {
		y := originY + index*providerCardHeight + 3
		got, ok := m.providerAt(x, y)
		if !ok || got != index {
			t.Fatalf("providerAt(%d,%d) = %d,%v want %d", x, y, got, ok, index)
		}
	}
}

func TestSyncPreferenceDefaultsAndHelpers(t *testing.T) {
	cfg := config.DefaultConfig()
	EnsureDefaults(cfg)

	if !SyncEnabled(cfg) {
		t.Fatal("expected SyncEnabled to be true by default")
	}
	if got := ConflictPolicy(cfg); got != "newest" {
		t.Fatalf("ConflictPolicy = %q, want newest", got)
	}

	cfg.Settings["sync.enabled"] = "off"
	if SyncEnabled(cfg) {
		t.Fatal("expected SyncEnabled to be false after setting off")
	}

	cfg.Settings["sync.policy"] = "local"
	if got := ConflictPolicy(cfg); got != "local" {
		t.Fatalf("ConflictPolicy = %q, want local", got)
	}

	cfg.Settings["sync.policy"] = "weird"
	if got := ConflictPolicy(cfg); got != "newest" {
		t.Fatalf("ConflictPolicy = %q, want newest for unknown policy", got)
	}
}

func TestAccountSyncPreferencesAdjustAndStandaloneSyncGate(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	m.SetAccount(AccountInfo{
		SignedIn:    true,
		UID:         "uid-1",
		Email:       "user@example.com",
		DisplayName: "neo",
	})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.cfg.Settings["sync.enabled"]; got != "off" {
		t.Fatalf("sync.enabled = %q, want off", got)
	}

	m, cmd := m.Update(keyRune('s'))
	if cmd != nil {
		t.Fatalf("expected nil cmd when sync is disabled, got %v", cmd)
	}
	if !strings.Contains(m.status, "Cloud sync is off") {
		t.Fatalf("status = %q, want containing 'Cloud sync is off'", m.status)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.cfg.Settings["sync.policy"]; got != "local" {
		t.Fatalf("sync.policy = %q, want local", got)
	}
}

func TestProviderCatalogIncludesGeminiAndNVIDIA(t *testing.T) {
	gemini, ok := ProviderByID("gemini")
	if !ok {
		t.Fatal("ProviderByID(gemini) not found")
	}
	if gemini.Label != "Google Gemini" {
		t.Errorf("gemini.Label = %q, want 'Google Gemini'", gemini.Label)
	}
	if gemini.BaseURL != "https://generativelanguage.googleapis.com" {
		t.Errorf("gemini.BaseURL = %q, want 'https://generativelanguage.googleapis.com'", gemini.BaseURL)
	}
	if gemini.Protocol != ProtocolGemini {
		t.Errorf("gemini.Protocol = %q, want %q", gemini.Protocol, ProtocolGemini)
	}

	nvidia, ok := ProviderByID("nvidia")
	if !ok {
		t.Fatal("ProviderByID(nvidia) not found")
	}
	if nvidia.BaseURL != "https://integrate.api.nvidia.com/v1" {
		t.Errorf("nvidia.BaseURL = %q, want 'https://integrate.api.nvidia.com/v1'", nvidia.BaseURL)
	}
	if nvidia.DefaultModel == "" {
		t.Error("nvidia.DefaultModel must not be empty")
	}
	if nvidia.Protocol != ProtocolOpenAI {
		t.Errorf("nvidia.Protocol = %q, want %q", nvidia.Protocol, ProtocolOpenAI)
	}
	if nvidia.KeyPrefix != "nvapi-" {
		t.Errorf("nvidia.KeyPrefix = %q, want 'nvapi-'", nvidia.KeyPrefix)
	}

	groq, ok := ProviderByID("groq")
	if !ok {
		t.Fatal("ProviderByID(groq) not found")
	}
	if groq.Label != "Groq" {
		t.Errorf("groq.Label = %q, want 'Groq'", groq.Label)
	}
	if groq.BaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("groq.BaseURL = %q, want 'https://api.groq.com/openai/v1'", groq.BaseURL)
	}
	if groq.DefaultModel != "openai/gpt-oss-120b" {
		t.Errorf("groq.DefaultModel = %q, want 'openai/gpt-oss-120b'", groq.DefaultModel)
	}
	if groq.Protocol != ProtocolOpenAI {
		t.Errorf("groq.Protocol = %q, want %q", groq.Protocol, ProtocolOpenAI)
	}
	if groq.KeyPrefix != "gsk_" {
		t.Errorf("groq.KeyPrefix = %q, want 'gsk_'", groq.KeyPrefix)
	}

	opencode, ok := ProviderByID("opencode")
	if !ok {
		t.Fatal("ProviderByID(opencode) not found")
	}
	if !opencode.Popular {
		t.Error("opencode.Popular must be true")
	}

	opts := CoachOptions()
	var hasAuto, hasGemini, hasNVIDIA bool
	for _, opt := range opts {
		switch opt.Value {
		case "auto":
			hasAuto = true
		case "gemini":
			hasGemini = true
		case "nvidia":
			hasNVIDIA = true
		}
	}
	if !hasAuto || !hasGemini || !hasNVIDIA {
		t.Errorf("CoachOptions() missing expected values: hasAuto=%v, hasGemini=%v, hasNVIDIA=%v", hasAuto, hasGemini, hasNVIDIA)
	}
}

func TestCoachProviderIDRejectsUnknown(t *testing.T) {
	cfg := config.DefaultConfig()
	EnsureDefaults(cfg)
	if got := CoachProviderID(cfg); got != "" {
		t.Errorf("CoachProviderID(default) = %q, want empty", got)
	}

	cfg.Settings["ai.coach"] = "nvidia"
	if got := CoachProviderID(cfg); got != "nvidia" {
		t.Errorf("CoachProviderID(nvidia) = %q, want 'nvidia'", got)
	}

	cfg.Settings["ai.coach"] = "unknown"
	if got := CoachProviderID(cfg); got != "" {
		t.Errorf("CoachProviderID(unknown) = %q, want empty", got)
	}
}

func TestProviderBaseAndModelHelpers(t *testing.T) {
	cfg := config.DefaultConfig()
	EnsureDefaults(cfg)
	p, _ := ProviderByID("gemini")

	if got := ProviderBase(cfg, p); got != p.BaseURL {
		t.Errorf("ProviderBase default = %q, want %q", got, p.BaseURL)
	}
	if got := ProviderModel(cfg, p); got != p.DefaultModel {
		t.Errorf("ProviderModel default = %q, want %q", got, p.DefaultModel)
	}

	cfg.Settings["ai.provider.gemini.base_url"] = "https://custom.gemini"
	cfg.Settings["ai.provider.gemini.model"] = "gemini-custom"

	if got := ProviderBase(cfg, p); got != "https://custom.gemini" {
		t.Errorf("ProviderBase override = %q, want https://custom.gemini", got)
	}
	if got := ProviderModel(cfg, p); got != "gemini-custom" {
		t.Errorf("ProviderModel override = %q, want gemini-custom", got)
	}
}

func TestIntegrationPickerOpensFiltersAndSelects(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAI)

	m, _ = m.Update(keyRune('c'))
	if m.integration == nil {
		t.Fatal("c should open the integration picker")
	}
	if !strings.Contains(ansi.Strip(m.View()), "CONNECT AN INTEGRATION") {
		t.Fatal("picker overlay is missing its title")
	}

	for _, r := range "nvid" {
		m, _ = m.Update(keyRune(r))
	}
	if len(m.integration.rows) == 0 {
		t.Fatal("filter should keep at least one row")
	}
	first := m.integration.rows[m.integration.selected]
	if first.Header || first.Provider.ID != "nvidia" {
		t.Fatalf("filtered selection = %#v, want nvidia item", first)
	}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := Value(m.Config(), Field{ID: "ai.coach"}); got != "nvidia" {
		t.Fatalf("ai.coach = %q, want nvidia", got)
	}
	if !m.providerDetail {
		t.Fatal("choosing an integration should open the provider detail panel")
	}
	if m.integration != nil {
		t.Fatal("choosing an integration should close the picker")
	}
	if cmd == nil {
		t.Fatal("choosing an integration should dispatch a command")
	}
	msg, ok := cmd().(AIProviderChangedMsg)
	if !ok || msg.ProviderID != "nvidia" {
		t.Fatalf("command = %#v, want AIProviderChangedMsg{nvidia}", cmd())
	}
}

func TestIntegrationPickerExactGrid(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {140, 45}} {
		m := testModel(t, size.width, size.height)
		m.OpenSection(SectionAI)
		m, _ = m.Update(keyRune('c'))
		frame := m.View()
		lines := strings.Split(frame, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d: %d rows, want %d", size.width, size.height, len(lines), size.height)
		}
		for i, line := range lines {
			if got := ansi.StringWidth(line); got != size.width {
				t.Fatalf("%dx%d row %d width %d, want %d", size.width, size.height, i, got, size.width)
			}
		}
		plain := ansi.Strip(frame)
		for _, want := range []string{"CONNECT AN INTEGRATION", "POPULAR", "SERVICES", "Google Gemini", "NVIDIA NIM"} {
			if !strings.Contains(plain, want) {
				t.Fatalf("%dx%d picker missing %q", size.width, size.height, want)
			}
		}
	}
}

func TestCoachReadyTracksSelectedKey(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAI)
	if m.CoachReady() {
		t.Fatal("no keys saved; CoachReady should be false")
	}
	if m.secrets == nil {
		t.Fatal("test model should have a secret store")
	}
	if err := m.secrets.Set("gemini", "test"); err != nil {
		t.Fatal(err)
	}
	SetValue(m.cfg, "ai.coach", "gemini")
	if !m.CoachReady() {
		t.Fatal("selected provider has a saved key; CoachReady should be true")
	}
	SetValue(m.cfg, "ai.coach", "anthropic")
	if m.CoachReady() {
		t.Fatal("selected provider has no saved key; CoachReady should be false")
	}
}

func TestAIModelSelectWritesProviderModelAndEmitsChanged(t *testing.T) {
	m := testModel(t, 140, 45)
	SetValue(m.cfg, "ai.coach", "nvidia")
	m, _ = m.Update(ProviderTestMsg{Provider: "nvidia", Models: []string{"nvidia/discovered-custom"}})
	m.OpenSection(SectionAI)

	m.selectedField = fieldIndex(m, "ai.model")
	if field := m.selectedFieldValue(); field == nil || field.ID != "ai.model" {
		t.Fatalf("selected field = %#v, want ai.model", field)
	}
	field := m.selectedFieldValue()
	found := false
	for _, opt := range field.Options {
		if opt.Value == "nvidia/discovered-custom" {
			found = true
		}
	}
	if !found {
		t.Fatalf("discovered probe model missing from ai.model options: %#v", field.Options)
	}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	const want = "nvidia/llama-3.1-nemotron-70b-instruct"
	if got := m.cfg.Settings["ai.provider.nvidia.model"]; got != want {
		t.Fatalf("ai.provider.nvidia.model = %q, want %q", got, want)
	}
	if cmd == nil {
		t.Fatal("selecting a model should dispatch a command")
	}
	msg, ok := cmd().(AIProviderChangedMsg)
	if !ok || msg.ProviderID != "nvidia" {
		t.Fatalf("command = %#v, want AIProviderChangedMsg{nvidia}", cmd())
	}
	if !strings.Contains(m.status, "model:") {
		t.Fatalf("status = %q, want model feedback", m.status)
	}
}

func TestProviderDetailBaseURLEditWritesConfig(t *testing.T) {
	m := testModel(t, 160, 50)
	m.OpenSection(SectionAI)
	m.selectedField = fieldIndex(m, "ai.providers")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open provider detail (anthropic)
	if !m.providerDetail {
		t.Fatal("enter on the provider list should open the detail panel")
	}
	m, _ = m.Update(keyRune('b')) // edit the base URL
	if m.editField == nil || m.editField.ID != "ai.provider.anthropic.base_url" {
		t.Fatalf("b should open the base URL editor, got %#v", m.editField)
	}
	m.editInput.SetValue("https://api.anthropic.example")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editing {
		t.Fatal("enter should commit the base URL edit")
	}
	if got := m.cfg.Settings["ai.provider.anthropic.base_url"]; got != "https://api.anthropic.example" {
		t.Fatalf("ai.provider.anthropic.base_url = %q, want https://api.anthropic.example", got)
	}
	if cmd == nil {
		t.Fatal("a provider setting change should dispatch a command")
	}
	msg, ok := cmd().(AIProviderChangedMsg)
	if !ok || msg.ProviderID != "anthropic" {
		t.Fatalf("command = %#v, want AIProviderChangedMsg{anthropic}", cmd())
	}
}

func TestIntegrationPickerClickOpensProviderDetail(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAI)
	m, _ = m.Update(keyRune('c')) // open the picker
	if m.integration == nil {
		t.Fatal("c should open the integration picker")
	}
	rowIdx := m.integration.selected
	if row := m.integration.rows[rowIdx]; row.Header {
		t.Fatalf("first selectable row is a header: %#v", row)
	}

	panelW, panelH := m.integrationPanelSize()
	panelX := (m.width - panelW) / 2
	panelY := (m.height - panelH) / 2
	listY := panelY + 1 + 3
	x := panelX + 2
	y := listY + (rowIdx - m.integration.offset)

	m, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.integration != nil {
		t.Fatal("clicking a picker row should close the picker")
	}
	if !m.providerDetail {
		t.Fatal("clicking a picker row should open the provider detail")
	}
	if got := Value(m.Config(), Field{ID: "ai.coach"}); got != "opencode" {
		t.Fatalf("ai.coach = %q, want opencode", got)
	}
	if cmd == nil {
		t.Fatal("clicking a picker row should dispatch a command")
	}
	msg, ok := cmd().(AIProviderChangedMsg)
	if !ok || msg.ProviderID != "opencode" {
		t.Fatalf("command = %#v, want AIProviderChangedMsg{opencode}", cmd())
	}
}

func TestProviderCardShowsOverriddenModel(t *testing.T) {
	m := testModel(t, 160, 50)
	m.OpenSection(SectionAI)
	SetValue(m.cfg, "ai.provider.anthropic.model", "claude-opus-custom")
	plain := ansi.Strip(m.renderAI(m.fieldsFor(SectionAI), m.sectionWidth()))
	if !strings.Contains(plain, "claude-opus-custom") {
		t.Fatal("provider card should render the configured model override")
	}
}

func TestEnsureDefaultsMigratesRetiredGeminiModel(t *testing.T) {
	for _, retired := range []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite"} {
		cfg := config.DefaultConfig()
		cfg.Settings["ai.provider.gemini.model"] = retired

		EnsureDefaults(cfg)

		if got := cfg.Settings["ai.provider.gemini.model"]; got != "gemini-3.8-flash" {
			t.Fatalf("%s migrated to %q, want gemini-3.8-flash", retired, got)
		}
		if cfg.SettingsVersion != SchemaVersion {
			t.Fatalf("SettingsVersion = %d, want %d", cfg.SettingsVersion, SchemaVersion)
		}
	}
}

func TestDiscoveredModelsSurviveRestart(t *testing.T) {
	cfg := testConfig(t)
	SetValue(cfg, "ai.coach", "gemini")
	m := New(cfg, theme.Default(), 140, 45)

	m, _ = m.Update(ProviderTestMsg{
		Provider: "gemini",
		Models:   []string{"gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.6-flash"},
	})

	restarted := New(m.cfg, theme.Default(), 140, 45)
	restarted.OpenSection(SectionAI)
	opts := restarted.modelOptions()
	for _, want := range []string{"gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.6-flash"} {
		found := false
		for _, opt := range opts {
			if opt.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("model %q missing from options after restart: %#v", want, opts)
		}
	}
}

func TestAccountGoogleKeyStartsBrowserFlow(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	m, cmd := m.Update(keyRune('g'))
	if !m.googleBusy {
		t.Fatal("g should mark the Google browser flow as pending")
	}
	if !strings.Contains(m.status, "Google") {
		t.Fatalf("status = %q, want Google hint", m.status)
	}
	if cmd == nil {
		t.Fatal("g should dispatch a Google auth request")
	}
	if _, ok := cmd().(GoogleAuthRequestMsg); !ok {
		t.Fatalf("g produced %T, want GoogleAuthRequestMsg", cmd())
	}
}

func TestAccountEmailAndSignupKeysOpenForm(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	m, _ = m.Update(keyRune('e'))
	if m.auth == nil || m.auth.Mode != AuthSignIn {
		t.Fatal("e should open the email sign-in form")
	}
	m.auth = nil
	m, _ = m.Update(keyRune('u'))
	if m.auth == nil || m.auth.Mode != AuthSignUp {
		t.Fatal("u should open the sign-up form")
	}
}

func TestAuthErrorWithoutFormShowsDetail(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	m.googleBusy = true
	m, _ = m.Update(AuthResultMsg{Err: errors.New("google sign-in failed: no browser")})
	if m.googleBusy {
		t.Fatal("auth result should clear the Google busy state")
	}
	if !strings.Contains(m.status, "no browser") {
		t.Fatalf("status = %q, want the auth error detail", m.status)
	}
}

func TestSignedOutAccountShowsGoogleFirst(t *testing.T) {
	m := testModel(t, 140, 45)
	m.OpenSection(SectionAccount)
	plain := ansi.Strip(m.View())
	for _, want := range []string{"Continue with Google", "Sign in with email", "Create account"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("signed-out account view missing %q", want)
		}
	}
}
