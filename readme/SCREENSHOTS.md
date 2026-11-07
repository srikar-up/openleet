# Screenshots

OpenLeet's interface is captured headlessly with VHS so layout changes can be reviewed without a live terminal session.

Most captures use `COLORTERM=truecolor` and a 1920×1080 tape at 16px font size. Narrower captures use 1200×720 or 100×30 logical cells.

## Home

<img src="screenshots/home.png" alt="OpenLeet home screen" width="100%">

A quiet landing screen with inline problem search, the current language, the
total problem count, and one line of progress under the search box. The header
is a single row: the breadcrumb on the left and plain `Settings` and account
labels on the right, with no icons or brackets.

<img src="screenshots/home-light.png" alt="OpenLeet home screen in light mode" width="100%">

OpenLeet Light uses warm paper, dark ink, and the brand-orange accent. The mini
stats line stays readable in both modes.

<img src="screenshots/home-gruvbox.png" alt="OpenLeet home screen in Gruvbox" width="100%">
<img src="screenshots/home-dracula.png" alt="OpenLeet home screen in Dracula" width="100%">
<img src="screenshots/home-tron.png" alt="OpenLeet home screen in Tron" width="100%">

The same header and stats line in Gruvbox, Dracula, and Tron.

## Header focus state

<img src="screenshots/home-focus.png" alt="OpenLeet header with keyboard focus on the account label and an Account tooltip" width="100%">

`F6` focuses the chrome, `Tab` cycles the breadcrumb, Settings, and account
labels, and `Enter` activates the focused item. Hover and focus show tooltips
(`Settings (F4)`, `Account`) as a one-row overlay under the label; the header
itself never grows a second row. Captured with `scripts/capture-focus.sh`.

## Quick actions

<img src="screenshots/quick-actions.png" alt="OpenLeet quick-action palette" width="100%">

The `/` and `Ctrl+P` palette is context-aware. It opens centered over the current screen with a dimmed backdrop; items apply in place instead of navigating. `/settings` and `/roadmap` are the only commands that change the page.

<img src="screenshots/theme-popup.png" alt="OpenLeet theme quick-action popup over Solve" width="100%">

The theme picker live-previews on cursor movement, applies on Enter, and reverts on Esc. The code buffer, cursor, and test results underneath are untouched. Theme, language, provider, and model choices write the same settings used by the full Settings page.

## Roadmap

<img src="screenshots/roadmap.png" alt="OpenLeet roadmap" width="100%">

A winding problem grid with difficulty badges, solved markers, section filters, and mouse hit-testing.

## Solve workspace

<img src="screenshots/solve.png" alt="OpenLeet solve workspace" width="100%">

The three-pane workspace: rendered statement, code editor, and a console with Test Results, Socratic Coach, Hint, and Custom tabs.

## Stats

<img src="screenshots/stats.png" alt="OpenLeet stats dashboard" width="100%">

Solved counter, activity heatmap, best streak, active days, difficulty breakdown, and the compact signed-in leaderboard line.

## Themes

<img src="screenshots/themes.png" alt="OpenLeet theme gallery" width="100%">

The theme gallery lives in Settings → Appearance. Moving the cursor previews a
palette live; Enter applies, Esc reverts. Light mode ships as `OpenLeet Light`
and `Catppuccin Latte`.

## Light mode

<img src="screenshots/home-light.png" alt="OpenLeet light landing page" width="100%">

OpenLeet Light uses warm paper, dark ink, and the brand-orange accent. The landing page paints its own canvas, so the light palette stays readable even when the host terminal is dark.

<img src="screenshots/solve-light.png" alt="OpenLeet light solve workspace" width="100%">

The solve workspace derives the problem statement, inline code, code blocks, editor syntax colors, and chrome from the active palette. Glamour switches to its light style config for light themes.

## AI Models

<img src="screenshots/ai-models.png" alt="OpenLeet AI Models settings" width="100%">

Provider cards for Anthropic, OpenAI, Groq, Google Gemini, NVIDIA NIM, OpenCode Console, OpenRouter, Ollama, and custom OpenAI-compatible endpoints.

## Connect an integration

<img src="screenshots/ai-integration-picker.png" alt="OpenLeet Connect an integration picker" width="100%">

Searchable provider picker with Popular and Services groups, connection status, keyboard navigation, and an accent-highlighted selection.

## Account and optional sync

<img src="screenshots/account.png" alt="OpenLeet account and sync settings" width="100%">

Signed-out account view with Continue with Google, email sign-in, account creation, cloud-sync toggle, and conflict policy.

## Capture commands

```bash
# Home (dark and light)
bash scripts/capture-tui.sh home openleet
bash scripts/capture-tui.sh home openleet-light

# Roadmap
bash scripts/capture-tui.sh roadmap

# Stats
bash scripts/capture-tui.sh stats

# Themes (Settings → Appearance)
bash scripts/capture-tui.sh settings openleet appearance
bash scripts/capture-tui.sh settings dracula appearance

# Light solve workspace
bash scripts/capture-tui.sh solve openleet-light

# Settings → AI Models
bash scripts/capture-tui.sh settings openleet ai

# Narrow viewports: ~100 and ~120 columns at 16px font
VHS_WIDTH=1050 VHS_HEIGHT=650 bash scripts/capture-tui.sh home openleet
VHS_WIDTH=1250 VHS_HEIGHT=700 bash scripts/capture-tui.sh home openleet
```

The capture helper stages the Linux binary on the WSL filesystem before running VHS. This avoids a known 9p mount issue when executing large Go binaries directly from a Windows drive mount.
