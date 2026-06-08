# CLAUDE.md — GiraffeMail Archive

This file is the authoritative guide for all Claude Code sessions on this project.

---

## Project Overview

**GiraffeMail Archive** — a self-hosted, single-binary email backup, archive, and cleanup tool.

Licensed under **GNU AGPL v3** — see [LICENSE](LICENSE) and [docs/LICENSING.md](docs/LICENSING.md).

```
giraffe_mailer/
├── cmd/giraffemail/      # CLI entry point (serve, migrate, seed, fsck, export)
├── internal/
│   ├── api/              # HTTP handlers + chi router
│   ├── archive/          # IMAP indexer, archiver, engine, progress SSE
│   ├── cleanup/          # Safety gate (golden rule enforcement)
│   ├── config/           # YAML config loader
│   ├── crypto/           # AES-256-GCM helpers
│   ├── db/               # SQLite open + embedded migrations
│   ├── mailconn/         # go-imap/v2 IMAP client wrapper
│   ├── store/            # Content-addressed blob store + fsck
│   └── ui/               # Embedded Next.js static export
├── frontend/             # Next.js 15 + FSD UI (GiraffeMail dark theme)
├── Makefile
├── go.mod                # module github.com/GiraffeSecurity/giraffemailer
└── CHANGELOG.md
```

---

## Golden Rule — Never Break This

A message may **only** be deleted from the mail server after:
```sql
archived_at IS NOT NULL AND blob_sha256 IS NOT NULL AND blob_sha256 != ''
```
Enforced in `internal/cleanup/gate.go` (`IsSafe`) and baked into every cleanup SQL query
via `SafetyCandidateSQL`. It is a code invariant, not a convention.

---

## Go Backend — Absolute Rules

### Architecture
Flat handler layer: `internal/api/` handlers call `internal/archive/` and `internal/store/` directly.

### Database
- Pure Go SQLite via `modernc.org/sqlite` (`CGO_ENABLED=0`).
- All PKs are UUID v4 via `github.com/google/uuid`.
- Every table has `created_at`, `updated_at`, `deleted_at` (TIMESTAMPTZ).
- Never hard-delete. Soft-delete via `deleted_at`.
- All queries filter `WHERE deleted_at IS NULL` unless fetching deleted records.
- Migrations: `internal/db/migrations/NNNNNN_name.up.sql` + `.down.sql`, run on startup.

### Blob Store
- Path: `<data_dir>/blobs/<account_uuid>/<sha256[0:2]>/<sha256[2:4]>/<sha256>.eml.zst`
- zstd-compressed; optionally AES-256-GCM encrypted.
- Content-addressed: SHA-256 of raw RFC 822 bytes.
- Atomic writes: temp → sync → rename.
- Post-write verify: decompress → rehash → compare before marking `archived_at`.

### API Response Format
```json
{ "status": "success", "data": ... }
{ "status": "fail", "message": "..." }
```

### Auth
- Opaque bearer tokens: 32 random bytes → base64 → stored as SHA-256 hash.
- 30-day TTL. bcrypt cost 12 for passwords.
- OTP: 6-digit, 10-min TTL, SHA-256 hashed, single-use.
- Rate limit: 5/min/IP on auth endpoints (in-memory sliding window).

### Logging
- `zerolog` JSON, `TimeFormatUnixMs`, `InfoLevel` in production.
- Levels: `Error` (unexpected), `Warn` (degraded), `Info` (lifecycle).
- Never log passwords, tokens, OTP codes, or encryption keys.

### Comments
- Write no comments by default.
- Only add one when the WHY is non-obvious.
- No TODO/FIXME, no godoc placeholders, no section separators.

---

## Frontend — GiraffeMail UI

### Theme (always dark)
| Token       | Value     |
|-------------|-----------|
| Background  | `#0E0E10` |
| Surface     | `#17171A` |
| Accent gold | `#C9A227` |
| Fonts       | Inter + Noto Sans Georgian |

### FSD Layer Order (never reverse)
```
app  →  (gm) route group  →  widgets  →  entities  →  shared
```

### Routes
| Path           | Widget         |
|----------------|----------------|
| `/gm`          | GmDashboard    |
| `/gm/accounts` | GmAccountsList |
| `/gm/search`   | GmSearch       |
| `/gm/archive`  | GmMessageList  |
| `/gm/cleanup`  | GmCleanup      |
| `/gm/settings` | SettingsPage   |

### GiraffeMail Entities
- `entities/mailAccount/` — IMAP account CRUD
- `entities/mailbox/` — mailboxes, messages, search, insights
- `entities/cleanup/` — cleanup jobs + runs

### Rules
- All HTTP calls go through `shared/api/gmHttpService.ts`.
- Pages are Server Components. `"use client"` only on leaf interactive widgets.
- No `<img>` — use `next/image`. No CDN fonts — use `next/font`.

---

## CLI Commands

```bash
# Always use package path form (not single-file go run):
go run ./cmd/giraffemail serve          # Start server
go run ./cmd/giraffemail migrate        # Run migrations and exit
go run ./cmd/giraffemail seed           # Create admin@localhost / admin123
go run ./cmd/giraffemail fsck           # Verify all blobs

# Or build first:
make build
./giraffemail serve --config config.yaml
```

Frontend dev server (separate process):
```bash
cd frontend && pnpm install && pnpm dev
```

Embed UI into binary:
```bash
make build-full   # runs build-ui then go build
```

---

## Changelog

- Update [CHANGELOG.md](CHANGELOG.md) for user-visible changes.

---

## Tech Stack

| Concern       | Library                                    |
|---------------|--------------------------------------------|
| HTTP router   | `github.com/go-chi/chi/v5`                 |
| SQLite        | `modernc.org/sqlite` (pure Go, no CGo)     |
| IMAP          | `github.com/emersion/go-imap/v2`           |
| Compression   | `github.com/klauspost/compress/zstd`       |
| UUID          | `github.com/google/uuid`                   |
| Logging       | `github.com/rs/zerolog`                    |
| CLI           | `github.com/spf13/cobra`                   |
| HTML sanitize | `github.com/microcosm-cc/bluemonday`       |
| Passwords     | `golang.org/x/crypto/bcrypt` + AES-256-GCM |
| Config        | `gopkg.in/yaml.v3`                         |
| Frontend      | Next.js 15, TanStack Query v5, Tailwind v4 |
