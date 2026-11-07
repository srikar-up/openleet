# OpenLeet Features

OpenLeet is a terminal-first data-structures and algorithms practice environment. This document explains what each part of the app does, how the pieces fit together, and which capabilities are intentionally still unfinished.

## At a glance

| Capability | Status | Notes |
| :--- | :--- | :--- |
| Offline local practice | **Implemented** | No account or network required |
| Roadmap and filters | **Implemented** | Keyboard, mouse, topic/company filters |
| Solve workspace | **Implemented** | Statement, editor, console, coach |
| Python / Go runner | **Implemented** | Uses the host toolchain |
| Custom test cases | **Implemented** | Per-problem input and expected output |
| Socratic AI Coach | **Implemented** | Streaming, provider picker, local/cloud |
| Google Gemini / NVIDIA NIM / Groq | **Implemented** | Native Gemini, OpenAI-compatible NVIDIA and Groq |
| Theme gallery | **Implemented** | Live preview, apply, multiple palettes |
| Schema-driven settings | **Implemented** | Search, nine categories, exact-grid layout |
| Encrypted provider keys | **Implemented** | AES-256-GCM `secrets.enc` |
| Export / import / backup | **Implemented** | JSON, CSV, timestamped snapshot |
| Firebase account + sync | **Optional** | Requires project config and rules |
| OAuth / device-code login | **Not implemented** | Email/password only |
| Code-buffer cloud sync | **Not implemented** | Code stays device-local |
| Automatic background sync | **Not implemented** | Sync is manual |

## 1. Home

The landing screen is deliberately quiet: a pixel wordmark, a search card, and one line of progress.

- Shared one-row header: breadcrumb on the left; plain `Settings` and account labels on the right
- Inline problem search with a `>` prompt
- Context line: coach status, language, total problem count, and runner mode
- Mini stats under the card: streak and solved count, centered; hidden when both are zero
- Context-aware quick-action palette (`/` or `Ctrl+P`)
- Keyboard and mouse navigation
- Footer hints for solve, stats, and the command palette

Slash commands are quick actions, not navigation. A command either runs immediately or opens a small centered picker over the current screen; applying it closes back to the same focus, scroll, and editor cursor. `/settings` and `/roadmap` are the only commands that change the page.

- `/themes`, `/lang`, `/connect`, and `/model` share the same underlying settings as Settings > Appearance and Settings > AI Models.
- `/stats` is a read-only streak, difficulty, and seven-day popup.
- `/filter` and `/topic` apply a roadmap filter in place.
- `/run`, `/save`, `/random`, and `/reset` act immediately; `/reset` confirms first.
- `/solve` selects a problem without navigating; press `F2` to open it.
- Typing a full command in the palette executes it, so `/settings ai` deep-links to AI Models.

Home exposes `/solve /roadmap /random /filter /topic /themes /lang /connect /stats /settings /quit`. Solve exposes `/run /save /coach /hint /reset /roadmap /lang /themes /model /connect /settings /quit`.

## 2. Roadmap

The roadmap is a vertical window over a winding grid of problem cards.

- Alternating left-to-right / right-to-left rows
- Difficulty-coded badges
- Solved and locked markers
- Six-card page windows with scroll-to-selection
- Topic and company filters
- Clickable filter rail
- Mouse hit-testing that shares the renderer geometry
- Inline `/` command bar for `/solve`, `/company`, `/topic`, `/dsa`, and `/clear`
- `/filter` and `/topic` quick-action pickers apply filters without leaving the roadmap

The grid offset is derived from the selected card on every render, so mouse targets cannot drift away from what is painted.

## 3. Solve workspace

The solve screen is a three-pane workspace:

