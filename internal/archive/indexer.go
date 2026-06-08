package archive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/GiraffeSecurity/giraffemailer/internal/mailconn"
	"github.com/gofrs/uuid/v5"
	"github.com/rs/zerolog/log"
)

const indexBatchSize = 500

func IndexMailbox(
	ctx context.Context,
	db *sql.DB,
	cl *mailconn.Client,
	accountID, mailboxID, mailboxName string,
) (int, error) {
	sel, err := cl.Select(ctx, mailboxName)
	if err != nil {
		return 0, err
	}

	var storedValidity uint32
	err = db.QueryRowContext(ctx,
		`SELECT uid_validity FROM mailboxes WHERE id = ?`, mailboxID,
	).Scan(&storedValidity)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("read uid_validity: %w", err)
	}

	if storedValidity != 0 && sel.UIDValidity != storedValidity {
		log.Warn().
			Str("mailbox", mailboxName).
			Uint32("old_validity", storedValidity).
			Uint32("new_validity", sel.UIDValidity).
			Msg("UIDVALIDITY changed — re-indexing mailbox")

		if _, err := db.ExecContext(ctx, `
			UPDATE messages
			SET    indexed_at = NULL, archived_at = NULL, blob_sha256 = NULL
			WHERE  mailbox_id = ?`, mailboxID,
		); err != nil {
			return 0, fmt.Errorf("reset messages on validity change: %w", err)
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE mailboxes SET highest_uid = 0 WHERE id = ?`, mailboxID,
		); err != nil {
			return 0, fmt.Errorf("reset highest_uid: %w", err)
		}
		storedValidity = 0
	}

	var highestUID imap.UID
	var highest uint64
	_ = db.QueryRowContext(ctx,
		`SELECT highest_uid FROM mailboxes WHERE id = ?`, mailboxID,
	).Scan(&highest)
	highestUID = imap.UID(highest)

	if sel.NumMessages == 0 {
		return updateMailboxMeta(ctx, db, mailboxID, sel.UIDValidity, highestUID, 0)
	}

	startUID := highestUID + 1
	if startUID < 1 {
		startUID = 1
	}
	uidSet := imap.UIDSet{{Start: startUID, Stop: 0}}

	metas, err := cl.FetchMetadata(ctx, uidSet)
	if err != nil {
		return 0, fmt.Errorf("fetch metadata: %w", err)
	}
	if len(metas) == 0 {
		if err := reconcileDeletions(ctx, db, cl, mailboxID); err != nil {
			log.Warn().Err(err).Str("mailbox", mailboxName).Msg("reconcile deletions failed (non-fatal)")
		}
		return 0, nil
	}

	inserted := 0
	var newHighest imap.UID
	for i := 0; i < len(metas); i += indexBatchSize {
		end := i + indexBatchSize
		if end > len(metas) {
			end = len(metas)
		}
		batch := metas[i:end]
		n, h, err := upsertMetaBatch(ctx, db, accountID, mailboxID, batch)
		if err != nil {
			return inserted, err
		}
		inserted += n
		if h > newHighest {
			newHighest = h
		}
	}

	if newHighest > highestUID {
		if _, err := db.ExecContext(ctx,
			`UPDATE mailboxes SET highest_uid = ?, uid_validity = ?, last_indexed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			uint64(newHighest), sel.UIDValidity, mailboxID,
		); err != nil {
			return inserted, fmt.Errorf("update highest_uid: %w", err)
		}
	}

	if err := reconcileDeletions(ctx, db, cl, mailboxID); err != nil {
		log.Warn().Err(err).Str("mailbox", mailboxName).Msg("reconcile deletions failed (non-fatal)")
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE mailboxes SET
			message_count       = (SELECT COUNT(*)       FROM messages WHERE mailbox_id = ? AND deleted_from_server_at IS NULL),
			total_size_bytes    = (SELECT COALESCE(SUM(size_bytes), 0) FROM messages WHERE mailbox_id = ? AND deleted_from_server_at IS NULL),
			updated_at          = CURRENT_TIMESTAMP
		WHERE id = ?`, mailboxID, mailboxID, mailboxID,
	); err != nil {
		return inserted, fmt.Errorf("update mailbox counts: %w", err)
	}

	return inserted, nil
}

func upsertMetaBatch(
	ctx context.Context,
	db *sql.DB,
	accountID, mailboxID string,
	metas []*mailconn.MessageMeta,
) (int, imap.UID, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	msgStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO messages (
			id, account_id, mailbox_id, uid, message_id_header, subject,
			sender_name, sender_email, recipients_json, date, size_bytes,
			flags_json, has_attachments, attachment_count, indexed_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(mailbox_id, uid) DO NOTHING
	`)
	if err != nil {
		return 0, 0, err
	}
	defer msgStmt.Close()

	ftsStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO messages_fts(message_db_id, subject, sender_name, sender_email, recipients_text, body_text)
		VALUES (?, ?, ?, ?, ?, '')
	`)
	if err != nil {
		return 0, 0, err
	}
	defer ftsStmt.Close()

	inserted := 0
	var maxUID imap.UID
	for _, m := range metas {
		if m.UID > maxUID {
			maxUID = m.UID
		}
		recipJSON, _ := json.Marshal(m.Recipients)
		flagsJSON, _ := json.Marshal(flagStrings(m.Flags))

		hasAtt, attCount := attachmentStats(m.BodyStructure)
		var dateVal interface{}
		if !m.Date.IsZero() {
			dateVal = m.Date.UTC().Format(time.RFC3339)
		}

		id := uuid.Must(uuid.NewV7()).String()
		res, err := msgStmt.ExecContext(ctx,
			id,
			accountID, mailboxID,
			uint64(m.UID),
			nilIfEmpty(m.MessageID),
			nilIfEmpty(m.Subject),
			nilIfEmpty(m.SenderName),
			m.SenderEmail,
			string(recipJSON),
			dateVal,
			m.SizeBytes,
			string(flagsJSON),
			hasAtt, attCount,
		)
		if err != nil {
			return inserted, maxUID, fmt.Errorf("insert message uid=%d: %w", m.UID, err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted++
			_, _ = ftsStmt.ExecContext(ctx, id, m.Subject, m.SenderName, m.SenderEmail, flattenRecipientsJSON(string(recipJSON)))
		}
	}

	return inserted, maxUID, tx.Commit()
}

func reconcileDeletions(ctx context.Context, db *sql.DB, cl *mailconn.Client, mailboxID string) error {
	serverUIDs, err := cl.SearchAll(ctx)
	if err != nil {
		return err
	}

	serverSet := make(map[imap.UID]struct{}, len(serverUIDs))
	for _, u := range serverUIDs {
		serverSet[u] = struct{}{}
	}

	rows, err := db.QueryContext(ctx,
		`SELECT uid FROM messages WHERE mailbox_id = ? AND deleted_from_server_at IS NULL`, mailboxID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var toDelete []uint64
	for rows.Next() {
		var uid uint64
		if err := rows.Scan(&uid); err != nil {
			continue
		}
		if _, ok := serverSet[imap.UID(uid)]; !ok {
			toDelete = append(toDelete, uid)
		}
	}
	rows.Close()

	if len(toDelete) == 0 {
		return nil
	}

	const batchSize = 500
	for i := 0; i < len(toDelete); i += batchSize {
		end := i + batchSize
		if end > len(toDelete) {
			end = len(toDelete)
		}
		chunk := toDelete[i:end]

		placeholders := make([]string, len(chunk))
		args := make([]any, len(chunk)+1)
		args[0] = mailboxID
		for j, uid := range chunk {
			placeholders[j] = "?"
			args[j+1] = uid
		}
		_, _ = db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE messages SET deleted_from_server_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE mailbox_id = ? AND uid IN (%s)`,
				strings.Join(placeholders, ",")),
			args...,
		)
	}
	return nil
}

