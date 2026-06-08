package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type InsightsRepo struct {
	db *sql.DB
}

func NewInsightsRepo(db *sql.DB) *InsightsRepo {
	return &InsightsRepo{db: db}
}

func (r *InsightsRepo) GetForSubject(ctx context.Context, sub domain.Subject, scopedAccountIDs []string) (domain.Insights, error) {
	var ins domain.Insights
	scopeSQL, scopeArgs := insightsScope(sub, scopedAccountIDs)

	topBySize, _ := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT sender_email, COUNT(*) as count, SUM(size_bytes) as total_bytes
		FROM   messages WHERE archived_at IS NOT NULL %s
		GROUP  BY sender_email ORDER BY total_bytes DESC LIMIT 50`, scopeSQL), scopeArgs...)
	if topBySize != nil {
		defer topBySize.Close()
		for topBySize.Next() {
			var s domain.SenderStat
			if err := topBySize.Scan(&s.Email, &s.Count, &s.Bytes); err == nil {
				ins.TopSenders = append(ins.TopSenders, s)
			}
		}
	}

	byYear, _ := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT STRFTIME('%%Y', date) as year, COUNT(*), SUM(size_bytes)
		FROM   messages WHERE date IS NOT NULL AND archived_at IS NOT NULL %s
		GROUP  BY year ORDER BY year`, scopeSQL), scopeArgs...)
	if byYear != nil {
		defer byYear.Close()
		for byYear.Next() {
			var y domain.YearStat
			if err := byYear.Scan(&y.Year, &y.Count, &y.Bytes); err == nil {
				ins.SizeByYear = append(ins.SizeByYear, y)
			}
		}
	}

	_ = r.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(size_bytes),0) FROM messages
		WHERE archived_at IS NOT NULL AND blob_sha256 IS NOT NULL AND deleted_from_server_at IS NULL %s`, scopeSQL),
		scopeArgs...).Scan(&ins.ReclaimableBytes)

	_ = r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(size_bytes),0) FROM messages WHERE 1=1 %s`, scopeSQL),
		scopeArgs...).Scan(&ins.TotalMessages, &ins.TotalBytes)
	_ = r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(size_bytes),0) FROM messages WHERE archived_at IS NOT NULL %s`, scopeSQL),
		scopeArgs...).Scan(&ins.ArchivedMessages, &ins.ArchivedBytes)

	return ins, nil
}

func insightsScope(sub domain.Subject, scopedAccountIDs []string) (string, []any) {
	if sub.IsAdmin() {
		return "", nil
	}
	if len(scopedAccountIDs) == 0 {
		return " AND 1=0", nil
	}
	return " AND account_id IN (" + placeholders(len(scopedAccountIDs)) + ")", toAnySlice(scopedAccountIDs)
}