```text
┌───────────────────────┬──────────────────────────────────────────────┐
│ Problem statement     │ Code editor                                  │
│ - title               │ - syntax-highlighted buffer                  │
│ - difficulty          │ - line numbers                               │
│ - tags                │ - active language                            │
│ - examples            │ - scroll position                            │
│ - constraints         │                                              │
│                       ├──────────────────────────────────────────────┤
│                       │ Console                                      │
│                       │ Test Results · Socratic Coach · Hint · Custom│
└───────────────────────┴──────────────────────────────────────────────┘
```

### Problem statement

- Rendered Markdown via Glamour
- Scrollable viewport
- Difficulty, topics, companies, and constraints
- Responsive width calculations

### Editor

- Language picker
- Line numbers
- Syntax highlighting
- Scroll percentage
- Save state
- Format/beautify action
- Per-problem, per-language code buffers

### Console tabs

| Tab | Purpose |
| :--- | :--- |
| **Test Results** | Pass/fail rows, expected vs. actual output, stderr, timing |
| **Socratic Coach** | Streaming AI conversation with slash commands |
| **Hint** | Local, result-aware next-step guidance without contacting a provider |
| **Custom** | Run one user-supplied input against an expected output |

Coach slash commands:

```text
/hint
/complexity
/explain
/diagnose
```

## 4. Host runner

OpenLeet does not bundle language runtimes. It uses what is already installed.

- Detects host `python` / `python3` and `go`
- Injects a hidden test harness around the user buffer
- Creates an isolated temporary working directory
- Applies a configurable timeout (default 5 seconds)
- Terminates the full process tree
- Maps Python traceback frames back to editor line numbers
- Hides temporary sandbox paths from user-facing errors
- Supports formatting:
  - Go: in-process `go/format`
  - Python: `ruff` or `black` when available, with a safe whitespace fallback

> **Current runtime scope:** host execution currently supports **Python and Go**. The editor exposes additional language templates, but the runner will report a runtime-unavailable error for languages that do not yet have a harness.

## 5. Socratic AI Coach

The coach is designed to behave like an interviewer, not a solution generator.

- Hardened system prompt that forbids copy-paste solutions
- Streaming token output
- Local-first Ollama support
- Cloud provider support
- Selected provider is shared with the workspace through a mutex-safe manager
- Provider changes take effect without restarting the app
- API keys are read from the encrypted vault, not config
- Connection picker with search and grouped providers
- Home → Coach opens the picker automatically when no provider is configured

### Provider picker

Open **Settings → AI Models** and:

- press `c`, or
- click **Connect an integration**

The modal has:

- Live search
- `POPULAR` and `SERVICES` groups
- Connected / local / not-connected status
- Keyboard navigation
- Mouse row selection
- Accent-highlighted selection
- `esc` to close

Selecting a provider:

1. Sets it as the Socratic Coach provider.
2. Opens its API-key / base-URL / model detail.
3. Rebuilds the active coach provider after the key is saved.

See [AI Providers](AI_PROVIDERS.md) for the provider table and security details.

## 6. Themes

The theme system is fully token-driven.

- Global theme gallery (`F4`)
- Live preview before applying
- First-class light mode: `OpenLeet Light` (warm paper, brand orange) and `Catppuccin Latte`
- `OpenLeet` remains the default dark palette; every other dark palette is still available
- Per-theme palette for background, surface, cards, text, accents, borders, difficulty colors, and status colors
- Theme names are exposed through `--list-themes`
- Views never embed literal hex colors
- The solve workspace derives Glamour Markdown and syntax colors from the active palette, including light-safe Markdown rendering
- Solve chrome and the landing page paint their own backgrounds, so light palettes stay legible even on a dark terminal
- `Appearance` settings can change theme, accent, density, border style, icons, and animation preference
- Narrow terminals collapse the Settings sidebar into short-label tabs

The goal is a terminal UI that remains legible across dark, light, and high-contrast palettes.

## 7. Settings

Settings is schema-driven, not a collection of hand-written forms.

### Categories

