package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	authsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/auth"
)

type AdminHandler struct {
	auth *authsvc.Service
}

func NewAdminHandler(auth *authsvc.Service) *AdminHandler {
	return &AdminHandler{auth: auth}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.auth.ListUsers(r.Context())
	if errors.Is(err, domain.ErrForbidden) {
		fail(w, http.StatusForbidden, "admin required")
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	type item struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
		IsActive bool   `json:"is_active"`
	}
	out := make([]item, len(users))
	for i, u := range users {
		out[i] = item{ID: u.ID, Email: u.Email, FullName: u.FullName, Role: u.Role, IsActive: u.IsActive}
	}
	ok(w, out)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Role == nil && req.IsActive == nil {
		badRequest(w, "role or is_active required")
		return
	}
	if req.Role != nil && *req.Role != domain.RoleAdmin && *req.Role != domain.RoleUser {
		badRequest(w, "role must be admin or user")
		return
	}
	err := h.auth.AdminUpdateUser(r.Context(), id, req.Role, req.IsActive)
	if errors.Is(err, domain.ErrForbidden) {
		fail(w, http.StatusForbidden, "admin required")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		notFound(w)
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		badRequest(w, err.Error())
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

func mapForbidden(w http.ResponseWriter, err error) bool {
	if errors.Is(err, domain.ErrForbidden) {
		fail(w, http.StatusForbidden, "forbidden")
		return true
	}
	return false
}
