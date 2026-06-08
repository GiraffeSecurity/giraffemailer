package archive

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	imap "github.com/emersion/go-imap/v2"
	"github.com/GiraffeSecurity/giraffemailer/internal/mailconn"
	"github.com/GiraffeSecurity/giraffemailer/internal/store"
	"github.com/gofrs/uuid/v5"
	"github.com/rs/zerolog/log"
)

type Engine struct {
	db             *sql.DB
	blobStore      *store.BlobStore
	batchSizeBytes int64
	workerCount    int
	tracker        *Tracker
	accountMu      sync.Map

	decryptCred func(encrypted string) (username, password string, err error)
}

func NewEngine(
	db *sql.DB,
	bs *store.BlobStore,
	batchSizeBytes int64,
	workerCount int,
	decryptCred func(string) (string, string, error),
) *Engine {
	return &Engine{
		db:             db,
		blobStore:      bs,
		batchSizeBytes: batchSizeBytes,
		workerCount:    workerCount,
		tracker:        NewTracker(),
		decryptCred:    decryptCred,
	}
}

func (e *Engine) Tracker() *Tracker { return e.tracker }

func (e *Engine) RunAll(ctx context.Context) error {
	rows, err := e.db.QueryContext(ctx,
		`SELECT id, name, imap_host, imap_port, imap_use_tls, username, credentials_encrypted FROM mail_accounts WHERE sync_enabled = 1`,
	)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}

	type accountRow struct {
		id, name, host    string
		port              int
		useTLS            int
		username, credEnc string
	}
	var accounts []accountRow
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(&a.id, &a.name, &a.host, &a.port, &a.useTLS, &a.username, &a.credEnc); err != nil {
			continue
		}
		accounts = append(accounts, a)
	}
	rows.Close()

	sem := make(chan struct{}, e.workerCount)
	var wg sync.WaitGroup

	for _, acct := range accounts {
		acct := acct
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			if err := e.withAccountLock(acct.id, func() error {
				return e.runAccount(ctx, acct.id, acct.name, acct.host, acct.port, acct.useTLS == 1, acct.username, acct.credEnc)
			}); err != nil {
				log.Error().Err(err).Str("account", acct.name).Msg("account sync failed")
			}
		}()
	}

	wg.Wait()
	return nil
}

func (e *Engine) RunAccount(ctx context.Context, accountID string) error {
	return e.withAccountLock(accountID, func() error {
		var name, host, username, credEnc string
		var port, useTLS int
		err := e.db.QueryRowContext(ctx,
			`SELECT name, imap_host, imap_port, imap_use_tls, username, credentials_encrypted FROM mail_accounts WHERE id = ?`,
			accountID,
		).Scan(&name, &host, &port, &useTLS, &username, &credEnc)
		if err != nil {
			return fmt.Errorf("account %s not found: %w", accountID, err)
		}
		return e.runAccount(ctx, accountID, name, host, port, useTLS == 1, username, credEnc)
	})
}

func (e *Engine) withAccountLock(accountID string, fn func() error) error {
	v, _ := e.accountMu.LoadOrStore(accountID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func (e *Engine) runAccount(
	ctx context.Context,
	accountID, name, host string,
	port int, useTLS bool,
	username, credEnc string,
) error {
	prog := e.tracker.get(accountID)
	prog.setIndexing(0)

	_, password, err := e.decryptCred(credEnc)
	if err != nil {
		prog.setError(err)
		return fmt.Errorf("decrypt credentials: %w", err)
	}

	cl, err := mailconn.Connect(host, port, useTLS, username, password)
	if err != nil {
		prog.setError(err)
		return fmt.Errorf("connect account %s: %w", name, err)
	}
	defer cl.Close()

	mailboxes, err := cl.ListMailboxes(ctx)
	if err != nil {
		prog.setError(err)
		return fmt.Errorf("list mailboxes %s: %w", name, err)
	}

	if err := e.ensureMailboxes(ctx, accountID, mailboxes); err != nil {
		return err
	}

	var totalIndexed int
	for _, mb := range mailboxes {
		if isSystemMailbox(mb) {
			continue
		}
		mailboxID, err := e.mailboxID(ctx, accountID, mb.Name)
		if err != nil {
			log.Warn().Str("mailbox", mb.Name).Err(err).Msg("could not resolve mailbox ID")
			continue
		}
		n, err := IndexMailbox(ctx, e.db, cl, accountID, mailboxID, mb.Name)
		if err != nil {
			log.Warn().Err(err).Str("mailbox", mb.Name).Msg("index mailbox failed (continuing)")
			continue
		}
		totalIndexed += n
		prog.incIndexed(int64(n))
	}

	log.Info().Int("indexed", totalIndexed).Str("account", name).Msg("phase 1 complete")

	var totalArchived int
	for _, mb := range mailboxes {
		if isSystemMailbox(mb) {
			continue
		}
		mailboxID, err := e.mailboxID(ctx, accountID, mb.Name)
		if err != nil {
			continue
		}
		n, bytes, err := ArchiveMailbox(ctx, e.db, e.blobStore, cl, accountID, mailboxID, mb.Name, e.batchSizeBytes, prog)
		if err != nil {
			log.Warn().Err(err).Str("mailbox", mb.Name).Msg("archive mailbox failed (continuing)")
			continue
		}
		totalArchived += n
		log.Info().Int("archived", n).Int64("bytes", bytes).Str("mailbox", mb.Name).Msg("mailbox archived")
	}

	_, _ = e.db.ExecContext(ctx,
		`UPDATE mail_accounts SET last_sync_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		accountID,
	)

	prog.setDone()
	log.Info().Int("total_archived", totalArchived).Str("account", name).Msg("sync complete")
	return nil
}

func (e *Engine) ensureMailboxes(ctx context.Context, accountID string, mailboxes []*mailconn.MailboxInfo) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO mailboxes(id, account_id, name, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, mb := range mailboxes {
		if isSystemMailbox(mb) {
			continue
		}
		if _, err := stmt.ExecContext(ctx, uuid.Must(uuid.NewV7()).String(), accountID, mb.Name); err != nil {
			return fmt.Errorf("ensure mailbox %s: %w", mb.Name, err)
		}
	}
	return tx.Commit()
}

func (e *Engine) mailboxID(ctx context.Context, accountID, name string) (string, error) {
	var id string
	err := e.db.QueryRowContext(ctx,
		`SELECT id FROM mailboxes WHERE account_id = ? AND name = ?`, accountID, name,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("mailbox %q not found: %w", name, err)
	}
	return id, nil
}

func isSystemMailbox(mb *mailconn.MailboxInfo) bool {
	if mb == nil {
		return true
	}
	for _, attr := range mb.Flags {
		if attr == imap.MailboxAttrNoSelect || attr == imap.MailboxAttrNonExistent {
			return true
		}
	}
	return false
}
