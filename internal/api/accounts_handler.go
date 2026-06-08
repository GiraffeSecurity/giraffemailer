package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/GiraffeSecurity/giraffemailer/internal/archive"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	accountsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/account"
)

type AccountsHandler struct {
	svc *accountsvc.Service
}

func NewAccountsHandler(svc *accountsvc.Service) *AccountsHandler {
	return &AccountsHandler{svc: svc}
}

func (h *AccountsHandler) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svc.List(r.Context())
	if err != nil {
		internalErr(w, err)
		return
	}
	type item struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		EmailAddress string  `json:"email_address"`
		IMAPHost     string  `json:"imap_host"`
		IMAPPort     int     `json:"imap_port"`
		UseTLS       bool    `json:"use_tls"`
		Username     string  `json:"username"`
		SyncEnabled  bool    `json:"sync_enabled"`
		LastSyncAt   *string `json:"last_sync_at"`
	}
	out := make([]item, len(accounts))
	for i, a := range accounts {
		out[i] = item{
			ID: a.ID, Name: a.Name, EmailAddress: a.EmailAddress,
			IMAPHost: a.IMAPHost, IMAPPort: a.IMAPPort, UseTLS: a.UseTLS,
			Username: a.Username, SyncEnabled: a.SyncEnabled, LastSyncAt: a.LastSyncAt,
		}
	}
	ok(w, out)
}

func (h *AccountsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		EmailAddress string `json:"email_address"`
		IMAPHost     string `json:"imap_host"`
		IMAPPort     int    `json:"imap_port"`
		UseTLS       bool   `json:"use_tls"`
		Username     string `json:"username"`
		Password     string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" || req.EmailAddress == "" || req.IMAPHost == "" || req.Username == "" || req.Password == "" {
		badRequest(w, "name, email_address, imap_host, username, password required")
		return
	}
	if req.IMAPPort == 0 {
		req.IMAPPort = 993
	}
	if req.IMAPPort < 1 || req.IMAPPort > 65535 {
		badRequest(w, "imap_port must be 1–65535")
		return
	}

	id, err := h.svc.Create(r.Context(), domain.CreateAccountInput{
		Name: req.Name, EmailAddress: req.EmailAddress, IMAPHost: req.IMAPHost,
		IMAPPort: req.IMAPPort, UseTLS: req.UseTLS, Username: req.Username, Password: req.Password,
	})
	if err != nil {
		internalErr(w, err)
		return
	}
	created(w, map[string]string{"id": id})
}

func (h *AccountsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	a, err := h.svc.Get(r.Context(), id)
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
	ok(w, map[string]any{
		"id": a.ID, "name": a.Name, "email_address": a.EmailAddress,
		"imap_host": a.IMAPHost, "imap_port": a.IMAPPort, "use_tls": a.UseTLS,
		"username": a.Username, "sync_enabled": a.SyncEnabled, "last_sync_at": a.LastSyncAt,
	})
}

func (h *AccountsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if mapForbidden(w, err) {
			return
		}
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

func (h *AccountsHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.svc.TestConnection(r.Context(), id)
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
	if result.Success {
		ok(w, map[string]any{"success": true})
	} else {
		ok(w, map[string]any{"success": false, "error": result.Error})
	}
}

func (h *AccountsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.TriggerSync(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			notFound(w)
			return
		}
		if mapForbidden(w, err) {
			return
		}
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"status": "syncing"})
}

func (h *AccountsHandler) Progress(tracker *archive.Tracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "accountId")
		_, err := h.svc.Get(r.Context(), id)
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
		tracker.SSEHandler(id)(w, r)
	}
}
