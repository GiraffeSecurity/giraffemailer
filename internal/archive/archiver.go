package archive

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/GiraffeSecurity/giraffemailer/internal/mailconn"
	"github.com/GiraffeSecurity/giraffemailer/internal/store"
	"github.com/gofrs/uuid/v5"
	"github.com/rs/zerolog/log"
)

const bodyPreviewLen = 200

type pendingRow struct {
	id        string
	uid       imap.UID
	sizeBytes int64
}

func ArchiveMailbox(
	ctx context.Context,
	db *sql.DB,
	bs *store.BlobStore,
	cl *mailconn.Client,
	accountID, mailboxID, mailboxName string,
	batchSizeBytes int64,
	progress *Progress,
) (archived int, bytesDown int64, err error) {
	if _, err := cl.Select(ctx, mailboxName); err != nil {
		return 0, 0, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, uid, size_bytes
		FROM   messages
		WHERE  mailbox_id  = ?
		  AND  archived_at IS NULL
		ORDER  BY date ASC, uid ASC
	`, mailboxID)
	if err != nil {
		return 0, 0, fmt.Errorf("query unarchived: %w", err)
	}

	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		var uid uint64
		if err := rows.Scan(&r.id, &uid, &r.sizeBytes); err != nil {
			continue
		}
		r.uid = imap.UID(uid)
		pending = append(pending, r)
	}
	rows.Close()

	if len(pending) == 0 {
		return 0, 0, nil
	}

	if progress != nil {
		progress.setArchiving(int64(len(pending)))
	}

	for _, batch := range buildBatches(pending, batchSizeBytes) {
		select {
		case <-ctx.Done():
			return archived, bytesDown, ctx.Err()
		default:
		}

		uidToRowID := make(map[imap.UID]string, len(batch))
		for _, r := range batch {
			uidToRowID[r.uid] = r.id
		}

		fetchErr := cl.FetchBodies(ctx, batchUIDSet(batch), func(uid imap.UID, raw []byte) error {
			rowID, ok := uidToRowID[uid]
			if !ok {
				return nil
			}
			if err := archiveOne(ctx, db, bs, accountID, rowID, uid, raw); err != nil {
				log.Warn().Err(err).Uint32("uid", uint32(uid)).Str("mailbox", mailboxName).Msg("archive message failed (skipping)")
				return nil
			}
			archived++
			bytesDown += int64(len(raw))
			if progress != nil {
				progress.incArchived(1, int64(len(raw)))
			}
			return nil
		})
		if fetchErr != nil {
			log.Warn().Err(fetchErr).Str("mailbox", mailboxName).Msg("fetch bodies partial error")
		}
	}

	_, _ = db.ExecContext(ctx, `
		UPDATE mailboxes SET
			archived_count      = (SELECT COUNT(*) FROM messages WHERE mailbox_id = ? AND archived_at IS NOT NULL),
			archived_size_bytes = (SELECT COALESCE(SUM(size_bytes),0) FROM messages WHERE mailbox_id = ? AND archived_at IS NOT NULL),
			last_archived_at    = CURRENT_TIMESTAMP,
			updated_at          = CURRENT_TIMESTAMP
		WHERE id = ?`, mailboxID, mailboxID, mailboxID,
	)

	return archived, bytesDown, nil
}

func archiveOne(
	ctx context.Context,
	db *sql.DB,
	bs *store.BlobStore,
	accountID, rowID string,
	uid imap.UID,
	raw []byte,
) error {
	sha256hex, err := bs.Write(accountID, raw)
	if err != nil {
		return fmt.Errorf("blob write uid=%d: %w", uid, err)
	}

	parsed, _ := ParseEmail(raw)
	preview := BodyPreview(parsed.BodyText, bodyPreviewLen)
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE messages
		SET blob_sha256 = ?, archived_at = ?, body_preview = ?, updated_at = ?
		WHERE id = ?
	`, sha256hex, now, nilIfEmpty(preview), now, rowID,
	); err != nil {
		return fmt.Errorf("update archived: %w", err)
	}

	var recipJSON, subject, senderName, senderEmail string
	_ = tx.QueryRowContext(ctx,
		`SELECT COALESCE(recipients_json,'[]'), COALESCE(subject,''), COALESCE(sender_name,''), COALESCE(sender_email,'') FROM messages WHERE id = ?`,
		rowID,
	).Scan(&recipJSON, &subject, &senderName, &senderEmail)

	if _, err := tx.ExecContext(ctx, `DELETE FROM messages_fts WHERE message_db_id = ?`, rowID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages_fts(message_db_id, subject, sender_name, sender_email, recipients_text, body_text)
		VALUES (?, ?, ?, ?, ?, ?)
	`, rowID, subject, senderName, senderEmail, flattenRecipientsJSON(recipJSON), parsed.BodyText,
	); err != nil {
		return fmt.Errorf("fts insert: %w", err)
	}

	for _, att := range parsed.Attachments {
		_, _ = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO attachments(id, message_id, filename, content_type, size_bytes, part_path)
			VALUES (?, ?, ?, ?, ?, ?)
		`, uuid.Must(uuid.NewV7()).String(), rowID, att.Filename, att.ContentType, att.SizeBytes, att.PartPath)
	}
	if len(parsed.Attachments) > 0 {
		_, _ = tx.ExecContext(ctx,
			`UPDATE messages SET has_attachments = 1, attachment_count = ? WHERE id = ?`,
			len(parsed.Attachments), rowID,
		)
	}

	return tx.Commit()
}

func buildBatches(pending []pendingRow, maxBytes int64) [][]pendingRow {
	var batches [][]pendingRow
	var cur []pendingRow
	var curBytes int64
	for _, r := range pending {
		if len(cur) > 0 && curBytes+r.sizeBytes > maxBytes {
			batches = append(batches, cur)
			cur = nil
			curBytes = 0
		}
		cur = append(cur, r)
		curBytes += r.sizeBytes
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

func batchUIDSet(batch []pendingRow) imap.UIDSet {
	uids := make([]imap.UID, len(batch))
	for i, r := range batch {
		uids[i] = r.uid
	}
	return imap.UIDSetNum(uids...)
}
