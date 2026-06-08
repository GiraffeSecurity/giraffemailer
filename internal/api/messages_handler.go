package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	msgsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/message"
)

type MessagesHandler struct {
	svc *msgsvc.Service
}

func NewMessagesHandler(svc *msgsvc.Service) *MessagesHandler {
	return &MessagesHandler{svc: svc}
}

func (h *MessagesHandler) ListMailboxes(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	mailboxes, err := h.svc.ListMailboxes(r.Context(), accountID)
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type item struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		MessageCount      int     `json:"message_count"`
		TotalSizeBytes    int64   `json:"total_size_bytes"`
		ArchivedCount     int     `json:"archived_count"`
		ArchivedSizeBytes int64   `json:"archived_size_bytes"`
		ArchivedPercent   float64 `json:"archived_pct"`
		LastIndexedAt     *string `json:"last_indexed_at"`
		LastArchivedAt    *string `json:"last_archived_at"`
	}
	out := make([]item, len(mailboxes))
	for i, mb := range mailboxes {
		out[i] = item{
			ID: mb.ID, Name: mb.Name, MessageCount: mb.MessageCount,
			TotalSizeBytes: mb.TotalSizeBytes, ArchivedCount: mb.ArchivedCount,
			ArchivedSizeBytes: mb.ArchivedSizeBytes, ArchivedPercent: mb.ArchivedPercent,
			LastIndexedAt: mb.LastIndexedAt, LastArchivedAt: mb.LastArchivedAt,
		}
	}
	ok(w, out)
}

func (h *MessagesHandler) ListAllMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, err := h.svc.List(r.Context(), domain.MessageListFilter{
		AccountID:      q.Get("account_id"),
		MailboxID:      q.Get("mailbox_id"),
		Sender:         q.Get("sender"),
		ArchiveState:   q.Get("archive_state"),
		HasAttachments: q.Get("has_attachments") == "true",
		Sort:           q.Get("sort"),
		Desc:           q.Get("dir") != "asc",
		Cursor:         q.Get("cursor"),
		Limit:          limit,
	})
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]any{
		"messages":    messageSummariesJSON(page.Messages),
		"next_cursor": page.NextCursor,
		"has_more":    page.HasMore,
		"limit":       page.Limit,
	})
}

func (h *MessagesHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	mailboxID := chi.URLParam(r, "mailboxId")
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, err := h.svc.List(r.Context(), domain.MessageListFilter{
		AccountID:      accountID,
		MailboxID:      mailboxID,
		Sender:         q.Get("sender"),
		ArchiveState:   q.Get("archive_state"),
		HasAttachments: q.Get("has_attachments") == "true",
		Unread:         q.Get("unread") == "true",
		Desc:           q.Get("dir") != "asc",
		Cursor:         q.Get("cursor"),
		Limit:          limit,
	})
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type msgItem struct {
		ID                  string  `json:"id"`
		UID                 int64   `json:"uid"`
		Subject             *string `json:"subject"`
		SenderName          *string `json:"sender_name"`
		SenderEmail         string  `json:"sender_email"`
		Date                *string `json:"date"`
		SizeBytes           int64   `json:"size_bytes"`
		HasAttachments      bool    `json:"has_attachments"`
		AttachmentCount     int     `json:"attachment_count"`
		BodyPreview         *string `json:"body_preview"`
		ArchivedAt          *string `json:"archived_at"`
		DeletedFromServerAt *string `json:"deleted_from_server_at"`
		ArchiveState        string  `json:"archive_state"`
	}
	msgs := make([]msgItem, len(page.Messages))
	for i, m := range page.Messages {
		msgs[i] = msgItem{
			ID: m.ID, UID: m.UID, Subject: m.Subject, SenderName: m.SenderName,
			SenderEmail: m.SenderEmail, Date: m.Date, SizeBytes: m.SizeBytes,
			HasAttachments: m.HasAttachments, AttachmentCount: m.AttachmentCount,
			BodyPreview: m.BodyPreview, ArchivedAt: m.ArchivedAt,
			DeletedFromServerAt: m.DeletedFromServerAt, ArchiveState: m.ArchiveState,
		}
	}
	ok(w, map[string]any{
		"messages":    msgs,
		"next_cursor": page.NextCursor,
		"has_more":    page.HasMore,
		"limit":       page.Limit,
	})
}

func (h *MessagesHandler) GetMessage(w http.ResponseWriter, r *http.Request) {
	msgID := chi.URLParam(r, "id")
	loadImages := r.URL.Query().Get("load_images") == "true"
	detail, err := h.svc.Get(r.Context(), msgID, loadImages)
	if errors.Is(err, domain.ErrNotFound) {
		notFound(w)
		return
	}
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	atts := make([]map[string]any, len(detail.Attachments))
	for i, a := range detail.Attachments {
		atts[i] = map[string]any{
			"filename":     msgsvc.AttachmentDisplayName(a),
			"content_type": a.ContentType,
			"size_bytes":   a.SizeBytes,
			"part_path":    a.PartPath,
		}
	}
	ok(w, map[string]any{
		"id":                     detail.ID,
		"subject":                detail.Subject,
		"sender_name":            detail.SenderName,
		"sender_email":           detail.SenderEmail,
		"date":                   detail.Date,
		"size_bytes":             detail.SizeBytes,
		"archived_at":            detail.ArchivedAt,
		"deleted_from_server_at": detail.DeletedFromServerAt,
		"archive_state":          detail.ArchiveState,
		"body_html":              detail.BodyHTML,
		"body_text":              detail.BodyText,
		"attachments":            atts,
	})
}

