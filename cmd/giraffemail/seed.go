package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gofrs/uuid/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const defaultAdminPassword = "admin123"
const defaultAdminEmail = "admin@localhost"

func seedAdmin(ctx context.Context, conn *sql.DB, force bool) error {
	var existingID string
	err := conn.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, defaultAdminEmail).Scan(&existingID)
	if err == nil && !force {
		log.Info().Str("email", defaultAdminEmail).Msg("seed: admin already exists — skipping (use --force to reset password)")
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("lookup admin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), 12)
	if err != nil {
		return err
	}

	if force && existingID != "" {
		_, err = conn.ExecContext(ctx, `
			UPDATE users SET password_hash = ?, role = 'admin', is_active = 1, updated_at = CURRENT_TIMESTAMP
			WHERE email = ?`, string(hash), defaultAdminEmail)
		if err != nil {
			return fmt.Errorf("force reset admin: %w", err)
		}
		log.Warn().Str("email", defaultAdminEmail).Msg("seed: --force reset admin password to default")
		return nil
	}

	adminID := uuid.Must(uuid.NewV7()).String()
	_, err = conn.ExecContext(ctx, `
		INSERT INTO users(id, email, password_hash, full_name, role, is_active)
		VALUES (?, ?, ?, 'Admin', 'admin', 1)`, adminID, defaultAdminEmail, string(hash))
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Info().Str("email", defaultAdminEmail).Str("password", defaultAdminPassword).Msg("seed: admin user ready")
	return nil
}
