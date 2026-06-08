package cleanup

import (
	"strings"
	"time"

	cleanupengine "github.com/GiraffeSecurity/giraffemailer/internal/cleanup"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

func BuildSQL(f domain.CleanupFilter) (string, []any) {
	where := []string{
		"m.account_id = ?",
		cleanupengine.SafetyCandidateSQL,
	}
	args := []any{f.AccountID}

	if f.MailboxName != "" {
		where = append(where, "mb.name = ?")
		args = append(args, f.MailboxName)
	}
	if f.SenderDomain != "" {
		where = append(where, "m.sender_email LIKE ?")
		args = append(args, "%@"+f.SenderDomain)
	}
	if f.SenderEmail != "" {
		where = append(where, "m.sender_email = ?")
		args = append(args, f.SenderEmail)
	}
	if f.OlderThanDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -f.OlderThanDays).UTC().Format(time.RFC3339)
		where = append(where, "m.date < ?")
		args = append(args, cutoff)
	}
	if f.LargerThanKB > 0 {
		where = append(where, "m.size_bytes >= ?")
		args = append(args, int64(f.LargerThanKB)*1024)
	}
	if f.HasAttachments != nil {
		if *f.HasAttachments {
			where = append(where, "m.has_attachments = 1")
		} else {
			where = append(where, "m.has_attachments = 0")
		}
	}
	if f.FlagNotSeen {
		where = append(where, "m.flags_json NOT LIKE '%Seen%'")
	}
	if f.SubjectContains != "" {
		where = append(where, "m.subject LIKE ?")
		args = append(args, "%"+f.SubjectContains+"%")
	}

	return strings.Join(where, " AND "), args
}
