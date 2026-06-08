package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/authz"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
	"github.com/gofrs/uuid/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenTTL    = 30 * 24 * time.Hour
	otpTTL      = 10 * time.Minute
	bcryptCost  = 12
	tokenLength = 32
)

type rateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	max     int
	window  time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{windows: make(map[string][]time.Time), max: max, window: window}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-r.window)
	ts := r.windows[key]
	valid := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= r.max {
		r.windows[key] = valid
		return false
	}
	r.windows[key] = append(valid, now)
	return true
}

type Service struct {
	users   port.UserRepository
	otp     port.OTPSender
	limiter *rateLimiter
}

func NewService(users port.UserRepository, otp port.OTPSender) *Service {
	return &Service{
		users:   users,
		otp:     otp,
		limiter: newRateLimiter(5, time.Minute),
	}
}

type RegisterResult struct {
	OTPID   string
	Message string
}

func (s *Service) Register(ctx context.Context, clientIP string, email, password, fullName string) (RegisterResult, error) {
	if !s.limiter.allow(clientIP) {
		return RegisterResult{}, domain.ErrRateLimited
	}
	if email == "" || password == "" || fullName == "" {
		return RegisterResult{}, domain.ErrInvalidInput
	}
	if len(password) < 8 {
		return RegisterResult{}, domain.ErrInvalidInput
	}

	exists, err := s.users.EmailExists(ctx, email)
	if err != nil {
		return RegisterResult{}, err
	}
	if exists {
		return RegisterResult{}, domain.ErrConflict
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return RegisterResult{}, err
	}

	code := mustOTP()
	otpID := uuid.Must(uuid.NewV7()).String()
	exp := time.Now().Add(otpTTL).UTC().Format(time.RFC3339)
	if err := s.users.StoreOTP(ctx, otpID, email, hashOTP(code), "registration", exp); err != nil {
		return RegisterResult{}, err
	}

	userID := uuid.Must(uuid.NewV7()).String()
	if err := s.users.CreatePending(ctx, userID, email, string(hash), fullName); err != nil {
		return RegisterResult{}, err
	}

	if s.otp != nil {
		go func() {
			if err := s.otp.SendOTP(context.Background(), email, code); err != nil {
				log.Warn().Err(err).Str("email", email).Msg("send OTP failed")
			}
		}()
	}

	return RegisterResult{OTPID: otpID, Message: "OTP sent to email"}, nil
}

func (s *Service) OTPRegistration(ctx context.Context, clientIP, identifier, code string) error {
	if !s.limiter.allow(clientIP) {
		return domain.ErrRateLimited
	}
	if err := s.consumeOTP(ctx, identifier, code, "registration"); err != nil {
		return err
	}
	return s.users.Activate(ctx, identifier)
}

type LoginResult struct {
	Token     string
	ExpiresAt string
}

func (s *Service) Login(ctx context.Context, clientIP, email, password string) (LoginResult, error) {
	if !s.limiter.allow(clientIP) {
		return LoginResult{}, domain.ErrRateLimited
	}

	userID, hash, isActive, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummydummydummydummydummydummydummydummydumm"), []byte(password))
		return LoginResult{}, domain.ErrUnauthorized
	}
	if err != nil {
		return LoginResult{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return LoginResult{}, domain.ErrUnauthorized
	}
	if !isActive {
		return LoginResult{}, domain.ErrForbidden
	}

	token, tokenHash := generateToken()
	exp := time.Now().Add(tokenTTL).UTC().Format(time.RFC3339)
	if err := s.users.StoreToken(ctx, uuid.Must(uuid.NewV7()).String(), userID, tokenHash, exp); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{Token: token, ExpiresAt: exp}, nil
}

func (s *Service) ForgotPassword(ctx context.Context, clientIP, email string) (string, error) {
	if !s.limiter.allow(clientIP) {
		return "", domain.ErrRateLimited
	}
	if email == "" {
		return "", domain.ErrInvalidInput
	}

	exists, err := s.users.ActiveEmailExists(ctx, email)
	if err != nil {
		return "", err
	}
	if exists {
		code := mustOTP()
		exp := time.Now().Add(otpTTL).UTC().Format(time.RFC3339)
		_ = s.users.StoreOTP(ctx, uuid.Must(uuid.NewV7()).String(), email, hashOTP(code), "forgot_password", exp)
		if s.otp != nil {
			go func() { _ = s.otp.SendOTP(context.Background(), email, code) }()
		}
	}
	return "if the address is registered, an OTP has been sent", nil
}

func (s *Service) OTPForgotPassword(ctx context.Context, clientIP, identifier, code, newPassword string) error {
	if !s.limiter.allow(clientIP) {
		return domain.ErrRateLimited
	}
	if newPassword == "" {
		return domain.ErrInvalidInput
	}
	if len(newPassword) < 8 {
		return domain.ErrInvalidInput
	}
	if err := s.consumeOTP(ctx, identifier, code, "forgot_password"); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, identifier, false, string(hash)); err != nil {
		return err
	}
	userID, _, isActive, err := s.users.FindByEmail(ctx, identifier)
	if err == nil && isActive {
		return s.users.RevokeAllUserTokens(ctx, userID)
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return domain.ErrInvalidInput
	}
	hash, err := s.users.GetPasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)) != nil {
		return domain.ErrUnauthorized
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, true, string(newHash)); err != nil {
		return err
	}
	return s.users.RevokeAllUserTokens(ctx, userID)
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])
	return s.users.RevokeToken(ctx, tokenHash)
}

func (s *Service) CheckUser(ctx context.Context, userID string) (domain.User, error) {
	return s.users.GetProfile(ctx, userID)
}

func (s *Service) ValidateToken(ctx context.Context, raw string) (string, string, error) {
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	return s.users.ValidateToken(ctx, hash)
}

func (s *Service) ListUsers(ctx context.Context) ([]domain.User, error) {
	sub, ok := authz.Subject(ctx)
	if !ok || !sub.IsAdmin() {
		return nil, domain.ErrForbidden
	}
	return s.users.ListUsers(ctx)
}

func (s *Service) AdminUpdateUser(ctx context.Context, targetID string, role *string, active *bool) error {
	sub, ok := authz.Subject(ctx)
	if !ok || !sub.IsAdmin() {
		return domain.ErrForbidden
	}
	if targetID == sub.UserID && role != nil && *role != domain.RoleAdmin {
		return domain.ErrInvalidInput
	}
	if role != nil {
		if err := s.users.UpdateUserRole(ctx, targetID, *role); err != nil {
			return err
		}
	}
	if active != nil {
		if err := s.users.SetUserActive(ctx, targetID, *active); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) consumeOTP(ctx context.Context, identifier, code, otpType string) error {
	err := s.users.ConsumeOTP(ctx, identifier, hashOTP(code), otpType)
	if errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("OTP not found or expired")
	}
	if errors.Is(err, domain.ErrUnprocessable) {
		return fmt.Errorf("invalid OTP code")
	}
	return err
}

func generateToken() (raw, hash string) {
	b := make([]byte, tokenLength)
	_, _ = rand.Read(b)
	raw = base64.URLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return
}

func mustOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

func hashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
