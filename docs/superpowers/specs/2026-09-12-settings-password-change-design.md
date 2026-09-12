# Design: Settings password change + seed safety (Issue #1)

**Date:** 2026-09-12  
**Status:** Approved for planning  
**Scope:** Issue #1 — cannot change admin password in Settings; password reset lost on Docker restart; docs/naming polish

## Problem

1. Docs tell users to change the admin password via **Settings → change password**, but Settings only shows a static `config.yaml` snippet — no password form.
2. Backend already implements `POST /api/v1/auth/change_password`.
3. Docker entrypoint runs `giraffemail seed` on every start; `seed` always upserts `admin@localhost` with password `admin123`, so password changes do not survive container restarts.
4. Install docs point to `/gm` as the first URL; the app redirects unauthenticated users to `/login`.
5. User-facing naming mixes GiraffeMail / GiraffeMail Archive / GiraffeMailer.

## Goals

- Users can change their password from Settings.
- Changed passwords persist across Docker restart/stop/start.
- Docs and Settings copy match Docker and source installs.
- User-facing product name is consistent: **GiraffeMail** (short) / **GiraffeMail Archive** (full).

## Non-goals

- Renaming the GitHub repository, Go module (`github.com/GiraffeSecurity/giraffemailer`), binary, or Docker image names.
- Editing server YAML from the UI.
- Profile editing, multi-user admin UI, or forgot-password from Settings.
- Changing the change-password API contract.

## Approach

Minimal fix (Approach 1): Settings password UI + idempotent `seed` with `--force` + docs/naming alignment. Docker entrypoint keeps calling `seed` without `--force`.

## Architecture

| Layer | Change |
|-------|--------|
| Frontend | Client password form on Settings; Docker-aware config note |
| Backend CLI | `seed` skip-if-exists; `--force` resets admin password |
| Docs | `/login`, Docker config clarification, naming, CHANGELOG |

No new HTTP endpoints.

## Frontend

### Settings page (`frontend/src/app/(gm)/gm/settings/page.tsx`)

Structure:

1. Heading: Settings
2. **Change password** section — client widget
3. **Server configuration** section — read-only note + key-fields reference

Config note must state:

- App settings come from the config file the process was started with.
- Source installs: typically `config.yaml` on the host.
- Docker: image copies `config.docker.yaml` → `/etc/giraffemail/config.yaml`; do **not** rename `config.docker.yaml` on the host.

### Password widget (`frontend/src/widgets/gmChangePassword/`)

- `"use client"` leaf widget (FSD: page → widget).
- Fields: current password, new password, confirm new password.
- Client validation: new password length ≥ 8; confirm matches.
- Submit: `POST /api/v1/auth/change_password` with `{ current_password, new_password }` via `GmHttpService`.
- Success: clear `gm_token`, redirect to `/login` (backend clears session cookie and revokes tokens).
- Errors: wrong current password and other API failures shown inline or via toast; stay on Settings.
- Styling: existing `gm-card`, `gm-input`, `gm-btn-primary` (match login / add-account).

Call `GmHttpService` directly from the widget (same pattern as `gmLogin`). Do not add a new auth entity layer for this change.

## Backend — seed

### Current behavior (broken)

`runSeed` always `INSERT ... ON CONFLICT(email) DO UPDATE` for `admin@localhost`, resetting `password_hash` to bcrypt(`admin123`). Production only logs a warning when users already exist, then still overwrites.

Docker `scripts/docker-entrypoint.sh`:

```sh
giraffemail migrate --config "$CONFIG"
giraffemail seed --config "$CONFIG"
exec giraffemail serve --config "$CONFIG" "$@"
```

### Target behavior

| Invocation | If `admin@localhost` missing | If present |
|------------|------------------------------|------------|
| `seed` | Create with `admin123` | No-op (log, exit 0) |
| `seed --force` | Create with `admin123` | Reset password to `admin123` (warn in log) |

- Add `--force` flag on the `seed` cobra command.
- Entrypoint unchanged (no `--force`).
- Document recovery: `docker compose exec … giraffemail seed --force --config /etc/giraffemail/config.yaml`.

## Docs & naming

### Docs updates

- `docs/INSTALLATION.md`, `docs/DEPLOYMENT.md` (and any other first-open URLs): use `http://localhost:9191/login`.
- Clarify Docker config path vs host `config.docker.yaml`.
- Document `seed --force` for lockout recovery.
- Ensure “change password in Settings” remains accurate after the UI lands.
- `CHANGELOG.md`: user-visible entry for password Settings UI + seed no longer resetting passwords on restart.

### Naming policy

- User-facing prose: **GiraffeMail** / **GiraffeMail Archive**.
- Technical identifiers unchanged: repo `giraffemailer`, module path, CLI `giraffemail`, volume/service names.
- README may note once that the repository is named `giraffemailer`.

## Error handling

| Case | Behavior |
|------|----------|
| New password shorter than 8 chars | Client-side; no request |
| Confirm mismatch | Client-side; no request |
| Wrong current password | API 401 message; stay on Settings |
| Success | Tokens revoked; clear client token; `/login` |
| Network / unexpected | Show error; stay on Settings |

## Testing

- Go: cover seed skip vs `--force` (extract helper if needed for testability).
- Manual or existing API tests for `change_password` remain valid; no API change expected.
- Frontend: smoke that form posts correct JSON and redirects on success (lightweight; match project norms).

## Success criteria

1. Logged-in user can change password from Settings and must re-login with the new password.
2. After Docker stop/start (without `--force` seed), new password still works.
3. Fresh install still gets `admin@localhost` / `admin123` via entrypoint seed.
4. Docs say `/login`; Settings does not tell Docker users to rename host config files.
5. User-facing docs/UI use GiraffeMail naming consistently.

## Implementation order (for planning)

1. Seed idempotency + `--force` + tests  
2. Settings password widget + page copy  
3. Docs + CHANGELOG + naming pass  
