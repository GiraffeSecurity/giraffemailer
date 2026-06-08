package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/GiraffeSecurity/giraffemailer/internal/api/middleware"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	authsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/auth"
)

type AuthHandler struct {
	svc          *authsvc.Service
	cookieSecure bool
}

func NewAuthHandler(svc *authsvc.Service, cookieSecure bool) *AuthHandler {
	return &AuthHandler{svc: svc, cookieSecure: cookieSecure}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		badRequest(w, "email, password and full_name are required")
		return
	}
	if len(req.Password) < 8 {
		badRequest(w, "password must be at least 8 characters")
		return
	}

	result, err := h.svc.Register(r.Context(), middleware.ClientIP(r), req.Email, req.Password, req.FullName)
	if errors.Is(err, domain.ErrRateLimited) {
		fail(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		fail(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"otp_id": result.OTPID, "message": result.Message})
}

func (h *AuthHandler) OTPRegistration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"`
		Code       string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if err := h.svc.OTPRegistration(r.Context(), middleware.ClientIP(r), req.Identifier, req.Code); err != nil {
		if errors.Is(err, domain.ErrRateLimited) {
			fail(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		fail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ok(w, map[string]string{"message": "email verified; account activated"})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	result, err := h.svc.Login(r.Context(), middleware.ClientIP(r), req.Email, req.Password)
	if errors.Is(err, domain.ErrRateLimited) {
		fail(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if errors.Is(err, domain.ErrUnauthorized) {
		fail(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if errors.Is(err, domain.ErrForbidden) {
		fail(w, http.StatusForbidden, "account not activated")
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, h.cookieSecure)
	ok(w, map[string]string{
		"expires_at": result.ExpiresAt,
		"message":    "logged in",
		"token":      result.Token,
	})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.Email == "" {
		badRequest(w, "email is required")
		return
	}
	msg, err := h.svc.ForgotPassword(r.Context(), middleware.ClientIP(r), req.Email)
	if errors.Is(err, domain.ErrRateLimited) {
		fail(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"message": msg})
}

func (h *AuthHandler) OTPForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier  string `json:"identifier"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if req.NewPassword == "" {
		badRequest(w, "new_password is required")
		return
	}
	if len(req.NewPassword) < 8 {
		badRequest(w, "password must be at least 8 characters")
		return
	}
	if err := h.svc.OTPForgotPassword(r.Context(), middleware.ClientIP(r), req.Identifier, req.Code, req.NewPassword); err != nil {
		if errors.Is(err, domain.ErrRateLimited) {
			fail(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			badRequest(w, err.Error())
			return
		}
		fail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ok(w, map[string]string{"message": "password updated"})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := UserID(r.Context())
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if len(req.NewPassword) < 8 {
		badRequest(w, "password must be at least 8 characters")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			fail(w, http.StatusUnauthorized, "current password incorrect")
			return
		}
		internalErr(w, err)
		return
	}
	clearSessionCookie(w, h.cookieSecure)
	ok(w, map[string]string{"message": "password changed; sign in again"})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if token == "" {
		raw := r.Header.Get("Authorization")
		if len(raw) > 7 && raw[:7] == "Bearer " {
			token = raw[7:]
		}
	}
	_ = h.svc.Logout(r.Context(), token)
	clearSessionCookie(w, h.cookieSecure)
	ok(w, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) CheckUser(w http.ResponseWriter, r *http.Request) {
	userID := UserID(r.Context())
	user, err := h.svc.CheckUser(r.Context(), userID)
	if errors.Is(err, domain.ErrNotFound) {
		unauthorized(w)
		return
	}
	if err != nil {
		internalErr(w, err)
		return
	}
	ok(w, map[string]string{"user_id": user.ID, "email": user.Email, "full_name": user.FullName, "role": user.Role})
}
