# Architecture & Design Rationale

GiraffeMail Archive follows **hexagonal architecture** with SOLID principles. Dependencies flow inward: HTTP → services → ports → repositories.

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/giraffemail          CLI composition                   │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│  internal/app             Composition root (DIP wiring)       │
└────────────────────────────┬────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
  internal/api         internal/archive     internal/store
  (HTTP adapters)      (sync engine)        (blob adapter)
        │                    │                    │
        └────────────────────┼────────────────────┘
                             ▼
                    internal/service/*
                    (use cases / domain logic)
                             │
                             ▼
              internal/repository/sqlite/*
              (persistence — Repository pattern)
                             │
                             ▼
                    internal/port/* (interfaces)
                    internal/domain/* (types)
```

---

## SOLID mapping

| Principle | Application | Why |
|-----------|-------------|-----|
| **S** — Single Responsibility | `CleanupHandler` only maps HTTP ↔ `cleanup.Service`; SQL lives in `CleanupRepo` | Change auth without touching cleanup queries |
| **O** — Open/Closed | New export formats extend `export.Service` + repo, not handlers | Avoid breaking API contracts when adding mbox/zip |
| **L** — Liskov | `port.UserRepository` implemented by `sqlite.UserRepo`; tests can swap fakes | Substitutable persistence without handler changes |
| **I** — Interface Segregation | Separate repos: `MessageRepository`, `SearchRepository`, `CleanupRepository` | Services depend only on methods they use |
| **D** — Dependency Inversion | `app.New()` wires concrete repos into services; handlers receive services | No `sql.DB` in HTTP layer |

### Patterns used (only where coupling drops)

| Pattern | Location | Why |
|---------|----------|-----|
| **Repository** | `internal/repository/sqlite/*` | Isolate SQL from business rules |
| **Strategy** | Cleanup `delete` vs `move` in `Executor` | Same candidate pipeline, pluggable IMAP action |
| **Adapter** | `mail.Connector` wraps go-imap | Domain never imports IMAP types in handlers |
| **Factory** | `app.New()` composition root | Single place to inject config, DB, blob store |

---

## Layer responsibilities

| Layer | Path | Changes when… |
|-------|------|---------------|
| CLI | `cmd/giraffemail/` | New commands (fsck, migrate) |
| Composition | `internal/app/` | Wiring / startup order |
| HTTP | `internal/api/` | Routes, middleware, envelopes |
| Domain | `internal/domain/` | Business types & errors |
| Ports | `internal/port/` | Contract changes |
| Services | `internal/service/` | Rules (golden rule, auth, export caps) |
| Repositories | `internal/repository/sqlite/` | Schema / query optimization |
| Archive | `internal/archive/` | IMAP sync pipeline (Phase 9: move to `service/sync`) |
| Store | `internal/store/` | Blob layout / encryption |

---

## Security boundaries

```
Browser ──► middleware (CORS allowlist, CSP, body limit)
         ──► SessionAuth (HttpOnly cookie or Bearer)
         ──► Handler (input validation)
         ──► Service (authorization rules, rate limits)
         ──► Repository (parameterized SQL)
```

- **Authn**: bcrypt passwords, SHA-256 hashed tokens, OTP single-use
- **Session**: `gm_session` HttpOnly cookie; Bearer header for API clients
- **Authz**: single-tenant today — all authenticated users share resources (documented in SECURITY.md)
- **Input**: FTS query escaped in `internal/search`; cleanup filters parameterized
- **Output**: bluemonday + DOMPurify; generic 500 messages

---

## Performance hot paths

| Path | Before | After | Big-O |
|------|--------|-------|-------|
| Search | Load all FTS IDs → `IN (...)` | SQL `JOIN messages_fts … LIMIT` | O(matches) memory → **O(limit)** |
| Blob write | Global mutex | Per-path mutex | Parallel writes across hash paths |
| Sync | Overlapping runs per account | Per-account `sync.Mutex` | Prevents duplicate IMAP work |
| Message list | OFFSET (search only) | Keyset cursor (lists) | Deep pages O(1) vs O(offset) |

### Profiling guidance

```bash
# CPU profile during archive sync
go test -cpuprofile=cpu.prof -run=XXX ./internal/archive/...
go tool pprof cpu.prof

# SQLite EXPLAIN QUERY PLAN
sqlite3 data/giraffemail.db "EXPLAIN QUERY PLAN SELECT ..."
```

Benchmark target: search <150ms at 100k messages — run after schema + data seed.

---

## Golden rule (non-negotiable)

```sql
archived_at IS NOT NULL AND blob_sha256 IS NOT NULL AND blob_sha256 != ''
```

Enforced in `internal/cleanup/gate.go` + every cleanup SQL fragment. Cleanup executor double-checks before IMAP delete.

---

## Dependency flow (runtime)

```
serve → config.Load → db.Open → migrations
     → store.New → app.New(cfg, db, blobs, key, smtp)
     → api.NewRouter(deps) → http.ListenAndServe
```

See also [API.md](./API.md) and [DEPLOYMENT.md](./DEPLOYMENT.md).
