package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/GiraffeSecurity/giraffemailer/internal/api/middleware"
	"github.com/GiraffeSecurity/giraffemailer/internal/archive"
	"github.com/GiraffeSecurity/giraffemailer/internal/config"
	accountsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/account"
	authsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/auth"
	cleanupsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/cleanup"
	exportsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/export"
	msgsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/message"
	"github.com/GiraffeSecurity/giraffemailer/internal/ui"
)

type Deps struct {
	Config   *config.Config
	DB       *sql.DB
	Engine   *archive.Engine
	Accounts *accountsvc.Service
	Messages *msgsvc.Service
	Cleanup  *cleanupsvc.Service
	Export   *exportsvc.Service
	Auth     *authsvc.Service
}

func NewRouter(deps Deps) http.Handler {
	tracker := deps.Engine.Tracker()
	r := chi.NewRouter()
	sec := deps.Config.Security
	cookieSecure := sec.CookieSecure || deps.Config.App.Env == "production"

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecureHeaders)
	r.Use(middleware.CORS(middleware.CORSConfig{AllowedOrigins: sec.CORSAllowedOrigins}))
	r.Use(middleware.BodyLimit)

	auth := NewAuthHandler(deps.Auth, cookieSecure)
	admin := NewAdminHandler(deps.Auth)
	accounts := NewAccountsHandler(deps.Accounts)
	msgs := NewMessagesHandler(deps.Messages)
	cleanup := NewCleanupHandler(deps.Cleanup)
	export := NewExportHandler(deps.Export)

	regGate := RequireRegistration(sec.AllowRegistration)

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.With(regGate).Post("/register", auth.Register)
		r.With(regGate).Post("/otp_registration", auth.OTPRegistration)
		r.Post("/login", auth.Login)
		r.Post("/forgot_password", auth.ForgotPassword)
		r.Post("/otp_forgot_password", auth.OTPForgotPassword)
	})

	r.Group(func(r chi.Router) {
		r.Use(SessionAuth(deps.Auth))

		r.Post("/api/v1/auth/change_password", auth.ChangePassword)
		r.Post("/api/v1/auth/logout", auth.Logout)
		r.Get("/api/v1/auth/check_user", auth.CheckUser)

		r.Get("/api/v1/accounts", accounts.List)
		r.Post("/api/v1/accounts", accounts.Create)
		r.Get("/api/v1/accounts/{id}", accounts.Get)
		r.Delete("/api/v1/accounts/{id}", accounts.Delete)
		r.Post("/api/v1/accounts/{id}/test", accounts.TestConnection)
		r.Post("/api/v1/accounts/{id}/sync", accounts.Sync)

		r.Get("/api/v1/accounts/{accountId}/progress", accounts.Progress(tracker))

		r.Get("/api/v1/accounts/{accountId}/mailboxes", msgs.ListMailboxes)

		r.Get("/api/v1/messages", msgs.ListAllMessages)
		r.Get("/api/v1/accounts/{accountId}/mailboxes/{mailboxId}/messages", msgs.ListMessages)
		r.Get("/api/v1/messages/{id}", msgs.GetMessage)
		r.Get("/api/v1/messages/{id}/attachments/{partPath}", msgs.DownloadAttachment)

		r.Get("/api/v1/search", msgs.Search)
		r.Get("/api/v1/insights", msgs.Insights)

		r.Post("/api/v1/cleanup/preview", cleanup.Preview)
		r.Get("/api/v1/cleanup/jobs", cleanup.ListJobs)
		r.Post("/api/v1/cleanup/jobs", cleanup.CreateJob)
		r.Put("/api/v1/cleanup/jobs/{id}", cleanup.UpdateJob)
		r.Delete("/api/v1/cleanup/jobs/{id}", cleanup.DeleteJob)
		r.Post("/api/v1/cleanup/jobs/{id}/run", cleanup.RunJob)
		r.Get("/api/v1/cleanup/jobs/{id}/runs", cleanup.ListRuns)

		r.Post("/api/v1/export", export.Export)
		r.Post("/api/v1/restore/{id}", export.Restore)

		r.Route("/api/v1/admin", func(r chi.Router) {
			r.Use(RequireAdmin)
			r.Get("/users", admin.ListUsers)
			r.Patch("/users/{id}", admin.UpdateUser)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := deps.DB.PingContext(ctx); err != nil {
			fail(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		ok(w, map[string]string{"status": "ready"})
	})

	uiHandler := ui.Handler()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		uiHandler.ServeHTTP(w, r)
	})

	return r
}