| Category | Controls |
| :--- | :--- |
| **General** | Startup screen, quit confirmation, locale, telemetry, update checks |
| **Appearance** | Theme, accent, density, borders, icons, animations, custom-theme actions |
| **Editor** | Font-size hint, tab width, keymap, auto-indent, bracket pairing, line numbers, format on save, default language |
| **Runner** | Runner mode, timeout, memory limit, interpreter paths, runtime detection |
| **AI Models** | Provider picker, provider cards, key/base/model editing, coach assignment, generation limits, privacy |
| **Keybindings** | Action table, capture/reset controls |
| **Account** | Sign-in, profile, sync preferences, manual sync, logout |
| **Data** | Export, import, backup, cache clear, roadmap reset, settings reset, deletion |
| **About** | Version, paths, diagnostics, links |

### Interaction model

- `j` / `k` or arrows move fields
- `left` / `right` adjust values
- `h` / `l` or `Tab` switch pane
- `/` searches settings
- `Ctrl+S` saves
- `Esc` closes overlays or returns to the prior screen
- Mouse can select sidebar rows, fields, theme cards, provider cards, and buttons

### Layout guarantees

- Dynamic width and height from the root model
- Exact-grid frame padding after truncation
- Cell-aware truncation for CJK and emoji
- Minimum target of 100×30
- Under 100 columns, the sidebar becomes a short-label tab row

## 8. Data and storage

OpenLeet uses a local-first storage model.

| File | Contents |
| :--- | :--- |
| `config.json` | Theme, language, runner, schema settings, sync preferences |
| `user_state.json` | Solved IDs, scores, attempts, code buffers, streaks, sessions |
| `workspace_layout.json` | Pane layout and workspace state |
| `secrets.enc` | Provider API keys, AES-256-GCM, mode `0600` |
| `profile.enc` | Encrypted solved-set summary for optional sync |
| `sync-meta.json` | Last successful sync UID/timestamp/note, mode `0600` |

### Data actions

- **Export progress** → `openleet-progress.json` and `.csv`
- **Import progress** → reads `openleet-progress.json`
- **Backup now** → timestamped snapshot of progress, layout, and config
- **Clear cache** → drops saved workspace layout
- **Reset roadmap** → clears solved problems/streaks, keeps code buffers
- **Reset settings** → restores schema defaults and the OpenLeet theme
- **Delete local data** → removes progress, code buffers, layout, and sync metadata; API keys are kept

### Security boundaries

- Provider keys never enter `config.json`.
- Firebase ID tokens are memory-only.
- Sync metadata contains no tokens, passwords, email addresses, or code.
- Code buffers remain device-local.

## 9. Optional account and sync

Cloud sync is optional. Local practice works without it.

OpenLeet reads `.env`, not `.env.example`. Copy the template once with `cp .env.example .env` (PowerShell: `Copy-Item .env.example .env`), then fill in the values.

- Firebase Google browser sign-in (OAuth 2.0 + PKCE + loopback redirect)
- Firebase Identity Toolkit email/password sign-in
- Optional email sign-up with display name
- Encrypted remembered session; the Firebase refresh token is stored in `session.enc`, ID tokens stay in memory
- Account card keys: `g` Google, `e` email sign-in, `u` create account
- `/login` (aliases `/google`, `/signin`) starts Google sign-in from any screen
- Profile card with UID, email, plan, and solved/points summary
- Sync card with enable/disable, conflict policy, last-sync time, and manual `s` sync
- Conflict policies:
  - **Newest wins** — pull, merge remote-only solved IDs, preserve local code/attempt details, then push
  - **Local wins** — skip the remote pull and push this device's solved set
- Expired token → clears the saved session and returns the Account page to signed out; active ID tokens refresh automatically on startup and during a sync retry
- Firestore access uses the user's ID token directly; no service account or Admin SDK

### Leaderboard and data minimization

Signed-in users can appear on a global score ranking. Each account owns exactly one document at `leaderboard/{uid}` and the entire record is four fields:

