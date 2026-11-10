# Security

The service is intentionally anonymous and read-only. It stores no accounts or personal records and exposes no mutation API or administration panel. Quote data is embedded in the binary.

HTTP requests are untrusted. The server applies request size, concurrency, and rate limits and emits security headers. Operators should put public deployments behind HTTPS. The health route reports operational status and build metadata; it does not expose configuration or secrets.

Security contact information is configured by the operator. Do not expose config, data, or log directories from a public web server.
