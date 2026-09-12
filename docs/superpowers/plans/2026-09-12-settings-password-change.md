# Settings Password Change + Seed Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix issue #1 so users can change the admin password in Settings and that change survives Docker restarts; align docs URL and naming.

**Architecture:** Extract testable `seedAdmin` helper with skip-unless-`--force`; add Settings client widget calling existing `POST /api/v1/auth/change_password`; update install/deploy docs and CHANGELOG. No new HTTP APIs.

**Tech Stack:** Go + cobra + modernc sqlite; Next.js 15 client widget + `GmHttpService`; Markdown docs.

## Global Constraints

- Product names (user-facing): **GiraffeMail** / **GiraffeMail Archive** only.
- Keep repo/module/binary names `giraffemailer` / `giraffemail` unchanged.
- Do not rename `config.docker.yaml` on the host; Docker uses `/etc/giraffemail/config.yaml` in-container.
- Password API unchanged: `current_password`, `new_password`; new password min length 8.
- Docker entrypoint must keep calling `seed` without `--force`.
- Commits only when the user explicitly asks (do not auto-commit).
- Work on a feature branch, not `main`.

## File structure

| File | Responsibility |
|------|----------------|
| `cmd/giraffemail/seed.go` | `seedAdmin(ctx, conn, force)` logic |
| `cmd/giraffemail/seed_test.go` | Skip vs force tests |
| `cmd/giraffemail/main.go` | Wire `--force`, call `seedAdmin` |
| `frontend/src/widgets/gmChangePassword/index.tsx` | Password form UI |
| `frontend/src/app/(gm)/gm/settings/page.tsx` | Compose widget + config note |
| `docs/INSTALLATION.md`, `docs/DEPLOYMENT.md`, `README.md` | `/login`, seed `--force`, Docker config |
| `CHANGELOG.md` | Unreleased fixes |

---

### Task 1: Idempotent seed + `--force`

**Files:**
- Create: `cmd/giraffemail/seed.go`
- Create: `cmd/giraffemail/seed_test.go`
- Modify: `cmd/giraffemail/main.go`

**Interfaces:**
- Produces: `func seedAdmin(ctx context.Context, conn *sql.DB, force bool) error`
- Behavior: if `admin@localhost` exists and `!force` → no-op; if missing → insert `admin123`; if `force` → upsert/reset hash to `admin123`

- [ ] **Step 1: Write failing tests**

Create `cmd/giraffemail/seed_test.go`:

```go
package main

import (
	"context"
	"database/sql"
	"testing"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func passwordHash(t *testing.T, db *sql.DB) string {
	t.Helper()
	var h string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE email = 'admin@localhost'`).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSeedAdminCreatesThenSkips(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := seedAdmin(ctx, db, false); err != nil {
		t.Fatal(err)
	}
	first := passwordHash(t, db)
	if err := bcrypt.CompareHashAndPassword([]byte(first), []byte("admin123")); err != nil {
		t.Fatalf("expected admin123: %v", err)
	}
	if err := seedAdmin(ctx, db, false); err != nil {
		t.Fatal(err)
	}
	if passwordHash(t, db) != first {
		t.Fatal("second seed without force must not change password hash")
	}
}

