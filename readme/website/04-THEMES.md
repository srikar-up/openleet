# 04 — Themes

OpenLeet ships twelve palettes. The app treats colour as a semantic contract, not
a stylesheet: every view reads tokens such as `surface_raised`, `border_focus`,
`text_dim`, `accent`, `pass`, `fail`, and `warn` from the active palette. The
website should reuse the same token names so a theme picker on the site can drive
the whole page.

## Where themes appear in the app

- **F4** opens Settings → Appearance with the theme grid.
- **`/themes`** opens the same picker as a quick action over the current screen.
- **Settings → Appearance** lists all themes with a live preview; Enter applies,
  Esc reverts.
- **`--list-themes`** prints the registry for scripting.
- **`/theme-audit`** renders a pass/fail table that checks every palette's
  semantic contrast at multiple widths.

## The twelve palettes

| Name | Label | Mode | Description |
| :--- | :--- | :--- | :--- |
| `openleet` | OpenLeet | Dark | Brand orange on ink — the default roadmap palette |
| `openleet-light` | OpenLeet Light | Light | Brand orange on warm paper — light mode |
| `opencode` | OpenCode | Dark | Warm ink and apricot, the reference TUI palette |
| `catppuccin-mocha` | Catppuccin Mocha | Dark | Soft pastel mocha with peach highlights |
| `catppuccin-latte` | Catppuccin Latte | Light | Soft pastel latte with peach highlights |
| `dracula` | Dracula | Dark | Night purple with vivid coral and mint |
| `flexoki` | Flexoki | Dark | Inky paper tones with earthen accents |
| `gruvbox` | Gruvbox | Dark | Retro warm greys with bright orange |
| `monokai` | Monokai | Dark | Editor classic — neon fruit on charcoal |
| `onedark` | One Dark | Dark | Atom's balanced blue-grey editor scheme |
| `tokyonight` | Tokyo Night | Dark | Neon city blues and warm coral |
| `tron` | Tron | Dark | Grid glow — cyan circuit on midnight |

## Base colours

These are the authored values. `surface_raised`, `card`, `card_highlight`,
`border_focus`, `pass`, `fail`, `warn`, `info`, `accent_fg`, and the heat scale
are derived at runtime by `theme.Normalize`, which enforces contrast guards.

### openleet (default, dark)

| Token | Hex |
| :--- | :--- |
| background | `#0d0a0b` |
| surface | `#131313` |
| text | `#f2f2f2` |
| subtext | `#8a8a8a` |
| muted | `#6e6e6e` |
| accent | `#ff7a1a` |
| primary | `#f2f2f2` |
| secondary | `#8fa8f5` |
| success | `#b5f9ad` |
| warning | `#ffb27a` |
| danger | `#ff5f5f` |
| border | `#3a3a3a` |
| easy / medium / hard | `#cfcfcf` / `#b5f9ad` / `#ffb27a` |

### openleet-light (light)

| Token | Hex |
| :--- | :--- |
| background | `#f7f2e8` |
| surface | `#efe7da` |
| text | `#211c17` |
| subtext | `#5a5248` |
| muted | `#6f655b` |
| accent | `#c2410c` |
| primary | `#211c17` |
| secondary | `#3355b5` |
| success | `#2f7d32` |
| warning | `#8a5300` |
| danger | `#b3261e` |
| border | `#c9bcaa` |
| easy / medium / hard | `#635b51` / `#2f7d32` / `#8a5300` |

### opencode (dark)

| Token | Hex |
| :--- | :--- |
| background | `#212121` |
| surface | `#252525` |
| text | `#e0e0e0` |
| subtext | `#6a6a6a` |
| muted | `#6a6a6a` |
| accent | `#fab283` |
| primary | `#fab283` |
| secondary | `#5c9cf5` |
| success | `#7fd88f` |
| warning | `#f5a742` |
| danger | `#e06c75` |
| border | `#4b4c5c` |
| easy / medium / hard | `#e0e0e0` / `#7fd88f` / `#f5a742` |

### catppuccin-mocha (dark)

