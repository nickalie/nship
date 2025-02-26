package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if !ok {
		t.Errorf("NewVaultDecrypter() did not return a *DefaultVaultDecrypter, got %T", decrypter)
	}
}

func TestDefaultVaultDecrypterDecrypt(t *testing.T) {
	// This is a limited test since we can't easily use real vault encryption in unit tests
	// In real environment, we would use integration tests to verify against real encrypted content

	decrypter := NewVaultDecrypter()

	// Test with invalid encrypted content
	_, err := decrypter.Decrypt("invalid content", "password")
	if err == nil {
		t.Errorf("Expected error when decrypting invalid content, but got nil")
	}
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

	if err == nil {
		t.Errorf("Expected error with empty password, got nil")
	}

	if err != nil && err.Error() != "vault password is required" {
		t.Errorf("Expected error message 'vault password is required', got '%s'", err.Error())
	}
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

	if err == nil {
		t.Errorf("Expected error with non-existent file, got nil")
	}

	if err != nil && !os.IsNotExist(unwrapError(err)) {
		t.Errorf("Expected file not found error, got: %v", err)
	}
}

func TestLoadVaultFileDecryptionError(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "vault-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "vault-file")
	content := "encrypted content"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Mock decrypter that returns an error
	decryptErr := fmt.Errorf("decryption failed")
	decrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			return "", decryptErr
		},
	}

	_, err = LoadVaultFile(filePath, "password", decrypter)

	if err == nil {
		t.Errorf("Expected error during decryption, got nil")
	}

	if err != nil && !errorContains(err, decryptErr.Error()) {
		t.Errorf("Expected error to contain '%s', got: %v", decryptErr.Error(), err)
	}
}

func TestLoadVaultFileSuccess(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "vault-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "vault-file")
	content := "encrypted content"
	decryptedContent := "decrypted content"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Mock decrypter that returns successfully
	decrypter := &MockVaultDecrypter{
		decryptFunc: func(receivedContent, receivedPassword string) (string, error) {
			// Verify the content and password passed to Decrypt
			if receivedContent != content {
				t.Errorf("Expected content '%s', got '%s'", content, receivedContent)
			}

			if receivedPassword != "correctpassword" {
				t.Errorf("Expected password 'correctpassword', got '%s'", receivedPassword)
			}

			return decryptedContent, nil
		},
	}

	result, err := LoadVaultFile(filePath, "correctpassword", decrypter)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result != decryptedContent {
		t.Errorf("Expected decrypted content '%s', got '%s'", decryptedContent, result)
	}
}

// Helper function to check if an error contains a specific substring
func errorContains(err error, substr string) bool {
	return err != nil && contains(fmt.Sprint(err), substr)
}

// Simple string contains helper
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
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
