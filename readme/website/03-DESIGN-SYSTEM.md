# 03 — Design System

The website should feel like the app: a deterministic character grid translated
to the browser. Sharp edges, one accent, generous space, monospace for anything
the user types.

## 1. Principles

1. **Grid first.** Everything aligns to a 4px base. Components use 8px steps.
2. **One accent.** Orange means action or current state. Never decorative.
3. **Borders, not shadows.** Separation comes from 1px lines and surface tones.
4. **Terminal honesty.** Shell text, keycaps, and code are monospace.
5. **Quiet hierarchy.** Size and contrast before colour.
6. **No fake chrome.** Never wrap screenshots in macOS traffic lights or a
   browser bar. A 1px border and a caption is enough.

## 2. Colour tokens

All app colours are semantic. The website should use the same names so theme
data can be reused directly.

| Token | App field | Role | Dark (OpenLeet) | Light (OpenLeet Light) |
| :--- | :--- | :--- | :--- | :--- |
| `background` | `Background` | Page canvas | `#0d0a0b` | `#f7f2e8` |
| `surface` | `Surface` | Cards, inset panels | `#131313` | `#efe7da` |
| `surface_raised` | `SurfaceRaised` | Hover pills, modals, toasts | `#2e2e2e` * | `#d6cfc3` * |
| `card` | `Card` | Card body | `#1e1e1e` * | `#e5ddd0` * |
| `card_highlight` | `CardHighlight` | Hovered card | `#2a2a2a` * | `#ddd4c6` * |
| `border` | `Border` | 1px outlines | `#3a3a3a` | `#c9bcaa` |
| `border_focus` | `BorderFocus` | Focused/hovered outline | `#ff7a1a` | `#c2410c` |
| `text` | `Text` | Primary copy | `#f2f2f2` | `#211c17` |
| `text_dim` | `TextDim` | Secondary copy, labels | `#8a8a8a` | `#5a5248` |
| `muted` | `Muted` | Tertiary copy | `#6e6e6e` | `#6f655b` |
| `accent` | `Accent` | Primary action | `#ff7a1a` | `#c2410c` |
| `accent_fg` | `AccentFG` | Text on accent | `#0d0a0b` * | `#f7f2e8` * |
| `primary` | `Primary` | Secondary highlight | `#f2f2f2` | `#211c17` |
| `secondary` | `Secondary` | Info / links | `#8fa8f5` | `#3355b5` |
| `pass` | `Pass` | Success | `#b5f9ad` | `#2f7d32` |
| `fail` | `Fail` | Errors | `#ff5f5f` | `#b3261e` |
| `warn` | `Warn` | Warnings | `#ffb27a` | `#8a5300` |
| `info` | `Info` | Informational | `#8fa8f5` | `#3355b5` |
| `easy` | `Easy` | Easy difficulty | `#cfcfcf` | `#635b51` |
| `medium` | `Medium` | Medium difficulty | `#b5f9ad` | `#2f7d32` |
| `hard` | `Hard` | Hard difficulty | `#ffb27a` | `#8a5300` |
| `heat_low` | `HeatLow` | Heatmap low | `#5a2e10` * | `#eab9a2` * |
| `heat_mid` | `HeatMid` | Heatmap mid | `#a34f14` * | `#de8460` * |
| `heat_high` | `HeatHigh` | Heatmap high | `#ff7a1a` | `#c2410c` |

`*` Derived at runtime from `surface`, `text`, `background`, and `accent`, with
WCAG contrast guards. The values shown are the resolved OpenLeet / OpenLeet Light
results and are safe defaults for the site.

Rules:

- Never use `accent` as a background for body copy. It is a foreground colour and
  a 1px outline colour. Filled accent is reserved for the wordmark block and the
  primary button.
- `accent_fg` is the only colour allowed on an accent fill.
- Difficulty always carries a text label (`EASY`, `MEDIUM`, `HARD`); colour is
  reinforcement, never the only signal.
- Links use `secondary` on dark and `#3355b5` on light. Underline on hover.

## 3. Typography

The app is a terminal, so the website should lean monospace for identity and sans
for long copy.

### Stacks

