# 01 — Brand

## Positioning

**OpenLeet is the terminal workspace for serious algorithm practice.**

Most interview-prep tools are browser tabs pretending to be a workspace. OpenLeet
is the opposite: a fast, keyboard-driven terminal app that keeps the problem,
your code, the test runner, and the AI coach in one focused surface.

The product is intentionally narrow. It is not a course platform, not a
leaderboard-first game, and not a cloud IDE. It is a dojo: curated problems, a
real runner, and a coach that asks better questions.

## Elevator pitch

> OpenLeet turns your terminal into a DSA practice dojo. Open the roadmap, pick a
> problem, run your Python or Go against real tests, and talk to a Socratic coach
> that hints instead of spoiling. Your progress stays on your machine unless you
> choose to sync.

## Audience

| Segment | What they want | Why OpenLeet fits |
| :--- | :--- | :--- |
| Interview candidates | Focused reps without tab sprawl | One screen from roadmap to tests |
| Terminal power users | Tools that respect the keyboard | Every action has a shortcut |
| CS students | A practice loop that teaches | Socratic hints, not answer dumps |
| Backend / systems engineers | Their own toolchain | Runs Python and Go locally |
| Privacy-minded users | Local-first by default | No account required; local vault |

### Jobs to be done

1. "Give me one problem I can solve in 20 minutes."
2. "Run my solution against the real tests without leaving the editor."
3. "Help me get unstuck without showing me the solution."
4. "Show me whether I'm actually improving."
5. "Let me practise on a plane."

## Pain points to speak to

- Browser-based problem sites are heavy, noisy, and account-gated.
- AI chat tools default to full solutions and short-circuit learning.
- Practice tools hide the actual toolchain behind a sandbox.
- Progress dashboards are fun but rarely help you decide what to do next.
- Terminal workflows are fast, but most practice tools live outside them.

## Differentiators

1. **Actually local.** Progress, code buffers, layout, and settings live on disk.
   The app works offline with no account.
2. **A real runner, not a sandbox demo.** It detects Python and Go on the host,
   injects a hidden harness, enforces a timeout, and cleans up the process tree.
3. **Socratic by design.** The coach asks questions and points at the next idea.
   A "never send solution code" switch exists in Settings.
4. **One surface, three panes.** Statement, editor, and console stay in view.
5. **Keyboard-first.** `F2` solve, `F3` stats, `F4` appearance, `ctrl+p` commands.
6. **A terminal UI that respects the grid.** Dynamic geometry, exact cell fill,
   wide-character-safe truncation, and mouse hit-testing that matches pixels.
7. **Twelve themes.** Dark and light palettes with live preview and a real
   contrast audit.

## Brand personality

| Trait | Expression on the website |
| :--- | :--- |
| Focused | Generous whitespace, one message per band, no auto-carousel |
| Technical | Real commands, real shortcuts, real code, real screenshots |
| Quiet confidence | No superlatives, no urgency timers, no "10x" claims |
| Warm | Brand orange, warm paper light mode, human microcopy |
| Playful in the details | Pixel wordmark, winding roadmap, heatmap |

**Not:** corporate SaaS, crypto, gamer neon, or "AI will replace you".

## Voice and tone

- Write in second person ("your terminal", "you run").
- Prefer short declarative sentences. One idea per sentence.
- Use concrete nouns: terminal, problem, test, hint, streak.
- Allow jargon when it is the clearest word: DSA, DP, stdout, toolchain.
- Never use exclamation marks. Never use "revolutionary", "seamless",
  "game-changing", or "unleash".
- For errors and empty states, be plain and helpful: "No ranked players yet."

### Microcopy examples

| Context | Copy |
| :--- | :--- |
| Primary CTA | `Install OpenLeet` |
| Secondary CTA | `Read the docs` |
| Install hint | `One binary. No account. Works offline.` |
| Empty stats | `Solve a level to populate topic stats` |
| Signed-out leaderboard | `Sign in from settings to rank` |
| Coach placeholder | `Ready. Press ctrl+r to run the fixtures.` |

## Tagline options

Pick one for the hero and reuse the others for metadata or section intros.

1. **Practice algorithms where you already work.** (recommended)
2. A terminal-first DSA dojo.
3. Your terminal is the dojo.
4. Real tests. Real hints. No browser.
5. Solve more. Tab less.

### One-liners by page

- **Features:** "Everything the loop needs, nothing it doesn't."
- **Screens:** "Three panes. Zero context switches."
- **Themes:** "Twelve palettes. One grid."
- **Install:** "One binary. No account."

## Wordmark

The app's wordmark is two blocks: `OPEN` in body text and `LEET` inside a solid
accent rectangle with `accent_fg` letters.

```text
OPENLEET
^^^^      plain text colour
    ^^^^  accent block, accent_fg letters
```

Rules:

- Keep the two blocks on one line. Never stack `OPEN` over `LEET` in the nav.
- The accent block is a hard rectangle. No gradient, no rounded corners, no glow.
- On dark surfaces, `OPEN` is `#f2f2f2` and the block is `#ff7a1a` with near-black
  letters. On light surfaces, `OPEN` is `#211c17` and the block is `#c2410c` with
  warm-paper letters.
- Minimum clear space around the wordmark: one cap height on all sides.
- Do not add a tagline inside the wordmark lockup.
- The pixel/arcade rendering in the app's Home and Stats screens is a display
  treatment, not the logo. Use the block wordmark for navigation and favicons.

## Naming

- Product name: **OpenLeet** (never "Open Leet", "openleet", or "OPENLEET" in
  prose; all-caps is acceptable inside the wordmark only).
- The command is `openleet`.
- The repo uses the module name `openleet`.

## Boilerplate

**Short (25 words):**
> OpenLeet is a terminal-first DSA practice dojo with a Socratic AI coach, local
> Python and Go execution, and a twelve-theme interface.

**Medium (50 words):**
> OpenLeet turns the terminal into a focused algorithm-practice workspace. Pick a
> problem from a winding roadmap, write Python or Go in a three-pane editor, run
> real tests against your own toolchain, and get Socratic hints from an AI coach
> that asks instead of answering. Local-first and account-optional.

**Long (100 words):**
> OpenLeet is a terminal-first data-structures and algorithms practice dojo. It
> keeps the problem statement, a syntax-highlighted editor, and the test console
> on one surface, so the loop from reading to running never leaves the keyboard.
> Solutions execute against the Python or Go toolchain already installed on your
> machine. When you get stuck, a Socratic coach streams questions and hints
> instead of a full solution. Progress, notes, and layout stay local by default;
> accounts and per-user cloud sync are optional. Twelve palettes, light and dark,
> keep the grid readable in any terminal.
