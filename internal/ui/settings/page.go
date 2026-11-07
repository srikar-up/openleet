package settings

import "strings"

// providerCardRows is the full height of a provider card, including borders.
func (m Model) providerCardRows(providerID string) int {
	discovered := m.providerStatus[providerID].Models
	if len(discovered) == 0 {
		discovered = CachedProviderModels(m.cfg, providerID)
	}
	if len(discovered) > 0 {
		return providerCardHeight + 1
	}
	return providerCardHeight
}

// sectionContentTop is the body row where the selected tab's first card
// starts: the three-row canvas badge plus one blank row.
const sectionContentTop = canvasBadgeHeight + 1

// canvasContentOriginY maps a content row to a terminal row: the breadcrumb is
// row 0, then the body starts. Narrow layouts add one top-tab row.
func (m Model) canvasContentOriginY() int {
	top := 1 + sectionContentTop
	if m.width < 100 {
		top++ // narrow mode adds the top tab row
	}
	return top
}

// sectionContent renders the selected tab, or the search result list while a
// filter is active.
func (m Model) sectionContent() string {
	if m.query != "" {
		return m.renderSearchResults(m.sectionWidth())
	}
	return m.renderSectionFor(m.ActiveSection(), m.currentFields(), m.sectionWidth())
}

// providerContentStart is the first row of the provider stack inside the AI
// canvas content.
func (m Model) providerContentStart(fields []Field) int {
	prelude := m.renderAIPrelude(fields)
	rows := 0
	if strings.TrimSpace(prelude) != "" {
		rows = strings.Count(prelude, "\n") + 1
	}
	return rows + 1
}

// fieldContentLine returns the content row for one field in the selected tab.
// The theme grid, key table, account panel, and about panel are custom cards
// whose field labels are not painted as standalone rows.
func (m Model) fieldContentLine(fields []Field, field Field) int {
	if m.query != "" {
		return -1
	}
	switch field.Kind {
	case KindProviderList:
		return m.providerContentStart(fields)
	case KindThemeGrid, KindKeyTable, KindAccount, KindAbout:
		return 0
	}
	content := m.renderSectionFor(m.ActiveSection(), fields, m.sectionWidth())
	return selectedFieldLine(content, field.Label)
}

// fieldScreenY maps a field row to the terminal row used by mouse hits.
func (m Model) fieldScreenY(id string) (int, bool) {
	if m.query != "" {
		return 0, false
	}
	fields := m.currentFields()
	for _, field := range fields {
		if field.ID != id {
			continue
		}
		line := m.fieldContentLine(fields, field)
		if line < 0 {
			return 0, false
		}
		return m.canvasContentOriginY() + line, true
	}
	return 0, false
}

// themeGridOrigin is the top-left cell of the first theme preview card.
func (m Model) themeGridOrigin() (x, y int) {
	x = m.canvasOriginX() + 3 // content indent, card border, card padding
	if m.ActiveSection() != SectionAppearance || m.query != "" {
		return x, -1
	}
	return x, m.canvasContentOriginY() + 3 // card border, heading, blank
}

// providerListOriginY is the first screen row of the first provider card.
func (m Model) providerListOriginY() int {
	if m.ActiveSection() != SectionAI || m.query != "" {
		return -1
	}
	fields := m.currentFields()
	if len(fields) == 0 {
		return -1
	}
	return m.canvasContentOriginY() + m.providerContentStart(fields)
}

// fieldRowAt maps a screen row to a field in the selected tab only.
func (m Model) fieldRowAt(y int) (int, bool) {
	if m.query != "" {
		return 0, false
	}
	fields := m.currentFields()
	for index, field := range fields {
		line := m.fieldContentLine(fields, field)
		if line < 0 {
			continue
		}
		if m.canvasContentOriginY()+line == y {
			return index, true
		}
	}
	return 0, false
}

func (m Model) themeAt(x, y int) (int, bool) {
	cols, cardW := themeGridLayout(m.sectionWidth())
	originX, originY := m.themeGridOrigin()
	if originY < 0 || x < originX || y < originY {
		return 0, false
	}
	relY := y - originY
	row := relY / themeCardHeight
	col := (x - originX) / (cardW + themeCardGap)
	if col < 0 || col >= cols {
		return 0, false
	}
	index := row*cols + col
	if index < 0 || index >= len(m.themes) {
		return 0, false
	}
	return index, true
}

func (m Model) providerAt(x, y int) (int, bool) {
	if m.ActiveSection() != SectionAI || m.query != "" || x < m.canvasOriginX() || x >= m.width {
		return 0, false
	}
	top := m.providerListOriginY()
	if top < 0 || y < top {
		return 0, false
	}
	for index := 0; index < len(m.providers); index++ {
		rows := m.providerCardRows(m.providers[index].ID)
		if y < top+rows {
			return index, true
		}
		top += rows
	}
	return 0, false
}
