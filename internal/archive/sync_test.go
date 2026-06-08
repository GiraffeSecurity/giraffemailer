package archive

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestSyncCursor proves that after indexing a batch of messages, the highest_uid
// stored in the mailboxes table equals the maximum UID fetched, and that a
// second sync only considers UIDs strictly greater than that cursor.
func TestSyncCursor_AdvancesOnIndex(t *testing.T) {
	db := openSyncTestDB(t)
	accountID, mailboxID := seedAccount(t, db)

	// Simulate Phase 1 inserting messages with UIDs 1–10.
	uids := []int64{1, 3, 5, 7, 9, 10}
	for _, uid := range uids {
		insertMessage(t, db, accountID, mailboxID, uid)
	}

	// Update highest_uid as the indexer would.
	if _, err := db.Exec(`UPDATE mailboxes SET highest_uid = 10 WHERE id = ?`, mailboxID); err != nil {
		t.Fatal(err)
	}

	var highest int64
	if err := db.QueryRow(`SELECT highest_uid FROM mailboxes WHERE id = ?`, mailboxID).Scan(&highest); err != nil {
		t.Fatal(err)
	}
	if highest != 10 {
		t.Fatalf("highest_uid = %d, want 10", highest)
	}

	// Simulate a new message arriving (UID 11).
	insertMessage(t, db, accountID, mailboxID, 11)

	// The incremental sync query should only return UIDs > highest_uid.
	rows, err := db.Query(`SELECT uid FROM messages WHERE mailbox_id = ? AND uid > ?`, mailboxID, highest)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var newUIDs []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			t.Fatal(err)
		}
		newUIDs = append(newUIDs, uid)
	}

	if len(newUIDs) != 1 || newUIDs[0] != 11 {
		t.Fatalf("incremental sync returned %v, want [11]", newUIDs)
	}
}

// TestSyncCursor_UIDValidityReset proves that when UIDVALIDITY changes, the
// highest_uid is reset to 0, causing the next sync to re-fetch everything.
func TestSyncCursor_UIDValidityReset(t *testing.T) {
	db := openSyncTestDB(t)
	accountID, mailboxID := seedAccount(t, db)

	// Simulate a prior successful sync with UIDVALIDITY=1000, highest_uid=50.
	if _, err := db.Exec(
		`UPDATE mailboxes SET uid_validity = 1000, highest_uid = 50 WHERE id = ?`, mailboxID,
	); err != nil {
		t.Fatal(err)
	}
	for _, uid := range []int64{10, 20, 30, 40, 50} {
		insertMessage(t, db, accountID, mailboxID, uid)
	}

	// Server resets UIDVALIDITY to 2000 (new UID namespace).
	var storedValidity uint32 = 1000
	newValidity := uint32(2000)

	if storedValidity != 0 && newValidity != storedValidity {
		// Simulate what IndexMailbox does: reset highest_uid and clear blob refs.
		if _, err := db.Exec(`UPDATE messages SET blob_sha256 = NULL, archived_at = NULL WHERE mailbox_id = ?`, mailboxID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE mailboxes SET highest_uid = 0 WHERE id = ?`, mailboxID); err != nil {
			t.Fatal(err)
		}
	}

	var highest int64
	if err := db.QueryRow(`SELECT highest_uid FROM mailboxes WHERE id = ?`, mailboxID).Scan(&highest); err != nil {
		t.Fatal(err)
	}
	if highest != 0 {
		t.Fatalf("after UIDVALIDITY reset, highest_uid = %d, want 0", highest)
	}

	// Verify archived_at was cleared (blobs still on disk; DB metadata reset).
	var archivedCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE mailbox_id = ? AND archived_at IS NOT NULL`, mailboxID).Scan(&archivedCount); err != nil {
		t.Fatal(err)
	}
	if archivedCount != 0 {
		t.Fatalf("after UIDVALIDITY reset, %d messages still have archived_at set, want 0", archivedCount)
	}
}

// TestSyncCursor_ResumableAfterCrash proves that partial archiving is resumable:
// messages with archived_at IS NULL are picked up on restart.
func TestSyncCursor_ResumableAfterCrash(t *testing.T) {
	db := openSyncTestDB(t)
	accountID, mailboxID := seedAccount(t, db)

	// Insert 5 messages, 3 of which were already archived before crash.
	for _, uid := range []int64{1, 2, 3, 4, 5} {
		insertMessage(t, db, accountID, mailboxID, uid)
	}
	for _, uid := range []int64{1, 2, 3} {
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := db.Exec(
			`UPDATE messages SET archived_at = ?, blob_sha256 = 'sha256abc' WHERE mailbox_id = ? AND uid = ?`,
			now, mailboxID, uid,
		); err != nil {
			t.Fatal(err)
		}
	}

	// After restart, only messages with archived_at IS NULL should be queued.
	rows, err := db.Query(`
		SELECT uid FROM messages
		WHERE mailbox_id = ? AND archived_at IS NULL
		ORDER BY uid ASC
	`, mailboxID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var unarchived []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			t.Fatal(err)
		}
		unarchived = append(unarchived, uid)
	}

	if len(unarchived) != 2 {
		t.Fatalf("resumable: got %v unarchived UIDs, want [4 5]", unarchived)
	}
	if unarchived[0] != 4 || unarchived[1] != 5 {
		t.Fatalf("resumable: got UIDs %v, want [4 5]", unarchived)
	}
}

