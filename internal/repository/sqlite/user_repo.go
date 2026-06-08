package sqlite

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&count)
	return count > 0, err
}

func (r *UserRepo) CreatePending(ctx context.Context, userID, email, hash, fullName string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users(id, email, password_hash, full_name, role, is_active) VALUES (?, ?, ?, ?, 'user', 0)
	`, userID, email, hash, fullName)
	return err
}

func (r *UserRepo) Activate(ctx context.Context, email string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_active = 1, updated_at = CURRENT_TIMESTAMP WHERE email = ?`, email)
	return err
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (id, hash string, isActive bool, err error) {
	var active int
	err = r.db.QueryRowContext(ctx,
		`SELECT id, password_hash, is_active FROM users WHERE email = ?`, email,
	).Scan(&id, &hash, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, domain.ErrNotFound
	}
	return id, hash, active == 1, err
}

func (r *UserRepo) GetPasswordHash(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return hash, err
}

func (r *UserRepo) UpdatePassword(ctx context.Context, emailOrID string, byID bool, hash string) error {
	if byID {
		_, err := r.db.ExecContext(ctx,
			`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			hash, emailOrID)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE email = ?`,
		hash, emailOrID)
	return err
}

func (r *UserRepo) GetProfile(ctx context.Context, userID string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, full_name, role FROM users WHERE id = ?`, userID,
	).Scan(&u.ID, &u.Email, &u.FullName, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}

func (r *UserRepo) StoreOTP(ctx context.Context, id, identifier, codeHash, otpType, expiresAt string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_otps(id, identifier, code, type, expires_at) VALUES (?, ?, ?, ?, ?)
	`, id, identifier, codeHash, otpType, expiresAt)
	return err
}

func (r *UserRepo) ConsumeOTP(ctx context.Context, identifier, codeHash, otpType string) error {
	var id, storedHash string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, code FROM user_otps
		WHERE  identifier = ? AND type = ? AND is_used = 0 AND expires_at > CURRENT_TIMESTAMP
		ORDER  BY created_at DESC LIMIT 1
	`, identifier, otpType).Scan(&id, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(codeHash)) != 1 {
		return domain.ErrUnprocessable
	}
	_, err = r.db.ExecContext(ctx, `UPDATE user_otps SET is_used = 1 WHERE id = ?`, id)
	return err
}

func (r *UserRepo) StoreToken(ctx context.Context, id, userID, tokenHash, expiresAt string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_tokens(id, user_id, token_hash, expires_at) VALUES (?, ?, ?, ?)
	`, id, userID, tokenHash, expiresAt)
	return err
}

func (r *UserRepo) ValidateToken(ctx context.Context, tokenHash string) (userID, role string, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT t.user_id, u.role FROM user_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE  t.token_hash = ? AND t.expires_at > CURRENT_TIMESTAMP AND u.is_active = 1
	`, tokenHash).Scan(&userID, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", domain.ErrUnauthorized
	}
	return userID, role, err
}

func (r *UserRepo) RevokeToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_tokens WHERE token_hash = ?`, tokenHash)
	return err
}

func (r *UserRepo) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_tokens WHERE user_id = ?`, userID)
	return err
}

func (r *UserRepo) ActiveEmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ? AND is_active = 1`, email).Scan(&count)
	return count > 0, err
}

func (r *UserRepo) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, email, full_name, role, is_active FROM users ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		var active int
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &active); err != nil {
			continue
		}
		u.IsActive = active == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepo) UpdateUserRole(ctx context.Context, userID, role string) error {
	if role != domain.RoleAdmin && role != domain.RoleUser {
		return domain.ErrInvalidInput
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, role, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) SetUserActive(ctx context.Context, userID string, active bool) error {
	v := 0
	if active {
		v = 1
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_active = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, v, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	if !active {
		_ = r.RevokeAllUserTokens(ctx, userID)
	}
	return nil
}
