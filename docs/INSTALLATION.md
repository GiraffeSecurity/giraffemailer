# Installation Guide

Step-by-step instructions to install GiraffeMail Archive on a fresh machine.

---

## Before you start

### What you need

| Requirement | Details |
|-------------|---------|
| **Mail access** | IMAP credentials (app password recommended for Gmail, Outlook, etc.) |
| **Disk** | ~**1.5–2×** the mail volume you plan to archive ([sizing table](./DEPLOYMENT.md#disk-sizing)) |
| **Network** | Outbound IMAP (typically port 993/TLS) from the host running GiraffeMail |

### Choose an install method

| Method | Best for | Time |
|--------|----------|------|
| [Docker](#docker-recommended) | Production, homelab, easiest upgrades | ~5 min |
| [Build from source](#build-from-source) | Development, custom builds, air-gapped | ~10 min |
| [Pre-built binary](#pre-built-binaries) | Linux/macOS/Windows without Docker | ~5 min |

---

## Docker (recommended)

### 1. Clone and configure

```bash
git clone https://github.com/GiraffeSecurity/giraffemailer.git
cd giraffemailer

cp .env.example .env
# Generate a 32-byte secret (64 hex chars) — required for Docker
echo "GM_SECRET_KEY=$(openssl rand -hex 32)" >> .env
```

### 2. Start the container

```bash
docker compose up -d --build
```

Data is stored in the `giraffemail-data` Docker volume (`/data` inside the container).

### 3. Initialize the database (first run only)

```bash
docker compose exec giraffemail giraffemail migrate --config /etc/giraffemail/config.yaml
docker compose exec giraffemail giraffemail seed --config /etc/giraffemail/config.yaml
```

### 4. Open the UI

Browse to **http://localhost:9191/gm**

| Field | Default (seed) |
|-------|----------------|
| Email | `admin@localhost` |
| Password | `admin123` |

**Change this password immediately** after first login (Settings → change password).

### 5. Add your first mail account

1. Go to **Accounts** → **Add account**
2. Enter IMAP host, port (993), username, and app password
3. Click **Test connection**, then **Save**
4. Click **Sync** to start archiving

Sync progress appears as a live bar on the account row (SSE).

### Docker environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GM_SECRET_KEY` | Yes | 64 hex characters — AES-256 master key |
| `GM_DATA_DIR` | No | Override blob/DB path (default `/data`) |
| `GM_ENV` | No | `dev` or `production` |

See [`config.docker.yaml`](../config.docker.yaml) for the baked-in production defaults (`encrypt_blobs: true`, `allow_registration: false`).

---

## Build from source

### Prerequisites

- **Go** 1.22+ (see [`go.mod`](../go.mod) for exact version)
- **Node.js** 20+ and **pnpm** (UI build only)
- **Git**

On Debian/Ubuntu:

```bash
sudo apt update
sudo apt install -y git golang nodejs npm
npm install -g pnpm
```

On macOS (Homebrew):

```bash
brew install go node pnpm
```

### 1. Clone and configure

```bash
git clone https://github.com/GiraffeSecurity/giraffemailer.git
cd giraffemailer

cp config.example.yaml config.yaml
```

For local development, leave `app.env: dev`. For production on bare metal, set:

```yaml
app:
  env: production
  secret_key: "<64 hex from: openssl rand -hex 32>"

security:
  allow_registration: false
  cookie_secure: true   # when behind HTTPS

storage:
  encrypt_blobs: true
  data_dir: /var/lib/giraffemail
```

### 2. Build the binary (UI embedded)

```bash
make build-full
```

This runs `pnpm build` for the Next.js UI and compiles a single `giraffemail` binary with the UI embedded.

Backend-only build (no UI):

```bash
make build
```

### 3. Initialize and run

```bash
./giraffemail migrate --config config.yaml
./giraffemail seed --config config.yaml
./giraffemail serve --config config.yaml
```

Open **http://localhost:9191/gm** and log in with `admin@localhost` / `admin123`.

### Development mode (hot-reload UI)

Run the API and frontend separately:

```bash
# Terminal 1 — API
go run ./cmd/giraffemail serve --config config.yaml

# Terminal 2 — UI (proxies API to :9191)
cd frontend && pnpm install && pnpm dev
```

Open **http://localhost:3000/gm**.

---

## Pre-built binaries

Cross-compile release artifacts:

```bash
make release
```

This produces:

| File | Platform |
|------|----------|
| `giraffemail-linux-amd64` | Linux x86_64 |
| `giraffemail-darwin-arm64` | macOS Apple Silicon |
| `giraffemail-darwin-amd64` | macOS Intel |
| `giraffemail-windows-amd64.exe` | Windows x86_64 |

**Note:** Release binaries from `make release` do **not** include the embedded UI. For a full single-binary experience, use `make build-full` or Docker.

After copying a binary to your server:

```bash
chmod +x giraffemail-linux-amd64
./giraffemail-linux-amd64 migrate --config /etc/giraffemail/config.yaml
./giraffemail-linux-amd64 seed --config /etc/giraffemail/config.yaml
./giraffemail-linux-amd64 serve --config /etc/giraffemail/config.yaml
```

Official GitHub Release artifacts (when published) include the UI-embedded build.

---

## Post-install checklist

- [ ] Change the default admin password
- [ ] Set `app.secret_key` (production) — never commit it
- [ ] Set `storage.encrypt_blobs: true` in production
- [ ] Set `security.allow_registration: false` unless you want public signup
- [ ] Put the server behind HTTPS (nginx/Caddy) — see [DEPLOYMENT.md](./DEPLOYMENT.md)
- [ ] Schedule backups of `storage.data_dir` (SQLite + `blobs/`)
- [ ] Run `giraffemail fsck` weekly via cron to verify blob integrity

---

## Verify the installation

```bash
# Liveness
curl -s http://localhost:9191/healthz

# Readiness (DB ping)
curl -s http://localhost:9191/readyz

# Blob integrity (after first sync)
./giraffemail fsck --config config.yaml
```

---

## Upgrading

See [UPGRADING.md](./UPGRADING.md) for version-to-version steps. In short:

1. Back up `storage.data_dir`
2. Stop the server
3. Replace the binary / rebuild the Docker image
4. Run `./giraffemail migrate --config ...`
5. Start the server

Migrations run automatically on `serve` startup as well, but running `migrate` explicitly before restart is recommended in production.

---

## Troubleshooting

### `migrate` fails with "database is locked"

Another process holds the SQLite file. Stop all `giraffemail serve` instances and retry.

### IMAP test connection fails

| Symptom | Fix |
|---------|-----|
| Gmail "Less secure apps" | Use an [App Password](https://support.google.com/accounts/answer/185833) |
| Wrong port | Default IMAP TLS port is **993** |
| Self-signed cert | Ensure `use_tls: true`; some servers need the correct hostname |
| Firewall | Allow outbound TCP 993 from the GiraffeMail host |

### UI shows blank page (binary without UI)

You built with `make build` instead of `make build-full`. Rebuild with `make build-full` or use Docker.

### Docker: `GM_SECRET_KEY` error on start

Set a 64-character hex key in `.env`:

```bash
echo "GM_SECRET_KEY=$(openssl rand -hex 32)" > .env
docker compose up -d
```

### Search returns no results

Messages are indexed during archive sync. Run **Sync** on the account and wait for Phase 1 (index) to complete before searching.

### Login fails or returns to sign-in immediately

| Symptom | Fix |
|---------|-----|
| Invalid credentials | Run `giraffemail seed --config config.yaml` (works on empty DB even in production/Docker) |
| Credentials | `admin@localhost` / `admin123` after seed |
| UI says "Run make build-ui" | Run `make build-full` before `serve` |
| Dev mode (`pnpm dev` on :3000) | API is proxied via Next.js — restart `pnpm dev` after pulling updates |
| Embedded UI at `:9191` | Open the same host you configured (don't mix `localhost` and `127.0.0.1`) |

### 403 Forbidden on accounts

RBAC is enabled: non-admin users only see accounts they created. Log in as admin or create accounts while logged in as the intended owner.

---

## Uninstall

```bash
# Docker
docker compose down
docker volume rm giraffemail_giraffemail-data   # destroys all archived mail

# Bare metal
rm -f ./giraffemail
rm -rf ./data   # or your configured storage.data_dir
```

Always back up `data_dir` before removing volumes or directories.

---

## Next steps

- [DEPLOYMENT.md](./DEPLOYMENT.md) — production, nginx, systemd, backups
- [API.md](./API.md) — REST API reference
- [ARCHITECTURE.md](./ARCHITECTURE.md) — how archiving and the golden rule work
- [SECURITY.md](../SECURITY.md) — vulnerability reporting
