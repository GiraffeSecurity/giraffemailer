package crypto

import (
	"encoding/json"
	"fmt"
)

type Vault struct {
	key [32]byte
}

func NewVault(key [32]byte) *Vault {
	return &Vault{key: key}
}

func (v *Vault) EncryptCredentials(password string) (string, error) {
	credJSON, _ := json.Marshal(map[string]string{"password": password})
	return Encrypt(v.key, credJSON)
}

func (v *Vault) DecryptPassword(encrypted string) (string, error) {
	raw, err := Decrypt(v.key, encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	var creds map[string]string
	if err := json.Unmarshal(raw, &creds); err != nil {
		return "", fmt.Errorf("parse creds: %w", err)
	}
	pw, ok := creds["password"]
	if !ok {
		return "", fmt.Errorf("no password field in credentials")
	}
	return pw, nil
}