func (h *MessagesHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	msgID := chi.URLParam(r, "id")
	partPath := chi.URLParam(r, "partPath")
	dl, err := h.svc.DownloadAttachment(r.Context(), msgID, partPath)
	if errors.Is(err, domain.ErrInvalidInput) {
		badRequest(w, "invalid part path")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		notFound(w)
		return
	}
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	w.Header().Set("Content-Type", dl.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, dl.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(dl.Body)))
	w.WriteHeader(http.StatusOK)
	w.Write(dl.Body)
}

func (h *MessagesHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	searchResult, err := h.svc.Search(r.Context(), domain.SearchQuery{
		Query:          q.Get("q"),
		Cursor:         q.Get("cursor"),
		Limit:          limit,
		Sender:         q.Get("sender"),
		AccountID:      q.Get("account_id"),
		HasAttachments: q.Get("has_attachments") == "true",
		ArchiveState:   q.Get("archive_state"),
		MinSize:        q.Get("min_size"),
	})
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type searchItem struct {
		ID                  string  `json:"id"`
		Subject             *string `json:"subject"`
		SenderName          *string `json:"sender_name"`
		SenderEmail         string  `json:"sender_email"`
		Date                *string `json:"date"`
		SizeBytes           int64   `json:"size_bytes"`
		HasAttachments      bool    `json:"has_attachments"`
		ArchivedAt          *string `json:"archived_at"`
		DeletedFromServerAt *string `json:"deleted_from_server_at"`
		BodyPreview         *string `json:"body_preview"`
		ArchiveState        string  `json:"archive_state"`
		MailboxName         string  `json:"mailbox_name"`
		AccountName         string  `json:"account_name"`
	}
	results := make([]searchItem, len(searchResult.Messages))
	for i, m := range searchResult.Messages {
		results[i] = searchItem{
			ID: m.ID, Subject: m.Subject, SenderName: m.SenderName, SenderEmail: m.SenderEmail,
			Date: m.Date, SizeBytes: m.SizeBytes, HasAttachments: m.HasAttachments,
			ArchivedAt: m.ArchivedAt, DeletedFromServerAt: m.DeletedFromServerAt,
			BodyPreview: m.BodyPreview, ArchiveState: m.ArchiveState,
			MailboxName: m.MailboxName, AccountName: m.AccountName,
		}
	}
	ok(w, map[string]any{
		"messages":    results,
		"total":       searchResult.Total,
		"next_cursor": searchResult.NextCursor,
		"has_more":    searchResult.HasMore,
		"limit":       searchResult.Limit,
	})
}

func (h *MessagesHandler) Insights(w http.ResponseWriter, r *http.Request) {
	ins, err := h.svc.Insights(r.Context())
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type senderStat struct {
		Email string `json:"sender_email"`
		Count int    `json:"count"`
		Bytes int64  `json:"total_bytes"`
	}
	type yearStat struct {
		Year  string `json:"year"`
		Count int    `json:"count"`
		Bytes int64  `json:"total_bytes"`
	}
	topSenders := make([]senderStat, len(ins.TopSenders))
	for i, s := range ins.TopSenders {
		topSenders[i] = senderStat{Email: s.Email, Count: s.Count, Bytes: s.Bytes}
	}
	byYear := make([]yearStat, len(ins.SizeByYear))
	for i, y := range ins.SizeByYear {
		byYear[i] = yearStat{Year: y.Year, Count: y.Count, Bytes: y.Bytes}
	}
	ok(w, map[string]any{
		"total_messages":    ins.TotalMessages,
		"archived_messages": ins.ArchivedMessages,
		"total_bytes":       ins.TotalBytes,
		"archived_bytes":    ins.ArchivedBytes,
		"reclaimable_bytes": ins.ReclaimableBytes,
		"top_senders":       topSenders,
		"size_by_year":      byYear,
	})
}

func messageSummariesJSON(msgs []domain.MessageSummary) []map[string]any {
	out := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		out[i] = map[string]any{
			"id": m.ID, "uid": m.UID, "subject": m.Subject,
			"sender_name": m.SenderName, "sender_email": m.SenderEmail,
			"date": m.Date, "size_bytes": m.SizeBytes,
			"has_attachments": m.HasAttachments, "attachment_count": m.AttachmentCount,
			"body_preview": m.BodyPreview, "archived_at": m.ArchivedAt,
			"deleted_from_server_at": m.DeletedFromServerAt, "archive_state": m.ArchiveState,
			"mailbox_name": m.MailboxName, "account_name": m.AccountName,
		}
	}
	return out
}
