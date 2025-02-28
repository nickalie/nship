package env

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockVaultDecrypter implements the VaultDecrypter interface for testing
type MockVaultDecrypter struct {
	decryptFunc func(content, password string) (string, error)
}

// Decrypt calls the mock function
func (m *MockVaultDecrypter) Decrypt(content, password string) (string, error) {
	return m.decryptFunc(content, password)
}

// setupTest creates a temporary directory and returns its path along with a cleanup function
func setupTest(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "env-loader-test")
	require.NoError(t, err, "Failed to create temp directory")

	return tempDir, func() {
		os.RemoveAll(tempDir)
	}
}

// captureStdin replaces os.Stdin with a pipe and returns a function to write to that pipe
// along with a cleanup function to restore the original stdin
func captureStdin(t *testing.T) (func(input string), func()) {
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err, "Failed to create pipe")

	os.Stdin = r

	return func(input string) {
			_, err := w.Write([]byte(input + "\n"))
			require.NoError(t, err, "Failed to write to mock stdin")
		}, func() {
			w.Close()
			r.Close()
			os.Stdin = origStdin
		}
}

func TestNewLoader(t *testing.T) {
	loader := NewLoader()

	// Verify the loader is of the correct type
	_, ok := loader.(*DefaultLoader)
	assert.True(t, ok, "NewLoader() did not return a *DefaultLoader")
}

func TestLoadEmptyPath(t *testing.T) {
	loader := NewLoader()

	// Empty path should not return an error
	err := loader.Load("", "")
	assert.NoError(t, err, "Load with empty path should not error")
}

func TestLoadRegularFile(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a simple .env file
	envFilePath := filepath.Join(tempDir, ".env")
	err := os.WriteFile(envFilePath, []byte("TEST_KEY=test_value"), 0644)
	require.NoError(t, err, "Failed to write test .env file")

	// Clear any pre-existing value
	os.Unsetenv("TEST_KEY")

	// Load the file
	loader := NewLoader()
	err = loader.Load(envFilePath, "")

	// Check that no error occurred and the variable was set
	assert.NoError(t, err, "Expected no error loading .env file")
	assert.Equal(t, "test_value", os.Getenv("TEST_KEY"), "Environment variable was not set correctly")
}

func TestLoadNonExistentFile(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	nonExistentPath := filepath.Join(tempDir, "non-existent.env")

	loader := NewLoader()
	err := loader.Load(nonExistentPath, "")

	assert.Error(t, err, "Expected error when loading non-existent file")
}

func TestLoadVaultFileWithPassword(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "test.vault")
	err := os.WriteFile(vaultFilePath, []byte("encrypted content"), 0644)
	require.NoError(t, err, "Failed to write test vault file")

	// Create mock decrypter that returns valid env content
	mockDecrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			if password != "test-password" {
				return "", errors.New("incorrect password")
			}
			return "TEST_VAULT_KEY=vault_value", nil
		},
	}

	// Create loader with mock decrypter
	loader := &DefaultLoader{
		vaultDecrypter: mockDecrypter,
	}

	// Clear any pre-existing value
	os.Unsetenv("TEST_VAULT_KEY")

	// Load the vault file with password
	err = loader.Load(vaultFilePath, "test-password")

	// Check that no error occurred and the variable was set
	assert.NoError(t, err, "Expected no error loading vault file")
	assert.Equal(t, "vault_value", os.Getenv("TEST_VAULT_KEY"), "Vault environment variable was not set correctly")
}

func TestLoadVaultFileWithEnvironmentPassword(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "env-password.vault")
	err := os.WriteFile(vaultFilePath, []byte("encrypted content"), 0644)
	require.NoError(t, err, "Failed to write test vault file")

	// Set environment password
	os.Setenv("VAULT_PASSWORD", "env-password")
	defer os.Unsetenv("VAULT_PASSWORD")

	// Create mock decrypter that checks for the right password
	mockDecrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			if password != "env-password" {
				return "", errors.New("incorrect password")
			}
			return "ENV_PASSWORD_KEY=env_password_value", nil
		},
	}

	// Create loader with mock decrypter
	loader := &DefaultLoader{
		vaultDecrypter: mockDecrypter,
	}

	// Clear any pre-existing value
	os.Unsetenv("ENV_PASSWORD_KEY")

	// Load the vault file without direct password (should use env var)
	err = loader.Load(vaultFilePath, "")

	// Check that no error occurred and the variable was set
	assert.NoError(t, err, "Expected no error loading vault file with env password")
	assert.Equal(t, "env_password_value", os.Getenv("ENV_PASSWORD_KEY"), "Environment variable from vault with env password was not set correctly")
}

func TestLoadVaultFileWithPromptedPassword(t *testing.T) {
	// Skip this test in automated environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test requiring stdin in CI environment")
	}

	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "prompted-password.vault")
	err := os.WriteFile(vaultFilePath, []byte("encrypted content"), 0644)
	require.NoError(t, err, "Failed to write test vault file")

	// Clear environment password
	os.Unsetenv("VAULT_PASSWORD")

	// Set up stdin capture to simulate user entering password
	writeToStdin, cleanupStdin := captureStdin(t)
	defer cleanupStdin()

	// Create mock decrypter that checks for the right password
	mockDecrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			if password != "prompted-password" {
				return "", errors.New("incorrect password")
			}
			return "PROMPTED_KEY=prompted_value", nil
		},
	}

	// Create loader with mock decrypter
	loader := &DefaultLoader{
		vaultDecrypter: mockDecrypter,
	}

	// Clear any pre-existing value
	os.Unsetenv("PROMPTED_KEY")

	// Write the password to stdin in a goroutine
	go func() {
		writeToStdin("prompted-password")
	}()

	// Load the vault file without any password (should prompt)
	err = loader.Load(vaultFilePath, "")

	// Check that no error occurred and the variable was set
	assert.NoError(t, err, "Expected no error loading vault file with prompted password")
	assert.Equal(t, "prompted_value", os.Getenv("PROMPTED_KEY"), "Environment variable from vault with prompted password was not set correctly")
}