```css
--font-sans: "Inter", "Geist", system-ui, -apple-system, "Segoe UI", sans-serif;
--font-mono: "JetBrains Mono", "IBM Plex Mono", "SFMono-Regular", Consolas, monospace;
```

The pixel wordmark in Home/Stats is a display render, not a webfont. Ship the
wordmark as an SVG built from hard rectangles, or recreate it with a CSS grid of
blocks. Do not load a novelty pixel font for body copy.

### Scale

| Token | Size / line-height | Weight | Use |
| :--- | :--- | :--- | :--- |
| `display` | 64 / 1.05 | 700 | Hero H1 on ≥1200px |
| `h1` | 48 / 1.1 | 700 | Page H1 |
| `h2` | 36 / 1.15 | 650 | Section titles |
| `h3` | 24 / 1.25 | 600 | Card titles |
| `h4` | 18 / 1.35 | 600 | Subheads |
| `body` | 16 / 1.65 | 400 | Paragraphs |
| `small` | 14 / 1.55 | 400 | Captions, metadata |
| `micro` | 12 / 1.4 | 600 | Eyebrows, labels, keycaps |

- Hero H1 max-width: 18ch–22ch. Keep it on 2–3 lines.
- Body measure: 60–72ch.
- Eyebrows: uppercase, 0.08em tracking, `text_dim`.
- Section titles: sentence case. Do not Title Case Every Word.

## 4. Spacing and layout

Base unit: **4px**.

| Token | px |
| :--- | :--- |
| `space-1` | 4 |
| `space-2` | 8 |
| `space-3` | 12 |
| `space-4` | 16 |
| `space-5` | 24 |
| `space-6` | 32 |
| `space-7` | 48 |
| `space-8` | 64 |
| `space-9` | 96 |

- Container: `max-width: 1200px`; wide media bands may go to `1280px`.
- Page gutter: 24px mobile, 40px tablet, 64px desktop.
- Section rhythm: 96px desktop, 64px tablet, 48px mobile.
- Grid: 12 columns desktop, 8 tablet, 4 mobile. Gutter 24px.
- Hero: 120px top padding desktop, 72px mobile.

## 5. Shape

| Element | Radius | Border |
| :--- | :--- | :--- |
| Buttons | 6px | 1px `border` (secondary), none (primary) |
| Tags (optional) | 999px or 4px | 1px `border` |
| Cards | 8px | 1px `border` |
| Screenshot frame | 8px | 1px `border` |
| Code blocks | 6px | 1px `border` |
| Inputs | 6px | 1px `border`, focus `border_focus` |

The app's header uses plain text labels with no border. On the website, tags and
secondary actions may still use a single-line bordered element with 12–16px
horizontal padding, 28px height, 13px text, and an optional dim icon.

## 6. Components

### Primary button

```text
background: accent
color: accent_fg
padding: 12px 20px
radius: 6px
font: 600 15px/1 sans
hover: brightness(1.06), translateY(-1px)
active: translateY(0)
focus-visible: 2px outline in border_focus, 2px offset
```

Copy: sentence case, verb first. `Install OpenLeet`, `Read the docs`.

### Secondary button

Transparent background, 1px `border`, `text` label. Hover: `surface_raised`
background and `border_focus` outline.

### Header label

Mirrors the app's top bar: plain text, no border, no icon.

```text
height: 28px
padding: 0 8px
background: transparent (rest)
color: text_dim (rest), text (hover/focus), accent (pressed or signed-out Log in)
border-radius: 6px
hover/focus: background surface_raised, no border
gap between labels: 16px (3 cells equivalent)
```

No square brackets, slashes, corner glyphs, or ASCII stand-in icons. On the web,
tooltips appear on hover/focus only.

### Tag (optional)

A bordered, single-line chip for metadata such as `Python`, `Local`, or
`17 problems`. It is not used in the app header.

```text
height: 28px
padding: 0 12px
background: surface_raised
border: 1px solid border
border-radius: 999px (rounded) or 4px (square)
font: 500 13px/1 sans
icon: text_dim, 14px, 6px gap
hover/focus: border-color: border_focus
pressed: border-color: accent
```

Examples: `Python`, `Local`, `17 problems`.

### Keycap