| Token | Hex |
| :--- | :--- |
| background | `#1e1e2e` |
| surface | `#181825` |
| text | `#cdd6f4` |
| subtext | `#a6adc8` |
| muted | `#6c7086` |
| accent | `#fab387` |
| primary | `#89b4fa` |
| secondary | `#cba6f7` |
| success | `#a6e3a1` |
| warning | `#f9e2af` |
| danger | `#f38ba8` |
| border | `#313244` |
| easy / medium / hard | `#cdd6f4` / `#a6e3a1` / `#fab387` |

### catppuccin-latte (light)

| Token | Hex |
| :--- | :--- |
| background | `#eff1f5` |
| surface | `#e6e9ef` |
| text | `#4c4f69` |
| subtext | `#6c6f85` |
| muted | `#8c8fa1` |
| accent | `#fe640b` |
| primary | `#1e66f5` |
| secondary | `#8839ef` |
| success | `#40a02b` |
| warning | `#df8e1d` |
| danger | `#d20f39` |
| border | `#ccd0da` |
| easy / medium / hard | `#4c4f69` / `#40a02b` / `#fe640b` |

### dracula (dark)

| Token | Hex |
| :--- | :--- |
| background | `#282a36` |
| surface | `#21222c` |
| text | `#f8f8f2` |
| subtext | `#6272a4` |
| muted | `#6272a4` |
| accent | `#ffb86c` |
| primary | `#bd93f9` |
| secondary | `#8be9fd` |
| success | `#50fa7b` |
| warning | `#f1fa8c` |
| danger | `#ff5555` |
| border | `#44475a` |
| easy / medium / hard | `#f8f8f2` / `#50fa7b` / `#ffb86c` |

### flexoki (dark)

| Token | Hex |
| :--- | :--- |
| background | `#100f0f` |
| surface | `#1c1b1a` |
| text | `#cecdc3` |
| subtext | `#878580` |
| muted | `#575653` |
| accent | `#da702c` |
| primary | `#4385be` |
| secondary | `#8b7ec8` |
| success | `#879a39` |
| warning | `#d0a215` |
| danger | `#d14d41` |
| border | `#282726` |
| easy / medium / hard | `#cecdc3` / `#879a39` / `#da702c` |

### gruvbox (dark)

| Token | Hex |
| :--- | :--- |
| background | `#282828` |
| surface | `#1d2021` |
| text | `#ebdbb2` |
| subtext | `#a89984` |
| muted | `#928374` |
| accent | `#fe8019` |
| primary | `#fabd2f` |
| secondary | `#83a598` |
| success | `#b8bb26` |
| warning | `#fabd2f` |
| danger | `#fb4934` |
| border | `#3c3836` |
| easy / medium / hard | `#ebdbb2` / `#b8bb26` / `#fe8019` |

### monokai (dark)

| Token | Hex |
| :--- | :--- |
| background | `#2d2a2e` |
| surface | `#221f22` |
| text | `#fcfcfa` |
| subtext | `#939293` |
| muted | `#727072` |
| accent | `#fc9867` |
| primary | `#a9dc76` |
| secondary | `#78dce8` |
| success | `#a9dc76` |
| warning | `#ffd866` |
| danger | `#ff6188` |
| border | `#403e41` |
| easy / medium / hard | `#fcfcfa` / `#a9dc76` / `#fc9867` |

### onedark (dark)

| Token | Hex |
| :--- | :--- |
| background | `#282c34` |
| surface | `#21252b` |
| text | `#abb2bf` |
| subtext | `#7f848e` |
| muted | `#5c6370` |
| accent | `#d19a66` |
| primary | `#61afef` |
| secondary | `#c678dd` |
| success | `#98c379` |
| warning | `#e5c07b` |
| danger | `#e06c75` |
| border | `#3b4048` |
| easy / medium / hard | `#abb2bf` / `#98c379` / `#d19a66` |

### tokyonight (dark)

