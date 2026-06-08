# Contributing to GiraffeMail Archive

Thank you for your interest in contributing. GiraffeMail is licensed under **AGPL-3.0** — by submitting a pull request, you agree that your contributions will be licensed under the same terms.

Please read the [Code of Conduct](../CODE_OF_CONDUCT.md) before participating.

---

## Ways to contribute

- **Bug reports** — use the [bug report template](../.github/ISSUE_TEMPLATE/bug_report.yml)
- **Feature ideas** — use the [feature request template](../.github/ISSUE_TEMPLATE/feature_request.yml)
- **Documentation** — fixes and improvements to `docs/` and `README.md` are always welcome
- **Code** — follow the workflow below
- **Security** — see [SECURITY.md](../SECURITY.md); do **not** open public issues for vulnerabilities

For large features, open an issue first to discuss scope and avoid duplicate work.

---

## Development setup

```bash
git clone https://github.com/GiraffeSecurity/giraffemailer.git
cd giraffemailer

cp config.example.yaml config.yaml
go run ./cmd/giraffemail migrate --config config.yaml
go run ./cmd/giraffemail seed --config config.yaml
```

```bash
# Terminal 1 — API
go run ./cmd/giraffemail serve --config config.yaml

# Terminal 2 — UI (hot reload)
cd frontend && pnpm install && pnpm dev
```

Full install details: [docs/INSTALLATION.md](docs/INSTALLATION.md)

---

## Before submitting a PR

1. `go test ./... -race`
2. `go build ./cmd/giraffemail`
3. `cd frontend && pnpm test run && pnpm build` (if frontend changed)
4. Update [CHANGELOG.md](CHANGELOG.md) for user-visible changes
5. Follow patterns in [CLAUDE.md](CLAUDE.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

CI runs the same checks on every pull request (see [`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

---

## Architecture rules (non-negotiable)

| Rule | Detail |
|------|--------|
| **Golden rule** | Never delete from IMAP without verified local archive (`archived_at` + `blob_sha256`) |
| **Handlers** | Thin HTTP layer in `internal/api/` — no SQL, no IMAP |
| **Business logic** | `internal/service/` |
| **SQL** | `internal/repository/sqlite/` only; parameterized queries |
| **Secrets** | Never commit `config.yaml`, `.env`, or real credentials |

---

## Code style

**Go**

- Match existing package layout (`domain` → `port` → `repository` → `service` → `api`)
- Pure Go SQLite (`CGO_ENABLED=0`)
- Comments only when the *why* is non-obvious
- No TODO/FIXME in committed code

**TypeScript / Frontend**

- FSD layer order: `app → widgets → entities → shared`
- Dark Giraffe theme (`#0E0E10` bg, `#C9A227` accent)
- HTTP via `shared/api/gmHttpService.ts` only
- `"use client"` only on leaf interactive components

---

## Pull request process

1. Fork and create a branch from `main` (e.g. `fix/search-pagination`, `feat/admin-ui`)
2. Keep PRs focused — one logical change per PR when possible
3. Fill out the [PR template](../.github/PULL_REQUEST_TEMPLATE.md)
4. Maintainers review for correctness, security, and golden-rule compliance
5. Squash or merge at maintainer discretion

---

## Commit messages

Use clear, imperative subject lines:

```
fix: return 403 on cross-account message access
docs: add Docker install troubleshooting
feat: admin user list endpoint
```

---

## Release process (maintainers)

1. Update [CHANGELOG.md](CHANGELOG.md) with release date and version
2. Tag: `git tag v0.x.y`
3. `make build-full` and attach artifacts to GitHub Release
4. Publish Docker image (when registry is configured)

---

## Questions?

Open a [GitHub Discussion](https://github.com/GiraffeSecurity/giraffemailer/discussions) or an issue labeled `question`.

Commercial licensing inquiries: contact [Giraffe.ge](https://giraffe.ge).
