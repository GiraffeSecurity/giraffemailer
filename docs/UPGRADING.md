# Upgrading GiraffeMail Archive

Safe upgrade procedure for Docker and bare-metal installs.

---

## General rules

1. **Back up** `storage.data_dir` (SQLite database + `blobs/` directory) before every upgrade.
2. **Stop** the running server (or scale Docker to 0) before replacing the binary.
3. **Run migrations** explicitly: `./giraffemail migrate --config <path>`
4. **Verify** with `./giraffemail fsck --config <path>` after major upgrades.
5. Read [CHANGELOG.md](../CHANGELOG.md) for breaking changes.

Migrations also run on `serve` startup, but explicit `migrate` before restart avoids downtime surprises.

---

## Docker upgrade

```bash
cd giraffemail
git pull   # or checkout the release tag

docker compose build --no-cache
docker compose up -d

docker compose exec giraffemail giraffemail migrate --config /etc/giraffemail/config.yaml
docker compose exec giraffemail giraffemail fsck --config /etc/giraffemail/config.yaml
```

The `giraffemail-data` volume persists across image rebuilds.

---

## Bare-metal upgrade

```bash
# 1. Backup
tar -czf giraffemail-backup-$(date +%F).tar.gz -C /var/lib/giraffemail .

# 2. Stop service
sudo systemctl stop giraffemail

# 3. Build or copy new binary
make build-full
sudo cp giraffemail /usr/local/bin/giraffemail

# 4. Migrate
giraffemail migrate --config /etc/giraffemail/config.yaml

# 5. Start
sudo systemctl start giraffemail

# 6. Verify
giraffemail fsck --config /etc/giraffemail/config.yaml
curl -s http://127.0.0.1:9191/readyz
```

---

## Migration history

| Migration | Change |
|-----------|--------|
| `000001_initial_schema` | Core schema, FTS5, auth tables |
| `000002_*` | Incremental schema updates (see `internal/db/migrations/`) |
| `000003_rbac` | `mail_accounts.owner_id` — existing accounts assigned to first admin |

After the RBAC migration, legacy accounts without an owner are visible to admins only. Reassign ownership via direct DB update or recreate accounts as the intended user.

---

## Rolling back

GiraffeMail does not ship automatic down-migrations in production. To roll back:

1. Stop the server
2. Restore the pre-upgrade backup tarball into `data_dir`
3. Run the previous binary version

Keep at least one backup before each upgrade.
