# HTTP API Reference

Base URL: `http://<host>:<port>` (default port `9191`)

All JSON endpoints return an envelope:

```json
{ "status": "success", "data": { ... } }
```

```json
{ "status": "fail", "message": "human-readable error" }
```

Authenticated routes require:

```
Authorization: Bearer <token>
```

Or an HttpOnly `gm_session` cookie set at login.

---

## Authorization (RBAC)

Each user has a role: `admin` or `user` (returned by `GET /api/v1/auth/check_user`).

| Role | Access |
|------|--------|
| `admin` | All mail accounts, users, cleanup jobs, search, export |
| `user` | Only mail accounts where `owner_id` matches their user ID |

Rules:

- New accounts created via API are owned by the creating user.
- Legacy accounts without `owner_id` are visible to admins only (migration backfills existing rows to the first admin).
- Non-admin users receive **403 Forbidden** when accessing another user's account or messages.
- Admins manage users via `/api/v1/admin/*` (see below).

---

## Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | No | Register user, send OTP |
| POST | `/api/v1/auth/otp_registration` | No | Verify registration OTP |
| POST | `/api/v1/auth/login` | No | Login → bearer token (30-day TTL) |
| POST | `/api/v1/auth/forgot_password` | No | Request password-reset OTP |
| POST | `/api/v1/auth/otp_forgot_password` | No | Reset password with OTP |
| POST | `/api/v1/auth/change_password` | Yes | Change password |
| POST | `/api/v1/auth/logout` | Yes | Revoke current token |
| GET | `/api/v1/auth/check_user` | Yes | Current user profile |

Rate limit: **5 requests/minute per IP** on auth routes.

---

## Mail accounts

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts` | List IMAP accounts |
| POST | `/api/v1/accounts` | Create account (credentials encrypted at rest) |
| GET | `/api/v1/accounts/{id}` | Get account |
| DELETE | `/api/v1/accounts/{id}` | Delete account |
| POST | `/api/v1/accounts/{id}/test` | Test IMAP connection |
| POST | `/api/v1/accounts/{id}/sync` | Trigger archive sync (background) |
| GET | `/api/v1/accounts/{accountId}/progress` | SSE sync progress stream |

### Create account body

```json
{
  "name": "Work Mail",
  "email_address": "user@company.com",
  "imap_host": "imap.company.com",
  "imap_port": 993,
  "use_tls": true,
  "username": "user@company.com",
  "password": "app-password"
}
```

---

## Mailboxes & messages

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts/{accountId}/mailboxes` | List mailboxes + archive stats |
| GET | `/api/v1/messages` | Cross-account message list (cursor pagination) |
| GET | `/api/v1/accounts/{accountId}/mailboxes/{mailboxId}/messages` | Mailbox-scoped list |
| GET | `/api/v1/messages/{id}` | Full message (body, attachments metadata) |
| GET | `/api/v1/messages/{id}/attachments/{partPath}` | Download attachment bytes |

### List query parameters

| Param | Description |
|-------|-------------|
| `cursor` | Keyset pagination token from previous response |
| `limit` | Page size (1–200, default 50) |
| `account_id` | Filter by account |
| `mailbox_id` | Filter by mailbox |
| `sender` | Substring match on sender email |
| `archive_state` | `archived`, `not_archived`, `deleted_from_server` |
| `has_attachments` | `true` |
| `sort` | `size` (default: date) |
| `dir` | `asc` (default: desc) |

### Message list response

```json
{
  "status": "success",
  "data": {
    "messages": [ ... ],
    "next_cursor": "eyJkIjoi...",
    "has_more": true,
    "limit": 50
  }
}
```

---

## Search & insights

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/search` | Full-text search (SQLite FTS5) |
| GET | `/api/v1/insights` | Dashboard KPIs, top senders, size by year |

### Search query parameters

| Param | Description |
|-------|-------------|
| `q` | Search terms (required) |
| `cursor` | Keyset pagination token from previous `next_cursor` |
| `limit` | Page size (1–200, default 50) |
| `account_id` | Filter by account |
| `sender` | Sender filter |
| `has_attachments` | `true` |
| `archive_state` | `archived`, `deleted_from_server` |
| `min_size` | Minimum size in bytes |

### Search response

```json
{
  "status": "success",
  "data": {
    "messages": [ ... ],
    "total": 1234,
    "next_cursor": "eyJkIjoi...",
    "has_more": true,
    "limit": 50
  }
}
```

`total` is returned on the first page only (no `cursor`). Subsequent pages use `cursor` + `next_cursor`.

---

## Cleanup

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/cleanup/preview` | Count/sum candidates matching filter |
| GET | `/api/v1/cleanup/jobs` | List cleanup jobs |
| POST | `/api/v1/cleanup/jobs` | Create job |
| PUT | `/api/v1/cleanup/jobs/{id}` | Update job |
| DELETE | `/api/v1/cleanup/jobs/{id}` | Delete job |
| POST | `/api/v1/cleanup/jobs/{id}/run` | Execute job (background) |
| GET | `/api/v1/cleanup/jobs/{id}/runs` | Run history |

Only messages with **verified local archive** (`archived_at` + `blob_sha256`) are eligible for server deletion. See [Golden Rule](../README.md#the-golden-rule).

### Cleanup filter fields

```json
{
  "account_id": "uuid",
  "mailbox_name": "INBOX",
  "sender_domain": "newsletter.com",
  "sender_email": "spam@example.com",
  "older_than_days": 365,
  "larger_than_kb": 512,
  "has_attachments": true,
  "flag_not_seen": false,
  "subject_contains": "invoice"
}
```

---

## Export & restore

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/export` | Export messages as mbox or zip (max 1000 IDs) |
| POST | `/api/v1/restore/{id}` | APPEND archived message back to IMAP server |

---

## Admin (admin role required)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/users` | List all users |
| PATCH | `/api/v1/admin/users/{id}` | Update role and/or `is_active` |

### Update user body

```json
{
  "role": "user",
  "is_active": true
}
```

At least one of `role` or `is_active` is required. An admin cannot demote or deactivate their own account.

---

## Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness probe (200 OK) |

---

## Web UI

All non-API routes serve the embedded static UI (`/gm`, `/login`, etc.).
