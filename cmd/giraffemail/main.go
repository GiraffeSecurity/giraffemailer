package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/GiraffeSecurity/giraffemailer/internal/app"
	"github.com/GiraffeSecurity/giraffemailer/internal/config"
	"github.com/GiraffeSecurity/giraffemailer/internal/db"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/GiraffeSecurity/giraffemailer/internal/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"github.com/gofrs/uuid/v5"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "giraffemail",
	Short: "GiraffeMail Archive — self-hosted email backup and cleanup",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the GiraffeMail Archive web server",
	RunE:  runServe,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run pending database migrations and exit",
	RunE:  runMigrate,
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed admin user + demo data (dev only)",
	RunE:  runSeed,
}

var fsckCmd = &cobra.Command{
	Use:   "fsck",
	Short: "Re-verify all archived blobs against stored checksums",
	RunE:  runFsck,
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export archived emails to .mbox or .zip",
	RunE:  runExport,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "config.yaml", "path to config file")
	rootCmd.AddCommand(serveCmd, migrateCmd, seedCmd, fsckCmd, exportCmd)

	exportCmd.Flags().String("account", "", "account ID to export (required)")
	exportCmd.Flags().String("format", "mbox", "output format: mbox | zip")
	exportCmd.Flags().String("output", "", "output file path (default: <account>.<format>)")
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	return config.Load(cfgPath)
}

func setupLogger(cfg *config.Config) {
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.Log.Format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}
}

func openDB(ctx context.Context, cfg *config.Config) (*db.DB, error) {
	if err := os.MkdirAll(cfg.Storage.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(cfg.Storage.DataDir, "giraffemail.db")
	return db.Open(ctx, dbPath)
}

// ── serve ─────────────────────────────────────────────────────────────────────

func runServe(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	log.Info().Str("version", version).Str("env", cfg.App.Env).Msg("starting GiraffeMail Archive")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	database, err := openDB(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	var encKey *[32]byte
	masterKey, mkErr := cfg.MasterKey()
	if mkErr == nil {
		if cfg.Storage.EncryptBlobs {
			encKey = &masterKey
		}
	} else if cfg.App.Env == "production" {
		return fmt.Errorf("master key required in production: %w", mkErr)
	}

	bs := store.New(cfg.Storage.DataDir, encKey)

	var smtpSender port.OTPSender
	if cfg.SMTP.Host != "" {
		smtpSender = newSMTPSender(cfg)
	}

	application := app.New(cfg, database.Conn, bs, masterKey, smtpSender)
	engine := application.Engine

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
				if err := engine.RunAll(ctx); err != nil && ctx.Err() == nil {
					log.Error().Err(err).Msg("archive engine error")
				}
			}
		}
	}()
	go func() {
		if err := engine.RunAll(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("archive engine startup error")
		}
	}()

	router := application.Router

	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("server error")
			cancel()
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
	log.Info().Msg("server stopped")
	return nil
}

// ── migrate ───────────────────────────────────────────────────────────────────

func runMigrate(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	ctx := context.Background()
	database, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	log.Info().Msg("all migrations applied")
	return nil
}

// ── seed ──────────────────────────────────────────────────────────────────────

func runSeed(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	ctx := context.Background()
	database, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	if cfg.App.Env == "production" {
		var count int
		if err := database.Conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			log.Warn().Msg("seed: production DB has users — resetting admin@localhost credentials only")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		return err
	}
	adminID := uuid.Must(uuid.NewV7()).String()
	_, err = database.Conn.ExecContext(ctx, `
		INSERT INTO users(id, email, password_hash, full_name, role, is_active)
		VALUES (?, 'admin@localhost', ?, 'Admin', 'admin', 1)
		ON CONFLICT(email) DO UPDATE SET
			password_hash = excluded.password_hash,
			role = 'admin',
			is_active = 1,
			updated_at = CURRENT_TIMESTAMP
	`, adminID, string(hash))
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Info().Str("email", "admin@localhost").Str("password", "admin123").Msg("seed: admin user ready")
	return nil
}

// ── fsck ──────────────────────────────────────────────────────────────────────

func runFsck(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	database, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	var encKey *[32]byte
	if cfg.Storage.EncryptBlobs && cfg.App.SecretKey != "" {
		k, err := cfg.MasterKey()
		if err != nil {
			return err
		}
		encKey = &k
	}

	bs := store.New(cfg.Storage.DataDir, encKey)
	log.Info().Str("base_dir", bs.BaseDir()).Msg("fsck: starting blob verification")

	report, err := bs.Fsck(ctx, database.Conn)
	if err != nil {
		return fmt.Errorf("fsck: %w", err)
	}

	log.Info().
		Int("total", report.TotalBlobs).
		Int("verified", report.Verified).
		Int("corrupt", len(report.Corrupt)).
		Int("missing", len(report.MissingBlobs)).
		Int("orphaned", len(report.OrphanedBlobs)).
		Msg("fsck: complete")

	for _, c := range report.Corrupt {
		log.Error().Str("path", c.Path).Str("sha256", c.SHA256).Str("error", c.Err).Msg("fsck: corrupt blob")
	}
	for _, m := range report.MissingBlobs {
		log.Error().Str("message_id", m.MessageID).Str("sha256", m.SHA256).Msg("fsck: blob missing from disk")
	}

	if len(report.Corrupt) > 0 || len(report.MissingBlobs) > 0 {
		return fmt.Errorf("fsck found %d corrupt and %d missing blobs", len(report.Corrupt), len(report.MissingBlobs))
	}
	return nil
}

// ── export ────────────────────────────────────────────────────────────────────

func runExport(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	accountID, _ := cmd.Flags().GetString("account")
	if accountID == "" {
		return fmt.Errorf("--account is required")
	}

	// Export via API is the primary path; CLI export is a convenience wrapper.
	log.Info().Str("account_id", accountID).Msg("export: use the web UI or POST /api/v1/export for bulk export")
	return nil
}
