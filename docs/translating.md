# Translation guide

Anime uses embedded locale catalogs shared by the server and CLI. The required language set is English (`en`), Spanish (`es`), French (`fr`), Arabic (`ar`), German (`de`), and Italian (`it`). English is the fallback; Arabic is right-to-left. The selector uses native language names and is available only from `/server/preferences`.

## Language selection

The server resolves language in this order:

1. A supported `?lang=` query value. This selects the language for the request and writes the persistent `lang` cookie.
2. A supported `lang` cookie.
3. The best supported match from `Accept-Language`, including regional fallback such as `fr-CA` to `fr`.
4. English.

Automatic detection does not write a cookie. Unsupported values silently fall back to English. The page's `lang` and `dir` attributes follow the resolved locale.

## Catalog format and validation

Catalogs live at `src/common/i18n/locales/{language}.json` and are embedded in the server and CLI. The locale registry, files, runtime language list, preferences selector, and documentation must describe the same supported set. Every locale must have the same translation keys and interpolation variables as English. Missing keys fall back to English; validation belongs in the normal test suite.

## Translation workflow

1. Add or update the English message key in `en.json`.
2. Add its translated value to every supported locale in the same change.
3. Preserve keys, interpolation variables, plural categories, route names, API field names, command names, and configuration keys.
4. Use the registered direction and CLDR plural rules for the language.
5. Update tests and user-facing documentation when the supported set or behavior changes.
6. Use AI translations for every supported locale and welcome human corrections through the normal contribution flow. Native-speaker review is welcome but is not a merge requirement; preserve the established terminology, context, accessibility meaning, and legal meaning when correcting text.

## Product glossary

Use these terms consistently across the interface, CLI, email, and documentation. Translate the concept for each locale; keep route names, API fields, and configuration keys unchanged.

| English term | Meaning and translation guidance |
|---|---|
| Anime Quotes API | Product name. Keep the brand name unchanged. |
| owner token | A credential that controls an owner's account and linked API tokens. Do not call it a password. |
| linked token | An API credential created under an owner account. Keep it distinct from the owner token. |
| token prefix | The visible identifier used to recognize a token; it is not the secret token value. |
| rotate a token | Replace a credential and revoke the previous credential. |
| revoke a token | Disable a credential so it can no longer authenticate. |
| preference sync | Transfer the supported preferences between this service and a client. |
| security report | A report about a security event or finding; preserve the distinction from an audit record. |
| disclosure | A security disclosure submitted through the published reporting process. |
| rate limit | A policy that limits request frequency. Keep the term distinct from account or token quotas. |
| backup | A restorable copy of application data and configuration. |
| restore | Recover application data from a backup. |

Translate all human-readable application strings: web pages, validation and API errors, OpenAPI and GraphQL descriptions, email content, health/status output, consent controls, server and client CLI help and messages, startup banners, and setup prompts. Do not put runtime translations in a second locale directory or fetch locale files from the host filesystem.
