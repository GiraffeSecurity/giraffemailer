package cleanup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	cleanupengine "github.com/GiraffeSecurity/giraffemailer/internal/cleanup"
	_ "modernc.org/sqlite"
)

func setupCleanupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE mail_accounts (id TEXT PRIMARY KEY, name TEXT);
		CREATE TABLE mailboxes (id TEXT PRIMARY KEY, account_id TEXT, name TEXT);
		CREATE TABLE messages (
			id                    TEXT PRIMARY KEY,
			account_id            TEXT NOT NULL,
			mailbox_id            TEXT NOT NULL,
			uid                   INTEGER NOT NULL,
			subject               TEXT,
			sender_name           TEXT,
			sender_email          TEXT NOT NULL DEFAULT '',
			date                  TEXT,
			size_bytes            INTEGER NOT NULL DEFAULT 0,
			flags_json            TEXT,
			has_attachments       INTEGER NOT NULL DEFAULT 0,
			attachment_count      INTEGER NOT NULL DEFAULT 0,
			blob_sha256           TEXT,
			archived_at           TEXT,
			deleted_from_server_at TEXT,
			body_preview          TEXT
		);
		CREATE INDEX idx_messages_cleanup_gate ON messages(archived_at, blob_sha256, deleted_from_server_at);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func insertTestMessage(t *testing.T, db *sql.DB, id, accountID, mailboxID, senderEmail string,
	sizeBytes int64, archivedAt *string, blobSHA256 *string, deletedAt *string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO messages(id, account_id, mailbox_id, uid, sender_email, size_bytes, archived_at, blob_sha256, deleted_from_server_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, accountID, mailboxID, 1, senderEmail, sizeBytes, archivedAt, blobSHA256, deletedAt)
	if err != nil {
		t.Fatalf("insert message %s: %v", id, err)
	}
}

func TestBuildSQL_AlwaysIncludesSafetyGate(t *testing.T) {
	filters := []domain.CleanupFilter{
		{AccountID: "acct-1"},
		{AccountID: "acct-1", SenderDomain: "spam.com"},
		{AccountID: "acct-1", OlderThanDays: 90, LargerThanKB: 500},
	}
	for _, f := range filters {
		clause, _ := BuildSQL(f)
		if !containsAll(clause, cleanupengine.SafetyCandidateSQL) {
			t.Errorf("safety gate missing from clause:\n%s", clause)
		}
	}
}

func TestBuildSQL_FilterFields(t *testing.T) {
	trueBool := true
	cases := []struct {
		name     string
		filter   domain.CleanupFilter
		contains string
	}{
		{"sender_domain", domain.CleanupFilter{AccountID: "a", SenderDomain: "spam.com"}, "sender_email LIKE ?"},
		{"sender_email", domain.CleanupFilter{AccountID: "a", SenderEmail: "x@y.com"}, "sender_email = ?"},
		{"older_than_days", domain.CleanupFilter{AccountID: "a", OlderThanDays: 30}, "m.date < ?"},
		{"larger_than_kb", domain.CleanupFilter{AccountID: "a", LargerThanKB: 1024}, "size_bytes >= ?"},
		{"has_attachments_true", domain.CleanupFilter{AccountID: "a", HasAttachments: &trueBool}, "has_attachments = 1"},
		{"flag_not_seen", domain.CleanupFilter{AccountID: "a", FlagNotSeen: true}, "flags_json NOT LIKE '%Seen%'"},
		{"subject_contains", domain.CleanupFilter{AccountID: "a", SubjectContains: "newsletter"}, "m.subject LIKE ?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clause, _ := BuildSQL(tc.filter)
			if !containsAll(clause, tc.contains) {
				t.Errorf("expected %q in clause, got:\n%s", tc.contains, clause)
			}
		})
	}
}

