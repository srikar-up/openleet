# 09 — Asset Manifest

All images live in [`../screenshots`](../screenshots). They are WebP at native
size, encoded at quality 88. They were captured from a real binary with VHS at
16px font, truecolor, no padding or margin. Terminal columns depend on the
capture width: 1920px ≈ 184 columns, 1670px ≈ 160 columns, 1250px ≈ 120 columns,
1050px ≈ 100 columns.

## Canonical screenshots

| File | Size | Theme | What it shows | Suggested use |
| :--- | :--- | :--- | :--- | :--- |
| `home.webp` | 1920×1080 · 18 KB | OpenLeet | Home: wordmark, search box, mini stats, plain header | Hero fallback, Home feature card |
| `home-light.webp` | 1920×1080 · 18 KB | OpenLeet Light | Home in light mode | Light-mode section, theme toggle proof |
| `home-gruvbox.webp` | 1920×1080 · 18 KB | Gruvbox | Home at the standard capture size | Theme gallery card |
| `home-dracula.webp` | 1920×1080 · 18 KB | Dracula | Home at the standard capture size | Theme gallery card |
| `home-tron.webp` | 1920×1080 · 19 KB | Tron | Home at the standard capture size | Theme gallery card |
| `roadmap.webp` | 1920×1080 · 84 KB | OpenLeet | Winding roadmap, topic rail, inspector | **Primary hero image** |
| `roadmap-dracula.webp` | 1920×1080 · 83 KB | Dracula | Roadmap in Dracula | Theme section |
| `solve.webp` | 1920×1080 · 93 KB | OpenLeet | Three-pane solve workspace + session rail | "The loop" section |
| `solve-light.webp` | 1920×1080 · 94 KB | OpenLeet Light | Solve in light mode | Light-mode proof |
| `solve-tron.webp` | 1920×1080 · 94 KB | Tron | Solve in Tron | Theme section |
| `stats.webp` | 1920×1080 · 28 KB | OpenLeet | Streak, heatmap, difficulty split, topics | Stats feature card |
| `stats-light.webp` | 1920×1080 · 30 KB | OpenLeet Light | Stats in light mode | Light-mode proof |
| `settings-general.webp` | 1920×1080 · 23 KB | OpenLeet | Settings → General | Settings feature card |
| `settings-appearance.webp` | 1920×1080 · 39 KB | OpenLeet | Theme grid + interface | Themes section |
| `settings-dracula-appearance.webp` | 1920×1080 · 38 KB | Dracula | Theme grid in Dracula | **Themes section hero** |
| `settings-ai.webp` | 1920×1080 · 45 KB | OpenLeet | Provider cards, coach/model assignment | AI feature card |
| `settings-account.webp` | 1920×1080 · 34 KB | OpenLeet | Signed-out account and sync | Privacy section |
| `settings-data.webp` | 1920×1080 · 40 KB | OpenLeet | Export, import, backup, reset | Data tools card |

## Theme gallery

[`theme/`](../screenshots/theme/README.md) contains the Home screen in all
twelve palettes at **1920×1080**. Use these for a theme switcher or gallery
band; keep the native size and do not crop or recolour. Paths below are relative
to `readme/screenshots`.

| File | Theme |
| :--- | :--- |
| `theme/openleet.webp` | OpenLeet |
| `theme/openleet-light.webp` | OpenLeet Light |
| `theme/opencode.webp` | OpenCode |
| `theme/catppuccin-mocha.webp` | Catppuccin Mocha |
| `theme/catppuccin-latte.webp` | Catppuccin Latte |
| `theme/dracula.webp` | Dracula |
| `theme/flexoki.webp` | Flexoki |
| `theme/gruvbox.webp` | Gruvbox |
| `theme/monokai.webp` | Monokai |
| `theme/onedark.webp` | One Dark |
| `theme/tokyonight.webp` | Tokyo Night |
| `theme/tron.webp` | Tron |

Each file has descriptive alt text in the theme index.

## Modal and palette captures

These focus on overlays. Their underlying chrome may predate the current header;
crop to the overlay if the difference matters.

| File | Size | What it shows | Suggested use |
| :--- | :--- | :--- | :--- |
| `quick-actions.webp` | 1920×1080 · 32 KB | Quick-action palette over Home | Commands section |
| `theme-popup.webp` | 1920×1080 · 101 KB | Theme picker preview over Solve | Themes / speed section |
| `ai-integration-picker.webp` | 900×434 · 22 KB | "Connect an integration" picker | AI feature card (cropped) |

## Focus and interaction proof

| File | Size | What it shows |
| :--- | :--- | :--- |
| `home-focus.webp` | 1670×900 · 18 KB | Keyboard focus on the account label (`F6`, `Tab`, `Tab`) with the `Account` tooltip on the row below |

