package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/search"
)

type SearchRepo struct {
	db *sql.DB
}

func NewSearchRepo(db *sql.DB) *SearchRepo {
	return &SearchRepo{db: db}
}

const searchSelectCols = `m.id, m.uid, m.subject, m.sender_name, m.sender_email, m.date, m.size_bytes,
	m.has_attachments, m.attachment_count, m.archived_at, m.deleted_from_server_at, m.body_preview,
	mb.name, ma.name`

const searchJoin = `FROM messages_fts fts
	JOIN messages m ON m.id = fts.message_db_id
	JOIN mailboxes mb ON mb.id = m.mailbox_id
	JOIN mail_accounts ma ON ma.id = m.account_id`

// Search uses FTS JOIN + keyset cursor — O(limit) rows per request (no OFFSET, no ID materialization).
func (r *SearchRepo) Search(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	query := strings.TrimSpace(q.Query)
	limit := q.Limit
	if limit < 1 || limit > 200 {
		limit = 50
	}
	fetchLimit := limit + 1

	if query == "" {
		return domain.SearchResult{Limit: limit}, nil
	}

	ftsQuery := search.EscapeQuery(query)
	if ftsQuery == "" {
		return domain.SearchResult{Limit: limit}, nil
	}

	result, err := r.searchFTS(ctx, q, ftsQuery, limit, fetchLimit)
	if err != nil {
		return r.searchLikeFallback(ctx, q, query, limit, fetchLimit)
	}
	return result, nil
}

func (r *SearchRepo) searchFTS(ctx context.Context, q domain.SearchQuery, ftsQuery string, limit, fetchLimit int) (domain.SearchResult, error) {
	where, args := buildSearchWhere(q, []string{"fts MATCH ?"}, []any{ftsQuery})
	whereClause := strings.Join(where, " AND ")

	var total int
	if q.Cursor == "" {
		if err := r.db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COUNT(*) %s WHERE %s`, searchJoin, whereClause),
			args...,
		).Scan(&total); err != nil {
			return domain.SearchResult{}, err
		}
	}

	pageArgs := append(append([]any{}, args...), fetchLimit)
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s %s WHERE %s ORDER BY m.date DESC, m.id DESC LIMIT ?`, searchSelectCols, searchJoin, whereClause),
		pageArgs...,
	)
	if err != nil {
		return domain.SearchResult{}, err
	}
	defer rows.Close()

	return finishSearchPage(rows, total, limit)
}

func (r *SearchRepo) searchLikeFallback(ctx context.Context, q domain.SearchQuery, query string, limit, fetchLimit int) (domain.SearchResult, error) {
	like := "%" + query + "%"
	textWhere := "(m.subject LIKE ? OR m.sender_email LIKE ? OR m.sender_name LIKE ? OR m.body_preview LIKE ?)"
	baseArgs := []any{like, like, like, like}

	where, args := buildSearchWhere(q, []string{textWhere}, baseArgs)
	whereClause := strings.Join(where, " AND ")

	fromClause := `FROM messages m
		JOIN mailboxes mb ON mb.id = m.mailbox_id
		JOIN mail_accounts ma ON ma.id = m.account_id`

	var total int
	if q.Cursor == "" {
		_ = r.db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COUNT(*) %s WHERE %s`, fromClause, whereClause),
			args...,
		).Scan(&total)
	}

	pageArgs := append(append([]any{}, args...), fetchLimit)
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s %s WHERE %s ORDER BY m.date DESC, m.id DESC LIMIT ?`, searchSelectCols, fromClause, whereClause),
		pageArgs...,
	)
	if err != nil {
		return domain.SearchResult{}, err
	}
	defer rows.Close()

	return finishSearchPage(rows, total, limit)
}

func buildSearchWhere(q domain.SearchQuery, where []string, args []any) ([]string, []any) {
	filterWhere, filterArgs := buildSearchFilters(q)
	where = append(where, filterWhere...)
	args = append(args, filterArgs...)
	if q.Cursor != "" {
		if cursorDate, cursorID, ok := decodeCursor(q.Cursor); ok {
			cursorWhereDesc(&where, &args, cursorDate, cursorID)
		}
	}
	return where, args
}

func buildSearchFilters(q domain.SearchQuery) ([]string, []any) {
	var where []string
	var args []any
	appendScopedAccounts(&where, &args, "m.account_id", q.AccountID, q.ScopedAccountIDs)
	if q.Sender != "" {
		where = append(where, "m.sender_email LIKE ?")
		args = append(args, "%"+q.Sender+"%")
	}
	if q.HasAttachments {
		where = append(where, "m.has_attachments = 1")
	}
	switch q.ArchiveState {
	case "archived":
		where = append(where, "m.archived_at IS NOT NULL")
	case "deleted_from_server":
		where = append(where, "m.deleted_from_server_at IS NOT NULL")
	}
	if q.MinSize != "" {
		where = append(where, "m.size_bytes >= ?")
		args = append(args, q.MinSize)
	}
	return where, args
}

func finishSearchPage(rows *sql.Rows, total, limit int) (domain.SearchResult, error) {
	results, err := scanSearchRows(rows)
	if err != nil {
		return domain.SearchResult{}, err
	}
	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	nextCursor := ""
	if hasMore && len(results) > 0 {
		last := results[len(results)-1]
		nextCursor = encodeCursor(last.Date, last.ID)
	}
	return domain.SearchResult{
		Messages:   results,
		Total:      total,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Limit:      limit,
	}, nil
}

func scanSearchRows(rows *sql.Rows) ([]domain.MessageSummary, error) {
	var results []domain.MessageSummary
	for rows.Next() {
		var id string
		var uid int64
		var subject, senderName, senderEmail, date sql.NullString
		var sizeBytes int64
		var hasAtt, attachmentCount int
		var archivedAt, deletedAt, bodyPreview sql.NullString
		var mailboxName, accountName string
		if err := rows.Scan(&id, &uid, &subject, &senderName, &senderEmail, &date, &sizeBytes,
			&hasAtt, &attachmentCount, &archivedAt, &deletedAt, &bodyPreview,
			&mailboxName, &accountName); err != nil {
			continue
		}
		results = append(results, scanMessageSummary(id, uid, subject, senderName, senderEmail, date,
			sizeBytes, hasAtt, attachmentCount, bodyPreview, archivedAt, deletedAt,
			mailboxName, accountName))
	}
	return results, rows.Err()
}
