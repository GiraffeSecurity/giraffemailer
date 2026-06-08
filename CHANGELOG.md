# Changelog

All notable changes to GiraffeMail Archive are documented here.

## [Unreleased]

### Changed

- Dependency upgrades: Go 1.26, Node 22, pnpm 10, latest npm packages
- Docker Compose loads `GM_SECRET_KEY` from `.env` (YAML fix)

## [0.1.0] — 2026-06-08

First public release ([GiraffeSecurity/giraffemailer](https://github.com/GiraffeSecurity/giraffemailer)).

### Added
- IMAP archive engine with incremental sync and zstd content-addressed blobs
- Golden rule: server delete only after verified local archive
- SQLite FTS5 search with keyset pagination
- Cleanup jobs (preview, filter, delete/move on server)
- Export (mbox/zip) and IMAP restore
- RBAC: `admin` / `user` roles, account ownership
- Docker image, compose, and production config validation
- Embedded Next.js UI (dark theme)
- Docs: installation, deployment, upgrade, API, architecture, licensing
- CI: Go tests + race, govulncheck, frontend vitest, Docker build
- `GET /healthz`, `GET /readyz`
- HttpOnly session cookies, CORS allowlist, CSP headers
- GitHub issue/PR templates, CODE_OF_CONDUCT, CONTRIBUTING, SECURITY

### Security

- AES-256-GCM credential and blob encryption (production)
- Token revocation on password change; inactive user check
- Auth rate limiting (5/min/IP)
- HTML sanitization (bluemonday)
