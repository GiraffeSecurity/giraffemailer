package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	exportsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/export"
)

type ExportHandler struct {
	svc *exportsvc.Service
}

func NewExportHandler(svc *exportsvc.Service) *ExportHandler {
	return &ExportHandler{svc: svc}
}

func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MessageIDs []string `json:"message_ids"`
		Format     string   `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if len(req.MessageIDs) == 0 {
		badRequest(w, "message_ids required")
		return
	}
	if len(req.MessageIDs) > 1000 {
		badRequest(w, "message_ids must not exceed 1000")
		return
	}
	if req.Format == "" {
		req.Format = "mbox"
	}
	if req.Format != "mbox" && req.Format != "zip" {
		badRequest(w, "format must be 'mbox' or 'zip'")
		return
	}

	if req.Format == "mbox" {
		w.Header().Set("Content-Type", "application/mbox")
		w.Header().Set("Content-Disposition", "attachment; filename=export.mbox")
	} else {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=export.zip")
	}

	if err := h.svc.Export(r.Context(), exportsvc.ExportInput{
		MessageIDs: req.MessageIDs,
		Format:     req.Format,
	}, w); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			badRequest(w, "invalid export request")
			return
		}
		if mapForbidden(w, err) {
			return
		}
		internalErr(w, err)
	}
}

func (h *ExportHandler) Restore(w http.ResponseWriter, r *http.Request) {
	msgID := chi.URLParam(r, "id")
	result, err := h.svc.Restore(r.Context(), msgID)
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
	ok(w, map[string]string{"message": result.Message})
}
