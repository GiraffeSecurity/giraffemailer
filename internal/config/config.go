package config

import (
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Security SecurityConfig `yaml:"security"`
	Storage  StorageConfig  `yaml:"storage"`
	Archive  ArchiveConfig  `yaml:"archive"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	Log      LogConfig      `yaml:"log"`
}

type AppConfig struct {
	Env       string `yaml:"env"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	SecretKey string `yaml:"secret_key"`
}

// SecurityConfig holds auth and HTTP hardening toggles (Strategy: secure-by-default in production).
type SecurityConfig struct {
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	AllowRegistration  bool     `yaml:"allow_registration"`
	CookieSecure       bool     `yaml:"cookie_secure"`
}

type StorageConfig struct {
	DataDir      string `yaml:"data_dir"`
	EncryptBlobs bool   `yaml:"encrypt_blobs"`
}

type ArchiveConfig struct {
	WorkerCount    int   `yaml:"worker_count"`
	BatchSizeBytes int64 `yaml:"batch_size_bytes"`
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

type LogConfig struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // json | text
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	cfg := defaults()
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GM_SECRET_KEY"); v != "" {
		cfg.App.SecretKey = v
	}
	if v := os.Getenv("GM_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}
	if os.Getenv("GM_ENV") != "" {
		cfg.App.Env = os.Getenv("GM_ENV")
	}
}

// MasterKey decodes the hex secret key into 32 bytes.
// In dev mode with no key configured, returns a zero key so credentials
// can still be stored and retrieved without requiring manual key setup.
func (c *Config) MasterKey() ([32]byte, error) {
	if c.App.SecretKey == "" {
		if c.App.Env != "production" {
			return [32]byte{}, nil
		}
		return [32]byte{}, fmt.Errorf("secret_key is required in production mode")
	}
	b, err := hex.DecodeString(c.App.SecretKey)
	if err != nil || len(b) != 32 {
		return [32]byte{}, fmt.Errorf("secret_key must be 64 hex characters (32 bytes)")
	}
	var key [32]byte
	copy(key[:], b)
	return key, nil
}

func defaults() *Config {
	return &Config{
		App: AppConfig{
			Env:  "production",
			Host: "0.0.0.0",
			Port: 9191,
		},
		Security: SecurityConfig{
			CORSAllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
			AllowRegistration:  false,
			CookieSecure:       false,
		},
		Storage: StorageConfig{
			DataDir:      "./data",
			EncryptBlobs: false,
		},
		Archive: ArchiveConfig{
			WorkerCount:    4,
			BatchSizeBytes: 8 * 1024 * 1024, // 8 MB
		},
		SMTP: SMTPConfig{
			Port: 587,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func validate(cfg *Config) error {
	if cfg.App.Env == "production" && cfg.App.SecretKey == "" {
		return fmt.Errorf("app.secret_key is required in production mode")
	}
	if cfg.App.SecretKey != "" {
		b, err := hex.DecodeString(cfg.App.SecretKey)
		if err != nil || len(b) != 32 {
			return fmt.Errorf("app.secret_key must be exactly 64 hex characters")
		}
	}
	if cfg.App.Port <= 0 || cfg.App.Port > 65535 {
		return fmt.Errorf("app.port must be 1–65535")
	}
	if cfg.App.Env == "production" && !cfg.Storage.EncryptBlobs {
		return fmt.Errorf("storage.encrypt_blobs must be true in production mode")
	}
	if cfg.App.Env == "production" && cfg.Security.AllowRegistration {
		return fmt.Errorf("security.allow_registration must be false in production mode")
	}
	if cfg.Archive.WorkerCount <= 0 {
		return fmt.Errorf("archive.worker_count must be >= 1")
	}
	return nil
}
