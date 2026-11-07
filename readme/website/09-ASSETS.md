# 09 — Asset Manifest

All images live in [`../screenshots`](../screenshots). They were captured from a
real binary with VHS at 16px font, truecolor, no padding or margin. Terminal
columns depend on the capture width: 1920px ≈ 184 columns, 1670px ≈ 160 columns,
1250px ≈ 120 columns, 1050px ≈ 100 columns.

## Canonical screenshots

| File | Size | Theme | What it shows | Suggested use |
| :--- | :--- | :--- | :--- | :--- |
| `home.png` | 1920×1080 · 65 KB | OpenLeet | Home: wordmark, search box, mini stats, plain header | Hero fallback, Home feature card |
| `home-light.png` | 1920×1080 · 67 KB | OpenLeet Light | Home in light mode | Light-mode section, theme toggle proof |
| `home-gruvbox.png` | 1670×900 · 67 KB | Gruvbox | Home at 160 columns | Theme gallery card |
| `home-dracula.png` | 1670×900 · 66 KB | Dracula | Home at 160 columns | Theme gallery card |
| `home-tron.png` | 1670×900 · 67 KB | Tron | Home at 160 columns | Theme gallery card |
| `roadmap.png` | 1920×1080 · 342 KB | OpenLeet | Winding roadmap, topic rail, inspector | **Primary hero image** |
| `roadmap-dracula.png` | 1920×1080 · 354 KB | Dracula | Roadmap in Dracula | Theme section |
| `solve.png` | 1920×1080 · 384 KB | OpenLeet | Three-pane solve workspace + session rail | "The loop" section |
| `solve-light.png` | 1920×1080 · 393 KB | OpenLeet Light | Solve in light mode | Light-mode proof |
| `solve-tron.png` | 1920×1080 · 396 KB | Tron | Solve in Tron | Theme section |
| `stats.png` | 1920×1080 · 103 KB | OpenLeet | Streak, heatmap, difficulty split, topics | Stats feature card |
| `stats-light.png` | 1920×1080 · 109 KB | OpenLeet Light | Stats in light mode | Light-mode proof |
| `settings-general.png` | 1920×1080 · 102 KB | OpenLeet | Settings → General | Settings feature card |
| `settings-appearance.png` | 1920×1080 · 163 KB | OpenLeet | Theme grid + interface | Themes section |
| `settings-dracula-appearance.png` | 1920×1080 · 165 KB | Dracula | Theme grid in Dracula | **Themes section hero** |
| `settings-ai.png` | 1920×1080 · 193 KB | OpenLeet | Provider cards, coach/model assignment | AI feature card |
| `settings-account.png` | 1920×1080 · 152 KB | OpenLeet | Signed-out account and sync | Privacy section |
| `settings-data.png` | 1920×1080 · 177 KB | OpenLeet | Export, import, backup, reset | Data tools card |

## Modal and palette captures

These focus on overlays. Their underlying chrome may predate the current header;
crop to the overlay if the difference matters.

| File | Size | What it shows | Suggested use |
| :--- | :--- | :--- | :--- |
| `quick-actions.png` | 1920×1080 · 142 KB | Quick-action palette over Home | Commands section |
| `theme-popup.png` | 1920×1080 · 482 KB | Theme picker preview over Solve | Themes / speed section |
| `ai-integration-picker.png` | 900×434 · 105 KB | "Connect an integration" picker | AI feature card (cropped) |

## Focus and interaction proof

| File | Size | What it shows |
| :--- | :--- | :--- |
| `home-focus.png` | 1670×900 · 67 KB | Keyboard focus on the account label (`F6`, `Tab`, `Tab`) with the `Account` tooltip on the row below |

This is a QA receipt for the header interaction, not a marketing hero. It is
captured by `scripts/capture-focus.sh`, which holds the focus state in tmux and
then attaches VHS for the screenshot.

## Compatibility aliases

These duplicate a canonical image and exist only for older links:

| Alias | Canonical |
| :--- | :--- |
| `themes.png` | `settings-appearance.png` |
| `ai-models.png` | `settings-ai.png` |
| `account.png` | `settings-account.png` |

## Narrow-width proofs

These show the Home header and mini stats at the responsive breakpoints. Keep
them as QA receipts, not as marketing hero images.

