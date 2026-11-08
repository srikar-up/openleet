# 11 — Launch Checklist

Run this before publishing the site. Tick every item; the site is not done until
the acceptance block passes.

## Content and brand

- [ ] Product name is always `OpenLeet` in prose.
- [ ] Hero H1 is one of the approved lines and fits 2–3 lines on desktop.
- [ ] No banned words: revolutionary, seamless, game-changing, 10x, LeetCode
      killer.
- [ ] Every feature claim matches `01-BRAND.md` and `10-TECHNICAL-STORY.md`.
- [ ] No claim of browser execution, automatic sync, or every-language support.
- [ ] Every screenshot has descriptive alt text from `09-ASSETS.md`.
- [ ] The install commands are copy-pasteable and tested.
- [ ] `install.sh` works through both `curl` and `wget` on Linux and macOS.
- [ ] Download links read [`version.json`](../../version.json); no release tag is
      hardcoded in the page.
- [ ] The footer includes the local-first sentence and a real contact link.
- [ ] FAQ answers are consistent with the docs.

## Header and chrome acceptance (current app build)

- [ ] Home top-right shows only `Settings` and the account label.
- [ ] No brackets, slashes, `*`, `o`, corner pieces, or box-drawing glyphs.
- [ ] The header is exactly one row; no second tooltip row is reserved.
- [ ] `Settings   email` uses a 3-cell/16px visual gap.
- [ ] Rest labels are dim; hover/focus is a borderless raised pill.
- [ ] Signed-out `Log in` is accent-coloured.
- [ ] Hover and keyboard focus (F6, then Tab) show `Settings (F4)` / `Account`.
- [ ] Clicking the Settings label opens Settings; clicking the account label
      opens the dropdown.
- [ ] `2d streak · 2/17 solved` appears centered under the Home search box only.
- [ ] The stats line is hidden when streak is 0 and nothing is solved.
- [ ] The in-box line reads `Coach · Python · 17 Problems · Local` with no
      duplicated solved count.
- [ ] At 100/120/160 columns nothing overlaps or clips.
- [ ] Five themes, light and dark, show readable labels and matching backgrounds.

## SEO

- [ ] Each page has a unique `<title>` and meta description.
- [ ] Exactly one `<h1>` per page; sections use `<h2>`/`<h3>` in order.
- [ ] Canonical URLs are set.
- [ ] `robots.txt` and `sitemap.xml` exist and list every public page.
- [ ] Open Graph and Twitter card tags are set; the OG image is 1200×630.
- [ ] JSON-LD `SoftwareApplication` includes name, description, OS, and a free
      offer. Do not add a fake aggregate rating.
- [ ] Internal links resolve; no `#` placeholders remain.

## Performance

- [ ] Hero image is preloaded and served as WebP ≤ 120 KB.
- [ ] Below-the-fold images lazy-load with explicit dimensions.
- [ ] Total JS ≤ 100 KB gzipped for the landing page.
- [ ] No layout shift from fonts: use `font-display: swap` and size-adjust.
- [ ] Lighthouse mobile ≥ 90 for Performance, Accessibility, Best Practices, SEO.
- [ ] The page works with JS disabled for content and links.

## Accessibility

- [ ] Colour contrast: body 4.5:1, large 3:1, borders 3:1.
- [ ] Keyboard order is logical; the skip link is first.
- [ ] Focus rings are visible on every interactive element.
- [ ] Nav, buttons, and FAQ accordions are reachable and operable by keyboard.
- [ ] `aria-label` on icon-only links; `aria-expanded` on accordions.
- [ ] Reduced-motion media query disables transforms and fades.
- [ ] Screenshots have alt text; decorative art has empty alt.
- [ ] The page is usable at 200% zoom and at 320px width.

## Responsive

- [ ] Tested at 320, 375, 768, 1024, 1280, 1440, 1920 px.
- [ ] Feature grid goes 3 → 2 → 1 columns.
- [ ] Screens band scrolls horizontally on mobile with snap points.
- [ ] Nav collapses to a sheet; the Install CTA stays visible.
- [ ] No horizontal scroll at any width.

## Cross-browser

- [ ] Chrome, Firefox, Safari, Edge (current versions).
- [ ] Safari iOS and Chrome Android on a real device or emulator.
- [ ] Backdrop blur has a solid fallback.
- [ ] Custom properties and `color-mix` have fallbacks if used.

## Theme and visual QA

- [ ] Dark and light site themes both pass contrast.
- [ ] Theme switch persists and does not flash on load.
- [ ] Screenshots stay intact in both site themes.
- [ ] Dark screenshots on the light site have a 1px border and 4px inner pad.
- [ ] No gradients, glows, or glassmorphism.
- [ ] One accent only; orange is reserved for actions and the wordmark block.

## Analytics and privacy

- [ ] Analytics is privacy-friendly and disclosed, or absent.
- [ ] No third-party fonts or scripts without a documented reason.
- [ ] No cookie banner needed (no tracking cookies).
- [ ] External links use `rel="noopener"`.

## Deployment

- [ ] HTTPS and HSTS.
- [ ] Security headers: CSP, X-Content-Type-Options, Referrer-Policy,
      Permissions-Policy.
- [ ] Cache headers: immutable for hashed assets, short for HTML.
- [ ] 404 page exists and links home.
- [ ] `www` and apex redirect to one canonical host.
- [ ] Uptime and error monitoring enabled.
- [ ] Rollback path documented.

## Final acceptance

- [ ] `go test ./... -count=1` passes in the app repository.
- [ ] `go vet ./...` passes.
- [ ] The screenshot set in `readme/screenshots` matches the shipped binary.
- [ ] The installed binary hash is recorded in the release notes.
- [ ] Every page in the sitemap is reachable from the nav or footer.
- [ ] A fresh visitor can understand the product, see it, and install it within
      one scroll.
