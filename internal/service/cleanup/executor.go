package cleanup

import (
	"context"
	"time"

	imap "github.com/emersion/go-imap/v2"
	cleanupengine "github.com/GiraffeSecurity/giraffemailer/internal/cleanup"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/GiraffeSecurity/giraffemailer/internal/service/mail"
)

type Executor struct {
	cleanup port.CleanupRepository
	messages port.MessageRepository
	connector *mail.Connector
}

func NewExecutor(cleanup port.CleanupRepository, messages port.MessageRepository, connector *mail.Connector) *Executor {
	return &Executor{cleanup: cleanup, messages: messages, connector: connector}
}

func (e *Executor) Execute(ctx context.Context, accountID string, filter domain.CleanupFilter, action string, moveTarget string) (domain.JobExecutionResult, error) {
	candidates, err := e.cleanup.ListCandidates(ctx, filter, 10000)
	if err != nil {
		return domain.JobExecutionResult{}, err
	}

	result := domain.JobExecutionResult{TotalCandidates: len(candidates)}

	cl, err := e.connector.Open(ctx, accountID)
	if err != nil {
		return result, err
	}
	defer cl.Close()

	type batchEntry struct {
		uid       imap.UID
		id        string
		sizeBytes int64
	}
	byMailbox := map[string][]batchEntry{}

	for _, c := range candidates {
		gc := cleanupengine.Candidate{
			ID:         c.ID,
			AccountID:  accountID,
			MailboxID:  c.MailboxID,
			UID:        c.UID,
			BlobSHA256: c.BlobSHA256,
		}
		if c.ArchivedAt != nil {
			t, _ := time.Parse(time.RFC3339, *c.ArchivedAt)
			gc.ArchivedAt = &t
		}
		if !cleanupengine.IsSafe(gc) {
			result.SkippedUnarchived++
			continue
		}
		byMailbox[c.MailboxName] = append(byMailbox[c.MailboxName], batchEntry{
			uid:       imap.UID(c.UID),
			id:        c.ID,
			sizeBytes: c.SizeBytes,
		})
	}

	for mailboxName, entries := range byMailbox {
		if _, err := cl.Select(ctx, mailboxName); err != nil {
			continue
		}
		for start := 0; start < len(entries); start += 1000 {
			end := start + 1000
			if end > len(entries) {
				end = len(entries)
			}
			chunk := entries[start:end]

			var uidSet imap.UIDSet
			for _, entry := range chunk {
				uidSet.AddNum(entry.uid)
			}

			var opErr error
			if action == "delete" {
				opErr = cl.MarkDeleted(ctx, uidSet)
				if opErr == nil {
					opErr = cl.Expunge(ctx)
				}
			} else {
				opErr = cl.Move(ctx, uidSet, moveTarget)
			}
			if opErr != nil {
				continue
			}

			ids := make([]string, len(chunk))
			for i, entry := range chunk {
				ids[i] = entry.id
				result.FreedBytes += entry.sizeBytes
			}
			_ = e.messages.MarkDeletedFromServer(ctx, ids)
			result.Processed += len(chunk)
		}
	}

	return result, nil
}
