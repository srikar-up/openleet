# 02 — Sitemap and Page Structure

Two viable shapes: a **single long landing page** (fastest to ship) or a
**five-page site** (better for SEO and docs). Both use the same section blocks.

## Recommended navigation

```text
OPENLEET     Features   Screens   Themes   Install   Docs        [ Install ]  [ GitHub ]
```

- The wordmark links to `/`.
- `Install` is the primary CTA and should also be the last nav item's button.
- `GitHub` is an external link with an outbound icon.
- On mobile, collapse to a sheet with the same order; keep the Install CTA
  visible in the bar.

## Footer

```text
OPENLEET
A terminal-first DSA practice dojo with a Socratic AI coach.

Product            Resources           Community
Features           Documentation       GitHub
Screens            Changelog           Discussions
Themes             Keyboard shortcuts  Report an issue
Install            AI providers

© 2026 OpenLeet. Built in the terminal.
"No account required. Your progress stays on your machine."
```

## Option A — one-page landing (ship first)

| # | Section | Purpose | Asset |
| :-- | :--- | :--- | :--- |
| 1 | Hero | Promise + install CTA | `home.webp` or `roadmap.webp` |
| 2 | Proof strip | 4 facts: 17 problems · 12 themes · Python + Go · local-first | text only |
| 3 | Problem | "Browser tabs are not a dojo" | text |
| 4 | The loop | 4 steps: Pick → Solve → Run → Ask | `solve.webp` |
| 5 | Feature grid | 6 cards: roadmap, runner, coach, stats, themes, settings | `roadmap.webp`, `stats.webp` |
| 6 | Screens band | Horizontal scroll of 4 screens with captions | gallery |
| 7 | Themes | Swatch strip + palette cards | `settings-dracula-appearance.webp` |
| 8 | Keyboard strip | Show 6 shortcuts as keycaps | text |
| 9 | Privacy | Local vault, optional sync | `settings-account.webp` |
| 10 | Install | Tabs: Windows / Linux / macOS | code blocks |
| 11 | FAQ | 6 questions | text |
| 12 | Footer CTA | Wordmark + install command | `home-light.webp` |

### Home wireframe

```text
┌──────────────────────────────────────────────────────────────┐
│ nav: wordmark · Features · Screens · Themes · Install · GitHub│
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  A terminal-first DSA dojo.                                  │
│  Real tests. Real hints. No browser.                          │
│                                                              │
│  [ Install OpenLeet ]   [ See the screens ]                   │
│  One binary · No account · Works offline                      │
│                                                              │
│  ┌──────────────────────────── screenshot ─────────────────┐ │
│  │  roadmap.webp                                             │ │
│  └──────────────────────────────────────────────────────────┘ │
├──────────────────────────────────────────────────────────────┤
│  17 problems · 12 themes · Python + Go · 100% local           │
├──────────────────────────────────────────────────────────────┤
│  The loop                                                     │
│  01 Pick        02 Solve        03 Run          04 Ask         │
├──────────────────────────────────────────────────────────────┤
│  Features (2 × 3 grid)                                        │
├──────────────────────────────────────────────────────────────┤
│  Screens (4 cards)                                            │
├──────────────────────────────────────────────────────────────┤
│  Themes (swatches + 2 palette cards)                          │
├──────────────────────────────────────────────────────────────┤
│  Install (tabs + code)                                        │
├──────────────────────────────────────────────────────────────┤
│  FAQ                                                          │
├──────────────────────────────────────────────────────────────┤
│  Footer                                                       │
└──────────────────────────────────────────────────────────────┘
```

## Option B — five pages

| Page | Route | Primary keyword | Key sections |
| :--- | :--- | :--- | :--- |
| Landing | `/` | terminal DSA practice | Hero, loop, features, screens, install, FAQ |
| Features | `/features` | DSA practice tool | Roadmap, runner, coach, stats, themes, settings, data tools |
| Screens | `/screens` | terminal LeetCode alternative | Home, Roadmap, Solve, Stats, Settings, Quick actions |
| Themes | `/themes` | terminal color themes | Swatch wall, all 12 palettes, light mode, audit |
| Install | `/install` | install OpenLeet | Requirements, Windows/Linux/macOS, first run, troubleshooting |

Optional later: `/changelog`, `/docs` (link to the repo readme), `/about`.

## Section recipes

### Hero

- **Eyebrow:** `TERMINAL-FIRST DSA PRACTICE`
- **H1:** `Practice algorithms where you already work.`
- **Sub:** `OpenLeet puts the roadmap, your code, the real tests, and a Socratic
  coach on one terminal surface. Python and Go run on your machine.`
