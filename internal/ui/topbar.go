package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"openleet/internal/ui/settings"
	"openleet/internal/ui/uikit"
)

// topBarTarget identifies a clickable/focusable item in the shared top bar.
type topBarTarget int

const (
	topBarTargetNone topBarTarget = iota
	topBarTargetBreadcrumb
	topBarTargetSettings
	topBarTargetAccount
)

// topBarHit is an inclusive cell range on the top bar. Breadcrumb hits carry
// the view their segment navigates to.
type topBarHit struct {
	start  int
	end    int
	target topBarTarget
	view   ViewMode
}

// iconsUnicode resolves the appearance.icons setting. Auto probes the terminal;
// on and off force the answer. Legacy unicode/ascii values are accepted so old
// config files keep working.
func (m AppModel) iconsUnicode() bool {
	mode := "auto"
	if m.config != nil {
		mode = settings.Value(m.config, settings.Field{ID: "appearance.icons", Default: "auto"})
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on", "unicode", "true":
		return true
	case "off", "ascii", "false":
		return false
	default:
		return detectUnicodeSupport()
	}
}

// hasChrome reports whether the current screen uses the shared top and bottom
// bars. Stats paints its own full-screen dashboard.
func (m AppModel) hasChrome() bool {
	switch m.currentView {
	case ViewHome, ViewRoadmap, ViewSolve, ViewSettings:
		return true
	default:
		return false
	}
}

// accountChipEmail returns the best label source for the account item.
func (m AppModel) accountChipEmail() string {
	if m.account == nil {
		return ""
	}
	if email := strings.TrimSpace(m.account.Email); email != "" {
		return email
	}
	return strings.TrimSpace(m.account.DisplayName)
}

// accountText is the account label for a given width. Under 120 columns it is
// the local part before "@"; under 100 columns it is the first letter. Signed
// out always reads "Log in".
func (m AppModel) accountText(width int) string {
	email := m.accountChipEmail()
	if email == "" {
		return "Log in"
	}
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	switch {
	case width < 100:
		runes := []rune(local)
		if len(runes) == 0 {
			return "Log in"
		}
		return strings.ToUpper(string(runes[0]))
	case width < 120:
		return local
	default:
		return email
	}
}

// topBarItem renders one plain label. The 1-cell padding on each side is always
// reserved so hover never reflows the row; only the background is painted on
// hover/focus. No border characters and no icon glyphs are ever emitted.
func (m AppModel) topBarItem(label string, target topBarTarget, signedOut bool) string {
	foreground := m.palette.TextDim
	if signedOut {
		foreground = m.palette.Accent
	}
	style := lipgloss.NewStyle().Foreground(foreground)

	active := m.topBarHover == target || m.topBarFocus == target
	pressed := m.topBarPressed == target
	if active || pressed {
		style = style.Foreground(m.palette.Text).Background(m.palette.SurfaceRaised)
	}
	if pressed {
		style = style.Foreground(m.palette.Accent)
	}
	return style.Render(" " + label + " ")
}

// topBarLayout renders exactly one chrome row: the breadcrumb on the left and,
// right to left, the account label and the Settings label with a fixed 3-cell
// gap. Nothing on the row touches the window edges.
func (m AppModel) topBarLayout() (string, []topBarHit) {
	if m.width < 1 {
		return "", nil
	}
	inner := m.width - 2
	if inner < 1 {
		inner = 1
	}

	// Each item reserves one padding cell on each side for the hover pill, so a
	// single gap cell yields the required three visual cells between labels.
	const itemGap = 1
	settingsItem := m.topBarItem("Settings", topBarTargetSettings, false)
	accountLabel := m.accountText(m.width)
	signedOut := m.account == nil || m.accountChipEmail() == ""
	accountItem := m.topBarItem(accountLabel, topBarTargetAccount, signedOut)

	// Keep the account label inside the row: settings item + gap + account
	// padding. The breadcrumb truncates first, never the right-hand items.
	maxAccountLabel := inner - lipgloss.Width(settingsItem) - itemGap - 2
	if maxAccountLabel < 1 {
		maxAccountLabel = 1
	}
	if lipgloss.Width(accountLabel) > maxAccountLabel {
		accountLabel = uikit.Truncate(accountLabel, maxAccountLabel, "…")
		accountItem = m.topBarItem(accountLabel, topBarTargetAccount, signedOut)
	}

	right := settingsItem + strings.Repeat(" ", itemGap) + accountItem
	rightW := lipgloss.Width(right)

	crumb, crumbHits := m.breadcrumb()
	leftBudget := inner - rightW - 2
	if leftBudget < 0 {
		leftBudget = 0
	}
	left := uikit.Truncate(crumb, leftBudget, "…")
	leftW := lipgloss.Width(left)
	gap := inner - leftW - rightW
	if gap < 0 {
		gap = 0
	}
	content := uikit.FitExact(left+strings.Repeat(" ", gap)+right, inner)

	// Breadcrumb hits are computed in screen cells starting at 1; clip them to
	// the visible crumb width (cells 1..leftW).
	hits := make([]topBarHit, 0, len(crumbHits)+2)
	for _, hit := range crumbHits {
		if hit.start > leftW {
			continue
		}
		end := hit.end
		if end > leftW {
			end = leftW
		}
		if end < hit.start {
			continue
		}
		hits = append(hits, topBarHit{start: hit.start, end: end, target: topBarTargetBreadcrumb, view: hit.view})
	}

	settingsStart := 1 + inner - rightW
	settingsEnd := settingsStart + lipgloss.Width(settingsItem) - 1
	accountStart := settingsEnd + 1 + itemGap
	accountEnd := accountStart + lipgloss.Width(accountItem) - 1
	hits = append(hits,
		topBarHit{start: settingsStart, end: settingsEnd, target: topBarTargetSettings},
		topBarHit{start: accountStart, end: accountEnd, target: topBarTargetAccount},
	)
	return uikit.PaintBackground(" "+content+" ", m.palette.Background), hits
}

