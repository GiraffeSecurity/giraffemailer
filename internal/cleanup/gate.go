package cleanup

import "time"

// Candidate is the minimum information needed to evaluate the safety gate for
// a single message. Fields are pointers because either column can be NULL in
// the database.
type Candidate struct {
	ID           string
	AccountID    string
	MailboxID    string
	UID          int64
	ArchivedAt   *time.Time
	BlobSHA256   *string
	SizeBytes    int64
}

// IsSafe returns true only when both archived_at and blob_sha256 are present
// and non-empty. A candidate that fails this check must never be passed to the
// cleanup engine's IMAP delete/move code — it is counted in skipped_unarchived.
//
// This is the golden rule of GiraffeMail Archive: a message may only be
// deleted from the mail server after its local archive copy is verified.
func IsSafe(c Candidate) bool {
	return c.ArchivedAt != nil && c.BlobSHA256 != nil && *c.BlobSHA256 != ""
}

// SafetyCandidateSQL is the WHERE clause fragment that must be prepended to
// every cleanup query. It is exported so the filter builder always composes it
// with AND rather than OR.
//
// Usage:
//
//	WHERE archived_at IS NOT NULL
//	  AND blob_sha256 IS NOT NULL
//	  AND deleted_from_server_at IS NULL
//	  AND <user_filter_conditions>
const SafetyCandidateSQL = `
	archived_at IS NOT NULL
	AND blob_sha256 IS NOT NULL
	AND blob_sha256 != ''
	AND deleted_from_server_at IS NULL`
