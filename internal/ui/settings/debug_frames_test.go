package settings

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func dumpFrame(t *testing.T, label string, m Model) {
	t.Helper()
	plain := ansi.Strip(m.View())
	lines := strings.Split(plain, "\n")
	fmt.Printf("===== %s active=%s =====\n", label, m.ActiveSection())
	for i, line := range lines {
		fmt.Printf("%02d|%s|\n", i, line)
	}
}

func TestDebugFrames(t *testing.T) {
	m := testModel(t, 120, 40)
	m.OpenSection(SectionGeneral)
	dumpFrame(t, "general tab", m)

	m.OpenSection(SectionAbout)
	dumpFrame(t, "about tab", m)

	m.OpenSection(SectionGeneral)
	m, _ = m.Update(keyRune('/'))
	for _, r := range "temperature" {
		m, _ = m.Update(keyRune(r))
	}
	dumpFrame(t, "search temperature", m)
}
