package sqlite

import (
	"context"
	"database/sql"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type MailboxRepo struct {
	db *sql.DB
}

func NewMailboxRepo(db *sql.DB) *MailboxRepo {
	return &MailboxRepo{db: db}
}

func (r *MailboxRepo) ListByAccount(ctx context.Context, accountID string) ([]domain.Mailbox, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, message_count, total_size_bytes, archived_count, archived_size_bytes, last_indexed_at, last_archived_at
		FROM   mailboxes
		WHERE  account_id = ?
		ORDER  BY name
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Mailbox
	for rows.Next() {
		var mb domain.Mailbox
		if err := rows.Scan(&mb.ID, &mb.Name, &mb.MessageCount, &mb.TotalSizeBytes,
			&mb.ArchivedCount, &mb.ArchivedSizeBytes, &mb.LastIndexedAt, &mb.LastArchivedAt); err != nil {
			continue
		}
		if mb.MessageCount > 0 {
			mb.ArchivedPercent = float64(mb.ArchivedCount) / float64(mb.MessageCount) * 100
		}
		out = append(out, mb)
	}
	return out, rows.Err()
}
