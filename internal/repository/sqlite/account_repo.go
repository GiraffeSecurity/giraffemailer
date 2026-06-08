package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type AccountRepo struct {
	db *sql.DB
}

func NewAccountRepo(db *sql.DB) *AccountRepo {
	return &AccountRepo{db: db}
}

func (r *AccountRepo) ListForSubject(ctx context.Context, sub domain.Subject) ([]domain.MailAccount, error) {
	var rows *sql.Rows
	var err error
	if sub.IsAdmin() {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, email_address, imap_host, imap_port, imap_use_tls,
			       username, sync_enabled, last_sync_at
			FROM   mail_accounts
			ORDER  BY name
		`)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, email_address, imap_host, imap_port, imap_use_tls,
			       username, sync_enabled, last_sync_at
			FROM   mail_accounts
			WHERE  owner_id = ?
			ORDER  BY name
		`, sub.UserID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

func scanAccounts(rows *sql.Rows) ([]domain.MailAccount, error) {
	var out []domain.MailAccount
	for rows.Next() {
		var a domain.MailAccount
		var useTLS, syncEnabled int
		if err := rows.Scan(&a.ID, &a.Name, &a.EmailAddress, &a.IMAPHost, &a.IMAPPort,
			&useTLS, &a.Username, &syncEnabled, &a.LastSyncAt); err != nil {
			continue
		}
		a.UseTLS = useTLS == 1
		a.SyncEnabled = syncEnabled == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AccountRepo) Get(ctx context.Context, id string) (domain.MailAccount, error) {
	var a domain.MailAccount
	var useTLS, syncEnabled int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, email_address, imap_host, imap_port, imap_use_tls,
		       username, sync_enabled, last_sync_at
		FROM   mail_accounts WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.EmailAddress, &a.IMAPHost, &a.IMAPPort,
		&useTLS, &a.Username, &syncEnabled, &a.LastSyncAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MailAccount{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.MailAccount{}, err
	}
	a.UseTLS = useTLS == 1
	a.SyncEnabled = syncEnabled == 1
	return a, nil
}

func (r *AccountRepo) GetOwnerID(ctx context.Context, id string) (string, error) {
	var ownerID sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT owner_id FROM mail_accounts WHERE id = ?`, id).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !ownerID.Valid {
		return "", nil
	}
	return ownerID.String, nil
}

func (r *AccountRepo) ListOwnedIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM mail_accounts WHERE owner_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

func (r *AccountRepo) Create(ctx context.Context, id string, in domain.CreateAccountInput, credEnc, ownerID string) error {
	useTLS := 0
	if in.UseTLS {
		useTLS = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO mail_accounts(id, name, email_address, imap_host, imap_port, imap_use_tls, username, credentials_encrypted, owner_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, in.Name, in.EmailAddress, in.IMAPHost, in.IMAPPort, useTLS, in.Username, credEnc, ownerID)
	return err
}

func (r *AccountRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM mail_accounts WHERE id = ?`, id)
	return err
}

func (r *AccountRepo) Exists(ctx context.Context, id string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mail_accounts WHERE id = ?`, id).Scan(&count)
	return count > 0, err
}

func (r *AccountRepo) GetIMAPCredentials(ctx context.Context, id string) (domain.IMAPCredentials, error) {
	var creds domain.IMAPCredentials
	var useTLS int
	err := r.db.QueryRowContext(ctx,
		`SELECT imap_host, imap_port, imap_use_tls, username, credentials_encrypted FROM mail_accounts WHERE id = ?`, id,
	).Scan(&creds.Host, &creds.Port, &useTLS, &creds.Username, &creds.CredEnc)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IMAPCredentials{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.IMAPCredentials{}, err
	}
	creds.UseTLS = useTLS == 1
	return creds, nil
}
