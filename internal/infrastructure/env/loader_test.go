package env

import (
	"errors"
	"os"
	"path/filepath"
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

// setupTest creates a temporary directory and returns its path along with a cleanup function
func setupTest(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "env-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return tempDir, func() {
		os.RemoveAll(tempDir)
	}
}

// captureStdin replaces os.Stdin with a pipe and returns a function to write to that pipe
// along with a cleanup function to restore the original stdin
func captureStdin(t *testing.T) (func(input string), func()) {
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r

	return func(input string) {
			_, err := w.Write([]byte(input + "\n"))
			if err != nil {
				t.Fatalf("Failed to write to mock stdin: %v", err)
			}
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
	if !ok {
		t.Errorf("NewLoader() did not return a *DefaultLoader, got %T", loader)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	loader := NewLoader()

	// Empty path should not return an error
	err := loader.Load("", "")
	if err != nil {
		t.Errorf("Load with empty path should not error, got: %v", err)
	}
}

func TestLoadRegularFile(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a simple .env file
	envFilePath := filepath.Join(tempDir, ".env")
	err := os.WriteFile(envFilePath, []byte("TEST_KEY=test_value"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	// Clear any pre-existing value
	os.Unsetenv("TEST_KEY")

	// Load the file
	loader := NewLoader()
	err = loader.Load(envFilePath, "")

	// Check that no error occurred and the variable was set
	if err != nil {
		t.Errorf("Expected no error loading .env file, got: %v", err)
	}

	value := os.Getenv("TEST_KEY")
	if value != "test_value" {
		t.Errorf("Expected TEST_KEY to be 'test_value', got '%s'", value)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	nonExistentPath := filepath.Join(tempDir, "non-existent.env")

	loader := NewLoader()
	err := loader.Load(nonExistentPath, "")

	if err == nil {
		t.Errorf("Expected error when loading non-existent file, got nil")
	}
}

func TestLoadVaultFileWithPassword(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "test.vault")
	err := os.WriteFile(vaultFilePath, []byte("encrypted content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test vault file: %v", err)
	}

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
	if err != nil {
		t.Errorf("Expected no error loading vault file, got: %v", err)
	}

	value := os.Getenv("TEST_VAULT_KEY")
	if value != "vault_value" {
		t.Errorf("Expected TEST_VAULT_KEY to be 'vault_value', got '%s'", value)
	}
}

func TestLoadVaultFileWithEnvironmentPassword(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "env-password.vault")
	err := os.WriteFile(vaultFilePath, []byte("encrypted content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test vault file: %v", err)
	}

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
	if err != nil {
		t.Errorf("Expected no error loading vault file with env password, got: %v", err)
	}

	value := os.Getenv("ENV_PASSWORD_KEY")
	if value != "env_password_value" {
		t.Errorf("Expected ENV_PASSWORD_KEY to be 'env_password_value', got '%s'", value)
	}
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
	if err != nil {
		t.Fatalf("Failed to write test vault file: %v", err)
	}

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
	if err != nil {
		t.Errorf("Expected no error loading vault file with prompted password, got: %v", err)
	}

	value := os.Getenv("PROMPTED_KEY")
	if value != "prompted_value" {
		t.Errorf("Expected PROMPTED_KEY to be 'prompted_value', got '%s'", value)
	}
}

func TestLoadVaultFileDecryptionError(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "error.vault")
	err := os.WriteFile(vaultFilePath, []byte("invalid content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test vault file: %v", err)
	}

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
	if err == nil {
		t.Errorf("Expected error when decryption fails, got nil")
	}
}

func TestSetEnvironmentVariablesInvalidContent(t *testing.T) {
	// Try to set environment variables from invalid content
	// Use a malformed string that should cause godotenv.Unmarshal to fail
	// The godotenv library requires a specific format and will fail on certain invalid inputs
	err := setEnvironmentVariables("===INVALID===CONTENT===")

	if err == nil {
		t.Errorf("Expected error when setting environment variables from invalid content, got nil")
	}
}

func TestSetEnvironmentVariablesMultipleVars(t *testing.T) {
	// Clear any pre-existing values
	os.Unsetenv("VAR1")
	os.Unsetenv("VAR2")
	os.Unsetenv("VAR3")

	// Set multiple environment variables
	err := setEnvironmentVariables("VAR1=value1\nVAR2=value2\nVAR3=value3")

	if err != nil {
		t.Errorf("Expected no error setting multiple environment variables, got: %v", err)
	}

	// Check that all variables were set
	if os.Getenv("VAR1") != "value1" {
		t.Errorf("Expected VAR1 to be 'value1', got '%s'", os.Getenv("VAR1"))
	}

	if os.Getenv("VAR2") != "value2" {
		t.Errorf("Expected VAR2 to be 'value2', got '%s'", os.Getenv("VAR2"))
	}

	if os.Getenv("VAR3") != "value3" {
		t.Errorf("Expected VAR3 to be 'value3', got '%s'", os.Getenv("VAR3"))
	}
}

func TestResolveVaultPasswordPrecedence(t *testing.T) {
	// Test password precedence: direct password > environment variable > prompt

	// Set environment password
	os.Setenv("VAULT_PASSWORD", "env-password")
	defer os.Unsetenv("VAULT_PASSWORD")

	// Direct password should take precedence
	password, err := resolveVaultPassword("direct-password")
	if err != nil {
		t.Errorf("Unexpected error resolving password: %v", err)
	}
	if password != "direct-password" {
		t.Errorf("Expected 'direct-password', got '%s'", password)
	}

	// Environment variable should be used if no direct password
	password, err = resolveVaultPassword("")
	if err != nil {
		t.Errorf("Unexpected error resolving password: %v", err)
	}
	if password != "env-password" {
		t.Errorf("Expected 'env-password', got '%s'", password)
	}

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
	if err != nil {
		t.Errorf("Expected no error from mock decrypter, got: %v", err)
	}
	if decrypted != "decrypted" {
		t.Errorf("Expected 'decrypted', got '%s'", decrypted)
	}

	// Test error case
	_, err = mockDecrypter.Decrypt("invalid", "wrong")
	if err == nil {
		t.Errorf("Expected error from mock decrypter, got nil")
	}
}

// TestSetEnvironmentVariablesError tests error handling in setEnvironmentVariables
func TestSetEnvironmentVariablesCustomError(t *testing.T) {
	// This is a limited test since it's difficult to force os.Setenv to fail
	// in a controlled way. In a real environment, this might happen with
	// invalid environment variable names or values.

	// Try with empty content - should not error
	err := setEnvironmentVariables("")
	if err != nil {
		t.Errorf("Expected no error for empty content, got: %v", err)
	}
}

// Integration-like test that uses a custom config.VaultDecrypter
func TestLoadWithCustomDecrypter(t *testing.T) {
	tempDir, cleanup := setupTest(t)
	defer cleanup()

	// Create a mock vault file
	vaultFilePath := filepath.Join(tempDir, "custom.vault")
	content := "custom encrypted content"
	err := os.WriteFile(vaultFilePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test vault file: %v", err)
	}

	// Create a custom decrypter
	customDecrypter := &MockVaultDecrypter{
		decryptFunc: func(receivedContent, receivedPassword string) (string, error) {
			if receivedContent != content {
				return "", errors.New("content mismatch")
			}
			if receivedPassword != "custom-password" {
				return "", errors.New("password mismatch")
			}
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
	if err != nil {
		t.Errorf("Expected no error loading vault file with custom decrypter, got: %v", err)
	}

	value := os.Getenv("CUSTOM_KEY")
	if value != "custom_value" {
		t.Errorf("Expected CUSTOM_KEY to be 'custom_value', got '%s'", value)
	}
}
