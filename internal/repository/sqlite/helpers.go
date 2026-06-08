package sqlite

import (
	"database/sql"
	"strings"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

func placeholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func appendScopedAccounts(where *[]string, args *[]any, column string, accountID string, scopedIDs []string) {
	if accountID != "" {
		*where = append(*where, column+" = ?")
		*args = append(*args, accountID)
		return
	}
	if len(scopedIDs) == 0 {
		return
	}
	*where = append(*where, column+" IN ("+placeholders(len(scopedIDs))+")")
	for _, id := range scopedIDs {
		*args = append(*args, id)
	}
}

func archiveState(archivedAt, deletedFromServer *string) string {
	if deletedFromServer != nil {
		return "deleted_from_server"
	}
	if archivedAt != nil {
		return "archived"
	}
	return "not_archived"
}

func nilString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func scanMessageSummary(
	id string, uid int64,
	subject, senderName, senderEmail, date sql.NullString,
	sizeBytes int64, hasAtt int, attachmentCount int,
	bodyPreview, archivedAt, deletedAt sql.NullString,
	mailboxName, accountName string,
) domain.MessageSummary {
	return domain.MessageSummary{
		ID:                  id,
		UID:                 uid,
		Subject:             nilString(subject),
		SenderName:          nilString(senderName),
		SenderEmail:         senderEmail.String,
		Date:                nilString(date),
		SizeBytes:           sizeBytes,
		HasAttachments:      hasAtt == 1,
		AttachmentCount:     attachmentCount,
		BodyPreview:         nilString(bodyPreview),
		ArchivedAt:          nilString(archivedAt),
		DeletedFromServerAt: nilString(deletedAt),
		ArchiveState:        archiveState(nilString(archivedAt), nilString(deletedAt)),
		MailboxName:         mailboxName,
		AccountName:         accountName,
	}
}
