# Changelog

## 2026-11-08 — v0.5.0-beta

The first public beta.

### Added

- Windows (x86_64, arm64), Linux (x86_64, arm64), and macOS (x86_64, arm64)
  release archives with SHA-256 checksums.
- `openleet --version`, and a version line in Settings → About.
- Structured GitHub issue forms for bugs, feature requests, and anything else,
  including an explicit out-of-scope path.
- A Windows installer that selects the matching architecture, accepts
  prereleases, and verifies the downloaded checksum.

### Changed

- Screenshot library converted to WebP at quality 88: about 5.5 MB of PNGs
  became about 1.2 MB, with every image keeping its native dimensions.
- Refreshed the roadmap, solve, stats, and settings captures against the
  current one-row header.
- Added the twelve-palette Home gallery under `readme/screenshots/theme/`.

### Verification

- `go vet ./...` and `go test ./... -count=1` pass.
- GitHub Actions build, vet, and test job passes on the release commit.
- Windows and Linux amd64 archives were extracted and smoke-tested.

## 2026-11-07 — Terminal header redesign and Home stats

### Changed

- The shared header is now exactly one row of plain text labels:
  `Settings` and the account label (`Log in` when signed out), right-aligned
  with three visual cells between them. No brackets, slashes, corner glyphs,
  ASCII icon stand-ins, or box-drawing fragments.
- Rest labels use `text_dim`; hover/focus uses `text` on a borderless
  `surface_raised` pill with one cell of padding; pressed uses `accent` text.
  Signed-out `Log in` stays accent-coloured at rest.
- The account label adapts: full email at ≥120 columns, the local part before
  `@` at 100–119, the first letter below 100.
- Hover/focus tooltips (`Settings (F4)`, `Account`) are now transient overlays
  on the first body row. The header never reserves a second row.
- Keyboard focus reaches the top bar on Home, Roadmap, Solve, and Settings:
  `F6` focuses the breadcrumb, then `Tab`/`Shift+Tab` cycle the labels and
  `Enter` activates them.
- The progress line (`1d streak · 3/17 solved`) moved from the header to Home,
  one blank row under the search box, centered on the card. Numbers use `text`;
  labels and the separator use `text_dim`. Hidden when the streak is 0 and
  nothing is solved; shows only `3/17 solved` at streak 0; shortens to
  `1d · 3/17` when needed.
- The Home status line no longer repeats the solved count:
  `Coach · Python · 17 Problems · Local`.
- The shared chrome is one row shorter (header + footer = 2 rows), so Home,
  Roadmap, Solve, and Settings bodies gained one row.
- `appearance.icons` is now `auto` / `on` / `off` (default `auto`) with a
  terminal capability probe. Legacy `unicode` / `ascii` values migrate to
  `on` / `off`.
- The Settings status row no longer duplicates the shared footer hints; it keeps
  only the `saved` / `unsaved` indicator. Settings context hints (including
  auth and provider overlays) are rendered by the shared footer.

### Tests

- New regression coverage: top-bar hit windows cover exactly their labels;
  clicks open the correct surface; the tooltip overlay stays out of the header;
  keyboard focus works on every chrome view; mini stats appear, hide, and
  shorten correctly; the icons setting and legacy migration are locked down.
- `go test ./... -count=1` and `go vet ./...` pass.

### Screenshots

- Fresh captures at 1920×1080, 160 columns, 120 columns, and 100 columns across
  OpenLeet, OpenLeet Light, Dracula, Gruvbox, and Tron.
- Canonical images live in `readme/screenshots/`; narrow proofs live in
  `readme/screenshots/narrow/`. Root-level capture duplicates were removed.

## 2026-11-07 — Shared chrome and identity

- Unified the app-level top bar and footer across Home, Roadmap, Solve, and
  Settings; removed the duplicated Home breadcrumb/footer and the per-view
  header rows.
- Fixed the top-bar hit map so clicks land on the painted label.
- Finished per-user encrypted local identity, scoped storage, guest claim, and
  legacy migration.
- Fixed logout from every Settings pane.
