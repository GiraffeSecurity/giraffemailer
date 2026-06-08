package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/smtp"

	apicrypto "github.com/GiraffeSecurity/giraffemailer/internal/crypto"
	"github.com/GiraffeSecurity/giraffemailer/internal/config"
)

// decryptCredential decrypts base64(nonce||ciphertext) using masterKey and
// returns the username (may be empty) and password from the JSON credential blob.
func decryptCredential(masterKey [32]byte, encrypted string) (username, password string, err error) {
	raw, e := apicrypto.Decrypt(masterKey, encrypted)
	if e != nil {
		return "", "", fmt.Errorf("decrypt: %w", e)
	}
	var creds map[string]string
	if e := json.Unmarshal(raw, &creds); e != nil {
		return "", "", fmt.Errorf("parse creds: %w", e)
	}
	pw, ok := creds["password"]
	if !ok {
		return "", "", fmt.Errorf("no password field in credentials")
	}
	return creds["username"], pw, nil
}

// smtpSenderImpl sends OTP emails via SMTP.
type smtpSenderImpl struct {
	cfg *config.Config
}

func newSMTPSender(cfg *config.Config) *smtpSenderImpl {
	return &smtpSenderImpl{cfg: cfg}
}

func (s *smtpSenderImpl) SendOTP(ctx context.Context, to, code string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTP.Host, s.cfg.SMTP.Port)

	msg := []byte(
		"To: " + to + "\r\n" +
			"From: " + s.cfg.SMTP.From + "\r\n" +
			"Subject: Your GiraffeMail verification code\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" +
			"Your one-time code is: " + code + "\r\n" +
			"It expires in 10 minutes.\r\n",
	)

	// Try STARTTLS or plain depending on port.
	if s.cfg.SMTP.Port == 465 {
		tlsCfg := &tls.Config{ServerName: s.cfg.SMTP.Host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, s.cfg.SMTP.Host)
		if err != nil {
			return err
		}
		defer client.Close()
		if s.cfg.SMTP.Username != "" {
			if err := client.Auth(smtp.PlainAuth("", s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.Host)); err != nil {
				return err
			}
		}
		if err := client.Mail(s.cfg.SMTP.From); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(msg)
		_ = w.Close()
		return err
	}

	var auth smtp.Auth
	if s.cfg.SMTP.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
	}
	return smtp.SendMail(addr, auth, s.cfg.SMTP.From, []string{to}, msg)
}
