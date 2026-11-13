# Security

The service is intentionally anonymous and read-only. It stores no accounts or visitor profiles and exposes no mutation API or administration panel. Quote data is embedded in the binary.

HTTP requests are untrusted. The server applies request size, concurrency, and rate limits and emits security headers. Operators should put public deployments behind HTTPS. The health route reports operational status and build metadata; it does not expose configuration or secrets. Application request logs include the request method, URI, and duration, so search terms in query strings may be logged.

The application serves `/security.txt` and `/.well-known/security.txt`. Its current security contact is `security@apimgr.us`. Do not expose config, data, or log directories from a public web server.
