package api

import (
	"net/http"

	"github.com/GiraffeSecurity/giraffemailer/internal/authz"
	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	authsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/auth"
)

func SessionAuth(auth *authsvc.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionToken(r)
			if token == "" {
				raw := r.Header.Get("Authorization")
				if len(raw) > 7 && raw[:7] == "Bearer " {
					token = raw[7:]
				}
			}
			if token == "" {
				unauthorized(w)
				return
			}
			userID, role, err := auth.ValidateToken(r.Context(), token)
			if err != nil {
				unauthorized(w)
				return
			}
			ctx := WithUserID(r.Context(), userID)
			ctx = authz.WithSubject(ctx, domain.Subject{UserID: userID, Role: role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, ok := authz.Subject(r.Context())
		if !ok || !sub.IsAdmin() {
			fail(w, http.StatusForbidden, "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireRegistration(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				fail(w, http.StatusForbidden, "registration disabled")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
