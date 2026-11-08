# 08 — Website Content

Ready-to-paste copy. Keep the tone short, declarative, and technical. Replace
links marked `#` with real destinations.

## Meta

| Page | Title | Description |
| :--- | :--- | :--- |
| Home | `OpenLeet — Terminal-first DSA practice with a Socratic AI coach` | `Practise algorithms in your terminal. Real Python and Go tests, Socratic AI hints, 12 themes, local-first progress. No browser, no account required.` |
| Features | `Features — OpenLeet` | `Roadmap, host runner, Socratic coach, stats, themes, and schema-driven settings. Everything the practice loop needs.` |
| Screens | `Screens — OpenLeet` | `See every OpenLeet screen: home, roadmap, solve workspace, stats, settings, and quick actions.` |
| Themes | `Themes — OpenLeet` | `Twelve terminal palettes, light and dark, with live preview and a contrast audit.` |
| Install | `Install — OpenLeet` | `Build or download OpenLeet for Windows, Linux, or macOS. One binary, no account.` |

Open Graph:

```text
og:title       OpenLeet — a terminal-first DSA dojo
og:description Real tests, Socratic hints, 12 themes. Your terminal, your toolchain.
og:image       /assets/og-home.png   (use home.webp, 1200×630 crop)
og:type        website
twitter:card   summary_large_image
```

## Hero

**Eyebrow:** `TERMINAL-FIRST DSA PRACTICE`

**H1 (recommended):**
> Practice algorithms where you already work.

**Sub:**
> OpenLeet puts the roadmap, your code, the real tests, and a Socratic coach on
> one terminal surface. Python and Go run on your machine.

**Primary CTA:** `Install OpenLeet`
**Secondary CTA:** `See the screens`
**Trust line:** `No account required · Works offline · Windows, Linux, macOS`

### Hero variants

1. **H1:** `Your terminal is the dojo.`
   **Sub:** `Curated DSA problems, a real Python and Go runner, and a coach that
   asks the next question.`
2. **H1:** `Real tests. Real hints. No browser.`
   **Sub:** `OpenLeet turns the terminal into a focused algorithm-practice
   workspace.`
3. **H1:** `Solve more. Tab less.`
   **Sub:** `One screen from problem statement to passing tests, with a Socratic
   coach when you get stuck.`

## Proof strip

`17 curated problems` · `12 themes` · `Python + Go` · `Local-first`

## Problem section

**Eyebrow:** `WHY OPENLEET`

**H2:** `Browser tabs are not a dojo.`

**Body:**
> Most interview-prep tools are browser tabs pretending to be a workspace.
> OpenLeet is the opposite: a fast, keyboard-driven terminal app that keeps the
> problem, your code, the runner, and the coach in one focused surface.
>
> No dashboards to maintain. No account gate. No sandbox hiding your toolchain.

**Three bullets:**

- `Local by default.` Progress, code buffers, and layout stay on your machine.
- `A real runner.` Execute against the Python or Go installed on your host.
- `A coach, not an answer machine.` Socratic hints that ask the next question.

## The loop

**Eyebrow:** `THE LOOP`

**H2:** `Read. Write. Run. Ask.`

| Step | Title | Copy |
| :--- | :--- | :--- |
| `01` | Pick | A winding roadmap with topic and company filters. |
| `02` | Solve | Statement, editor, and console in one view. |
| `03` | Run | Real tests against your installed Python or Go. |
| `04` | Ask | Socratic hints that ask the next question. |

## Feature grid

**Eyebrow:** `FEATURES`
**H2:** `Everything the loop needs, nothing it doesn't.`

1. **Roadmap**
   A winding grid with solved markers, six-card windows, and topic or company
   filters. Mouse targets share the renderer geometry, so clicks never drift.

2. **Host runner**
   Detects Python and Go on your machine, injects a hidden harness, enforces a
   timeout, cleans up the process tree, and maps failures back to editor lines.

3. **Socratic coach**
   Streams questions and hints from local Ollama or a cloud provider. Works with
   Anthropic, OpenAI, Groq, Gemini, NVIDIA NIM, OpenCode, OpenRouter, and custom
   OpenAI-compatible endpoints.

4. **Stats**
   Current streak, best streak, a twelve-month heatmap, active days, difficulty
   split, topic counts, and an optional leaderboard line.

5. **Themes**
   Twelve palettes with live preview, two light modes, and a contrast audit that
   checks every theme at multiple widths.

6. **Settings**
   Schema-driven, searchable, nine sections, mouse support, and a save indicator.
   Terminal minimum is 100×30.

## Screens band