This is a QA receipt for the header interaction, not a marketing hero. It is
captured by `scripts/capture-focus.sh`, which holds the focus state in tmux and
then attaches VHS for the screenshot.

## Narrow-width proofs

These show the Home header and mini stats at the responsive breakpoints. Keep
them as QA receipts, not as marketing hero images.

| File | Size | Columns | Theme |
| :--- | :--- | :--- | :--- |
| `narrow/home-100-openleet.webp` | 1050×650 · 16 KB | ~100 | OpenLeet |
| `narrow/home-100-openleet-light.webp` | 1050×650 · 16 KB | ~100 | OpenLeet Light |
| `narrow/home-120-openleet.webp` | 1250×700 · 15 KB | ~120 | OpenLeet |
| `narrow/home-120-openleet-light.webp` | 1250×700 · 16 KB | ~120 | OpenLeet Light |

## Alt text

Use the screen name plus the concrete content. Keep it under ~140 characters.

| File | Alt text |
| :--- | :--- |
| `home.webp` | `OpenLeet home screen with a centered wordmark, a problem search box, and a line reading 1d streak · 3/17 solved.` |
| `home-light.webp` | `OpenLeet home screen in light mode on warm paper, with the same search box and progress line.` |
| `roadmap.webp` | `OpenLeet roadmap with a topic rail, a winding grid of problem cards, and a problem inspector.` |
| `roadmap-dracula.webp` | `OpenLeet roadmap in the Dracula palette.` |
| `solve.webp` | `OpenLeet solve workspace with statement, Python editor, session rail, and a tabbed console.` |
| `solve-light.webp` | `OpenLeet solve workspace in light mode.` |
| `solve-tron.webp` | `OpenLeet solve workspace in the Tron palette.` |
| `stats.webp` | `OpenLeet stats dashboard with a solved counter, twelve-month heatmap, streaks, and difficulty split.` |
| `stats-light.webp` | `OpenLeet stats dashboard in light mode.` |
| `settings-general.webp` | `OpenLeet Settings on the General section with startup and quit options.` |
| `settings-appearance.webp` | `OpenLeet Settings on Appearance with a grid of twelve theme previews.` |
| `settings-dracula-appearance.webp` | `OpenLeet theme grid in the Dracula palette.` |
| `settings-ai.webp` | `OpenLeet AI Models settings with provider cards and coach model assignment.` |
| `settings-account.webp` | `OpenLeet account settings signed out, with Google sign-in, email, and cloud sync.` |
| `settings-data.webp` | `OpenLeet data settings with export, import, backup, and reset actions.` |
| `quick-actions.webp` | `OpenLeet quick-action palette open over the home screen.` |
| `home-focus.webp` | `OpenLeet header with keyboard focus on the account label and an Account tooltip below it.` |
| `theme-popup.webp` | `OpenLeet theme picker previewing a palette over the solve workspace.` |
| `ai-integration-picker.webp` | `OpenLeet Connect an integration picker with search and provider rows.` |

## Placement guide

| Page area | Asset | Notes |
| :--- | :--- | :--- |
| Hero | `roadmap.webp` | Full width, native 16:9, 1px border, caption `Roadmap — DSA · 3/17 solved` |
| Loop section | `solve.webp` | Right column on desktop, below on mobile |
| Feature grid | `home.webp`, `stats.webp` | Optional; the grid works without images |
| Screens band | home, roadmap, solve, stats | Horizontal scroll on mobile |
| Themes | `settings-dracula-appearance.webp` + swatch rows | Show the palette name in the caption |
| Privacy | `settings-account.webp` | Pair with the four privacy bullets |
| Install | `home-light.webp` | Optional closing image |

## Image rules

- Keep the native aspect ratio. Never crop mid-panel.
- Do not recolour, invert, or add gradients.
- On light pages, wrap dark screenshots in a 1px `border` with 4px inner padding.
- No fake browser chrome, traffic lights, tilts, or perspective.
- The WebP files are the source and the shipped format; keep the native dimensions.
- If a platform needs PNG or JPEG, export a derivative without replacing the WebP source.
- Set explicit `width`/`height` attributes to avoid layout shift.
- Lazy-load everything below the hero.
- The captures are already encoded at quality 88; do not recompress them.

## Social and OG

- `og-home.png`: 1200×630 crop of `home.webp` with the wordmark and the header
  visible. Do not add text overlays over the screenshot.
- `og-themes.png`: 1200×630 crop of `settings-dracula-appearance.webp`.
- Favicon: build the block wordmark at 512×512 (accent square with the letter
  `L` or the full `LEET` block), then export 32/180/512 sizes.
