# OpenLeet — Website Reference Pack

This folder is everything a designer or developer needs to build a marketing
website for **OpenLeet** without reverse-engineering the app. It is written to be
handed to a web team as a single source of truth.

The application docs live one level up: [Features](../FEATURES.md),
[Architecture](../ARCHITECTURE.md), [AI Providers](../AI_PROVIDERS.md), and
[Screenshots](../SCREENSHOTS.md). This pack translates those facts into brand,
design, content, and asset guidance for a public website.

## Read in this order

| # | File | What it gives you |
| :-- | :--- | :--- |
| 1 | [Brand](01-BRAND.md) | Positioning, audience, voice, taglines, wordmark rules |
| 2 | [Sitemap](02-SITEMAP.md) | Recommended pages, section order, CTAs, wireframes |
| 3 | [Design System](03-DESIGN-SYSTEM.md) | Tokens, typography, spacing, components, accessibility |
| 4 | [Themes](04-THEMES.md) | All 12 palettes with hex values and usage guidance |
| 5 | [Screens](05-SCREENS.md) | Every screen described zone by zone, with states |
| 6 | [Keybindings](06-KEYBINDINGS.md) | Global and per-screen shortcuts for the "keyboard-first" story |
| 7 | [Commands](07-COMMANDS.md) | Every slash command, alias, and behaviour |
| 8 | [Content](08-CONTENT.md) | Ready-to-paste hero, features, FAQ, footer, and meta copy |
| 9 | [Assets](09-ASSETS.md) | Screenshot manifest with alt text and placement advice |
| 10 | [Technical Story](10-TECHNICAL-STORY.md) | Stack, architecture, security, and quality claims you can make |
| 11 | [Launch Checklist](11-LAUNCH-CHECKLIST.md) | SEO, performance, accessibility, and QA before shipping |

## One-paragraph summary

OpenLeet is a terminal-first data-structures and algorithms practice dojo with a
Socratic AI coach. It runs in the terminal, keeps progress and code local by
default, executes solutions against the host Python or Go toolchain, and guides
the user with questions instead of dumping answers. It ships with 17 curated
problems, a winding roadmap, a three-pane solve workspace, a streak/heatmap
dashboard, 12 colour themes, optional encrypted AI providers, and optional
Firebase account sync.

## Quick facts

| Fact | Value |
| :--- | :--- |
| Product name | OpenLeet (one word, capital O and L) |
| Category | Terminal DSA practice / developer tool |
| Tagline | "A terminal-first DSA practice dojo with a Socratic AI coach." |
| Platform | Windows, Linux, macOS (terminal) |
| Built with | Go 1.27, Bubble Tea, Lip Gloss, Glamour |
| Problem set | 17 curated problems across arrays, strings, DP, graphs, and CP |
| Execution | Python and Go via the host toolchain |
| Accounts | Optional; the app is fully usable offline |
| Themes | 12 palettes, 2 light + 10 dark |
| Data | Local-first; encrypted provider keys; optional per-user sync |
| Minimum terminal | 100 × 30 cells |

## Source-of-truth rules

1. **Behaviour comes from the app.** If copy and code disagree, the code wins.
   The screen descriptions in this pack were written from the current renderers.
2. **Screenshots are receipts, not mockups.** Every image in
   [`../screenshots`](../screenshots) was captured from a real binary with VHS at
   1920×1080 (truecolor) unless noted.
3. **Do not invent features.** The app does not have browser-based code
   execution, a web dashboard, automatic background sync, or OAuth device-code
   login. See the "Not implemented" rows in [Features](../FEATURES.md).
4. **No fake UI in marketing art.** If you need a screenshot of a state we have
   not captured (for example a signed-in chip), compose a real screenshot with
   the binary rather than drawing a mock terminal.

## Golden rules for the website

- **Dark first, light supported.** The brand is orange on ink. Light mode is a
  first-class alternate, not an afterthought.
- **One accent.** Orange is the action colour. Do not add a second saturated
  accent for decoration.
- **Terminal honesty.** Use monospace for commands, keycaps, and any shell text.
  Do not render fake window chrome with traffic lights around screenshots.
- **Plain labels, not pills.** The app header is plain text with a borderless
  raised pill on hover/focus. Reserve bordered chips for metadata tags on the
  site, never for the primary navigation.
- **Short, declarative copy.** The voice is quiet and technical. No hype, no
  exclamation marks, no "revolutionary".
- **Keyboard-first.** Show shortcuts next to actions. The product's promise is
  that you never have to leave the keyboard.