func TestLoadVaultFileDecryptionError(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "error.vault")
	err := os.WriteFile(vaultFilePath, []byte("invalid content"), 0644)
	require.NoError(t, err, "Failed to write test vault file")

	// Create mock decrypter that always returns an error
	decryptErr := errors.New("decryption failed")
	mockDecrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			return "", decryptErr
		},
	}

	// Create loader with mock decrypter
	loader := &DefaultLoader{
		vaultDecrypter: mockDecrypter,
	}

	// Load the vault file
	err = loader.Load(vaultFilePath, "any-password")

	// Check that an error occurred
	assert.Error(t, err, "Expected error when decryption fails")
}

func TestSetEnvironmentVariablesInvalidContent(t *testing.T) {
	// Try to set environment variables from invalid content
	// Use a malformed string that should cause godotenv.Unmarshal to fail
	// The godotenv library requires a specific format and will fail on certain invalid inputs
	err := setEnvironmentVariables("===INVALID===CONTENT===")

	assert.Error(t, err, "Expected error when setting environment variables from invalid content")
}

func TestSetEnvironmentVariablesMultipleVars(t *testing.T) {
	// Clear any pre-existing values
	os.Unsetenv("VAR1")
	os.Unsetenv("VAR2")
	os.Unsetenv("VAR3")

	// Set multiple environment variables
	err := setEnvironmentVariables("VAR1=value1\nVAR2=value2\nVAR3=value3")
	assert.NoError(t, err, "Expected no error setting multiple environment variables")

	// Check that all variables were set
	assert.Equal(t, "value1", os.Getenv("VAR1"), "VAR1 was not set correctly")
	assert.Equal(t, "value2", os.Getenv("VAR2"), "VAR2 was not set correctly")
	assert.Equal(t, "value3", os.Getenv("VAR3"), "VAR3 was not set correctly")
}

func TestResolveVaultPasswordPrecedence(t *testing.T) {
	// Test password precedence: direct password > environment variable > prompt

	// Set environment password
	os.Setenv("VAULT_PASSWORD", "env-password")
	defer os.Unsetenv("VAULT_PASSWORD")

	// Direct password should take precedence
	password, err := resolveVaultPassword("direct-password", "test-vault.vault")
	assert.NoError(t, err, "Unexpected error resolving password")
	assert.Equal(t, "direct-password", password, "Direct password should be used")

	// Environment variable should be used if no direct password
	password, err = resolveVaultPassword("", "test-vault.vault")
	assert.NoError(t, err, "Unexpected error resolving password")
	assert.Equal(t, "env-password", password, "Environment variable password should be used")

	// Skip testing prompt since it requires stdin interaction
}

// TestMockVaultDecrypter tests the mock implementation to ensure it behaves as expected
func TestMockVaultDecrypter(t *testing.T) {
	mockDecrypter := &MockVaultDecrypter{
		decryptFunc: func(content, password string) (string, error) {
			if content == "valid" && password == "correct" {
				return "decrypted", nil
			}
			return "", errors.New("mock error")
		},
	}

	// Test successful case
	decrypted, err := mockDecrypter.Decrypt("valid", "correct")
	assert.NoError(t, err, "Expected no error from mock decrypter in successful case")
	assert.Equal(t, "decrypted", decrypted, "Unexpected decrypted output")

	// Test error case
	_, err = mockDecrypter.Decrypt("invalid", "wrong")
	assert.Error(t, err, "Expected error from mock decrypter in error case")
}

// TestSetEnvironmentVariablesCustomError tests error handling in setEnvironmentVariables
func TestSetEnvironmentVariablesCustomError(t *testing.T) {
	// This is a limited test since it's difficult to force os.Setenv to fail
	// in a controlled way. In a real environment, this might happen with
	// invalid environment variable names or values.

	// Try with empty content - should not error
	err := setEnvironmentVariables("")
	assert.NoError(t, err, "Expected no error for empty content")
}

// Integration-like test that uses a custom config.VaultDecrypter
func TestLoadWithCustomDecrypter(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "custom.vault")
	content := "custom encrypted content"
	err := os.WriteFile(vaultFilePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write test vault file")

	// Create a custom decrypter
	customDecrypter := &MockVaultDecrypter{
		decryptFunc: func(receivedContent, receivedPassword string) (string, error) {
			assert.Equal(t, content, receivedContent, "Content mismatch")
			assert.Equal(t, "custom-password", receivedPassword, "Password mismatch")
			return "CUSTOM_KEY=custom_value", nil
		},
	}

	// Create loader with custom decrypter
	loader := &DefaultLoader{
		vaultDecrypter: customDecrypter,
	}

	// Clear any pre-existing value
	os.Unsetenv("CUSTOM_KEY")

	// Load the vault file
	err = loader.Load(vaultFilePath, "custom-password")

	// Check that no error occurred and the variable was set
	assert.NoError(t, err, "Expected no error loading vault file with custom decrypter")
	assert.Equal(t, "custom_value", os.Getenv("CUSTOM_KEY"), "Environment variable from vault with custom decrypter was not set correctly")
}