func updateMailboxMeta(ctx context.Context, db *sql.DB, mailboxID string, uidValidity uint32, highestUID imap.UID, inserted int) (int, error) {
	_, err := db.ExecContext(ctx,
		`UPDATE mailboxes SET uid_validity = ?, highest_uid = ?, last_indexed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		uidValidity, uint64(highestUID), mailboxID,
	)
	return inserted, err
}

func attachmentStats(bs imap.BodyStructure) (hasAtt int, count int) {
	if bs == nil {
		return 0, 0
	}
	bs.Walk(func(_ []int, part imap.BodyStructure) bool {
		sp, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		if sp.Filename() != "" {
			count++
		} else if d := sp.Disposition(); d != nil && d.Value == "attachment" {
			count++
		}
		return true
	})
	if count > 0 {
		hasAtt = 1
	}
	return
}

func flagStrings(flags []imap.Flag) []string {
	s := make([]string, len(flags))
	for i, f := range flags {
		s[i] = string(f)
	}
	return s
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func flattenRecipientsJSON(recipJSON string) string {
	type addr struct {
		Name  string `json:"Name"`
		Email string `json:"Email"`
	}
	var addrs []addr
	if err := json.Unmarshal([]byte(recipJSON), &addrs); err != nil {
		return ""
	}
	parts := make([]string, 0, len(addrs)*2)
	for _, a := range addrs {
		if a.Name != "" {
			parts = append(parts, a.Name)
		}
		if a.Email != "" {
			parts = append(parts, a.Email)
		}
	}
	return strings.Join(parts, " ")
}
