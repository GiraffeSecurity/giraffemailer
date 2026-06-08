# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| `main` branch | yes — active development |
| Latest release tag | yes — recommended for production |

Install and upgrade: [docs/INSTALLATION.md](docs/INSTALLATION.md) · [docs/UPGRADING.md](docs/UPGRADING.md)

## Reporting a vulnerability

**Do not open public GitHub issues for security vulnerabilities.**

Email security reports to the maintainers at Giraffe.ge with:

- Description of the issue
- Steps to reproduce
- Impact assessment
- Suggested fix (optional)

We aim to acknowledge within 72 hours and provide a fix timeline within 7 days for critical issues.

## Security model

GiraffeMail Archive is designed as a **self-hosted** tool with optional multi-user RBAC:

- Deploy behind TLS reverse proxy
- Set `app.env: production`, strong `app.secret_key`, `storage.encrypt_blobs: true`
- Set `security.allow_registration: false` unless you need public signup
- Restrict network access (VPN / firewall)
- Assign `admin` role sparingly; regular users only see mail accounts they own

Multi-tenant SaaS deployment still requires additional hardening (WAF, centralized secrets, distributed rate limits).

## Known limitations

- In-memory rate limiting (resets on restart; not multi-instance safe)
- SQLite single-writer (not horizontally scaled)
- IMAP `use_tls: false` sends credentials in cleartext
