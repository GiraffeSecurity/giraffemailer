package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type AttachmentRepo struct {
	db *sql.DB
}

func NewAttachmentRepo(db *sql.DB) *AttachmentRepo {
	return &AttachmentRepo{db: db}
}

func (r *AttachmentRepo) ListByMessage(ctx context.Context, messageID string) ([]domain.AttachmentMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT filename, content_type, size_bytes, part_path
		FROM   attachments
		WHERE  message_id = ?
		ORDER  BY part_path`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []domain.AttachmentMeta
	for rows.Next() {
		var a domain.AttachmentMeta
		if err := rows.Scan(&a.Filename, &a.ContentType, &a.SizeBytes, &a.PartPath); err != nil {
			return nil, err
		}
		atts = append(atts, a)
	}
	return atts, rows.Err()
}

func (r *AttachmentRepo) GetMeta(ctx context.Context, messageID, partPath string) (filename, contentType string, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT filename, content_type FROM attachments WHERE message_id = ? AND part_path = ?`,
		messageID, partPath,
	).Scan(&filename, &contentType)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", domain.ErrNotFound
	}
	return filename, contentType, err
}
