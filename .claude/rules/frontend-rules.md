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
- Every registered server-rendered page follows the PART 16 layout, privacy, language/direction, metadata, and no-JavaScript requirements; tests enumerate the full page registry.
- Keep each user-facing page paired with its API representation when AI.md requires one; health and one-shot emailed report pages are explicit exceptions.
- Verify the full responsive viewport, theme, RTL, stress-content, and interaction matrix with the backstop disabled.
- Source project-specific about/help facts from IDEA.md and keep configured operator content distinct from built-in translated text.

For complete details, see AI.md PART 16
