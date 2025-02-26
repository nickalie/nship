// Package env provides functionality for loading environment variables from different sources.
package env

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/nickalie/nship/internal/config"
	"golang.org/x/term"
)

// Loader defines the interface for loading environment variables.
type Loader interface {
	Load(path, vaultPassword string) error
}

// DefaultLoader implements the Loader interface using godotenv.
type DefaultLoader struct {
	vaultDecrypter config.VaultDecrypter
}

// NewLoader creates a new environment loader with default implementations.
func NewLoader() Loader {
	return &DefaultLoader{
		vaultDecrypter: config.NewVaultDecrypter(),
	}
}

// Load loads environment variables from a file.
func (l *DefaultLoader) Load(path, vaultPassword string) error {
	if path == "" {
		return nil
	}

	if strings.HasSuffix(path, ".vault") {
		return l.loadVaultFile(path, vaultPassword)
	}

	return godotenv.Load(path)
}

// loadVaultFile loads environment variables from an Ansible Vault encrypted file.
func (l *DefaultLoader) loadVaultFile(path, password string) error {
	// Handle password resolution
	password, err := resolveVaultPassword(password)
	if err != nil {
		return err
	}

	// Load and decrypt vault file
	decrypted, err := config.LoadVaultFile(path, password, l.vaultDecrypter)
	if err != nil {
		return err
	}

	// Parse environment variables
	return setEnvironmentVariables(decrypted)
}

// resolveVaultPassword determines the password to use for decryption
func resolveVaultPassword(password string) (string, error) {
	if password != "" {
		return password, nil
	}

	// Check environment variable
	if envPwd := os.Getenv("VAULT_PASSWORD"); envPwd != "" {
		return envPwd, nil
	}

	// Prompt user for password
	promptedPwd, err := promptVaultPassword()
	if err != nil {
		return "", fmt.Errorf("failed to get vault password: %w", err)
	}
	return promptedPwd, nil
}

// setEnvironmentVariables parses and sets environment variables from decrypted content
func setEnvironmentVariables(decrypted string) error {
	envMap, err := godotenv.Unmarshal(decrypted)
	if err != nil {
		return fmt.Errorf("environment unmarshaling failed: %w", err)
	}

	for k, v := range envMap {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", k, err)
		}
	}

	return nil
}

// promptVaultPassword prompts the user for a vault password.
func promptVaultPassword() (string, error) {
	fmt.Print("Enter vault password: ")

	if password, err := term.ReadPassword(int(syscall.Stdin)); err == nil {
		fmt.Println() // Add newline after password input
		return string(password), nil
	}

	// Fallback to standard input if term.ReadPassword fails
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadBytes('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	return strings.TrimSpace(string(password)), nil
}
