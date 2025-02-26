package config

import (
	"fmt"
	"os"

	"github.com/sosedoff/ansible-vault-go"
)

// VaultDecrypter defines the interface for decrypting Ansible Vault
// encrypted content.
type VaultDecrypter interface {
	Decrypt(content, password string) (string, error)
}

// DefaultVaultDecrypter implements VaultDecrypter using ansible-vault-go.
type DefaultVaultDecrypter struct{}

// NewVaultDecrypter creates a new instance of the default vault decrypter.
func NewVaultDecrypter() VaultDecrypter {
	return &DefaultVaultDecrypter{}
}

// Decrypt decrypts content encrypted with Ansible Vault.
func (d *DefaultVaultDecrypter) Decrypt(content, password string) (string, error) {
	return vault.Decrypt(content, password)
}

// LoadVaultFile loads and decrypts an Ansible Vault file.
func LoadVaultFile(path, password string, decrypter VaultDecrypter) (string, error) {
	if password == "" {
		return "", fmt.Errorf("vault password is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read vault file: %w", err)
	}

	decrypted, err := decrypter.Decrypt(string(data), password)
	if err != nil {
		return "", fmt.Errorf("vault decryption failed: %w", err)
	}

	return decrypted, nil
}