```text
display: inline-flex; align-items: center;
height: 22px; padding: 0 6px;
font: 500 12px/1 mono;
color: text;
background: surface_raised;
border: 1px solid border;
border-bottom-width: 2px;
border-radius: 4px;
```

### Feature card

- Background `surface`, 1px `border`, radius 8px, padding 24px.
- Optional 28px icon in `text_dim`; title `h3`; body `body` in `text_dim`.
- Hover: border `border_focus`, background `card_highlight`.
- No drop shadows. No gradient borders.

### Screenshot frame

- 1px `border`, radius 8px, background `background`.
- Optional caption below in `small` / `text_dim`: `Roadmap — DSA · 3/17 solved`.
- No browser dots, no title bar, no tilt, no perspective.
- Keep native aspect ratio (16:9 for 1920×1080 captures).
- On dark pages, do not add a vignette. On light pages, use a 1px border and a
  4px inner padding so the dark screenshot does not bleed.

### Stat block

Three-line block: value in `h2`, label in `micro` uppercase `text_dim`, one-line
note in `small`. Example:

```text
3/17        SOLVED
17          PROBLEMS
2 days      BEST STREAK
```

### Theme swatch

A row of five 16×16 squares: `accent`, `primary`, `secondary`, `pass`, `fail`,
with a 1px `border` around each. No rounded corners.

### Code block

- Background `surface`, 1px `border`, radius 6px.
- Mono 14px/1.6, `text`.
- A copy button in the top-right, 32×32, ghost style.
- Prompt glyph `$` or `>` in `text_dim`, never coloured.
- Windows examples use PowerShell syntax and a `>` prompt.

### Nav

- Height 64px, background `background` with 92% opacity and 12px backdrop blur,
  1px bottom border.
- Wordmark left, links center-right, primary CTA right.
- Active link: `text`; inactive: `text_dim`; hover: `text` + underline offset 6px.

### Footer

- Background `surface`, top border `border`.
- Four columns on desktop, stacked on mobile.
- Wordmark and one sentence at the top.
- No newsletter form.

## 7. Interaction and motion

- Duration: 120ms for colour/border, 180ms for transforms.
- Easing: `cubic-bezier(0.2, 0.8, 0.2, 1)`.
- Allowed: fade-in on scroll (subtle), hover border lift, copy-button check.
- Forbidden: parallax, autoplay carousels, scroll-jacking, bouncing icons.
- Respect `prefers-reduced-motion: reduce` by disabling transforms and fades.

## 8. Accessibility

- Body text contrast ≥ 4.5:1; large text ≥ 3:1; borders ≥ 3:1 against their
  surface (the app's Normalize pass enforces this for themes).
- Never rely on colour alone for difficulty, save state, or status. Pair with
  text (`EASY`, `saved`, `unsaved`).
- Every icon-only control needs an `aria-label`.
- Keycaps are `<kbd>`; shortcuts in copy use `<kbd>` too.
- Focus rings: 2px `border_focus`, 2px offset, never removed.
- Screenshots need descriptive alt text. See `09-ASSETS.md`.
- Motion respects `prefers-reduced-motion`.
- Terminal screenshots contain small text; always link to the full-size image.

## 9. Light and dark mode

- Default to dark (`openleet`). Persist the user's choice in `localStorage`.
- The light theme is not an inverted dark theme: use `OpenLeet Light` values
  (`#f7f2e8` paper, `#211c17` ink, `#c2410c` accent).
- Screenshots stay as captured (dark or light); do not recolour them.
- When the site is light, frame dark screenshots with a visible border and 4px
  inner padding so the edge reads as intentional.
- Theme switch control: a two-state chip labelled `Dark` / `Light`, not a
  three-way toggle with "auto" unless you also respect the OS setting.

## 10. Do / Don't

| Do | Don't |
| :--- | :--- |
| Use hard-edged rectangles and 1px borders | Use glassmorphism or neon glows |
| Show real screenshots | Draw fake terminal UI |
| Use mono for keys and commands | Use mono for paragraphs |
| Keep orange for actions | Paint large orange backgrounds |
| Write short, concrete copy | Use hype adjectives |
| Show the whole grid at native ratio | Crop screenshots mid-panel |
| Use 12 themes as a feature | Use a rainbow gradient as decoration |
