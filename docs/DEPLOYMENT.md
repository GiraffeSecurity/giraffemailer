# Deployment Guide

For first-time installation (Docker, source build, seed user, first IMAP account), start with **[INSTALLATION.md](./INSTALLATION.md)**.

This guide covers **production** deployment: TLS, process management, backups, and monitoring.

---

## Requirements

| Resource | Minimum | Recommended (20GB mail) |
|----------|---------|-------------------------|
| CPU | 1 core | 2+ cores |
| RAM | 512 MB | 2–4 GB |
| Disk | 2× mail size | 2.5× mail size (blobs + DB + FTS) |
| OS | Linux, macOS, Windows | Linux x86_64 |

Build from source or use the official Docker image (see below).

---

## Docker (recommended)

```bash
# Generate a production secret key
export GM_SECRET_KEY=$(openssl rand -hex 32)

# Build and start
docker compose up -d --build

# First-time setup (run once inside container)
docker compose exec giraffemail giraffemail migrate --config /etc/giraffemail/config.yaml
docker compose exec giraffemail giraffemail seed --config /etc/giraffemail/config.yaml
```

Open **http://localhost:9191/gm** — default seed: `admin@localhost` / `admin123` (change immediately).

Data persists in the `giraffemail-data` Docker volume at `/data` inside the container.

Environment overrides:

| Variable | Purpose |
|----------|---------|
| `GM_SECRET_KEY` | 64 hex chars — **required** for Docker production |
| `GM_DATA_DIR` | Override `storage.data_dir` |
| `GM_ENV` | `dev` or `production` |

Production `config.docker.yaml` enforces: `encrypt_blobs: true`, `allow_registration: false`.

---

## Quick production deploy

```bash
# 1. Build
make build-full

# 2. Configure
cp config.example.yaml /etc/giraffemail/config.yaml
# Edit: storage.data_dir, app.port, app.secret_key

# 3. Initialize
./giraffemail migrate --config /etc/giraffemail/config.yaml
./giraffemail seed --config /etc/giraffemail/config.yaml   # first user only

# 4. Run
./giraffemail serve --config /etc/giraffemail/config.yaml
```

Default UI: `http://localhost:9191/gm`  
Change the seed password immediately after first login.

---

## Configuration checklist

- [ ] Set a strong `app.secret_key` (64 hex chars: `openssl rand -hex 32`) before adding accounts
- [ ] Set `storage.encrypt_blobs: true` and `app.env: production`
- [ ] Set `security.allow_registration: false` unless you need public signup
- [ ] Set `security.cookie_secure: true` when serving over HTTPS
- [ ] Place `storage.data_dir` on durable storage (local SSD or reliable NFS — not ephemeral container FS without volumes)
- [ ] Restrict network: bind to localhost or put behind reverse proxy with TLS
- [ ] Firewall: do not expose IMAP credentials path; only expose HTTPS to users
- [ ] Back up `data_dir` regularly (DB + `blobs/`)

---

## Reverse proxy (nginx example)

```nginx
server {
    listen 443 ssl;
    server_name mail-archive.example.com;

    ssl_certificate     /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9191;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE sync progress
        proxy_buffering off;
        proxy_read_timeout 3600s;
    }
}
```

---

## systemd unit

```ini
[Unit]
Description=GiraffeMail Archive
After=network.target

[Service]
Type=simple
User=giraffemail
ExecStart=/usr/local/bin/giraffemail serve --config /etc/giraffemail/config.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/giraffemail

[Install]
WantedBy=multi-user.target
```

---

## Backup & restore

### What to back up

```
<data_dir>/
├── giraffemail.db      # metadata, FTS index, credentials (encrypted)
└── blobs/              # archived message bodies
```

### Backup command

```bash
# Stop server or use snapshot-friendly FS
tar -czf giraffemail-backup-$(date +%F).tar.gz -C /var/lib/giraffemail .
```

### Restore

1. Stop `giraffemail serve`
2. Extract tarball into `data_dir`
3. Run `giraffemail fsck --config ...` to verify blobs
4. Start server

---

## Disk sizing

| Mail on server | Blob store (zstd) | SQLite + FTS | Suggested free space |
|----------------|-------------------|--------------|----------------------|
| 5 GB | ~3–4 GB | ~200 MB | 10 GB |
| 20 GB | ~12–16 GB | ~0.5–2 GB | 40 GB |
| 100 GB | ~60–80 GB | ~2–5 GB | 200 GB |

Blobs are content-addressed; deduplication across mailboxes is automatic for identical bodies.

---

## Health & monitoring

- **Liveness:** `GET /healthz` → `200 OK`
- **Readiness:** `GET /readyz` → JSON `{ "status": "ready" }` (DB ping)
- **Logs:** JSON via zerolog (stdout)
- **Integrity:** cron `giraffemail fsck` weekly
- **Sync failures:** check account test endpoint and server logs

Upgrades: see [UPGRADING.md](./UPGRADING.md).

---

## Security notes

- IMAP passwords stored encrypted when `app.secret_key` is set
- Never log passwords, tokens, OTP codes, or encryption keys
- Auth endpoints rate-limited (5/min/IP)
- HTML bodies sanitized with bluemonday before API response
- AGPL §13: if you offer this as a network service to third parties, provide corresponding source — see [LICENSING.md](./LICENSING.md)

---

## Development

```bash
# Terminal 1 — API
go run ./cmd/giraffemail serve --config config.yaml

# Terminal 2 — UI hot reload
cd frontend && pnpm install && pnpm dev
```

Frontend dev server proxies API to `http://localhost:9191` via `shared/config/env.ts`.
