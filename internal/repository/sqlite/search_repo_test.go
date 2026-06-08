package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	_ "modernc.org/sqlite"
)

func TestSearchRepo_KeysetPagination(t *testing.T) {
	db := openSearchTestDB(t)
	repo := NewSearchRepo(db)
	ctx := context.Background()

	insertSearchMessage(t, db, "msg-1", "2024-01-03T10:00:00Z", "hello world")
	insertSearchMessage(t, db, "msg-2", "2024-01-02T10:00:00Z", "hello again")
	insertSearchMessage(t, db, "msg-3", "2024-01-01T10:00:00Z", "hello old")

	page1, err := repo.Search(ctx, domain.SearchQuery{Query: "hello", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 3 {
		t.Fatalf("total=%d want 3", page1.Total)
	}
	if len(page1.Messages) != 2 || !page1.HasMore || page1.NextCursor == "" {
		t.Fatalf("page1: len=%d hasMore=%v cursor=%q", len(page1.Messages), page1.HasMore, page1.NextCursor)
	}
	if page1.Messages[0].ID != "msg-1" {
		t.Fatalf("page1 first id=%s", page1.Messages[0].ID)
	}

	page2, err := repo.Search(ctx, domain.SearchQuery{Query: "hello", Limit: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Messages) != 1 || page2.HasMore {
		t.Fatalf("page2: len=%d hasMore=%v", len(page2.Messages), page2.HasMore)
	}
	if page2.Messages[0].ID != "msg-3" {
		t.Fatalf("page2 id=%s want msg-3", page2.Messages[0].ID)
	}
}

func openSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE mail_accounts (id TEXT PRIMARY KEY, name TEXT);
		CREATE TABLE mailboxes (id TEXT PRIMARY KEY, account_id TEXT, name TEXT);
		CREATE TABLE messages (
			id TEXT PRIMARY KEY, account_id TEXT, mailbox_id TEXT, uid INTEGER,
			subject TEXT, sender_name TEXT, sender_email TEXT, date TEXT,
			size_bytes INTEGER, has_attachments INTEGER, attachment_count INTEGER,
			body_preview TEXT, archived_at TEXT, deleted_from_server_at TEXT
		);
		CREATE VIRTUAL TABLE messages_fts USING fts5(
			message_db_id UNINDEXED, subject, sender_name, sender_email, recipients_text, body_text
		);
		INSERT INTO mail_accounts VALUES ('acc-1', 'Test');
		INSERT INTO mailboxes VALUES ('mb-1', 'acc-1', 'INBOX');
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func insertSearchMessage(t *testing.T, db *sql.DB, id, date, body string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO messages(id, account_id, mailbox_id, uid, subject, sender_email, date, size_bytes, has_attachments, attachment_count, body_preview)
		VALUES (?, 'acc-1', 'mb-1', 1, 'sub', 'a@b.com', ?, 100, 0, 0, ?)
	`, id, date, body)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO messages_fts(message_db_id, subject, sender_name, sender_email, recipients_text, body_text)
		VALUES (?, 'sub', '', 'a@b.com', '', ?)
	`, id, body)
	if err != nil {
		t.Fatal(err)
	}
}