| File | Size | Columns | Theme |
| :--- | :--- | :--- | :--- |
| `narrow/home-100-openleet.png` | 1050×650 · 60 KB | ~100 | OpenLeet |
| `narrow/home-100-openleet-light.png` | 1050×650 · 62 KB | ~100 | OpenLeet Light |
| `narrow/home-120-openleet.png` | 1250×700 · 61 KB | ~120 | OpenLeet |
| `narrow/home-120-openleet-light.png` | 1250×700 · 64 KB | ~120 | OpenLeet Light |

## Alt text

Use the screen name plus the concrete content. Keep it under ~140 characters.

| File | Alt text |
| :--- | :--- |
| `home.png` | `OpenLeet home screen with a centered wordmark, a problem search box, and a line reading 1d streak · 3/17 solved.` |
| `home-light.png` | `OpenLeet home screen in light mode on warm paper, with the same search box and progress line.` |
| `roadmap.png` | `OpenLeet roadmap with a topic rail, a winding grid of problem cards, and a problem inspector.` |
| `roadmap-dracula.png` | `OpenLeet roadmap in the Dracula palette.` |
| `solve.png` | `OpenLeet solve workspace with statement, Python editor, session rail, and a tabbed console.` |
| `solve-light.png` | `OpenLeet solve workspace in light mode.` |
| `solve-tron.png` | `OpenLeet solve workspace in the Tron palette.` |
| `stats.png` | `OpenLeet stats dashboard with a solved counter, twelve-month heatmap, streaks, and difficulty split.` |
| `stats-light.png` | `OpenLeet stats dashboard in light mode.` |
| `settings-general.png` | `OpenLeet Settings on the General section with startup and quit options.` |
| `settings-appearance.png` | `OpenLeet Settings on Appearance with a grid of twelve theme previews.` |
| `settings-dracula-appearance.png` | `OpenLeet theme grid in the Dracula palette.` |
| `settings-ai.png` | `OpenLeet AI Models settings with provider cards and coach model assignment.` |
| `settings-account.png` | `OpenLeet account settings signed out, with Google sign-in, email, and cloud sync.` |
| `settings-data.png` | `OpenLeet data settings with export, import, backup, and reset actions.` |
| `quick-actions.png` | `OpenLeet quick-action palette open over the home screen.` |
| `home-focus.png` | `OpenLeet header with keyboard focus on the account label and an Account tooltip below it.` |
| `theme-popup.png` | `OpenLeet theme picker previewing a palette over the solve workspace.` |
| `ai-integration-picker.png` | `OpenLeet Connect an integration picker with search and provider rows.` |

## Placement guide

| Page area | Asset | Notes |
| :--- | :--- | :--- |
| Hero | `roadmap.png` | Full width, native 16:9, 1px border, caption `Roadmap — DSA · 3/17 solved` |
| Loop section | `solve.png` | Right column on desktop, below on mobile |
| Feature grid | `home.png`, `stats.png` | Optional; the grid works without images |
| Screens band | home, roadmap, solve, stats | Horizontal scroll on mobile |
| Themes | `settings-dracula-appearance.png` + swatch rows | Show the palette name in the caption |
| Privacy | `settings-account.png` | Pair with the four privacy bullets |
| Install | `home-light.png` | Optional closing image |

## Image rules

- Keep the native aspect ratio. Never crop mid-panel.
- Do not recolour, invert, or add gradients.
- On light pages, wrap dark screenshots in a 1px `border` with 4px inner padding.
- No fake browser chrome, traffic lights, tilts, or perspective.
- Export WebP or AVIF at 1600px wide for the page; keep the PNGs as source.
- Set explicit `width`/`height` attributes to avoid layout shift.
- Lazy-load everything below the hero.
- Compress the large captures (`theme-popup`, `solve`, `roadmap`) before shipping.

## Social and OG

- `og-home.png`: 1200×630 crop of `home.png` with the wordmark and the header
  visible. Do not add text overlays over the screenshot.
- `og-themes.png`: 1200×630 crop of `settings-dracula-appearance.png`.
- Favicon: build the block wordmark at 512×512 (accent square with the letter
  `L` or the full `LEET` block), then export 32/180/512 sizes.
