package port

import (
	"context"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type BlobReader interface {
	Read(accountID, sha256 string) ([]byte, error)
}

type CredentialVault interface {
	EncryptCredentials(password string) (string, error)
	DecryptPassword(encrypted string) (string, error)
}

type AccountRepository interface {
	ListForSubject(ctx context.Context, sub domain.Subject) ([]domain.MailAccount, error)
	Get(ctx context.Context, id string) (domain.MailAccount, error)
	GetOwnerID(ctx context.Context, id string) (string, error)
	ListOwnedIDs(ctx context.Context, userID string) ([]string, error)
	Create(ctx context.Context, id string, in domain.CreateAccountInput, credEnc, ownerID string) error
	Delete(ctx context.Context, id string) error
	Exists(ctx context.Context, id string) (bool, error)
	GetIMAPCredentials(ctx context.Context, id string) (domain.IMAPCredentials, error)
}

type MailboxRepository interface {
	ListByAccount(ctx context.Context, accountID string) ([]domain.Mailbox, error)
}

type MessageRepository interface {
	List(ctx context.Context, filter domain.MessageListFilter) (domain.MessagePage, error)
	GetDetail(ctx context.Context, id string, loadImages bool) (domain.MessageDetail, error)
	GetAccountID(ctx context.Context, messageID string) (string, error)
	GetBlobRef(ctx context.Context, id string) (accountID, sha256 string, err error)
	MarkRestored(ctx context.Context, id string) error
	MarkDeletedFromServer(ctx context.Context, ids []string) error
}

type AttachmentRepository interface {
	ListByMessage(ctx context.Context, messageID string) ([]domain.AttachmentMeta, error)
	GetMeta(ctx context.Context, messageID, partPath string) (filename, contentType string, err error)
}

type SearchRepository interface {
	Search(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error)
}

type InsightsRepository interface {
	GetForSubject(ctx context.Context, sub domain.Subject, scopedAccountIDs []string) (domain.Insights, error)
}

type CleanupRepository interface {
	Preview(ctx context.Context, filter domain.CleanupFilter) (domain.CleanupPreview, error)
	ListJobsForSubject(ctx context.Context, sub domain.Subject, ownedAccountIDs []string) ([]domain.CleanupJob, error)
	CreateJob(ctx context.Context, id string, in domain.CreateCleanupJobInput, filterJSON string) error
	UpdateJob(ctx context.Context, id string, in domain.UpdateCleanupJobInput, filterJSON string) error
	DeleteJob(ctx context.Context, id string) error
	GetJob(ctx context.Context, id string) (domain.CleanupJob, error)
	CreateRun(ctx context.Context, runID, jobID string) error
	UpdateRun(ctx context.Context, runID string, status string, result domain.JobExecutionResult, errMsg string) error
	ListRuns(ctx context.Context, jobID string) ([]domain.CleanupRun, error)
	ListCandidates(ctx context.Context, filter domain.CleanupFilter, limit int) ([]CleanupCandidate, error)
}

type CleanupCandidate struct {
	ID          string
	UID         int64
	MailboxID   string
	MailboxName string
	ArchivedAt  *string
	BlobSHA256  *string
	SizeBytes   int64
}

type ExportRepository interface {
	ListArchivedBlobs(ctx context.Context, messageIDs []string) ([]ExportBlob, error)
	GetRestoreTarget(ctx context.Context, messageID string) (RestoreTarget, error)
}

type ExportBlob struct {
	Subject   string
	AccountID string
	BlobSHA256 string
}

type RestoreTarget struct {
	AccountID   string
	BlobSHA256  string
	MailboxName string
	FlagsJSON   *string
	Date        *string
}

type UserRepository interface {
	EmailExists(ctx context.Context, email string) (bool, error)
	CreatePending(ctx context.Context, userID, email, hash, fullName string) error
	Activate(ctx context.Context, email string) error
	FindByEmail(ctx context.Context, email string) (id, hash string, isActive bool, err error)
	GetPasswordHash(ctx context.Context, userID string) (string, error)
	UpdatePassword(ctx context.Context, emailOrID string, byID bool, hash string) error
	GetProfile(ctx context.Context, userID string) (domain.User, error)
	StoreOTP(ctx context.Context, id, identifier, codeHash, otpType, expiresAt string) error
	ConsumeOTP(ctx context.Context, identifier, codeHash, otpType string) error
	StoreToken(ctx context.Context, id, userID, tokenHash, expiresAt string) error
	ValidateToken(ctx context.Context, tokenHash string) (userID, role string, err error)
	RevokeToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error
	ActiveEmailExists(ctx context.Context, email string) (bool, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	UpdateUserRole(ctx context.Context, userID, role string) error
	SetUserActive(ctx context.Context, userID string, active bool) error
}

type OTPSender interface {
	SendOTP(ctx context.Context, to, code string) error
}

type SyncRunner interface {
	RunAll(ctx context.Context) error
	RunAccount(ctx context.Context, accountID string) error
}
