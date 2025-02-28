package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockVaultDecrypter implements the VaultDecrypter interface for testing
type MockVaultDecrypter struct {
	decryptFunc func(content, password string) (string, error)
}

// Decrypt calls the mock function
func (m *MockVaultDecrypter) Decrypt(content, password string) (string, error) {
	return m.decryptFunc(content, password)
}

func TestNewVaultDecrypter(t *testing.T) {
	decrypter := NewVaultDecrypter()

	// Ensure the returned instance is of the correct type
	_, ok := decrypter.(*DefaultVaultDecrypter)
	assert.True(t, ok, "NewVaultDecrypter() did not return a *DefaultVaultDecrypter")
}

func TestDefaultVaultDecrypterDecrypt(t *testing.T) {
	// This is a limited test since we can't easily use real vault encryption in unit tests
	// In real environment, we would use integration tests to verify against real encrypted content

	decrypter := NewVaultDecrypter()

	// Test with invalid encrypted content
	_, err := decrypter.Decrypt("invalid content", "password")
	assert.Error(t, err, "Expected error when decrypting invalid content")
}

func TestLoadVaultFileEmptyPassword(t *testing.T) {
	// Mock decrypter won't be called because the function should fail early
	decrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			t.Errorf("Decrypt should not be called with empty password")
			return "", nil
		},
	}

	_, err := LoadVaultFile("dummy/path", "", decrypter)

	assert.Error(t, err, "Expected error with empty password")
	assert.Equal(t, "vault password is required", err.Error(), "Expected specific error message")
}

func TestLoadVaultFileNonExistentFile(t *testing.T) {
	// Mock decrypter won't be called because the file doesn't exist
	decrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			t.Errorf("Decrypt should not be called for non-existent file")
			return "", nil
		},
	}

	_, err := LoadVaultFile("non/existent/file", "password", decrypter)

	assert.Error(t, err, "Expected error with non-existent file")
	assert.True(t, os.IsNotExist(unwrapError(err)), "Expected file not found error")
}

func TestLoadVaultFileDecryptionError(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "vault-test")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "vault-file")
	content := "encrypted content"

	err = os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err, "Failed to write test file")

	// Mock decrypter that returns an error
	decryptErr := fmt.Errorf("decryption failed")
	decrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			return "", decryptErr
		},
	}

	_, err = LoadVaultFile(filePath, "password", decrypter)

	assert.Error(t, err, "Expected error during decryption")
	assert.True(t, errorContains(err, decryptErr.Error()),
		"Error should contain the decryption error message")
}

func TestLoadVaultFileSuccess(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "vault-test")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "vault-file")
	content := "encrypted content"
	decryptedContent := "decrypted content"

	err = os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err, "Failed to write test file")

	// Mock decrypter that returns successfully
	decrypter := &MockVaultDecrypter{
		decryptFunc: func(receivedContent, receivedPassword string) (string, error) {
			// Verify the content and password passed to Decrypt
			assert.Equal(t, content, receivedContent, "Content mismatch")
			assert.Equal(t, "correctpassword", receivedPassword, "Password mismatch")

			return decryptedContent, nil
		},
	}

	result, err := LoadVaultFile(filePath, "correctpassword", decrypter)

	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, decryptedContent, result, "Decrypted content mismatch")
}

// Helper function to check if an error contains a specific substring
func errorContains(err error, substr string) bool {
	return err != nil && strings.Contains(err.Error(), substr)
}

// Helper function to get the original error from a wrapped error
func unwrapError(err error) error {
	if err == nil {
		return nil
	}

	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			break
		}
		err = unwrapped
	}

	return err
}
