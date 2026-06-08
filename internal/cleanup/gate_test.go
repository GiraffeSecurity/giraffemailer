package cleanup

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── IsSafe unit tests ─────────────────────────────────────────────────────────

func TestIsSafe(t *testing.T) {
	now := time.Now()
	hash := "a3f1b2c9d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1"

	tests := []struct {
		name string
		c    Candidate
		want bool
	}{
		{
			name: "both archived_at and blob_sha256 set — safe to delete",
			c:    Candidate{ArchivedAt: &now, BlobSHA256: &hash},
			want: true,
		},
		{
			name: "archived_at nil — must not delete",
			c:    Candidate{ArchivedAt: nil, BlobSHA256: &hash},
			want: false,
		},
		{
			name: "blob_sha256 nil — must not delete (archive not verified)",
			c:    Candidate{ArchivedAt: &now, BlobSHA256: nil},
			want: false,
		},
		{
			name: "both nil — must not delete",
			c:    Candidate{ArchivedAt: nil, BlobSHA256: nil},
			want: false,
		},
		{
			name: "blob_sha256 empty string — must not delete (invariant: empty = unset)",
			c:    Candidate{ArchivedAt: &now, BlobSHA256: ptr("")},
			want: false,
		},
		{
			name: "zero time archived_at — treated as set (safety gate allows zero time)",
			c:    Candidate{ArchivedAt: &time.Time{}, BlobSHA256: &hash},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafe(tt.c); got != tt.want {
				t.Errorf("IsSafe() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── DB-level safety gate proof ────────────────────────────────────────────────
//
// This test proves that the SQL query anchored by SafetyCandidateSQL — which
// the cleanup engine must use for every candidate set — structurally prevents
// unarchived messages from ever being returned as deletion candidates,
// regardless of any other filter conditions the user may supply.

func TestSafetyGateSQL_ProveUnarchivableCannotBeSelected(t *testing.T) {
	db := openTestDB(t)

	// Insert messages covering every unsafe state.
	unsafe := []struct {
		id          string
		archivedAt  interface{}
		blobSHA256  interface{}
		deletedAt   interface{}
		description string
	}{
		{"msg-1", nil, nil, nil, "never indexed; nothing set"},
		{"msg-2", nil, "sha256abc", nil, "blob written but archived_at never set (verify failed)"},
		{"msg-3", "2026-01-01 10:00:00", nil, nil, "archived_at set but blob_sha256 NULL (write not finished)"},
		{"msg-4", "2026-01-01 10:00:00", "", nil, "archived_at set but blob_sha256 empty string"},
		{"msg-5", "2026-01-01 10:00:00", "sha256xyz", "2026-01-15 10:00:00", "already deleted from server"},
	}
	safe := []struct {
		id         string
		archivedAt string
		blobSHA256 string
	}{
		{"msg-safe-1", "2026-01-01 10:00:00", "sha256safeabc"},
		{"msg-safe-2", "2026-01-02 10:00:00", "sha256safedef"},
	}

	for _, m := range unsafe {
		_, err := db.Exec(`
			INSERT INTO messages(id, account_id, mailbox_id, uid, sender_email, archived_at, blob_sha256, deleted_from_server_at)
			VALUES (?, 'acct-1', 'mbox-1', 1, 'x@x.com', ?, ?, ?)`,
			m.id, m.archivedAt, m.blobSHA256, m.deletedAt,
		)
		if err != nil {
			t.Fatalf("insert unsafe %s: %v", m.id, err)
		}
	}
	for _, m := range safe {
		_, err := db.Exec(`
			INSERT INTO messages(id, account_id, mailbox_id, uid, sender_email, archived_at, blob_sha256)
			VALUES (?, 'acct-1', 'mbox-1', 2, 'x@x.com', ?, ?)`,
			m.id, m.archivedAt, m.blobSHA256,
		)
		if err != nil {
			t.Fatalf("insert safe %s: %v", m.id, err)
		}
	}

	// Run the safety gate query — this is exactly what the cleanup engine will
	// run before any delete/move operation.
	query := fmt.Sprintf(`SELECT id FROM messages WHERE %s`, SafetyCandidateSQL)
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("safety gate query: %v", err)
	}
	defer rows.Close()

	var selected []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		selected = append(selected, id)
	}

	// No unsafe message may appear in results.
	for _, m := range unsafe {
		for _, sel := range selected {
			if sel == m.id {
				t.Errorf("unsafe message %q (%s) was returned by safety gate query — this is a critical bug",
					m.id, m.description)
			}
		}
	}

	// All safe messages must appear.
	for _, m := range safe {
		found := false
		for _, sel := range selected {
			if sel == m.id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("safe message %q missing from safety gate results", m.id)
		}
	}

	if t.Failed() {
		t.Logf("selected IDs: %s", strings.Join(selected, ", "))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptr(s string) *string { return &s }

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.ExecContext(context.Background(), `
		PRAGMA foreign_keys = OFF;
		CREATE TABLE messages (
			id                     TEXT    PRIMARY KEY,
			account_id             TEXT    NOT NULL,
			mailbox_id             TEXT    NOT NULL,
			uid                    INTEGER NOT NULL,
			sender_email           TEXT    NOT NULL DEFAULT '',
			archived_at            DATETIME,
			blob_sha256            TEXT,
			deleted_from_server_at DATETIME,
			created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	return db
}
