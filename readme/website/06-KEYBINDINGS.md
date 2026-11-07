# 06 — Keybindings

OpenLeet is keyboard-first. This is the reference for the website's keyboard
strip, feature copy, and any "cheat sheet" page.

> **Terminal caveat:** `Alt+...` combinations depend on the terminal host. Some
> terminals intercept `Ctrl+PageUp/Down` for tab switching; the app also accepts
> `Alt+Up/Down` for problem navigation.

## Global

| Keys | Action | Notes |
| :--- | :--- | :--- |
| `Ctrl+P` | Command palette | Works on every screen |
| `Ctrl+,` or `F5` | Open Settings | On Settings, moves focus to the Settings label |
| `F4` | Settings → Appearance | Theme grid |
| `F3` | Stats dashboard | From any other screen |
| `F2` | Open the selected problem in Solve | From Home, Roadmap, Stats |
| `F1` | Home → Roadmap; Stats → Home | Screen-dependent back/forward |
| `Shift+Tab` | Roadmap → Solve | Also switches view in list contexts |
| `Ctrl+N` | Notifications | Recent app notifications |
| `?` | Help | Home, Roadmap, Stats |
| `F6` | Focus the top bar | Then `Tab`/`Shift+Tab` cycle breadcrumb → Settings → account; `Enter` activates; `Esc` clears |
| `Q` or `Ctrl+C` | Quit | Confirmation is configurable |
| `Esc` | Back / close overlay | Closes the newest toast first when no dialog is open |

## Home

| Keys | Action |
| :--- | :--- |
| Type | Search problems |
| `↑` / `↓` | Move through results |
| `Enter` | Open the selected problem |
| `Esc` | Close the result list |
| `/` | Quick-action palette |
| `F1` | Roadmap |
| `F2` | Solve the selected problem |
| `F3` | Stats |
| `F4` | Appearance |
| `?` | Help |

## Roadmap

| Keys | Action |
| :--- | :--- |
| `J` / `↓` | Move down |
| `K` / `↑` | Move up |
| `H` / `←` | Move left |
| `L` / `→` | Move right |
| `Enter` | Open the selected problem |
| `/` | Inline roadmap commands (`/solve`, `/topic`, `/company`, `/dsa`, `/clear`) |
| `Esc` | Back to Home |
| `F2` or `Shift+Tab` | Open the selected problem in Solve |
| `Click` | Select a card or a topic filter |
| `Double-click` / `Enter` | Solve the card |

## Solve workspace

### Run and file

| Keys | Action |
| :--- | :--- |
| `Ctrl+R` | Run tests |
| `Ctrl+S` | Save the buffer |
| `Ctrl+B` | Beautify / format the buffer |
| `Ctrl+T` | Cycle console tabs: Test Results → Coach → Hint → Custom |

### Focus and panes

| Keys | Action |
| :--- | :--- |
| `Tab` | Cycle focus (statement → editor → console) |
| `Shift+Tab` | Cycle focus backwards |
| `Ctrl+W` | Cycle focus forward |
| `Alt+1` | Focus the statement pane |
| `Alt+2` | Focus the editor pane |
| `Alt+3` | Focus the console |
| `Alt+D` or `Alt+[` | Toggle the statement pane |
| `Alt+S` or `Alt+]` | Toggle the session pane |
| Drag the rail | Resize a pane |
| Double-click the rail | Reset pane widths |

### Problems

| Keys | Action |
| :--- | :--- |
| `Ctrl+PgDn` or `Alt+Down` | Next problem |
| `Ctrl+PgUp` or `Alt+Up` | Previous problem |
| `Esc` | Back to Roadmap |
| `Ctrl+L` | Language picker |

### Editor

| Keys | Action |
| :--- | :--- |
| `Ctrl+Z` / `Ctrl+Y` | Undo / redo |
| `Ctrl+A` / `Ctrl+E` | Line start / line end |
| `Ctrl+Home` / `Ctrl+End` | Buffer start / buffer end |
| `Ctrl+←` / `Ctrl+→` or `Alt+←` / `Alt+→` | Move by word |
| `Ctrl+K` | Kill to end of line |
| `Ctrl+U` / `Ctrl+Y` | Kill line / yank |

### Console

| Keys | Action |
| :--- | :--- |
| `Tab` | Move between console controls |
| `Enter` | Open a failing test diff |
| `D` | Open the diff for the selected test |
| `Ctrl+R` | Run the fixtures again |

## Settings

| Keys | Action |
| :--- | :--- |
| `J` / `K` | Move between fields |
| `H` / `L` | Move focus between the rail and the canvas |
| `Enter` | Edit the selected field |
| `Esc` | Cancel editing / back |
| `Ctrl+S` | Save settings |
| `/` | Focus search |
| `Tab` | Next field |
| `Click` | Select a section, toggle a value, open a picker |

### Account modal

| Keys | Action |
| :--- | :--- |
| `Tab` | Next field |
| `Enter` | Submit |
| `Ctrl+T` | Toggle sign in / sign up |
| `Esc` | Go offline / close |
| `Ctrl+R` | Reveal a masked secret |

## Pickers and palettes

| Keys | Action |
| :--- | :--- |
| Type | Filter |
| `↑` / `↓` or `J` / `K` | Move |
| `Enter` | Apply / run |
| `Esc` | Revert preview / close |
| `Tab` | Next group (where present) |

The theme picker previews live on cursor movement and reverts on `Esc`. The
language, provider, and model pickers apply on `Enter`.

## Account dropdown

| Keys | Action |
| :--- | :--- |
| `↑` / `↓`, `K` / `J` | Move |
| `Enter` | Activate |
| `Esc` | Close |

## Mouse

| Target | Action |
| :--- | :--- |
| Top-bar breadcrumb | Navigate to that segment |
| Settings label | Open Settings |
| Account label | Open the account dropdown |
| Roadmap cards / topic rail | Select / filter |
| Solve console tabs | Switch tab |
| Solve rails | Drag to resize, double-click to reset |
| Toasts | Hover pauses, click dismisses, action span runs |
| Settings rail / fields | Select / edit |

## Website keyboard strip

Use these six on the landing page. They fit one row on desktop and wrap to two
on mobile.

```text
F2  solve     F3  stats     F4  themes
ctrl+p  commands   ctrl+r  run   ctrl+s  save
```

Caption: `Every action has a key. Nothing is buried in a menu.`

## Design note for keycaps

- Keycaps are `<kbd>` elements, mono, 22px tall, 1px `border`, 2px bottom edge.
- Render chords as separate caps with a thin `+`: `ctrl` `+` `p`.
- Keep the label next to the cap in `text_dim`.
- Do not use the Apple command symbol; the product is cross-platform and uses
  `ctrl` everywhere.
