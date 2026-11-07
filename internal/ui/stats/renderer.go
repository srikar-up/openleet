package stats

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"openleet/internal/ui/theme"
	"openleet/internal/ui/uikit"
)

func (m Model) render(summary Summary) string {
	header := uikit.JoinLeftRight(
		m.width,
		lipgloss.NewStyle().Bold(true).Foreground(m.pal.Text).Render("openleet")+
			lipgloss.NewStyle().Foreground(m.pal.Muted).Render(" / stats"),
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render(m.rangeLabel()),
	)

	// The counter is intentionally compact: the wordmark is the hero, and the
	// solved total reads as a caption under it rather than a second headline.
	counterText := fmt.Sprintf("%d/%d", summary.Solved, summary.Total)
	hero := uikit.RenderPixelText(counterText, m.pal.Text)
	label := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("SOLVED")

	block := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		uikit.RenderWordmarkBlock("OPEN", "LEET", m.pal.Text, m.pal.Accent, m.pal.Background),
		"",
		"",
		hero,
		"",
		label,
		"",
		m.renderHeatmap(summary),
		"",
		m.renderStats(summary),
		"",
		m.renderDifficulty(summary),
		"",
		m.renderLeaderboard(),
		"",
		m.renderTopics(summary),
	)

	bodyHeight := m.height - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, block)
	frame := uikit.PadFrame(header+"\n"+body, m.width, m.height)
	return uikit.PaintBackground(frame, m.pal.Background)
}

// rangeLabel renders the mockup's "Jan – Nov 2026" caption.
func (m Model) rangeLabel() string {
	now := time.Now()
	if now.Month() == time.January {
		return fmt.Sprintf("Jan %d", now.Year())
	}
	return fmt.Sprintf("Jan – %s %d", now.Format("Jan"), now.Year())
}

// renderHeatmap draws the contribution grid: weekday rows by week columns.
func (m Model) renderHeatmap(summary Summary) string {
	today := time.Now()
	janFirst := time.Date(today.Year(), time.January, 1, 0, 0, 0, 0, time.Local)
	start := janFirst.AddDate(0, 0, -mondayOffset(janFirst))
	weeks := int(today.Sub(start).Hours()/24)/7 + 1
	if weeks < 1 {
		weeks = 1
	}

	step := 2
	if m.width < 110 {
		step = 1
	}

	header := m.monthHeader(start, weeks, today, step)
	rows := make([]string, 0, 7)
	labels := "MTWTFSS"
	for weekday := 0; weekday < 7; weekday++ {
		var row strings.Builder
		row.WriteString(lipgloss.NewStyle().Foreground(m.pal.Muted).Render(string(labels[weekday]) + " "))
		for week := 0; week < weeks; week++ {
			day := start.AddDate(0, 0, week*7+weekday)
			cell := "·"
			foreground := m.pal.Faint
			if day.Before(janFirst) || day.After(today) {
				cell = " "
			} else if count := summary.Days[day.Format("2006-01-02")]; count > 0 {
				cell = "■"
				switch {
				case count >= 3:
					foreground = m.pal.HeatHigh
				case count == 2:
					foreground = m.pal.HeatMid
				default:
					foreground = m.pal.HeatLow
				}
			}
			row.WriteString(lipgloss.NewStyle().Foreground(foreground).Render(cell))
			if step == 2 {
				row.WriteString(" ")
			}
		}
		rows = append(rows, row.String())
	}

	legend := lipgloss.NewStyle().Foreground(m.pal.Muted).Render("less  ") +
		lipgloss.NewStyle().Foreground(m.pal.Faint).Render("·  ") +
		lipgloss.NewStyle().Foreground(m.pal.HeatLow).Render("■  ") +
		lipgloss.NewStyle().Foreground(m.pal.HeatMid).Render("■  ") +
		lipgloss.NewStyle().Foreground(m.pal.HeatHigh).Render("■") +
		lipgloss.NewStyle().Foreground(m.pal.Muted).Render("  more")

	gridWidth := 2 + weeks*step
	if pad := gridWidth - lipgloss.Width(legend); pad > 0 {
		legend = strings.Repeat(" ", pad) + legend
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		legend,
	)
}

// monthHeader labels the week column where each month starts.
func (m Model) monthHeader(start time.Time, weeks int, today time.Time, step int) string {
	line := make([]rune, 2+weeks*step)
	for i := range line {
		line[i] = ' '
	}
	lastEnd := -1
	for month := time.January; month <= today.Month(); month++ {
		first := time.Date(today.Year(), month, 1, 0, 0, 0, 0, time.Local)
		week := int(first.Sub(start).Hours()/24) / 7
		pos := 2 + week*step
		label := first.Format("Jan")
		if pos <= lastEnd+1 {
			continue
		}
		labelRunes := []rune(label)
		for i, r := range labelRunes {
			if pos+i < len(line) {
				line[pos+i] = r
			}
		}
		lastEnd = pos + len(labelRunes) - 1
	}
	return lipgloss.NewStyle().Foreground(m.pal.Muted).Render(strings.TrimRight(string(line), " "))
}

