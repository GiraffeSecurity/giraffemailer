package export

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/GiraffeSecurity/giraffemailer/internal/authz"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/GiraffeSecurity/giraffemailer/internal/service/mail"
)

type Service struct {
	export    port.ExportRepository
	messages  port.MessageRepository
	accounts  port.AccountRepository
	blob      port.BlobReader
	connector *mail.Connector
}

func NewService(export port.ExportRepository, messages port.MessageRepository, accounts port.AccountRepository, blob port.BlobReader, connector *mail.Connector) *Service {
	return &Service{export: export, messages: messages, accounts: accounts, blob: blob, connector: connector}
}

type ExportInput struct {
	MessageIDs []string
	Format     string
}

func (s *Service) Export(ctx context.Context, in ExportInput, w io.Writer) error {
	if len(in.MessageIDs) == 0 {
		return domain.ErrInvalidInput
	}
	if len(in.MessageIDs) > 1000 {
		return domain.ErrInvalidInput
	}
	for _, id := range in.MessageIDs {
		accountID, err := s.messages.GetAccountID(ctx, id)
		if err != nil {
			return err
		}
		if err := authz.EnsureAccountAccess(ctx, s.accounts, accountID); err != nil {
			return err
		}
	}
	format := in.Format
	if format == "" {
		format = "mbox"
	}
	if format != "mbox" && format != "zip" {
		return domain.ErrInvalidInput
	}

	blobs, err := s.export.ListArchivedBlobs(ctx, in.MessageIDs)
	if err != nil {
		return err
	}

	if format == "mbox" {
		for _, m := range blobs {
			raw, err := s.blob.Read(m.AccountID, m.BlobSHA256)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "From MAILER-DAEMON %s\r\n", time.Now().UTC().Format("Mon Jan 02 15:04:05 2006"))
			w.Write(raw)
			fmt.Fprintf(w, "\r\n")
		}
		return nil
	}

	zw := zip.NewWriter(w)
	defer zw.Close()
	for i, m := range blobs {
		raw, err := s.blob.Read(m.AccountID, m.BlobSHA256)
		if err != nil {
			continue
		}
		f, err := zw.Create(fmt.Sprintf("%04d.eml", i+1))
		if err != nil {
			continue
		}
		f.Write(raw)
	}
	return nil
}

type RestoreResult struct {
	Message string
}

func (s *Service) Restore(ctx context.Context, messageID string) (RestoreResult, error) {
	accountID, err := s.messages.GetAccountID(ctx, messageID)
	if err != nil {
		return RestoreResult{}, err
	}
	if err := authz.EnsureAccountAccess(ctx, s.accounts, accountID); err != nil {
		return RestoreResult{}, err
	}

	target, err := s.export.GetRestoreTarget(ctx, messageID)
	if errors.Is(err, domain.ErrNotFound) {
		return RestoreResult{}, domain.ErrNotFound
	}
	if err != nil {
		return RestoreResult{}, err
	}

	raw, err := s.blob.Read(target.AccountID, target.BlobSHA256)
	if err != nil {
		return RestoreResult{}, err
	}

	cl, err := s.connector.Open(ctx, target.AccountID)
	if err != nil {
		return RestoreResult{}, err
	}
	defer cl.Close()

	var flags []imap.Flag
	if target.FlagsJSON != nil {
		var fs []string
		if err := json.Unmarshal([]byte(*target.FlagsJSON), &fs); err == nil {
			for _, f := range fs {
				flags = append(flags, imap.Flag(f))
			}
		}
	}

	var date time.Time
	if target.Date != nil {
		date, _ = time.Parse(time.RFC3339, *target.Date)
	}
	if date.IsZero() {
		date = time.Now()
	}

	if err := cl.AppendMessage(ctx, target.MailboxName, raw, flags, date); err != nil {
		return RestoreResult{}, err
	}

	_ = s.messages.MarkRestored(ctx, messageID)
	return RestoreResult{Message: "restored to " + target.MailboxName}, nil
}