func (m AppModel) topBarHits() []topBarHit {
	_, hits := m.topBarLayout()
	return hits
}

// topBarTooltipTarget reports which item owns the hover/focus tooltip.
func (m AppModel) topBarTooltipTarget() topBarTarget {
	target := m.topBarHover
	if target == topBarTargetNone {
		target = m.topBarFocus
	}
	switch target {
	case topBarTargetSettings, topBarTargetAccount:
		return target
	default:
		return topBarTargetNone
	}
}

// topBarTooltip returns the tooltip label for the active item. Tooltips are
// drawn as a transient overlay below the header, never as a second header row.
func (m AppModel) topBarTooltip() string {
	switch m.topBarTooltipTarget() {
	case topBarTargetSettings:
		return "Settings (F4)"
	case topBarTargetAccount:
		return "Account"
	default:
		return ""
	}
}

// topBarTargetHit returns the hit range for a target on the rendered row.
func (m AppModel) topBarTargetHit(target topBarTarget) (topBarHit, bool) {
	for _, hit := range m.topBarHits() {
		if hit.target == target {
			return hit, true
		}
	}
	return topBarHit{}, false
}

// overlayTopBarTooltip splices a one-row pill on to row 1, right-aligned under
// the hovered item. The header itself stays exactly one row tall.
func (m AppModel) overlayTopBarTooltip(base string) string {
	tip := m.topBarTooltip()
	if tip == "" || m.height < 2 {
		return base
	}
	hit, ok := m.topBarTargetHit(m.topBarTooltipTarget())
	if !ok {
		return base
	}
	pill := lipgloss.NewStyle().
		Foreground(m.palette.Text).
		Background(m.palette.SurfaceRaised).
		Render(" " + tip + " ")
	pillW := lipgloss.Width(pill)
	x := hit.end - pillW + 1
	if x < 1 {
		x = 1
	}
	if x+pillW > m.width-1 {
		x = m.width - 1 - pillW
	}
	if x < 0 {
		x = 0
	}

	lines := strings.Split(base, "\n")
	if len(lines) < 2 {
		return base
	}
	row := lines[1]
	left := ansi.Cut(row, 0, x)
	if pad := x - ansi.StringWidth(left); pad > 0 {
		left += strings.Repeat(" ", pad)
	}
	right := ansi.Cut(row, x+pillW, m.width)
	if pad := m.width - x - pillW - ansi.StringWidth(right); pad > 0 {
		right += strings.Repeat(" ", pad)
	}
	lines[1] = left + pill + right
	return strings.Join(lines, "\n")
}

// nextTopBarTarget advances keyboard focus through the breadcrumb and the two
// right-hand labels.
func nextTopBarTarget(target topBarTarget, delta int) topBarTarget {
	order := []topBarTarget{topBarTargetBreadcrumb, topBarTargetSettings, topBarTargetAccount}
	index := 0
	for i, candidate := range order {
		if candidate == target {
			index = i
			break
		}
	}
	index = (index + delta + len(order)) % len(order)
	return order[index]
}
