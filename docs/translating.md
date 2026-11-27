# Translation guide

Anime uses embedded locale catalogs shared by the server and CLI. The required language set is English (`en`), Spanish (`es`), French (`fr`), Arabic (`ar`), German (`de`), and Italian (`it`). English is the fallback; Arabic is right-to-left. The selector uses native language names and is available only from `/server/preferences`.

## Language selection

The server resolves language in this order:

1. A supported `?lang=` query value. This selects the language for the request. It writes a persistent `lang` cookie only when the Preferences consent category has been accepted; without consent, it applies to that response only.
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

Use these terms consistently across the interface, CLI, email, and documentation. Translate the concept for each locale; keep route names, API fields, and configuration keys unchanged. Preserve `token`, product names, protocol names, and other terms that AI.md marks as untranslated.

| English | Español | Français | Deutsch | Italiano | العربية |
|---|---|---|---|---|---|
| Anime Quotes API | Anime Quotes API | Anime Quotes API | Anime Quotes API | Anime Quotes API | Anime Quotes API |
| owner token | token del propietario | token du propriétaire | Owner-Token | token del proprietario | token المالك |
| linked token | token vinculado | token lié | verknüpfter Token | token collegato | token المرتبط |
| token prefix | prefijo del token | préfixe du token | Token-Präfix | prefisso del token | بادئة token |
| paste the token | pegue el token | collez le token | fügen Sie den Token ein | incolli il token | قم بلصق token |
| rotate a token | rotar un token | renouveler un token | einen Token erneuern | ruotare un token | تدوير token |
| revoke a token | revocar un token | révoquer un token | einen Token widerrufen | revocare un token | إبطال token |
| preference sync | sincronización de preferencias | synchronisation des préférences | Synchronisierung der Einstellungen | sincronizzazione delle preferenze | مزامنة التفضيلات |
| security report | informe de seguridad | signalement de sécurité | Sicherheitsbericht | segnalazione di sicurezza | بلاغ أمني |
| security disclosure | divulgación de vulnerabilidades | divulgation de vulnérabilités | Offenlegung von Sicherheitslücken | divulgazione delle vulnerabilità | الإفصاح عن الثغرات الأمنية |
| rate limit | límite de solicitudes | limite de requêtes | Ratenbegrenzung | limite di richieste | حد الطلبات |
| backup | copia de seguridad | sauvegarde | Sicherung | copia di backup | نسخة احتياطية |
| restore | restaurar | restaurer | wiederherstellen | ripristinare | استعادة |

## Translator credits

The supported locale files are `en.json`, `es.json`, `fr.json`, `ar.json`, `de.json`, and `it.json` (6 languages). No individual translator names or handles are currently recorded. Each locale's `meta.translators` list is empty until a contributor provides a preferred public name or handle; credited contributors may add it through a normal pull request. Do not infer or assign credit without the contributor's preference.

Usage notes:

- An owner token controls an owner's account and linked API tokens; never call it a password.
- A token prefix is a visible identifier, not the secret token value.
- Rotation replaces a credential and revokes the previous one. Revocation disables a credential so it can no longer authenticate.
- Preference sync transfers supported settings between this service and a client.
- Keep a security report distinct from an audit record. A backup is a restorable copy of application data and configuration.

Translate all human-readable application strings: web pages, validation and API errors, OpenAPI and GraphQL descriptions, email content, health/status output, consent controls, server and client CLI help and messages, startup banners, and setup prompts. Do not put runtime translations in a second locale directory or fetch locale files from the host filesystem.