func TestBuildSQL_SafetyGateProof_UnarchivableCantBeSelected(t *testing.T) {
	db := setupCleanupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	sha := "abc123"

	insertTestMessage(t, db, "msg-safe", "acct-1", "mb-1", "x@safe.com", 1024, &now, &sha, nil)
	insertTestMessage(t, db, "msg-no-archive", "acct-1", "mb-1", "x@spam.com", 512, nil, nil, nil)
	insertTestMessage(t, db, "msg-no-blob", "acct-1", "mb-1", "x@spam.com", 256, &now, nil, nil)
	insertTestMessage(t, db, "msg-already-deleted", "acct-1", "mb-1", "x@spam.com", 100, &now, &sha, &now)

	filter := domain.CleanupFilter{AccountID: "acct-1"}
	clause, args := BuildSQL(filter)

	rows, err := db.Query(
		fmt.Sprintf(`SELECT id FROM messages m WHERE %s`, clause),
		args...,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) != 1 || ids[0] != "msg-safe" {
		t.Errorf("expected only [msg-safe], got %v", ids)
	}
}

func TestBuildSQL_DomainFilter(t *testing.T) {
	db := setupCleanupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	sha := "def456"

	insertTestMessage(t, db, "msg-spam-1", "acct-1", "mb-1", "a@spam.com", 1024, &now, &sha, nil)
	insertTestMessage(t, db, "msg-spam-2", "acct-1", "mb-1", "b@spam.com", 2048, &now, &sha, nil)
	insertTestMessage(t, db, "msg-other", "acct-1", "mb-1", "c@other.com", 512, &now, &sha, nil)
	insertTestMessage(t, db, "msg-unarchived", "acct-1", "mb-1", "d@spam.com", 999, nil, nil, nil)

	filter := domain.CleanupFilter{AccountID: "acct-1", SenderDomain: "spam.com"}
	clause, args := BuildSQL(filter)

	rows, err := db.Query(fmt.Sprintf(`SELECT id FROM messages m WHERE %s`, clause), args...)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) != 2 {
		t.Errorf("expected 2 spam messages, got %d: %v", len(ids), ids)
	}
	for _, id := range ids {
		if id == "msg-unarchived" || id == "msg-other" {
			t.Errorf("unexpected message in results: %s", id)
		}
	}
}

func TestBuildSQL_OlderThanDays(t *testing.T) {
	db := setupCleanupTestDB(t)

	sha := "ghi789"
	archivedAt := time.Now().UTC().Format(time.RFC3339)
	old := time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339)
	recent := time.Now().AddDate(0, 0, -5).UTC().Format(time.RFC3339)

	_, err := db.Exec(`
		INSERT INTO messages(id, account_id, mailbox_id, uid, sender_email, size_bytes, date, archived_at, blob_sha256, deleted_from_server_at)
		VALUES ('old-msg', 'acct-1', 'mb-1', 2, 'a@b.com', 100, ?, ?, ?, NULL)
	`, old, archivedAt, sha)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO messages(id, account_id, mailbox_id, uid, sender_email, size_bytes, date, archived_at, blob_sha256, deleted_from_server_at)
		VALUES ('recent-msg', 'acct-1', 'mb-1', 3, 'a@b.com', 100, ?, ?, ?, NULL)
	`, recent, archivedAt, sha)
	if err != nil {
		t.Fatal(err)
	}

	filter := domain.CleanupFilter{AccountID: "acct-1", OlderThanDays: 30}
	clause, args := BuildSQL(filter)

	rows, _ := db.Query(fmt.Sprintf(`SELECT id FROM messages m WHERE %s`, clause), args...)
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) != 1 || ids[0] != "old-msg" {
		t.Errorf("expected [old-msg], got %v", ids)
	}
}

func containsAll(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || len(haystack) >= len(needle) && searchString(haystack, needle))
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCleanupFilterJSONRoundtrip(t *testing.T) {
	raw := `{"account_id":"acc-1","older_than_days":30,"sender_domain":"spam.com"}`
	var f domain.CleanupFilter
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatal(err)
	}
	if f.AccountID != "acc-1" || f.OlderThanDays != 30 || f.SenderDomain != "spam.com" {
		t.Fatalf("unexpected filter: %+v", f)
	}
}
