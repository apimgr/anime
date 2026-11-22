# Security

The public catalog is read-only and the service has no visitor accounts. Its contact and coordinated-disclosure forms accept moderated submissions and store security reports in the application database as encrypted ciphertext.

HTTP requests are untrusted. The server applies request size, concurrency, and rate limits and emits security headers. Operators should put public deployments behind HTTPS. The health route reports operational status and build metadata; it does not expose configuration or secrets. Application request logs include the request method, URI, and duration, so search terms in query strings may be logged.

The application serves `/security.txt` and `/.well-known/security.txt` with a current expiration and a canonical URL based on the request origin. The `/server/security` page links to the disclosure form, policy, acknowledgments, and public PGP key. Its JSON mirrors are available under `/api/v1/server/security`. The form uses CAPTCHA, single-use time-gated tokens, review confirmation, content checks, duplicate detection, per-IP/email/global limits, and configurable strike bans. Security reports use the PGP key when configured; otherwise report bodies use AES-256-GCM with `server.security.encryption_key`. Contact details are always AES-256-GCM encrypted.

Operators manage reports with `anime --maintenance security list`, `show`, and `update`. The commands expose metadata and researcher comments, never report bodies. PGP keys are managed with `anime --maintenance pgp generate`, `rotate`, `publish`, `import`, `export`, and `delete`. The private key is encrypted with the installation secret on disk; private exports require a separate operator-token confirmation and are written with mode 0600. Configure keyservers under `web-security.keyservers` to enable publication. Rotation re-encrypts retained PGP report bodies before activating the new key and retains the previous encrypted key for 30 days.

Do not expose config, data, or log directories from a public web server.

The built-in disclosure policy covers the application and API, excludes denial-of-service testing and other harmful activity, and requests private reporting with a 90-day coordinated-disclosure window. The window ends early when a fix is released and may be extended by agreement. Good-faith research that follows the policy is authorized.

The production Docker Compose file enables a Tor hidden service by default. The container writes the onion hostname under `/data/tor/hidden_service/hostname`; the public help page shows the address after Tor creates it. Set `ENABLE_TOR=false` to disable this service.
