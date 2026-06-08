// Package crypto provides AES-256-GCM helpers for encrypting IMAP credentials
// and other sensitive fields stored in SQLite.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const gcmNonceSize = 12

// Encrypt encrypts plaintext with AES-256-GCM using key, then base64-encodes
// the result as "nonce||ciphertext".
func Encrypt(key [32]byte, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	combined := append(nonce, ct...) //nolint:gocritic
	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt reverses Encrypt.
func Decrypt(key [32]byte, encoded string) ([]byte, error) {
	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	if len(combined) < gcmNonceSize {
		return nil, errors.New("ciphertext too short")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ct := combined[:gcmNonceSize], combined[gcmNonceSize:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plain, nil
}

// EncryptString encrypts a UTF-8 string.
func EncryptString(key [32]byte, s string) (string, error) {
	return Encrypt(key, []byte(s))
}

// DecryptString decrypts to a UTF-8 string.
func DecryptString(key [32]byte, encoded string) (string, error) {
	b, err := Decrypt(key, encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
