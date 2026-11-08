# Contributing to OpenLeet

Thanks for helping improve OpenLeet. This project is a terminal-first Go
application, so changes should respect the terminal grid and the single-threaded
Bubble Tea loop.

## Before you start

- Read [Architecture](readme/ARCHITECTURE.md) and [Features](readme/FEATURES.md).
- For a large change, open an issue first so the design can be discussed.
- Never commit `.env`, provider keys, local data, or build artifacts. See
  [`.gitignore`](.gitignore).

## Requirements

- Go matching [`go.mod`](go.mod)
- Optional: Python 3 and/or Go on the host for the runner
- Optional: WSL, VHS, ttyd, ffmpeg, and Chrome for visual captures

## Build and test

```bash
git clone https://github.com/srikar-up/openleet.git
cd openleet
go build -o ./bin/openleet ./cmd/openleet
go vet ./...
go test ./... -count=1
```

On Windows:

```powershell
go build -o .\bin\openleet.exe .\cmd\openleet
.\bin\openleet.exe
```

## Code rules

1. **One Elm loop.** All state changes happen in `Update`. Do not mutate models
   from goroutines.
2. **Side effects are commands.** Network calls, timers, and process execution
   return a `tea.Msg` through `tea.Cmd`; never block `Update`.
3. **The grid is exact.** Derive sizes from `width`/`height` at render time.
   Never hardcode terminal dimensions.
4. **Measure cells, not bytes.** Use `uikit.Truncate`, `uikit.Fit`,
   `uikit.FitExact`, `lipgloss.Width`, or `ansi.StringWidth`.
5. **Join with Lip Gloss.** Use `JoinHorizontal` / `JoinVertical` instead of
   manual newline concatenation.
6. **Colours come from the palette.** Add tokens to `internal/ui/theme`; never
   embed hex values in a view.
7. **No placeholders.** Do not merge `TODO`, `panic("unimplemented")`, or
   stub returns.

## Tests

- Add a regression test for every behaviour change.
- Keep exact-grid, mouse hit-testing, and theme-audit tests green.
- Run the focused pipeline receipt as well:

```bash
go test ./internal/eventbus ./internal/ui/uikit ./internal/ui/workspace ./internal/ui -count=1
```

## Visual changes

Layout, border, spacing, and colour changes need a screenshot. From a WSL
checkout:

```bash
bash scripts/capture-tui.sh roadmap
bash scripts/capture-tui.sh settings openleet appearance

# Convert the new PNG captures to WebP
python scripts/convert-screenshots.py --delete
```

Check at 100, 120, and 160 columns. The app must stay usable at its **100×30**
minimum. Include the before/after image in the pull request.

## Commit and pull request

- Keep commits focused; avoid unrelated formatting churn.
- Write an imperative subject: `fix: keep the top bar on one row`.
- Describe the behaviour change, tests run, and screenshots.
- Link the issue the change resolves.
- Do not force-push a branch with an open review unless asked.

## Security

Do not report secret-handling vulnerabilities in a public issue. Follow
[SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contribution is licensed under the
[MIT License](LICENSE).
