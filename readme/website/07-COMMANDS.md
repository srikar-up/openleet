# 07 — Commands

OpenLeet uses a slash-command palette (`Ctrl+P` anywhere, `/` on Home). Commands
are quick actions: they run or open a small picker over the current screen.
Only `/settings` and `/roadmap` change the page.

## Palette commands

| Command | Aliases | What it does | Navigates |
| :--- | :--- | :--- | :--- |
| `/solve` | `/pick` | Choose a problem; press `F2` to open it | No |
| `/roadmap` | `/map` | Open the problem roadmap | Yes |
| `/login` | `/google`, `/signin` | Sign in with Google in the browser | No |
| `/random` | `/rand` | Pick a random unsolved problem | No |
| `/filter` | — | Filter by difficulty | No |
| `/topic` | — | Filter by topic | No |
| `/themes` | `/theme` | Switch colour theme (live preview) | No |
| `/lang` | `/language` | Switch language | No |
| `/connect` | — | Connect an AI provider | No |
| `/model` | — | Switch the Coach model | No |
| `/stats` | — | Streak, solved count, last seven days | No |
| `/settings` | `/prefs`, `/config` | Open full settings | Yes |
| `/theme-audit` | — | Audit semantic colours across every theme | No |
| `/notifications` | — | Show recent notifications | No |
| `/notify-test` | — | Fire one notification of each kind (debug) | No |
| `/update` | — | Show update status | No |
| `/quit` | `/exit` | Exit OpenLeet | — |
| `/run` | — | Run tests (`ctrl+r`) | No |
| `/save` | — | Save the solution (`ctrl+s`) | No |
| `/coach` | — | Open the Socratic Coach tab | No |
| `/hint` | — | Show the Hint tab | No |
| `/reset` | — | Reset the code buffer (confirms first) | No |

## Availability by screen

The palette filters to the commands that make sense in the current context.

- **Home:** `/solve` `/roadmap` `/login` `/random` `/filter` `/topic` `/themes`
  `/lang` `/connect` `/stats` `/settings` `/quit`
- **Solve:** `/run` `/save` `/coach` `/hint` `/reset` `/roadmap` `/login`
  `/lang` `/themes` `/model` `/connect` `/settings` `/quit`
- **Diagnostics** (`/theme-audit`, `/notifications`, `/notify-test`, `/update`)
  are available from the palette where the corresponding subsystem is loaded.

## Roadmap inline commands

`/` on the Roadmap opens its inline command bar rather than the global palette.
These commands filter and select in place:

| Command | What it does |
| :--- | :--- |
| `/solve` | Open the selected problem |
| `/topic <name>` | Filter by topic |
| `/company <name>` | Filter by company |
| `/dsa` | Clear filters and return to the full set |
| `/clear` | Clear the command bar / active filter |

`Ctrl+P` from the Roadmap still opens the global palette.

## Behaviour rules

1. **Apply in place.** Theme, language, provider, and model pickers write the
   same settings as the full Settings page, then close back to the same focus,
   scroll position, and editor cursor.
2. **Preview then apply.** Pickers preview on cursor movement, apply on `Enter`,
   and revert on `Esc`. The theme picker never touches the code buffer.
3. **Typed commands execute.** Typing a full command and pressing Enter runs it,
   so `/settings ai` deep-links to AI Models.
4. **Two navigation commands.** `/settings` and `/roadmap` are the only commands
   that change the page. Everything else is an overlay.
5. **Destructive commands confirm.** `/reset` asks before discarding the buffer.
6. **Slash text in the editor is code.** On Solve, `/` types into the buffer;
   only `Ctrl+P` opens the palette.

## Copy for the website

- Eyebrow: `QUICK ACTIONS`
- Title: `Everything is a keystroke away.`
- Body: `Type / or press ctrl+p. Theme, language, provider, model, stats, and
  filters open over the current screen. The page changes only when you ask it to.`
- Keycaps: `ctrl+p` `commands` · `/` `quick actions`

## Suggested command table on the site

Show a 4 × 3 grid of the most useful commands, not all 22:

| Command | Label |
| :--- | :--- |
| `/themes` | Switch palette |
| `/lang` | Switch language |
| `/connect` | Connect a provider |
| `/model` | Choose a model |
| `/stats` | Streaks and heatmap |
| `/filter` | Filter by difficulty |
| `/topic` | Filter by topic |
| `/random` | Random unsolved |
| `/run` | Run tests |
| `/save` | Save buffer |
| `/hint` | Ask for a hint |
| `/settings` | Open settings |
