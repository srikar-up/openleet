package theme

import (
	"sort"
	"strings"
)

// specs is the authoring order; the picker lists themes in exactly this order.
var specs = []spec{
	{
		name:        "openleet",
		label:       "OpenLeet",
		description: "Brand orange on ink — the default roadmap palette",
		background:  "#0d0a0b",
		surface:     "#131313",
		text:        "#f2f2f2",
		subtext:     "#8a8a8a",
		muted:       "#6e6e6e",
		accent:      "#ff7a1a",
		primary:     "#f2f2f2",
		secondary:   "#8fa8f5",
		success:     "#b5f9ad",
		warning:     "#ffb27a",
		danger:      "#ff5f5f",
		easy:        "#cfcfcf",
		medium:      "#b5f9ad",
		hard:        "#ffb27a",
		border:      "#3a3a3a",
	},
	{
		name:        "openleet-light",
		label:       "OpenLeet Light",
		description: "Brand orange on warm paper — light mode",
		light:       true,
		background:  "#f7f2e8",
		surface:     "#efe7da",
		text:        "#211c17",
		subtext:     "#5a5248",
		muted:       "#6f655b",
		accent:      "#c2410c",
		primary:     "#211c17",
		secondary:   "#3355b5",
		success:     "#2f7d32",
		warning:     "#8a5300",
		danger:      "#b3261e",
		easy:        "#635b51",
		medium:      "#2f7d32",
		hard:        "#8a5300",
		border:      "#c9bcaa",
	},
	{
		name:        "opencode",
		label:       "OpenCode",
		description: "Warm ink and apricot, the reference TUI palette",
		background:  "#212121",
		surface:     "#252525",
		text:        "#e0e0e0",
		subtext:     "#6a6a6a",
		muted:       "#6a6a6a",
		accent:      "#fab283",
		primary:     "#fab283",
		secondary:   "#5c9cf5",
		success:     "#7fd88f",
		warning:     "#f5a742",
		danger:      "#e06c75",
		easy:        "#e0e0e0",
		medium:      "#7fd88f",
		hard:        "#f5a742",
		border:      "#4b4c5c",
	},
	{
		name:        "catppuccin-mocha",
		label:       "Catppuccin Mocha",
		description: "Soft pastel mocha with peach highlights",
		background:  "#1e1e2e",
		surface:     "#181825",
		text:        "#cdd6f4",
		subtext:     "#a6adc8",
		muted:       "#6c7086",
		accent:      "#fab387",
		primary:     "#89b4fa",
		secondary:   "#cba6f7",
		success:     "#a6e3a1",
		warning:     "#f9e2af",
		danger:      "#f38ba8",
		easy:        "#cdd6f4",
		medium:      "#a6e3a1",
		hard:        "#fab387",
		border:      "#313244",
	},
	{
		name:        "catppuccin-latte",
		label:       "Catppuccin Latte",
		description: "Soft pastel latte with peach highlights",
		light:       true,
		background:  "#eff1f5",
		surface:     "#e6e9ef",
		text:        "#4c4f69",
		subtext:     "#6c6f85",
		muted:       "#8c8fa1",
		accent:      "#fe640b",
		primary:     "#1e66f5",
		secondary:   "#8839ef",
		success:     "#40a02b",
		warning:     "#df8e1d",
		danger:      "#d20f39",
		easy:        "#4c4f69",
		medium:      "#40a02b",
		hard:        "#fe640b",
		border:      "#ccd0da",
	},
	{
		name:        "dracula",
		label:       "Dracula",
		description: "Night purple with vivid coral and mint",
		background:  "#282a36",
		surface:     "#21222c",
		text:        "#f8f8f2",
		subtext:     "#6272a4",
		muted:       "#6272a4",
		accent:      "#ffb86c",
		primary:     "#bd93f9",
		secondary:   "#8be9fd",
		success:     "#50fa7b",
		warning:     "#f1fa8c",
		danger:      "#ff5555",
		easy:        "#f8f8f2",
		medium:      "#50fa7b",
		hard:        "#ffb86c",
		border:      "#44475a",
	},
	{
		name:        "flexoki",
		label:       "Flexoki",
		description: "Inky paper tones with earthen accents",
		background:  "#100f0f",
		surface:     "#1c1b1a",
		text:        "#cecdc3",
		subtext:     "#878580",
		muted:       "#575653",
		accent:      "#da702c",
		primary:     "#4385be",
		secondary:   "#8b7ec8",
		success:     "#879a39",
		warning:     "#d0a215",
		danger:      "#d14d41",
		easy:        "#cecdc3",
		medium:      "#879a39",
		hard:        "#da702c",
		border:      "#282726",
	},
	{
		name:        "gruvbox",
		label:       "Gruvbox",
		description: "Retro warm greys with bright orange",
		background:  "#282828",
		surface:     "#1d2021",
		text:        "#ebdbb2",
		subtext:     "#a89984",
		muted:       "#928374",
		accent:      "#fe8019",
		primary:     "#fabd2f",
		secondary:   "#83a598",
		success:     "#b8bb26",
		warning:     "#fabd2f",
		danger:      "#fb4934",
		easy:        "#ebdbb2",
		medium:      "#b8bb26",
		hard:        "#fe8019",
		border:      "#3c3836",
	},
	{
		name:        "monokai",
		label:       "Monokai",
		description: "Editor classic — neon fruit on charcoal",
		background:  "#2d2a2e",
		surface:     "#221f22",
		text:        "#fcfcfa",
		subtext:     "#939293",
		muted:       "#727072",
		accent:      "#fc9867",
		primary:     "#a9dc76",
		secondary:   "#78dce8",
		success:     "#a9dc76",
		warning:     "#ffd866",
		danger:      "#ff6188",
		easy:        "#fcfcfa",
		medium:      "#a9dc76",
		hard:        "#fc9867",
		border:      "#403e41",
	},
	{
		name:        "onedark",
		label:       "One Dark",
		description: "Atom's balanced blue-grey editor scheme",
		background:  "#282c34",
		surface:     "#21252b",
		text:        "#abb2bf",
		subtext:     "#7f848e",
		muted:       "#5c6370",
		accent:      "#d19a66",
		primary:     "#61afef",
		secondary:   "#c678dd",
		success:     "#98c379",
		warning:     "#e5c07b",
		danger:      "#e06c75",
		easy:        "#abb2bf",
		medium:      "#98c379",
		hard:        "#d19a66",
		border:      "#3b4048",
	},
	{
		name:        "tokyonight",
		label:       "Tokyo Night",
		description: "Neon city blues and warm coral",
		background:  "#222436",
		surface:     "#1e2030",
		text:        "#c8d3f5",
		subtext:     "#828bb8",
		muted:       "#636da6",
		accent:      "#ff966c",
		primary:     "#82aaff",
		secondary:   "#c099ff",
		success:     "#c3e88d",
		warning:     "#ffc777",
		danger:      "#ff757f",
		easy:        "#c8d3f5",
		medium:      "#c3e88d",
		hard:        "#ff966c",
		border:      "#3b4261",
	},
	{
		name:        "tron",
		label:       "Tron",
		description: "Grid glow — cyan circuit on midnight",
		background:  "#0c141f",
		surface:     "#0a1018",
		text:        "#caf0ff",
		subtext:     "#4d6b87",
		muted:       "#4d6b87",
		accent:      "#00d9ff",
		primary:     "#00d9ff",
		secondary:   "#ff9000",
		success:     "#00ff8f",
		warning:     "#ffcc00",
		danger:      "#ff3333",
		easy:        "#caf0ff",
		medium:      "#00ff8f",
		hard:        "#ff9000",
		border:      "#1a2633",
	},
}

