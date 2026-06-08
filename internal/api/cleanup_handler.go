package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	cleanupsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/cleanup"
)

type CleanupHandler struct {
	svc *cleanupsvc.Service
}

func NewCleanupHandler(svc *cleanupsvc.Service) *CleanupHandler {
	return &CleanupHandler{svc: svc}
}

func (h *CleanupHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filter domain.CleanupFilter `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Filter.AccountID == "" {
		badRequest(w, "account_id required")
		return
	}
	preview, err := h.svc.Preview(r.Context(), req.Filter)
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]any{"count": preview.Count, "total_bytes": preview.TotalBytes})
}

func (h *CleanupHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.svc.ListJobs(r.Context())
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type job struct {
		ID               string  `json:"id"`
		Name             string  `json:"name"`
		AccountID        string  `json:"account_id"`
		FilterJSON       string  `json:"filter"`
		Action           string  `json:"action"`
		MoveTargetFolder *string `json:"move_target_folder"`
		CreatedAt        string  `json:"created_at"`
	}
	out := make([]job, len(jobs))
	for i, j := range jobs {
		out[i] = job{
			ID: j.ID, Name: j.Name, AccountID: j.AccountID, FilterJSON: j.FilterJSON,
			Action: j.Action, MoveTargetFolder: j.MoveTargetFolder, CreatedAt: j.CreatedAt,
		}
	}
	ok(w, out)
}

func (h *CleanupHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string               `json:"name"`
		Filter           domain.CleanupFilter `json:"filter"`
		Action           string               `json:"action"`
		MoveTargetFolder *string              `json:"move_target_folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" || req.Filter.AccountID == "" {
		badRequest(w, "name and filter.account_id required")
		return
	}
	if req.Action != "delete" && req.Action != "move" {
		badRequest(w, "action must be 'delete' or 'move'")
		return
	}
	if req.Action == "move" && (req.MoveTargetFolder == nil || *req.MoveTargetFolder == "") {
		badRequest(w, "move_target_folder required for action=move")
		return
	}
	id, err := h.svc.CreateJob(r.Context(), domain.CreateCleanupJobInput{
		Name: req.Name, Filter: req.Filter, Action: req.Action,
		MoveTargetFolder: req.MoveTargetFolder, CreatedBy: UserID(r.Context()),
	})
	if errors.Is(err, domain.ErrInvalidInput) {
		badRequest(w, "invalid job input")
		return
	}
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	created(w, map[string]string{"id": id})
}

func (h *CleanupHandler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name             string               `json:"name"`
		Filter           domain.CleanupFilter `json:"filter"`
		Action           string               `json:"action"`
		MoveTargetFolder *string              `json:"move_target_folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" || req.Filter.AccountID == "" {
		badRequest(w, "name and filter.account_id required")
		return
	}
	if req.Action != "delete" && req.Action != "move" {
		badRequest(w, "action must be 'delete' or 'move'")
		return
	}
	if req.Action == "move" && (req.MoveTargetFolder == nil || *req.MoveTargetFolder == "") {
		badRequest(w, "move_target_folder required for action=move")
		return
	}
	err := h.svc.UpdateJob(r.Context(), id, domain.UpdateCleanupJobInput{
		Name: req.Name, Filter: req.Filter, Action: req.Action,
		MoveTargetFolder: req.MoveTargetFolder,
	})
	if mapForbidden(w, err) {
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		notFound(w)
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		badRequest(w, "invalid job input")
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

func (h *CleanupHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteJob(r.Context(), id); err != nil {
		if mapForbidden(w, err) {
			return
		}
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

func (h *CleanupHandler) RunJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	result, err := h.svc.Run(r.Context(), jobID)
	if mapForbidden(w, err) {
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"run_id": result.RunID, "status": result.Status})
}

func (h *CleanupHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	runs, err := h.svc.ListRuns(r.Context(), jobID)
	if mapForbidden(w, err) {
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type run struct {
		ID                string  `json:"id"`
		Status            string  `json:"status"`
		TotalCandidates   int     `json:"total_candidates"`
		Processed         int     `json:"processed"`
		SkippedUnarchived int     `json:"skipped_unarchived"`
		FreedBytes        int64   `json:"freed_bytes"`
		ErrorMessage      *string `json:"error_message"`
		StartedAt         *string `json:"started_at"`
		FinishedAt        *string `json:"finished_at"`
	}
	out := make([]run, len(runs))
	for i, rr := range runs {
		out[i] = run{
			ID: rr.ID, Status: rr.Status, TotalCandidates: rr.TotalCandidates,
			Processed: rr.Processed, SkippedUnarchived: rr.SkippedUnarchived,
			FreedBytes: rr.FreedBytes, ErrorMessage: rr.ErrorMessage,
			StartedAt: rr.StartedAt, FinishedAt: rr.FinishedAt,
		}
	}
	ok(w, out)
}
