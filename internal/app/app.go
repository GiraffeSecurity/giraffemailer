package app

import (
	"database/sql"
	"net/http"

	"github.com/GiraffeSecurity/giraffemailer/internal/api"
	"github.com/GiraffeSecurity/giraffemailer/internal/archive"
	"github.com/GiraffeSecurity/giraffemailer/internal/config"
	"github.com/GiraffeSecurity/giraffemailer/internal/crypto"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/GiraffeSecurity/giraffemailer/internal/repository/sqlite"
	accountsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/account"
	authsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/auth"
	cleanupsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/cleanup"
	exportsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/export"
	"github.com/GiraffeSecurity/giraffemailer/internal/service/mail"
	msgsvc "github.com/GiraffeSecurity/giraffemailer/internal/service/message"
	"github.com/GiraffeSecurity/giraffemailer/internal/store"
)

type App struct {
	Router http.Handler
	Engine *archive.Engine
}

func New(cfg *config.Config, db *sql.DB, bs *store.BlobStore, masterKey [32]byte, smtp port.OTPSender) *App {
	vault := crypto.NewVault(masterKey)

	accountRepo := sqlite.NewAccountRepo(db)
	mailboxRepo := sqlite.NewMailboxRepo(db)
	messageRepo := sqlite.NewMessageRepo(db, bs)
	attachmentRepo := sqlite.NewAttachmentRepo(db)
	searchRepo := sqlite.NewSearchRepo(db)
	insightsRepo := sqlite.NewInsightsRepo(db)
	cleanupRepo := sqlite.NewCleanupRepo(db)
	exportRepo := sqlite.NewExportRepo(db)
	userRepo := sqlite.NewUserRepo(db)

	connector := mail.NewConnector(accountRepo, vault)

	authService := authsvc.NewService(userRepo, smtp)
	messageService := msgsvc.NewService(accountRepo, mailboxRepo, messageRepo, attachmentRepo, searchRepo, insightsRepo, bs)
	cleanupExecutor := cleanupsvc.NewExecutor(cleanupRepo, messageRepo, connector)
	cleanupService := cleanupsvc.NewService(cleanupRepo, accountRepo, cleanupExecutor)
	exportService := exportsvc.NewService(exportRepo, messageRepo, accountRepo, bs, connector)

	decryptCred := func(encrypted string) (username, password string, err error) {
		pw, e := vault.DecryptPassword(encrypted)
		return "", pw, e
	}

	engine := archive.NewEngine(
		db, bs,
		cfg.Archive.BatchSizeBytes,
		cfg.Archive.WorkerCount,
		decryptCred,
	)

	accountService := accountsvc.NewService(accountRepo, vault, connector, engine)

	router := api.NewRouter(api.Deps{
		Config:   cfg,
		DB:       db,
		Engine:   engine,
		Accounts: accountService,
		Messages: messageService,
		Cleanup:  cleanupService,
		Export:   exportService,
		Auth:     authService,
	})

	return &App{Router: router, Engine: engine}
}
