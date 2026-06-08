package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/GiraffeSecurity/giraffemailer/internal/archive"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
)

type MessageRepo struct {
	db   *sql.DB
	blob port.BlobReader
}

func NewMessageRepo(db *sql.DB, blob port.BlobReader) *MessageRepo {
	return &MessageRepo{db: db, blob: blob}
}

func (r *MessageRepo) List(ctx context.Context, filter domain.MessageListFilter) (domain.MessagePage, error) {
	limit := filter.Limit
	if limit < 1 || limit > 200 {
		limit = 50
	}

	mailboxScoped := filter.AccountID != "" && filter.MailboxID != ""

	var where []string
	var args []any

	if mailboxScoped {
		where = []string{"m.account_id = ?", "m.mailbox_id = ?"}
		args = []any{filter.AccountID, filter.MailboxID}
	} else {
		where = []string{"1=1"}
		appendScopedAccounts(&where, &args, "m.account_id", filter.AccountID, filter.ScopedAccountIDs)
		if filter.MailboxID != "" {
			where = append(where, "m.mailbox_id = ?")
			args = append(args, filter.MailboxID)
		}
	}

	if filter.Sender != "" {
		where = append(where, "m.sender_email LIKE ?")
		args = append(args, "%"+filter.Sender+"%")
	}
	switch filter.ArchiveState {
	case "archived":
		where = append(where, "m.archived_at IS NOT NULL")
	case "not_archived":
		where = append(where, "m.archived_at IS NULL")
	case "deleted_from_server":
		where = append(where, "m.deleted_from_server_at IS NOT NULL")
	}
	if filter.HasAttachments {
		where = append(where, "m.has_attachments = 1")
	}
	if filter.Unread {
		where = append(where, "m.flags_json NOT LIKE '%Seen%'")
	}

	sortField := "m.date"
	if filter.Sort == "size" {
		sortField = "m.size_bytes"
	}
	desc := filter.Desc

	if filter.Cursor != "" {
		cursorDate, cursorID, ok := decodeCursor(filter.Cursor)
		if ok {
			if desc {
				cursorWhereDesc(&where, &args, cursorDate, cursorID)
			} else {
				cursorWhereAsc(&where, &args, cursorDate, cursorID)
			}
		}
	}

	sortDir := "DESC"
	if !desc {
		sortDir = "ASC"
	}

	var query string
	if mailboxScoped {
		query = fmt.Sprintf(`
			SELECT m.id, m.uid, m.subject, m.sender_name, m.sender_email,
			       m.date, m.size_bytes, m.has_attachments, m.attachment_count,
			       m.body_preview, m.archived_at, m.deleted_from_server_at,
			       '', ''
			FROM   messages m
			WHERE  %s
			ORDER  BY m.date %s, m.id %s
			LIMIT  ?
		`, strings.Join(where, " AND "), sortDir, sortDir)
	} else {
		query = fmt.Sprintf(`
			SELECT m.id, m.uid, m.subject, m.sender_name, m.sender_email,
			       m.date, m.size_bytes, m.has_attachments, m.attachment_count,
			       m.body_preview, m.archived_at, m.deleted_from_server_at,
			       mb.name, ma.name
			FROM   messages m
			JOIN   mailboxes mb ON mb.id = m.mailbox_id
			JOIN   mail_accounts ma ON ma.id = m.account_id
			WHERE  %s
			ORDER  BY %s %s, m.id %s
			LIMIT  ?
		`, strings.Join(where, " AND "), sortField, sortDir, sortDir)
	}

	queryArgs := append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return domain.MessagePage{}, err
	}
	defer rows.Close()

	var msgs []domain.MessageSummary
	for rows.Next() {
		var id string
		var uid int64
		var subject, senderName, senderEmail, date sql.NullString
		var sizeBytes int64
		var hasAtt, attachmentCount int
		var bodyPreview, archivedAt, deletedAt sql.NullString
		var mailboxName, accountName string
		if err := rows.Scan(&id, &uid, &subject, &senderName, &senderEmail,
			&date, &sizeBytes, &hasAtt, &attachmentCount,
			&bodyPreview, &archivedAt, &deletedAt,
			&mailboxName, &accountName); err != nil {
			continue
		}
		msgs = append(msgs, scanMessageSummary(id, uid, subject, senderName, senderEmail, date,
			sizeBytes, hasAtt, attachmentCount, bodyPreview, archivedAt, deletedAt,
			mailboxName, accountName))
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	var nextCursor string
	if hasMore && len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		nextCursor = encodeCursor(last.Date, last.ID)
	}

	return domain.MessagePage{
		Messages:   msgs,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Limit:      limit,
	}, rows.Err()
}

func (r *MessageRepo) GetDetail(ctx context.Context, id string, _ bool) (domain.MessageDetail, error) {
	var accountID, blobSHA256 sql.NullString
	var subject, senderName, senderEmail, date, archivedAt, deletedAt sql.NullString
	var sizeBytes int64

	err := r.db.QueryRowContext(ctx, `
		SELECT account_id, subject, sender_name, sender_email, date, size_bytes,
		       blob_sha256, archived_at, deleted_from_server_at
		FROM   messages WHERE id = ?`, id,
	).Scan(&accountID, &subject, &senderName, &senderEmail, &date, &sizeBytes,
		&blobSHA256, &archivedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MessageDetail{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.MessageDetail{}, err
	}

	detail := domain.MessageDetail{
		ID:                  id,
		Subject:             nilString(subject),
		SenderName:          nilString(senderName),
		SenderEmail:         nilString(senderEmail),
		Date:                nilString(date),
		SizeBytes:           sizeBytes,
		ArchivedAt:          nilString(archivedAt),
		DeletedFromServerAt: nilString(deletedAt),
		ArchiveState:        archiveState(nilString(archivedAt), nilString(deletedAt)),
	}

	if blobSHA256.Valid && accountID.Valid {
		raw, err := r.blob.Read(accountID.String, blobSHA256.String)
		if err == nil {
			parsed, _ := archive.ParseEmail(raw)
			if parsed != nil {
				if parsed.BodyHTML != "" {
					html := parsed.BodyHTML
					detail.BodyHTML = &html
				}
				if parsed.BodyText != "" {
					text := parsed.BodyText
					detail.BodyText = &text
				}
			}
		}
	}

	return detail, nil
}

func (r *MessageRepo) GetAccountID(ctx context.Context, messageID string) (string, error) {
	var accountID string
	err := r.db.QueryRowContext(ctx, `SELECT account_id FROM messages WHERE id = ?`, messageID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return accountID, err
}

func (r *MessageRepo) GetBlobRef(ctx context.Context, id string) (accountID, sha256 string, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT account_id, blob_sha256 FROM messages WHERE id = ? AND blob_sha256 IS NOT NULL`, id,
	).Scan(&accountID, &sha256)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", domain.ErrNotFound
	}
	return accountID, sha256, err
}

func (r *MessageRepo) MarkRestored(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE messages SET deleted_from_server_at = NULL WHERE id = ?`, id)
	return err
}

func (r *MessageRepo) MarkDeletedFromServer(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE messages SET deleted_from_server_at = CURRENT_TIMESTAMP WHERE id IN (%s)`,
			strings.Join(placeholders, ",")),
		args...)
	return err
}
