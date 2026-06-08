package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
)

type ExportRepo struct {
	db *sql.DB
}

func NewExportRepo(db *sql.DB) *ExportRepo {
	return &ExportRepo{db: db}
}

func (r *ExportRepo) ListArchivedBlobs(ctx context.Context, messageIDs []string) ([]port.ExportBlob, error) {
	placeholders := make([]string, len(messageIDs))
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, account_id, subject, blob_sha256 FROM messages
		             WHERE id IN (%s) AND blob_sha256 IS NOT NULL`, strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobs []port.ExportBlob
	for rows.Next() {
		var b port.ExportBlob
		var id string
		var subject sql.NullString
		if err := rows.Scan(&id, &b.AccountID, &subject, &b.BlobSHA256); err != nil {
			continue
		}
		if subject.Valid {
			b.Subject = subject.String
		}
		blobs = append(blobs, b)
	}
	return blobs, rows.Err()
}

func (r *ExportRepo) GetRestoreTarget(ctx context.Context, messageID string) (port.RestoreTarget, error) {
	var target port.RestoreTarget
	var flagsJSON sql.NullString
	var msgDate sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT m.account_id, m.blob_sha256, mb.name, m.flags_json, m.date
		FROM   messages m JOIN mailboxes mb ON mb.id = m.mailbox_id
		WHERE  m.id = ? AND m.blob_sha256 IS NOT NULL AND m.archived_at IS NOT NULL
	`, messageID).Scan(&target.AccountID, &target.BlobSHA256, &target.MailboxName, &flagsJSON, &msgDate)
	if errors.Is(err, sql.ErrNoRows) {
		return port.RestoreTarget{}, domain.ErrNotFound
	}
	if err != nil {
		return port.RestoreTarget{}, err
	}
	if flagsJSON.Valid {
		s := flagsJSON.String
		target.FlagsJSON = &s
	}
	if msgDate.Valid {
		s := msgDate.String
		target.Date = &s
	}
	return target, nil
}