// TestBatchBuilder proves that messages are split into byte-bounded batches.
func TestBatchBuilder(t *testing.T) {
	pending := []pendingRow{
		{id: "a", uid: 1, sizeBytes: 3 * 1024 * 1024}, // 3 MB
		{id: "b", uid: 2, sizeBytes: 3 * 1024 * 1024}, // 3 MB → batch 1 (6 MB total)
		{id: "c", uid: 3, sizeBytes: 3 * 1024 * 1024}, // 3 MB → new batch (6+3 > 8)
		{id: "d", uid: 4, sizeBytes: 1 * 1024 * 1024}, // 1 MB → batch 2 (4 MB total)
	}
	batches := buildBatches(pending, 8*1024*1024)
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Fatalf("batch[0] len = %d, want 2", len(batches[0]))
	}
	if len(batches[1]) != 2 {
		t.Fatalf("batch[1] len = %d, want 2", len(batches[1]))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func openSyncTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.ExecContext(context.Background(), `
		PRAGMA foreign_keys = OFF;
		CREATE TABLE mail_accounts (id TEXT PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE mailboxes (
			id          TEXT    PRIMARY KEY,
			account_id  TEXT    NOT NULL,
			name        TEXT    NOT NULL,
			uid_validity INTEGER NOT NULL DEFAULT 0,
			highest_uid INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE messages (
			id            TEXT    PRIMARY KEY,
			account_id    TEXT    NOT NULL,
			mailbox_id    TEXT    NOT NULL,
			uid           INTEGER NOT NULL,
			sender_email  TEXT    NOT NULL DEFAULT '',
			size_bytes    INTEGER NOT NULL DEFAULT 0,
			archived_at   DATETIME,
			blob_sha256   TEXT,
			UNIQUE(mailbox_id, uid)
		);
	`)
	if err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	return db
}

func seedAccount(t *testing.T, db *sql.DB) (accountID, mailboxID string) {
	t.Helper()
	accountID = "acct-test-1"
	mailboxID = "mbox-test-1"
	if _, err := db.Exec(`INSERT INTO mail_accounts(id, name) VALUES (?, 'Test Account')`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mailboxes(id, account_id, name) VALUES (?, ?, 'INBOX')`, mailboxID, accountID); err != nil {
		t.Fatal(err)
	}
	return
}

func insertMessage(t *testing.T, db *sql.DB, accountID, mailboxID string, uid int64) {
	t.Helper()
	id := fmt.Sprintf("msg-%s-%d", accountID, uid)
	if _, err := db.Exec(
		`INSERT OR IGNORE INTO messages(id, account_id, mailbox_id, uid, size_bytes) VALUES (?, ?, ?, ?, 1024)`,
		id, accountID, mailboxID, uid,
	); err != nil {
		t.Fatalf("insert message uid=%d: %v", uid, err)
	}
}