| Field | Type | Purpose |
| :--- | :--- | :--- |
| `name` | string, ≤ 32 runes | Display name shown on the ranking row |
| `score` | integer | Total points |
| `solved` | integer | Number of solved problems |
| `updated` | integer | Unix timestamp of the last publish |

- Nothing is collected while signed out.
- `name` is the display name chosen at sign-up (or Firebase's stored display name); it is never derived from the email address. Accounts without one publish as `Anonymous`.
- No email address, Firebase UID body field, solution code, or per-problem solved map is uploaded.
- The document ID is the UID, so the ranking row itself does not repeat it.
- Writes are one small PATCH per successful solve, sync, import, or reset; reads are a single top-N list when Stats opens.
- Firestore rules allow any signed-in user to read rankings but only the matching UID to write or delete its own row.
- The compact leaderboard line on Stats shows the top three plus the signed-in user's position; `r` refreshes it.
- Scores are computed by the client. The rules enforce shape and ownership, not gameplay; add a server-side verifier if competitive anti-cheat becomes a requirement.

Full offline data, including code, remains local. If cross-device backups are added later, the plan is to write them directly to the user's own Google Drive rather than collecting them in the app's Firestore project.

### Google browser sign-in setup

1. Enable **Google** under Firebase Authentication → Sign-in method.
2. In Google Cloud Console → **Google Auth Platform → Clients** (older consoles: **APIs & Services → Credentials**), create an OAuth client of type **Desktop app** in the same project.
3. Set `GOOGLE_OAUTH_CLIENT_ID` and `GOOGLE_OAUTH_CLIENT_SECRET` in `.env`. Desktop client secrets are not confidential; PKCE is always used.
4. The loopback redirect binds `127.0.0.1` on a random port; no redirect URI needs to be registered for a **Desktop app** client.

Do not use a **Web application** client. Google rejects its loopback redirect with `redirect_uri_mismatch`. The Firebase auth domain (`openleet-cd127.firebaseapp.com`) is not an OAuth client ID.

> Live two-device validation requires a deployed Firebase project, `FIREBASE_API_KEY`, `FIREBASE_PROJECT_ID`, and the included `firestore.rules`.

## 10. Responsive terminal behavior

- Root `Model` owns width and height.
- Every child is sized during render from the current canvas.
- No hardcoded terminal dimensions.
- No raw ANSI writes in views.
- No blocking I/O inside `Update`.
- The Settings frame is truncated and repadded to the exact terminal grid.
- On Windows, resize polling publishes `tea.WindowSizeMsg` through the event bus because Bubble Tea v1 cannot rely on `SIGWINCH`.

## 11. Async architecture

Background producers do not mutate models.

```text
provider tokens ─┐
geometry probes ─┼─► eventbus.Hub.Publish ─► one Pump goroutine ─► Program.Send
logs ────────────┤
agent activity ──┘
```

- `Publish` is non-blocking and bounded.
- Token and log events coalesce into one batch per 33 ms frame window.
- Resize events bypass the batch path and are delivered immediately.
- Lifecycle events are protected from token pressure.
- Dropped events are counted.

## 12. Quality and visual testing

- Unit tests across UI, runner, coach, storage, secrets, eventbus, and network packages
- Exact-grid and mouse hit-testing tests
- AI provider streaming tests with fake SSE servers
- Headless VHS captures for layout review
- `go vet ./...`
- `go test ./...`
- Race fallback documented when a host lacks a C compiler

## Current limitations

- Host execution supports Python and Go only.
- Keybinding edits are stored, but some global shortcuts remain fixed.
- OAuth/device-code login is not implemented.
- Account management (change password, linked providers, log out everywhere) is not implemented.
- Code buffers and attempt history do not round-trip through cloud sync.
- Sync is manual; there is no background interval.
- Live Firestore rules deployment and a two-user smoke test remain external validation tasks.
- Gemini, Anthropic, and NVIDIA calls require your own API keys; no bundled credentials are included.