| Token | Hex |
| :--- | :--- |
| background | `#222436` |
| surface | `#1e2030` |
| text | `#c8d3f5` |
| subtext | `#828bb8` |
| muted | `#636da6` |
| accent | `#ff966c` |
| primary | `#82aaff` |
| secondary | `#c099ff` |
| success | `#c3e88d` |
| warning | `#ffc777` |
| danger | `#ff757f` |
| border | `#3b4261` |
| easy / medium / hard | `#c8d3f5` / `#c3e88d` / `#ff966c` |

### tron (dark)

| Token | Hex |
| :--- | :--- |
| background | `#0c141f` |
| surface | `#0a1018` |
| text | `#caf0ff` |
| subtext | `#4d6b87` |
| muted | `#4d6b87` |
| accent | `#00d9ff` |
| primary | `#00d9ff` |
| secondary | `#ff9000` |
| success | `#00ff8f` |
| warning | `#ffcc00` |
| danger | `#ff3333` |
| border | `#1a2633` |
| easy / medium / hard | `#caf0ff` / `#00ff8f` / `#ff9000` |

## Derived tokens (implementation notes)

- `surface_raised` = `mix(surface, text, 0.12)` with a minimum 1.15 contrast
  against `background`.
- `card` = `mix(surface, text, 0.05)`.
- `card_highlight` = `mix(surface, text, 0.10)`.
- `border_focus` defaults to `accent`, guarded to 3:1 against `surface_raised`.
- `accent_fg` is chosen as the best text colour on `accent`, guarded to 4.5:1.
- `pass` / `fail` / `warn` / `info` fall back to `success` / `danger` /
  `warning` / `secondary` and are nudged until they are ≥3:1 on
  `surface_raised` and visually distinct from each other.
- `heat_low` = `mix(background, accent, 0.32)`.
- `heat_mid` = `mix(background, accent, 0.62)`.
- `heat_high` = `accent`.

For the website, you can ship the resolved values above. If you add a live theme
switcher, port `mix` and the contrast guard, or import the generated JSON (see
next section).

## Suggested JSON shape

If the site needs data-driven themes, use this shape:

```json
{
  "name": "openleet",
  "label": "OpenLeet",
  "mode": "dark",
  "description": "Brand orange on ink — the default roadmap palette",
  "colors": {
    "background": "#0d0a0b",
    "surface": "#131313",
    "surface_raised": "#2e2e2e",
    "text": "#f2f2f2",
    "text_dim": "#8a8a8a",
    "muted": "#6e6e6e",
    "border": "#3a3a3a",
    "border_focus": "#ff7a1a",
    "accent": "#ff7a1a",
    "accent_fg": "#0d0a0b",
    "primary": "#f2f2f2",
    "secondary": "#8fa8f5",
    "pass": "#b5f9ad",
    "fail": "#ff5f5f",
    "warn": "#ffb27a",
    "info": "#8fa8f5",
    "easy": "#cfcfcf",
    "medium": "#b5f9ad",
    "hard": "#ffb27a",
    "heat_low": "#5a2e10",
    "heat_mid": "#a34f14",
    "heat_high": "#ff7a1a"
  }
}
```

## Aliases

The CLI resolves friendly spellings:

| Alias | Theme |
| :--- | :--- |
| `default`, `open-leet` | `openleet` |
| `light`, `paper` | `openleet-light` |
| `latte` | `catppuccin-latte` |
| `open-code` | `opencode` |
| `catppuccin`, `mocha` | `catppuccin-mocha` |
| `gruvbox-dark` | `gruvbox` |
| `one-dark` | `onedark` |
| `tokyo-night`, `tokyo` | `tokyonight` |
| `flexoki-dark` | `flexoki` |
| `dracula-dark` | `dracula` |
| `monokai-dark` | `monokai` |
| `tron-legacy` | `tron` |

## Theme usage on the website

- **Theme wall:** show all 12 as cards with 5-swatch strips and the mode label.
- **Live preview:** picker changes CSS custom properties on `:root`; persist in
  `localStorage` under `openleet.theme`.
- **Screenshots:** attach the theme name in the caption when an image was
  captured with a non-default palette (`Roadmap — Dracula`).
- **Contrast:** run the same audit claim in copy: every palette is guarded to
  4.5:1 for body text and 3:1 for borders on raised surfaces.
