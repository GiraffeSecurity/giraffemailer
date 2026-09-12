# Changelog

All notable changes to GiraffeMail Archive are documented here.

## [Unreleased]

### Fixed

- Settings now includes a change-password form backed by `POST /api/v1/auth/change_password` (issue #1)
- `giraffemail seed` no longer overwrites an existing `admin@localhost` password on every run (including Docker restarts); use `seed --force` to reset to the default
- Embedded / export UI builds use same-origin API calls by default (avoids CSP `connect-src 'self'` failures from a baked-in `localhost:9191`)
- Docs: first-open URL is `/login`; Docker config path clarified (`config.docker.yaml` → `/etc/giraffemail/config.yaml`)
- Bump Go toolchain to 1.26.6 for stdlib CVE fixes reported by govulncheck (GO-2026-6090, GO-2026-6089, GO-2026-5972, GO-2026-5856)

### Changed

- Dependency upgrades: Go 1.26, Node 24, pnpm 10, latest npm packages
- Docker Compose loads `GM_SECRET_KEY` from `.env` (YAML fix)
- CI: GitHub Actions v6, Node 24 runner, fix static UI serving (white screen)

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
