# Security

## Public access and credentials

Quote reads and public information routes are anonymous. The MCP endpoint and token-management operations require an owner Bearer token. The token page uses a secure owner cookie only for its CSRF-protected browser actions; API routes ignore cookies. Primary tokens manage an anonymous owner identity, while linked tokens are intended for separate clients. Token lists expose masked values only. Store the primary token securely and rotate it if it is lost or exposed.

There are no visitor accounts or passwords. The server operator token is separate from owner tokens and is stored in `secrets.yml`. Never publish configuration, database, backup, or log directories.

## HTTP and request handling

The server treats requests as untrusted and applies request-size, concurrency, and rate limits, origin checks, CSRF protection for browser mutations, and security headers. Put public deployments behind HTTPS. The health route reports operational status and build metadata without exposing configuration or secrets. Request logs include the request method, URI, and duration, so search terms in query strings may appear in logs.

The canonical security contact file is `/.well-known/security.txt`; the legacy `/security.txt` path returns 404. The file includes the configured GitHub private reporting URL, a time-limited report-form URL when the form is enabled, an optional email contact, a policy URL, expiration, and the public encryption-key URL when configured. It supports GET and HEAD.

## Security reporting

The public security overview is `/server/security`; its JSON mirror is `/api/v1/server/security`. The page links to the security-report form, disclosure policy, acknowledgments, and public PGP key. The contact form is reached through the security overview or a valid `security_id` link, not through general site navigation. Its API mirror is `/api/v1/server/contact`.

Submissions use single-use time-gated form tokens, CAPTCHA, content checks, duplicate suppression, per-IP/email/global limits, and configurable strike bans. Security report contact details and report bodies are encrypted at rest. Reports use the configured OpenPGP key when available; the application encryption key protects the contact fields and provides the configured fallback for report bodies.

The disclosure policy covers the application and API, excludes denial-of-service testing and other harmful activity, and requests private reporting with a coordinated-disclosure window. The policy and acknowledgments are available at `/server/security/policy` and `/server/security/thanks`.

## Operator controls

The maintenance CLI provides security-report listing, metadata viewing, and status updates. It does not display decrypted report bodies. PGP key operations include generation, import, publication, rotation, export, and deletion. The private key is encrypted with the installation secret at rest. Private exports require operator authorization and use file mode `0600`. Configure publication with `web.security.keyservers` and `web.security.publish_pgp_key`; rotation re-encrypts retained PGP reports and preserves the retired encrypted key for the configured retention period.

Backups include the PGP key material, installation secret, and report records. Run `anime --maintenance backup test` to perform the dry-run key restore check and confirm that the encrypted private key decrypts successfully.

## Discovery

`/.well-known/pgp-key.asc` is available when a public key exists. `/llms.txt` and `/.well-known/llms.txt` publish API and MCP discovery information. `robots.txt`, `sitemap.xml`, `/feed.xml`, and `/rss.xml` are public discovery files. The sitemap contains only public pages allowed by the configured crawler rules; private action routes, APIs, and the contact form are omitted.

## Tor

The production Docker Compose deployment enables a Tor hidden service by default. Its hostname is stored in the Tor data volume and shown on `/server/help` after the service is published. Set `ENABLE_TOR=false` to disable it.
