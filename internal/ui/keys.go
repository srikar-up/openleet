package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the global key bindings across OpenLeet.
type KeyMap struct {
	Quit       key.Binding
	Help       key.Binding
	SwitchView key.Binding
	Palette    key.Binding
	Settings   key.Binding
	Next       key.Binding
	Prev       key.Binding
	Run        key.Binding
	Submit     key.Binding
	Save       key.Binding
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Escape     key.Binding
}

// DefaultKeyMap returns the default keybindings conforming to OpenLeet roadmap.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?", "help"),
		),
		SwitchView: key.NewBinding(
			key.WithKeys("shift+tab", "f2"),
			key.WithHelp("shift+tab", "switch view"),
		),
		Palette: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
		Settings: key.NewBinding(
			key.WithKeys("ctrl+,", "f5"),
			key.WithHelp("ctrl+,", "settings"),
		),
		Next: key.NewBinding(
			key.WithKeys("ctrl+pgdown", "alt+down"),
			key.WithHelp("ctrl+pgdown", "next"),
		),
		Prev: key.NewBinding(
			// ctrl+b belongs to the workspace beautify action (and to word
			// navigation in editors). Page Up and Alt+Up are unambiguous at
			// the app level; Alt+Up also survives terminals that use
			// Ctrl+PageUp for tab switching.
			key.WithKeys("ctrl+pgup", "alt+up"),
			key.WithHelp("alt+up", "prev"),
		),
		Run: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "run"),
		),
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "submit"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}