// renderStats draws the three headline figures.
func (m Model) renderStats(summary Summary) string {
	dayWord := "days"
	if summary.BestStreak == 1 {
		dayWord = "day"
	}
	sessionWord := "sessions"
	if summary.Sessions == 1 {
		sessionWord = "session"
	}
	blocks := []struct {
		value string
		label string
	}{
		{fmt.Sprintf("%d %s", summary.BestStreak, dayWord), "best streak"},
		{fmt.Sprintf("%d", summary.ActiveDays), "active days"},
		{fmt.Sprintf("%d", summary.Sessions), sessionWord},
	}
	rendered := make([]string, 0, len(blocks))
	for _, block := range blocks {
		width := lipgloss.Width(block.value)
		if labelWidth := lipgloss.Width(block.label); labelWidth > width {
			width = labelWidth
		}
		value := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.pal.Text).
			Width(width).
			Align(lipgloss.Center).
			Render(block.value)
		caption := lipgloss.NewStyle().
			Foreground(m.pal.Muted).
			Width(width).
			Align(lipgloss.Center).
			Render(block.label)
		rendered = append(rendered, lipgloss.JoinVertical(lipgloss.Left, value, caption))
	}

	gap := strings.Repeat(" ", 6)
	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		rendered[0], gap, rendered[1], gap, rendered[2],
	)
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(row)
}

// renderDifficulty summarizes solved levels per difficulty.
func (m Model) renderDifficulty(summary Summary) string {
	item := func(color lipgloss.AdaptiveColor, name string, solved, total int) string {
		return lipgloss.NewStyle().Foreground(color).Render("■") +
			lipgloss.NewStyle().Foreground(color).Render(" "+name+" ") +
			lipgloss.NewStyle().Foreground(m.pal.Text).Render(fmt.Sprintf("%d/%d", solved, total))
	}
	row := item(m.pal.Easy, "easy", summary.EasySolved, summary.EasyTotal) +
		strings.Repeat(" ", 4) +
		item(m.pal.Medium, "medium", summary.MediumSolved, summary.MediumTotal) +
		strings.Repeat(" ", 4) +
		item(m.pal.Hard, "hard", summary.HardSolved, summary.HardTotal)
	return lipgloss.NewStyle().Align(lipgloss.Center).Render(row)
}

// renderLeaderboard draws the compact global ranking. Only name, score, and
// solved count are stored in Firestore; the rest of the profile stays local.
func (m Model) renderLeaderboard() string {
	label := lipgloss.NewStyle().Bold(true).Foreground(m.pal.Muted).Render("LEADERBOARD  ")
	muted := func(text string) string {
		return lipgloss.NewStyle().Foreground(m.pal.Muted).Render(text)
	}

	switch {
	case !m.leaderboardSignedIn:
		return m.centerLine(label + muted("sign in from settings to rank"))
	case m.leaderboardErr != "":
		return m.centerLine(label + muted("unavailable"))
	case m.leaderboardLoading && len(m.leaderboard) == 0:
		return m.centerLine(label + muted("loading…"))
	case len(m.leaderboard) == 0:
		return m.centerLine(label + muted("no ranked players yet"))
	}

	top := m.leaderboard
	if len(top) > 3 {
		top = top[:3]
	}
	selfRank := 0
	for i, entry := range m.leaderboard {
		if m.leaderboardSelf != "" && entry.UID == m.leaderboardSelf {
			selfRank = i + 1
			break
		}
	}

	separator := lipgloss.NewStyle().Foreground(m.pal.Faint).Render(" · ")
	parts := make([]string, 0, len(top)+1)
	for i, entry := range top {
		style := lipgloss.NewStyle().Foreground(m.pal.Text)
		if i+1 == selfRank {
			style = lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent)
		}
		name := uikit.Truncate(entry.Name, 16, "…")
		parts = append(parts, style.Render(fmt.Sprintf("#%d %s %d", i+1, name, entry.Score)))
	}
	if selfRank > len(top) {
		parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(m.pal.Accent).
			Render(fmt.Sprintf("you #%d", selfRank)))
	}
	return m.centerLine(label + strings.Join(parts, separator))
}

func (m Model) centerLine(line string) string {
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(uikit.Truncate(line, m.width, "…"))
}

// renderTopics lists the most practiced problem tags.
func (m Model) renderTopics(summary Summary) string {
	if len(summary.Topics) == 0 {
		return lipgloss.NewStyle().Foreground(m.pal.Muted).Render("solve a level to populate topic stats")
	}
	parts := make([]string, 0, len(summary.Topics))
	for _, topic := range summary.Topics {
		parts = append(parts, fmt.Sprintf("%s %d", topic.Name, topic.Count))
	}
	return lipgloss.NewStyle().
		Foreground(m.pal.Muted).
		Align(lipgloss.Center).
		Render(strings.Join(parts, " · "))
}

// SetPalette repaints the dashboard with the shared application theme.
func (m *Model) SetPalette(p theme.Palette) {
	m.pal = paletteFromTheme(p)
}

// mondayOffset returns how many days back the Monday of date's week is.
func mondayOffset(date time.Time) int {
	return (int(date.Weekday()) + 6) % 7
}