// aliases maps friendly spellings onto registered theme names.
var aliases = map[string]string{
	"default":      "openleet",
	"open-leet":    "openleet",
	"light":        "openleet-light",
	"paper":        "openleet-light",
	"latte":        "catppuccin-latte",
	"open-code":    "opencode",
	"catppuccin":   "catppuccin-mocha",
	"mocha":        "catppuccin-mocha",
	"gruvbox-dark": "gruvbox",
	"one-dark":     "onedark",
	"tokyo-night":  "tokyonight",
	"tokyo":        "tokyonight",
	"flexoki-dark": "flexoki",
	"dracula-dark": "dracula",
	"monokai-dark": "monokai",
	"tron-legacy":  "tron",
}

var (
	// Order is the display order used by the picker and the CLI listing.
	order    []string
	registry = map[string]Palette{}
)

func init() {
	for _, s := range specs {
		registry[s.name] = build(s)
		order = append(order, s.name)
	}
}

// All returns every registered theme in authoring order.
func All() []Palette {
	themes := make([]Palette, 0, len(order))
	for _, name := range order {
		themes = append(themes, registry[name])
	}
	return themes
}

// Names returns every registered theme name in authoring order.
func Names() []string {
	return append([]string(nil), order...)
}

// Get resolves a name (including aliases) to a palette.
func Get(name string) (Palette, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if canonical, ok := aliases[key]; ok {
		key = canonical
	}
	palette, ok := registry[key]
	return palette, ok
}

// Resolve returns the named theme, falling back to the brand default.
func Resolve(name string) Palette {
	if palette, ok := Get(name); ok {
		return palette
	}
	return Default()
}

// Default returns the OpenLeet brand theme.
func Default() Palette {
	return registry["openleet"]
}

// SortedNames returns the registered names alphabetically. It is used by
// diagnostics and the --list-themes flag where authoring order is not useful.
func SortedNames() []string {
	names := Names()
	sort.Strings(names)
	return names
}
