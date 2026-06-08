package message

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/GiraffeSecurity/giraffemailer/internal/archive"
	"github.com/GiraffeSecurity/giraffemailer/internal/authz"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
)

var validPartPath = regexp.MustCompile(`^[\d.]+$`)

type Service struct {
	accounts    port.AccountRepository
	mailboxes   port.MailboxRepository
	messages    port.MessageRepository
	attachments port.AttachmentRepository
	search      port.SearchRepository
	insights    port.InsightsRepository
	blob        port.BlobReader
}

func NewService(
	accounts port.AccountRepository,
	mailboxes port.MailboxRepository,
	messages port.MessageRepository,
	attachments port.AttachmentRepository,
	search port.SearchRepository,
	insights port.InsightsRepository,
	blob port.BlobReader,
) *Service {
	return &Service{
		accounts:    accounts,
		mailboxes:   mailboxes,
		messages:    messages,
		attachments: attachments,
		search:      search,
		insights:    insights,
		blob:        blob,
	}
}

func (s *Service) ListMailboxes(ctx context.Context, accountID string) ([]domain.Mailbox, error) {
	if err := authz.EnsureAccountAccess(ctx, s.accounts, accountID); err != nil {
		return nil, err
	}
	return s.mailboxes.ListByAccount(ctx, accountID)
}

func (s *Service) List(ctx context.Context, filter domain.MessageListFilter) (domain.MessagePage, error) {
	scoped, err := s.scopeListFilter(ctx, filter)
	if err != nil {
		return domain.MessagePage{}, err
	}
	return s.messages.List(ctx, scoped)
}

func (s *Service) scopeListFilter(ctx context.Context, filter domain.MessageListFilter) (domain.MessageListFilter, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return filter, domain.ErrUnauthorized
	}
	if sub.IsAdmin() {
		return filter, nil
	}
	if filter.AccountID != "" {
		if err := authz.EnsureAccountAccess(ctx, s.accounts, filter.AccountID); err != nil {
			return filter, err
		}
		return filter, nil
	}
	ids, err := authz.AccessibleAccountIDs(ctx, s.accounts)
	if err != nil {
		return filter, err
	}
	filter.ScopedAccountIDs = ids
	return filter, nil
}

func (s *Service) ensureMessageAccess(ctx context.Context, messageID string) error {
	accountID, err := s.messages.GetAccountID(ctx, messageID)
	if err != nil {
		return err
	}
	return authz.EnsureAccountAccess(ctx, s.accounts, accountID)
}

func (s *Service) Get(ctx context.Context, id string, loadImages bool) (domain.MessageDetail, error) {
	if err := s.ensureMessageAccess(ctx, id); err != nil {
		return domain.MessageDetail{}, err
	}
	detail, err := s.messages.GetDetail(ctx, id, loadImages)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.MessageDetail{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.MessageDetail{}, err
	}

	if detail.BodyHTML != nil {
		sanitized := SanitizeHTML(*detail.BodyHTML, loadImages)
		detail.BodyHTML = &sanitized
	}

	atts, err := s.attachments.ListByMessage(ctx, id)
	if err != nil {
		return domain.MessageDetail{}, err
	}
	if len(atts) == 0 {
		accountID, sha256, err := s.messages.GetBlobRef(ctx, id)
		if err == nil {
			if raw, err := s.blob.Read(accountID, sha256); err == nil {
				if parsed, _ := archive.ParseEmail(raw); parsed != nil {
					for _, a := range parsed.Attachments {
						atts = append(atts, domain.AttachmentMeta{
							Filename:    a.Filename,
							ContentType: a.ContentType,
							SizeBytes:   a.SizeBytes,
							PartPath:    a.PartPath,
						})
					}
				}
			}
		}
	}
	detail.Attachments = atts
	return detail, nil
}

func (s *Service) DownloadAttachment(ctx context.Context, messageID, partPath string) (domain.AttachmentDownload, error) {
	if !validPartPath.MatchString(partPath) {
		return domain.AttachmentDownload{}, domain.ErrInvalidInput
	}
	if err := s.ensureMessageAccess(ctx, messageID); err != nil {
		return domain.AttachmentDownload{}, err
	}

	accountID, sha256, err := s.messages.GetBlobRef(ctx, messageID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.AttachmentDownload{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.AttachmentDownload{}, err
	}

	filename, contentType, _ := s.attachments.GetMeta(ctx, messageID, partPath)

	raw, err := s.blob.Read(accountID, sha256)
	if err != nil {
		return domain.AttachmentDownload{}, domain.ErrNotFound
	}

	body, detectedCT, err := archive.ExtractAttachment(raw, partPath)
	if err != nil {
		return domain.AttachmentDownload{}, domain.ErrNotFound
	}

	if contentType == "" {
		contentType = detectedCT
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if filename == "" {
		filename = partPath + ".bin"
	}

	return domain.AttachmentDownload{
		Filename:    filename,
		ContentType: contentType,
		Body:        body,
	}, nil
}

func (s *Service) Search(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return domain.SearchResult{}, domain.ErrUnauthorized
	}
	if !sub.IsAdmin() {
		if q.AccountID != "" {
			if err := authz.EnsureAccountAccess(ctx, s.accounts, q.AccountID); err != nil {
				return domain.SearchResult{}, err
			}
		} else {
			ids, err := authz.AccessibleAccountIDs(ctx, s.accounts)
			if err != nil {
				return domain.SearchResult{}, err
			}
			q.ScopedAccountIDs = ids
		}
	}
	return s.search.Search(ctx, q)
}

func (s *Service) Insights(ctx context.Context) (domain.Insights, error) {
	sub, ok := authz.Subject(ctx)
	if !ok {
		return domain.Insights{}, domain.ErrUnauthorized
	}
	ids, err := authz.AccessibleAccountIDs(ctx, s.accounts)
	if err != nil {
		return domain.Insights{}, err
	}
	return s.insights.GetForSubject(ctx, sub, ids)
}

func AttachmentDisplayName(a domain.AttachmentMeta) string {
	if a.Filename != "" {
		return a.Filename
	}
	return fmt.Sprintf("attachment-%s", a.PartPath)
}