func TestSeedAdminForceResetsPassword(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := seedAdmin(ctx, db, false); err != nil {
		t.Fatal(err)
	}
	custom, err := bcrypt.GenerateFromPassword([]byte("changed-pass"), 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE users SET password_hash = ? WHERE email = 'admin@localhost'`, string(custom)); err != nil {
		t.Fatal(err)
	}
	if err := seedAdmin(ctx, db, true); err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash(t, db)), []byte("admin123")); err != nil {
		t.Fatalf("force should reset to admin123: %v", err)
	}
}
```

- [ ] **Step 2: Run tests — expect fail**

Run: `go test ./cmd/giraffemail/ -run SeedAdmin -count=1`

Expected: FAIL (`seedAdmin` undefined)

- [ ] **Step 3: Implement `seedAdmin` + wire CLI**

Create `cmd/giraffemail/seed.go`:

```go
package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const defaultAdminPassword = "admin123"
const defaultAdminEmail = "admin@localhost"

func seedAdmin(ctx context.Context, conn *sql.DB, force bool) error {
	var existingID string
	err := conn.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, defaultAdminEmail).Scan(&existingID)
	if err == nil && !force {
		log.Info().Str("email", defaultAdminEmail).Msg("seed: admin already exists — skipping (use --force to reset password)")
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("lookup admin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), 12)
	if err != nil {
		return err
	}

	if force && existingID != "" {
		_, err = conn.ExecContext(ctx, `
			UPDATE users SET password_hash = ?, role = 'admin', is_active = 1, updated_at = CURRENT_TIMESTAMP
			WHERE email = ?`, string(hash), defaultAdminEmail)
		if err != nil {
			return fmt.Errorf("force reset admin: %w", err)
		}
		log.Warn().Str("email", defaultAdminEmail).Msg("seed: --force reset admin password to default")
		return nil
	}

	adminID := uuid.Must(uuid.NewV7()).String()
	_, err = conn.ExecContext(ctx, `
		INSERT INTO users(id, email, password_hash, full_name, role, is_active)
		VALUES (?, ?, ?, 'Admin', 'admin', 1)`, adminID, defaultAdminEmail, string(hash))
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Info().Str("email", defaultAdminEmail).Str("password", defaultAdminPassword).Msg("seed: admin user ready")
	return nil
}
```

In `main.go`:

1. Add `var seedForce bool`
2. In `init()`: `seedCmd.Flags().BoolVar(&seedForce, "force", false, "reset admin@localhost password to default")`
3. Replace `runSeed` body after `openDB` with:

```go
	if err := seedAdmin(ctx, database.Conn, seedForce); err != nil {
		return err
	}
	return nil
```

Remove the production count/warn block and the inline INSERT.

Update `seedCmd.Short` to: `"Create admin user if missing (use --force to reset password)"`

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./cmd/giraffemail/ -run SeedAdmin -count=1`

Expected: PASS

- [ ] **Step 5: Verify entrypoint unchanged**

Confirm `scripts/docker-entrypoint.sh` still runs `giraffemail seed` without `--force`.

---

### Task 2: Settings change-password UI

**Files:**
- Create: `frontend/src/widgets/gmChangePassword/index.tsx`
- Modify: `frontend/src/app/(gm)/gm/settings/page.tsx`

**Interfaces:**
- Consumes: `GmHttpService.post('/api/v1/auth/change_password', { current_password, new_password })`
- Produces: `export function GmChangePassword()` client component

- [ ] **Step 1: Create widget**

`frontend/src/widgets/gmChangePassword/index.tsx`:

```tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Loader2 } from 'lucide-react'
import toast from 'react-hot-toast'
import GmHttpService from '@/shared/api/gmHttpService'

const api = new GmHttpService()

export function GmChangePassword() {
  const router = useRouter()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (newPassword.length < 8) {
      setError('New password must be at least 8 characters')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('New passwords do not match')
      return
    }
    setLoading(true)
    try {
      await api.post<{ message: string }>('/api/v1/auth/change_password', {
        current_password: currentPassword,
        new_password: newPassword,
      })
      api.clearToken()
      toast.success('Password changed — sign in again')
      router.push('/login')
      router.refresh()
    } catch (err: unknown) {
      setError((err as Error).message ?? 'Failed to change password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="gm-card space-y-3">
      <h2 className="text-[15px] font-semibold text-foreground">Change password</h2>
      {error && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[13px] text-red-400">
          {error}
        </div>
      )}
      <div className="space-y-1">
        <label className="text-[12px] text-muted-foreground">Current password</label>
        <input
          type="password"
          className="gm-input"
          value={currentPassword}
          onChange={e => setCurrentPassword(e.target.value)}
          required
          autoComplete="current-password"
        />
      </div>
      <div className="space-y-1">
        <label className="text-[12px] text-muted-foreground">New password</label>
        <input
          type="password"
          className="gm-input"
          value={newPassword}
          onChange={e => setNewPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </div>
      <div className="space-y-1">
        <label className="text-[12px] text-muted-foreground">Confirm new password</label>
        <input
          type="password"
          className="gm-input"
          value={confirmPassword}
          onChange={e => setConfirmPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </div>
      <button type="submit" disabled={loading} className="gm-btn-primary">
        {loading && <Loader2 size={13} className="animate-spin" />}
        Update password
      </button>
    </form>
  )
}
```

- [ ] **Step 2: Update Settings page**

Replace `frontend/src/app/(gm)/gm/settings/page.tsx` with:

```tsx
import { GmChangePassword } from '@/widgets/gmChangePassword'

export default function SettingsPage() {
  return (
    <div className="h-full overflow-auto p-8 lg:p-10 animate-slide-up">
      <div className="mx-auto max-w-2xl space-y-6">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-foreground">Settings</h1>
          <p className="mt-1 text-[13px] text-muted-foreground">
            Manage your account. Server options are set in the process config file.
          </p>
        </div>

        <GmChangePassword />

        <div className="gm-card">
          <h2 className="text-[15px] font-semibold text-foreground">Server configuration</h2>
          <p className="mt-2 text-[13px] text-muted-foreground">
            Edit the config file and restart GiraffeMail to change server settings.
            Source installs typically use <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">config.yaml</code>.
            Docker builds copy <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">config.docker.yaml</code> to{' '}
            <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">/etc/giraffemail/config.yaml</code> inside the image — do not rename the Docker source file on the host.
          </p>
          <p className="mt-4 mb-2 text-[12px] text-muted-foreground">Key fields</p>
          <pre className="overflow-x-auto text-[12px] leading-relaxed text-foreground/80">{`app:
  env: dev | production
  port: 9191
  secret_key: <64-char hex>

storage:
  data_dir: ./data
  encrypt_blobs: false

archive:
  worker_count: 4
  batch_size_bytes: 8388608

smtp:
  host: ""
  port: 587
  username: ""
  password: ""
  from: ""
`}</pre>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Typecheck / lint if available**

Run from `frontend/`: `pnpm exec tsc --noEmit` (or project’s usual check). Fix any errors.

---

### Task 3: Docs + CHANGELOG + naming

**Files:**
- Modify: `docs/INSTALLATION.md`, `docs/DEPLOYMENT.md`, `README.md`, `CHANGELOG.md`

- [ ] **Step 1: Fix first-open URLs to `/login`**

Replace `http://localhost:9191/gm` with `http://localhost:9191/login` wherever it is the first-browse / open-UI instruction (INSTALLATION, DEPLOYMENT, README). Keep `/gm` when referring to the app after login if needed.

- [ ] **Step 2: Document seed `--force` + Docker config**

In INSTALLATION troubleshooting / credentials section, note:

- Re-running `seed` without `--force` does not reset an existing admin password.
- Lockout recovery: `giraffemail seed --force --config …`
- Docker: do not rename `config.docker.yaml` on the host.

- [ ] **Step 3: CHANGELOG**

Under `## [Unreleased]` add:

```markdown
### Fixed

- Settings: change-password form (issue #1)
- `seed` no longer resets `admin@localhost` on every run; use `--force` to reset (Docker restarts preserve passwords)
- Docs: first-open URL `/login`; clarify Docker config paths
```

- [ ] **Step 4: Naming pass**

Ensure user-facing install/README prose uses GiraffeMail / GiraffeMail Archive. Leave `giraffemailer` in clone URLs and module paths.

---

### Task 4: Verification

- [ ] **Step 1:** `go test ./cmd/giraffemail/ -count=1`
- [ ] **Step 2:** `go test ./... -count=1` (or project CI subset if too slow)
- [ ] **Step 3:** Confirm `scripts/docker-entrypoint.sh` has no `--force`
- [ ] **Step 4:** Summarize for human review; commit only if asked

## Spec coverage

| Spec requirement | Task |
|------------------|------|
| Password UI in Settings | 2 |
| Seed skip / `--force` | 1 |
| Docker persistence | 1 (entrypoint unchanged) |
| Docs `/login` | 3 |
| Docker config clarification | 2 + 3 |
| Naming | 3 |
| CHANGELOG | 3 |
| Tests for seed | 1 |
