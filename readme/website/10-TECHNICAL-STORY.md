# 10 — Technical Story

Use this for an "Under the hood" page, engineering blog post, or the README
"what this demonstrates" section. Every claim here is verifiable in the
repository.

## Stack

| Layer | Choice |
| :--- | :--- |
| Language | Go 1.27 |
| TUI runtime | Bubble Tea 0.25 (Elm architecture) |
| Styling | Lip Gloss 1.1 |
| Markdown rendering | Glamour 1.0 |
| Cell measurement | `charmbracelet/x/ansi` |
| Terminal detection | `mattn/go-isatty`, `muesli/termenv` |
| Auth / sync (optional) | Firebase Auth + Firestore |
| Capture / QA | VHS + ttyd + ffmpeg in WSL |

## High-level flow

```text
keyboard / mouse / provider tokens / resize / logs
                     │
                     ▼
              eventbus.Hub.Publish        (non-blocking, bounded)
                     │
                     ▼
              one Pump goroutine
                     │  Program.Send
                     ▼
             Bubble Tea Update loop       (single-threaded state)
                     │
                     ▼
              AppModel.View                (exact-grid frame)
```

- Producers never mutate a model. They publish events.
- A single pump is the only bridge into the Elm loop.
- Resize events bypass the frame batch; token/log events coalesce into one
  `BatchMsg` per 33 ms (30 FPS).
- On overflow, tokens and logs are dropped first; lifecycle events are never
  evicted.

## Package map

```text
cmd/openleet            entrypoint, flags, startup view
internal/ui             AppModel, chrome, quick actions, overlays
  home                  landing page, search, mini stats
  mapview               winding roadmap, geometry, hit-testing
  workspace             solve panes, editor, console, session rail
  stats                 dashboard, heatmap, leaderboard line
  settings              schema-driven settings, providers, account
  theme                 palettes, contrast guards, audit
  uikit                 cell-aware text, exact fill, background paint
  commands              slash-command registry and contexts
  quickdialog           centered pickers and dialogs
  toast                 notification stack
  eventbus              bounded hub, pump, coalescing
internal/runner         runtime detection, harness, execution, formatting
internal/coach          provider clients, streaming, prompt construction
internal/storage        local persistence, scoped managers
internal/identity       encrypted local identity, guest/claim mapping
internal/secrets        AES-256-GCM vault
internal/network        optional Firebase auth and sync
internal/model          problems, user state, results
```

## Rendering model

The terminal is a deterministic character grid, so the UI is built like one.

- The root model owns `width` and `height`; child sizes are derived in `View`,
  never cached as deltas.
- The frame is truncated and padded to the exact terminal size after every
  composition. Cutting in front of a double-width grapheme can leave a cell
  unused, so padding happens after truncation.
- Text is measured in cells, not bytes or runes: `uikit.Truncate`, `Fit`,
  `FitExact`, `lipgloss.Width`, `ansi.StringWidth`.
- Panels are joined with `lipgloss.JoinHorizontal` / `JoinVertical`, never
  manual `\n` concatenation.
- Every view paints its own background via `uikit.PaintBackground`; nested
  resets restore the last background so light palettes stay readable on dark
  terminals.
- Mouse hit-testing reuses the same geometry as the renderer, so targets cannot
  drift from the glyphs.

## Runner

1. Detect `python`/`python3` and `go` on the host, including versions.
2. Inject a hidden harness around the user's buffer.
3. Run with a configurable timeout (default 5 s).
4. Kill the whole process tree on timeout.
5. Parse stdout/stderr and fixtures; map failures back to editor lines.
6. Support per-problem custom input/expected output.
7. Clean temporary files.

Formatting is language-aware: Python and Go have dedicated formatters; other
languages report that formatting is unsupported.

## Coach

- Native Gemini and Anthropic streaming clients, plus an OpenAI-compatible
  adapter used by OpenAI, Groq, NVIDIA NIM, OpenRouter, OpenCode, and custom
  endpoints.
- Local Ollama path with no key required.
- `auto` provider selection: Ollama first, then the first saved cloud key in a
  documented priority order; an explicit provider never silently falls back.
- Streaming channels are owned by the coach manager: it queues the terminal
  error before closing, and consumers drain buffered tokens before publishing
  completion.
- The Socratic prompt asks questions and points at the next idea instead of
  producing a full solution.
- "Never send solution code" is a persisted privacy switch.

## Security

- Provider keys live in `secrets.enc`, encrypted with AES-256-GCM and mode
  `0600`. The key is derived from the current OS user and host; it is not
  compiled into the binary.
- Keys are masked in provider cards and picker rows.
- Local identity is encrypted (`identity.enc`); each account gets an opaque
  namespace under `users/<opaque-id>/`.
- Guest profiles can be claimed once; accounts are never merged or exposed.
- Firebase ID tokens are memory-only and never written to disk.
- Firestore rules enforce per-user access.
- The leaderboard stores only display name, score, solved count, and update
  time. No email, code, or per-problem history is uploaded.

## Themes

- A palette is the single source of truth for every view.
- Twelve palettes, two light.
- `Normalize` fills missing tokens and applies contrast guards: body text 4.5:1,
  borders 3:1, distinct pass/fail/warn/info colours.
- The `/theme-audit` table renders every palette at multiple widths and checks
  for clipped components, newlines, and unreadable chips/labels.

## Quality

- Unit tests across UI, runner, coach, storage, secrets, eventbus, and network.
- Exact-grid tests, mouse hit-testing tests, and theme-audit tests.
- Headless VHS captures serve as visual receipts for layout changes.
- The pipeline receipt includes `go test -race` where the host supports it,
  `go vet`, and a focused run over `eventbus`, `uikit`, `workspace`, and `ui`.

## Claims you can make

- "Local-first" — true: progress and code are stored locally; accounts optional.
- "Runs your toolchain" — true: Python and Go on the host.
- "Streaming AI hints" — true for the supported providers.
- "Encrypted key vault" — true: AES-256-GCM, user-derived key, mode 0600.
- "Twelve themes with a contrast audit" — true.
- "Exact-grid terminal rendering with mouse support" — true.

## Claims to avoid

- "Sandboxed execution" — the runner uses the host toolchain, not a sandbox.
- "Supports every language" — only Python and Go execute.
- "Automatic background sync" — sync is manual.
- "OAuth device-code login" — not implemented.
- "Web dashboard" — there is no web app.
