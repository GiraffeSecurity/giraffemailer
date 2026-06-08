package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps a SQLite connection with the migration runner.
type DB struct {
	Conn *sql.DB
}

// Open opens the SQLite database at path, applies WAL + FK pragmas, and runs
// any pending migrations. The caller owns the returned DB and must call Close.
func Open(ctx context.Context, path string) (*DB, error) {
	// modernc.org/sqlite driver name is "sqlite"
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// WAL mode allows concurrent readers alongside one writer.
	// Writes serialize via busy_timeout (5 s). Keep idle connections low.
	conn.SetMaxOpenConns(16)
	conn.SetMaxIdleConns(4)

	if err := applyPragmas(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}

	db := &DB{Conn: conn}
	if err := db.migrate(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Close closes the underlying connection.
func (db *DB) Close() error {
	return db.Conn.Close()
}

func applyPragmas(ctx context.Context, conn *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA cache_size = -32000", // 32 MB page cache
		"PRAGMA temp_store = MEMORY",
	}
	for _, p := range pragmas {
		if _, err := conn.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}

// migrate runs all *.sql files in the embedded migrations directory that have
// not yet been recorded in schema_migrations.
func (db *DB) migrate(ctx context.Context) error {
	if _, err := db.Conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER  PRIMARY KEY,
			filename   TEXT     NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort by filename to guarantee ascending order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}

		var version int
		if _, err := fmt.Sscanf(name, "%d", &version); err != nil {
			log.Warn().Str("file", name).Msg("skipping migration with non-numeric prefix")
			continue
		}

		var count int
		if err := db.Conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version,
		).Scan(&count); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if count > 0 {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.Conn.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := db.Conn.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, filename) VALUES (?, ?)`, version, name,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}

		log.Info().Int("version", version).Str("file", name).Msg("applied migration")
	}

	return nil
}
