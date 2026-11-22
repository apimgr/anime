# Translation guide

Anime is currently English-only: the server, API messages, CLI, and browser
pages do not yet load locale catalogs. The localization baseline required by
the project specification is not complete, so this guide describes the
catalog contract and must not be read as a claim that these languages are
already supported.

## Required locales

The minimum supported set is English (`en`), Spanish (`es`), French (`fr`),
Arabic (`ar`), German (`de`), and Italian (`it`). English is the fallback.
Arabic is right-to-left; the other required locales are left-to-right. Each
locale must have a complete catalog with the same message keys and plural
message categories as English before it can be listed as supported. A partial
translation must not be selected by the server or CLI.

The catalogs belong in `src/common/i18n/locales/{language}.json` and are
embedded into the server and CLI. The locale registry is the canonical source
for language code, native name, direction, and CLDR plural categories. The
registry, catalog files, runtime language list, preference selector, and
documented language list must agree. Build-time validation must reject a
missing locale or mismatched keys.

## Translation workflow

When localization support is implemented:

1. Copy the English catalog and translate every human-readable message,
   including browser UI, errors, health/status labels, consent, legal content,
   API documentation descriptions, and CLI help and output.
2. Preserve message keys, placeholders, route names, API field names, command
   names, and configuration keys. Translate values only.
3. Preserve the English catalog's plural categories and placeholder names.
   Use the locale's registered direction and CLDR plural rules.
4. Add the catalog and registry metadata together. Add the locale to the
   language table in this guide in the same change, and update the stated
   supported-language count.
5. Have a fluent reviewer check terminology, context, accessibility labels,
   and legal text before enabling the locale.

The application must resolve language in this order: a supported `?lang=`
value, a supported `lang` cookie, the best supported `Accept-Language` match
(including regional fallback such as `fr-CA` to `fr`), then English. Missing
keys fall back to English. Unsupported languages silently resolve to English.
The language-cookie behavior for `?lang=` is still being reconciled in the
implementation specification: one rule describes it as a one-shot link, while
another says it persists the choice. Follow the resolved template contract
once that contradiction is fixed; automatic detection itself must not persist
a language cookie.

The language selector belongs only on `/server/preferences`, lists native
language names, and starts with an Automatic option that identifies the
detected language. The footer links the active language name to that selector.
The document language and direction must match the resolved locale.
