package account

import (
	"context"
	"errors"

	"github.com/GiraffeSecurity/giraffemailer/internal/authz"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/GiraffeSecurity/giraffemailer/internal/service/mail"
	"github.com/gofrs/uuid/v5"
)

type Service struct {
	accounts  port.AccountRepository
	vault     port.CredentialVault
	connector *mail.Connector
	sync      port.SyncRunner
}

func NewService(accounts port.AccountRepository, vault port.CredentialVault, connector *mail.Connector, sync port.SyncRunner) *Service {
	return &Service{accounts: accounts, vault: vault, connector: connector, sync: sync}
}

func (s *Service) List(ctx context.Context) ([]domain.MailAccount, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	return s.accounts.ListForSubject(ctx, sub)
}

func (s *Service) Get(ctx context.Context, id string) (domain.MailAccount, error) {
	if err := authz.EnsureAccountAccess(ctx, s.accounts, id); err != nil {
		return domain.MailAccount{}, err
	}
	return s.accounts.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, in domain.CreateAccountInput) (string, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return "", domain.ErrUnauthorized
	}
	if in.Name == "" || in.EmailAddress == "" || in.IMAPHost == "" || in.Username == "" || in.Password == "" {
		return "", domain.ErrInvalidInput
	}
	if in.IMAPPort == 0 {
		in.IMAPPort = 993
	}
	if in.IMAPPort < 1 || in.IMAPPort > 65535 {
		return "", domain.ErrInvalidInput
	}

	credEnc, err := s.vault.EncryptCredentials(in.Password)
	if err != nil {
		return "", err
	}

	id := uuid.Must(uuid.NewV7()).String()
	if err := s.accounts.Create(ctx, id, in, credEnc, sub.UserID); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := authz.EnsureAccountAccess(ctx, s.accounts, id); err != nil {
		return err
	}
	return s.accounts.Delete(ctx, id)
}

type TestResult struct {
	Success bool
	Error   string
}

func (s *Service) TestConnection(ctx context.Context, id string) (TestResult, error) {
	if err := authz.EnsureAccountAccess(ctx, s.accounts, id); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return TestResult{}, domain.ErrForbidden
		}
		return TestResult{}, err
	}
	cl, err := s.connector.Open(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return TestResult{}, domain.ErrNotFound
	}
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}, nil
	}
	_ = cl.Close()
	return TestResult{Success: true}, nil
}

func (s *Service) TriggerSync(ctx context.Context, id string) error {
	if err := authz.EnsureAccountAccess(ctx, s.accounts, id); err != nil {
		return err
	}
	exists, err := s.accounts.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrNotFound
	}
	go func() {
		_ = s.sync.RunAccount(context.Background(), id)
	}()
	return nil
}