**Eyebrow:** `INSIDE THE APP`
**H2:** `Three panes. Zero context switches.`

| Card | Caption |
| :--- | :--- |
| Home | Search, language, problem count, and one line of progress. |
| Roadmap | A winding grid with filters and solved state. |
| Solve | Statement, editor, and console, together. |
| Stats | Streaks, heatmap, difficulty split. |
| Settings | Nine sections, one schema. |
| Quick actions | Theme, language, model, and filters without leaving the screen. |

## Themes band

**Eyebrow:** `THEMES`
**H2:** `Twelve palettes. One grid.`

**Body:**
> Switch with `F4` and preview before you apply. Two light modes and ten dark
> modes, each guarded for readable text and borders.

**Palette names:**
`OpenLeet` · `OpenLeet Light` · `OpenCode` · `Catppuccin Mocha` ·
`Catppuccin Latte` · `Dracula` · `Flexoki` · `Gruvbox` · `Monokai` ·
`One Dark` · `Tokyo Night` · `Tron`

**Note:** `Every palette passes the same semantic contrast audit. No theme ships
with an unreadable label.`

## Keyboard strip

**Eyebrow:** `KEYBOARD-FIRST`

`F2 solve` · `F3 stats` · `F4 themes` · `ctrl+p commands` · `ctrl+r run` ·
`ctrl+s save`

**Caption:** `Every action has a key. Nothing is buried in a menu.`

## Privacy section

**Eyebrow:** `LOCAL-FIRST`

**H2:** `Your progress stays on your machine.`

- Progress, code buffers, layout, and settings are stored locally.
- Provider keys live in an encrypted local vault (`AES-256-GCM`, mode `0600`).
- Accounts and per-user sync are optional.
- Firebase ID tokens stay in memory and are never written to disk.
- The leaderboard stores only display name, score, solved count, and update time.

**CTA:** `Read the security notes` → `#`

## Install

**Eyebrow:** `INSTALL`
**H2:** `One binary. No account.`

**Requirements:**

- Go toolchain matching `go.mod` (Go 1.27)
- Optional: Python 3 and/or Go installed for code execution
- Optional: Ollama or a cloud provider key for the coach

### Windows (PowerShell)

```powershell
git clone https://github.com/srikar-up/openleet.git
cd openleet
go build -o .\bin\openleet.exe .\cmd\openleet
.\bin\openleet.exe
```

### Linux / macOS

```bash
git clone https://github.com/srikar-up/openleet.git
cd openleet
go build -o ./bin/openleet ./cmd/openleet
./bin/openleet
```

**First run:**

```text
F2  solve     F3  stats     F4  themes     ctrl+p  commands
```

**Note:** `The app needs at least 100×30 terminal cells.`

## FAQ

**Is it free?**
> The app is local-first and runs without an account. Cloud AI providers are
> optional and use your own keys.

**Does it require an account?**
> No. Accounts and per-user sync are optional. Everything works offline.

**Which languages can I run?**
> Python and Go, using the toolchains installed on your machine.

**Does it send my code to an AI?**
> Only when you use the coach, and only to the provider you connect. Settings
> includes a "never send solution code" switch.

**Does it work on Windows?**
> Yes. Windows, Linux, and macOS are supported terminals. Resize handling uses a
> console watcher on Windows.

**How many problems are there?**
> 17 curated problems across arrays, strings, linked lists, DP, graphs, binary
> search, and competitive programming.

**Can I add my own problems?**
> The problem set is file-based JSON with Markdown descriptions. See the
> repository's `assets/problems` folder.

**Where is my data stored?**
> Under the app's data directory, per OS user. Nothing leaves the machine unless
> you enable sync.

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
No account required. Your progress stays on your machine.
```

## Alt text examples

- `OpenLeet home screen with a centered wordmark, a problem search box, and a line reading 1d streak · 3/17 solved.`
- `OpenLeet roadmap: a topic rail on the left, a winding grid of problem cards in the center, and a problem inspector on the right.`
- `OpenLeet solve workspace with the problem statement, a Python editor, a session rail, and a console showing Test Results, Socratic Coach, Hint, and Custom tabs.`
- `OpenLeet stats dashboard with a 3/17 solved counter, a twelve-month activity heatmap, streak and active-day counts, and a difficulty split.`
- `OpenLeet Settings on the Appearance section, showing a grid of twelve theme previews and save state.`

## Do not say

- "AI-powered" as the first claim.
- "Revolutionary", "seamless", "game-changing", "10x".
- "LeetCode killer".
- "Cloud IDE", "web dashboard", "browser-based runner".
- "Automatic sync" (sync is manual).
- "Supports every language" (Python and Go execute).