- **Primary CTA:** `Install OpenLeet` → scrolls to `#install`.
- **Secondary CTA:** `See the screens` → scrolls to `#screens`.
- **Trust line:** `No account required · Works offline · Windows, Linux, macOS`
- **Visual:** full-bleed `roadmap.webp` inside a 1px `border` frame, no fake
  browser chrome, caption `Roadmap — DSA · 3/17 solved`.

### Proof strip

Four inline facts separated by `·`, no icons required:

`17 curated problems` · `12 themes` · `Python + Go` · `Local-first`

Set in 12–14px uppercase with 0.08em tracking, `text_dim`.

### The loop

Four steps, numbered `01–04`. Each step gets one sentence and one icon at most.

| Step | Title | Copy |
| :-- | :--- | :--- |
| 01 | Pick | A winding roadmap with topic and company filters. |
| 02 | Solve | Statement, editor, and console in one view. |
| 03 | Run | Real tests against your installed Python or Go. |
| 04 | Ask | Socratic hints that ask the next question. |

Pair with `solve.webp` on the right (desktop) or below (mobile).

### Feature grid

Six cards, 3×2 on desktop, 1 column on mobile. Each card: small label, title,
two-line body, optional `text_dim` footnote.

1. **Roadmap** — A winding grid with solved markers and six-card windows.
2. **Host runner** — Detects Python and Go, injects a harness, enforces a timeout.
3. **Socratic coach** — Streams questions and hints from local Ollama or a cloud
   provider. A "never send solution code" switch is built in.
4. **Stats** — Streaks, a 12-month heatmap, difficulty split, and topic counts.
5. **Themes** — 12 palettes with live preview and a contrast audit.
6. **Settings** — Schema-driven, searchable, mouse-friendly, nine sections.

### Screens band

Four cards in a horizontal scroller. Each: image, screen name, one line.

| Card | Image | Caption |
| :--- | :--- | :--- |
| Home | `home.webp` | Search, language, solved count, and quick actions. |
| Roadmap | `roadmap.webp` | A winding grid with filters and solved state. |
| Solve | `solve.webp` | Statement, editor, and console, together. |
| Stats | `stats.webp` | Streaks, heatmap, difficulty split. |

### Themes band

- Lead with the swatch strip: five squares per theme (accent, primary, secondary,
  success, danger).
- Show two large cards: `OpenLeet` (dark) and `OpenLeet Light` (light), each with
  background, text, accent, and border swatches.
- Link to the full palette table in `04-THEMES.md`.
- Copy: `Twelve palettes. One grid. Switch with F4 and preview before you apply.`

### Keyboard strip

Render 6 keycaps: `F2 solve`, `F3 stats`, `F4 themes`, `ctrl+p commands`,
`ctrl+r run`, `ctrl+s save`. Use `<kbd>` elements with mono type.

Copy: `Every action has a key. Nothing is buried in a menu.`

### Privacy band

Two columns: copy left, `settings-account.webp` right.

- `Local by default. Your progress, code buffers, and layout stay on disk.`
- `Provider keys live in an encrypted local vault.`
- `Accounts and per-user sync are optional.`
- `Firebase ID tokens stay in memory.`

### Install band

Tabs: **Windows**, **Linux**, **macOS**. Each shows a code block and a one-line
note. See `08-CONTENT.md` for exact commands.

### FAQ

Six items, `<details>` or an accordion. See `08-CONTENT.md`.

### Footer CTA

Wordmark, one sentence, install command in a copyable block, and the three
primary links. Keep it quieter than the hero.

## Responsive behaviour

| Breakpoint | Rules |
| :--- | :--- |
| ≥ 1200px | 12-col grid, 3-up features, screenshots at full width |
| 768–1199px | 8-col grid, 2-up features, screenshots cropped to content |
| < 768px | Single column, hero H1 40px, screenshots scroll horizontally |
| < 480px | Hide the proof strip icons; keep the text facts |

## SEO page titles

| Page | Title | Meta description |
| :--- | :--- | :--- |
| Landing | `OpenLeet — Terminal-first DSA practice with a Socratic AI coach` | `Practise algorithms in your terminal. Real Python and Go tests, Socratic AI hints, 12 themes, local-first progress. No browser, no account required.` |
| Features | `Features — OpenLeet` | `Roadmap, host runner, Socratic coach, stats, themes, and schema-driven settings. Everything the practice loop needs.` |
| Screens | `Screens — OpenLeet` | `See every OpenLeet screen: home, roadmap, solve workspace, stats, settings, and quick actions.` |
| Themes | `Themes — OpenLeet` | `Twelve terminal palettes, light and dark, with live preview and a contrast audit.` |
| Install | `Install — OpenLeet` | `Build or download OpenLeet for Windows, Linux, or macOS. One binary, no account.` |
