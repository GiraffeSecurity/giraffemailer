package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	cleanupsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/cleanup"
)

type CleanupRepo struct {
	db *sql.DB
}

func NewCleanupRepo(db *sql.DB) *CleanupRepo {
	return &CleanupRepo{db: db}
}

func (r *CleanupRepo) Preview(ctx context.Context, filter domain.CleanupFilter) (domain.CleanupPreview, error) {
	clause, args := cleanupsvc.BuildSQL(filter)
	var preview domain.CleanupPreview
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(m.size_bytes),0)
		             FROM messages m JOIN mailboxes mb ON mb.id = m.mailbox_id
		             WHERE %s`, clause),
		args...,
	).Scan(&preview.Count, &preview.TotalBytes)
	return preview, err
}

func (r *CleanupRepo) ListJobsForSubject(ctx context.Context, sub domain.Subject, ownedAccountIDs []string) ([]domain.CleanupJob, error) {
	var rows *sql.Rows
	var err error
	if sub.IsAdmin() {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, account_id, filter_json, action, move_target_folder, created_by, created_at
			FROM   cleanup_jobs ORDER BY created_at DESC
		`)
	} else if len(ownedAccountIDs) == 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, account_id, filter_json, action, move_target_folder, created_by, created_at
			FROM   cleanup_jobs WHERE created_by = ? ORDER BY created_at DESC
		`, sub.UserID)
	} else {
		query := fmt.Sprintf(`
			SELECT id, name, account_id, filter_json, action, move_target_folder, created_by, created_at
			FROM   cleanup_jobs
			WHERE  created_by = ? OR account_id IN (%s)
			ORDER  BY created_at DESC`, placeholders(len(ownedAccountIDs)))
		args := append([]any{sub.UserID}, toAnySlice(ownedAccountIDs)...)
		rows, err = r.db.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CleanupJob
	for rows.Next() {
		var j domain.CleanupJob
		var createdBy sql.NullString
		if err := rows.Scan(&j.ID, &j.Name, &j.AccountID, &j.FilterJSON,
			&j.Action, &j.MoveTargetFolder, &createdBy, &j.CreatedAt); err != nil {
			continue
		}
		if createdBy.Valid {
			j.CreatedBy = createdBy.String
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func (r *CleanupRepo) CreateJob(ctx context.Context, id string, in domain.CreateCleanupJobInput, filterJSON string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cleanup_jobs(id, name, account_id, filter_json, action, move_target_folder, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, in.Name, in.Filter.AccountID, filterJSON, in.Action, in.MoveTargetFolder, in.CreatedBy)
	return err
}

func (r *CleanupRepo) UpdateJob(ctx context.Context, id string, in domain.UpdateCleanupJobInput, filterJSON string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE cleanup_jobs
		SET    name = ?, account_id = ?, filter_json = ?, action = ?, move_target_folder = ?,
		       updated_at = CURRENT_TIMESTAMP
		WHERE  id = ?
	`, in.Name, in.Filter.AccountID, filterJSON, in.Action, in.MoveTargetFolder, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *CleanupRepo) DeleteJob(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM cleanup_jobs WHERE id = ?`, id)
	return err
}

func (r *CleanupRepo) GetJob(ctx context.Context, id string) (domain.CleanupJob, error) {
	var j domain.CleanupJob
	var createdBy sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, account_id, filter_json, action, move_target_folder, created_by, created_at FROM cleanup_jobs WHERE id = ?`, id,
	).Scan(&j.ID, &j.Name, &j.AccountID, &j.FilterJSON, &j.Action, &j.MoveTargetFolder, &createdBy, &j.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CleanupJob{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CleanupJob{}, err
	}
	if createdBy.Valid {
		j.CreatedBy = createdBy.String
	}
	return j, nil
}

func (r *CleanupRepo) CreateRun(ctx context.Context, runID, jobID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cleanup_job_runs(id, job_id, status, started_at)
		VALUES (?, ?, 'running', CURRENT_TIMESTAMP)
	`, runID, jobID)
	return err
}

func (r *CleanupRepo) UpdateRun(ctx context.Context, runID string, status string, result domain.JobExecutionResult, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE cleanup_job_runs
		SET    status = ?, total_candidates = ?, processed = ?, skipped_unarchived = ?,
		       freed_bytes = ?, error_message = ?, finished_at = CURRENT_TIMESTAMP
		WHERE  id = ?
	`, status, result.TotalCandidates, result.Processed, result.SkippedUnarchived,
		result.FreedBytes, errMsg, runID)
	return err
}

func (r *CleanupRepo) ListRuns(ctx context.Context, jobID string) ([]domain.CleanupRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, status, total_candidates, processed, skipped_unarchived,
		       freed_bytes, error_message, started_at, finished_at
		FROM   cleanup_job_runs WHERE job_id = ? ORDER BY started_at DESC LIMIT 20
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CleanupRun
	for rows.Next() {
		var rr domain.CleanupRun
		if err := rows.Scan(&rr.ID, &rr.Status, &rr.TotalCandidates, &rr.Processed,
			&rr.SkippedUnarchived, &rr.FreedBytes, &rr.ErrorMessage,
			&rr.StartedAt, &rr.FinishedAt); err != nil {
			continue
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *CleanupRepo) ListCandidates(ctx context.Context, filter domain.CleanupFilter, limit int) ([]port.CleanupCandidate, error) {
	clause, args := cleanupsvc.BuildSQL(filter)
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`
			SELECT m.id, m.uid, m.mailbox_id, mb.name, m.archived_at, m.blob_sha256, m.size_bytes
			FROM   messages m JOIN mailboxes mb ON mb.id = m.mailbox_id
			WHERE  %s
			LIMIT  ?
		`, clause),
		append(args, limit)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []port.CleanupCandidate
	for rows.Next() {
		var c port.CleanupCandidate
		if err := rows.Scan(&c.ID, &c.UID, &c.MailboxID, &c.MailboxName,
			&c.ArchivedAt, &c.BlobSHA256, &c.SizeBytes); err != nil {
			continue
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}
