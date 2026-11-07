# OpenLeet Documentation

This folder contains the long-form project documentation referenced by the root [README](../README.md).

| Document | What it covers |
| :--- | :--- |
| [Features](FEATURES.md) | Detailed feature breakdown, interaction model, data handling, and current limitations |
| [AI Providers](AI_PROVIDERS.md) | Provider endpoints, selection behavior, key security, live endpoint examples, troubleshooting |
| [Architecture](ARCHITECTURE.md) | Elm loop, event pipeline, exact-grid rendering, runner, coach, storage, and sync design |
| [Screenshots](SCREENSHOTS.md) | Full visual gallery and capture commands |
| [Website pack](website/README.md) | Brand, sitemap, design system, all 12 themes, screen specs, keybindings, commands, copy, assets, technical story, launch checklist |

## Website reference pack

The `website/` folder is a self-contained handoff for building the public site:

| # | File | What it covers |
| :-- | :--- | :--- |
| 1 | [Brand](website/01-BRAND.md) | Positioning, audience, voice, taglines, wordmark |
| 2 | [Sitemap](website/02-SITEMAP.md) | Pages, section order, CTAs, wireframes |
| 3 | [Design system](website/03-DESIGN-SYSTEM.md) | Tokens, type, spacing, components, accessibility |
| 4 | [Themes](website/04-THEMES.md) | All 12 palettes with hex values |
| 5 | [Screens](website/05-SCREENS.md) | Every screen described zone by zone |
| 6 | [Keybindings](website/06-KEYBINDINGS.md) | Global and per-screen shortcuts |
| 7 | [Commands](website/07-COMMANDS.md) | Every slash command and alias |
| 8 | [Content](website/08-CONTENT.md) | Ready-to-paste hero, features, FAQ, meta |
| 9 | [Assets](website/09-ASSETS.md) | Screenshot manifest with alt text |
| 10 | [Technical story](website/10-TECHNICAL-STORY.md) | Stack, architecture, security, quality |
| 11 | [Launch checklist](website/11-LAUNCH-CHECKLIST.md) | SEO, performance, a11y, QA |

## Quick links

- [Project README](../README.md)
- [Feature status](FEATURES.md#at-a-glance)
- [AI provider table](AI_PROVIDERS.md#supported-providers)
- [Architecture diagram](ARCHITECTURE.md#high-level-flow)

## Documentation conventions

- **Implemented** means the feature is wired into the app and covered by code/tests.
- **Optional** means it requires external configuration or credentials.
- **Not implemented** means it is intentionally outside the current build.
- Screenshots are generated from the current binary with VHS and are treated as visual receipts.
