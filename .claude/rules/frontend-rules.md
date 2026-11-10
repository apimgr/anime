# Frontend Rules (PART 16)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never make core browsing or search depend on JavaScript.
- Never use inline CSS, inline event handlers, JavaScript alerts, or client-side rendering.
- Never hardcode a dark-only theme or ignore keyboard and screen-reader use.
- Never render unescaped quote or request content.

## CRITICAL - ALWAYS DO

- Use server-rendered Go templates with progressive enhancement and PWA support.
- Support dark, light, and auto themes with server-rendered `theme` cookie state.
- Meet WCAG 2.1 AA, mobile-first layout, and 44px minimum touch targets.
- Version static assets and keep HTML and service-worker responses fresh.
- Provide metadata, sitemap, and allowlisted well-known resources as specified.

## Key rules

- Keep essential forms functional without JavaScript.
- External stylesheets and scripts use the shared embedded asset system.
- Localized text comes from translation catalogs, not hardcoded UI strings.

For complete details, see AI.md PART 16
