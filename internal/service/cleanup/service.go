package cleanup

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/GiraffeSecurity/giraffemailer/internal/authz"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/gofrs/uuid/v5"
)

type Service struct {
	cleanup  port.CleanupRepository
	accounts port.AccountRepository
	executor *Executor
}

func NewService(cleanup port.CleanupRepository, accounts port.AccountRepository, executor *Executor) *Service {
	return &Service{cleanup: cleanup, accounts: accounts, executor: executor}
}

func (s *Service) Preview(ctx context.Context, filter domain.CleanupFilter) (domain.CleanupPreview, error) {
	if filter.AccountID == "" {
		return domain.CleanupPreview{}, domain.ErrInvalidInput
	}
	if err := authz.EnsureAccountAccess(ctx, s.accounts, filter.AccountID); err != nil {
		return domain.CleanupPreview{}, err
	}
	return s.cleanup.Preview(ctx, filter)
}

func (s *Service) ListJobs(ctx context.Context) ([]domain.CleanupJob, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	owned, err := authz.AccessibleAccountIDs(ctx, s.accounts)
	if err != nil {
		return nil, err
	}
	return s.cleanup.ListJobsForSubject(ctx, sub, owned)
}

func (s *Service) CreateJob(ctx context.Context, in domain.CreateCleanupJobInput) (string, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return "", domain.ErrUnauthorized
	}
	if in.Name == "" || in.Filter.AccountID == "" {
		return "", domain.ErrInvalidInput
	}
	if err := authz.EnsureAccountAccess(ctx, s.accounts, in.Filter.AccountID); err != nil {
		return "", err
	}
	if in.Action != "delete" && in.Action != "move" {
		return "", domain.ErrInvalidInput
	}
	if in.Action == "move" && (in.MoveTargetFolder == nil || *in.MoveTargetFolder == "") {
		return "", domain.ErrInvalidInput
	}
	in.CreatedBy = sub.UserID

	filterJSON, _ := json.Marshal(in.Filter)
	id := uuid.Must(uuid.NewV7()).String()
	if err := s.cleanup.CreateJob(ctx, id, in, string(filterJSON)); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) ensureJobAccess(ctx context.Context, job domain.CleanupJob) error {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}
	if sub.IsAdmin() {
		return nil
	}
	if job.CreatedBy == sub.UserID {
		return nil
	}
	if job.AccountID != "" {
		return authz.EnsureAccountAccess(ctx, s.accounts, job.AccountID)
	}
	return domain.ErrForbidden
}

func (s *Service) DeleteJob(ctx context.Context, id string) error {
	job, err := s.cleanup.GetJob(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureJobAccess(ctx, job); err != nil {
		return err
	}
	return s.cleanup.DeleteJob(ctx, id)
}

func (s *Service) UpdateJob(ctx context.Context, id string, in domain.UpdateCleanupJobInput) error {
	if in.Name == "" || in.Filter.AccountID == "" {
		return domain.ErrInvalidInput
	}
	if in.Action != "delete" && in.Action != "move" {
		return domain.ErrInvalidInput
	}
	if in.Action == "move" && (in.MoveTargetFolder == nil || *in.MoveTargetFolder == "") {
		return domain.ErrInvalidInput
	}
	job, err := s.cleanup.GetJob(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureJobAccess(ctx, job); err != nil {
		return err
	}
	if err := authz.EnsureAccountAccess(ctx, s.accounts, in.Filter.AccountID); err != nil {
		return err
	}
	filterJSON, _ := json.Marshal(in.Filter)
	return s.cleanup.UpdateJob(ctx, id, in, string(filterJSON))
}

type RunResult struct {
	RunID  string
	Status string
}

func (s *Service) Run(ctx context.Context, jobID string) (RunResult, error) {
	job, err := s.cleanup.GetJob(ctx, jobID)
	if errors.Is(err, domain.ErrNotFound) {
		return RunResult{}, domain.ErrNotFound
	}
	if err != nil {
		return RunResult{}, err
	}
	if err := s.ensureJobAccess(ctx, job); err != nil {
		return RunResult{}, err
	}

	var filter domain.CleanupFilter
	if err := json.Unmarshal([]byte(job.FilterJSON), &filter); err != nil {
		filter = domain.CleanupFilter{}
	}
	if filter.AccountID == "" {
		filter.AccountID = job.AccountID
	}

	runID := uuid.Must(uuid.NewV7()).String()
	if err := s.cleanup.CreateRun(ctx, runID, jobID); err != nil {
		return RunResult{}, err
	}

	moveTarget := ""
	if job.MoveTargetFolder != nil {
		moveTarget = *job.MoveTargetFolder
	}

	go func() {
		bgCtx := context.Background()
		result, execErr := s.executor.Execute(bgCtx, job.AccountID, filter, job.Action, moveTarget)
		status := "done"
		errMsg := ""
		if execErr != nil {
			status = "failed"
			errMsg = execErr.Error()
		}
		_ = s.cleanup.UpdateRun(bgCtx, runID, status, result, errMsg)
	}()

	return RunResult{RunID: runID, Status: "running"}, nil
}

func (s *Service) ListRuns(ctx context.Context, jobID string) ([]domain.CleanupRun, error) {
	job, err := s.cleanup.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureJobAccess(ctx, job); err != nil {
		return nil, err
	}
	return s.cleanup.ListRuns(ctx, jobID)
}
