package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

func TestLoadYAMLConfig(t *testing.T) {
	// Create temporary YAML config file
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary private key file
	privateKeyPath := filepath.Join(tmpDir, "key.pem")
	if err := os.WriteFile(privateKeyPath, []byte("dummy private key"), 0600); err != nil {
		t.Fatalf("Failed to create private key file: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
targets:
  - name: web-server
    host: web.example.com
    user: admin
    port: 2222
    private_key: ` + privateKeyPath + `
  - host: db.example.com
    user: admin
    password: password123

jobs:
  - name: setup
    steps:
      - run: mkdir -p /var/www
      - run: chown www-data:www-data /var/www
  - name: deploy
    steps:
      - copy:
          src: ./config/nginx.conf
          dst: /etc/nginx/nginx.conf
      - docker:
          image: nginx:latest
          name: web
          ports:
            - 80:80
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	loader := NewLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify targets
	if len(config.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(config.Targets))
	}

	// First target
	if config.Targets[0].Name != "web-server" {
		t.Errorf("Expected first target name to be 'web-server', got %s", config.Targets[0].Name)
	}
	if config.Targets[0].Host != "web.example.com" {
		t.Errorf("Expected first target host to be 'web.example.com', got %s", config.Targets[0].Host)
	}
	if config.Targets[0].Port != 2222 {
		t.Errorf("Expected first target port to be 2222, got %d", config.Targets[0].Port)
	}
	if config.Targets[0].PrivateKey != privateKeyPath {
		t.Errorf("Expected first target private key to be '%s', got %s", privateKeyPath, config.Targets[0].PrivateKey)
	}

	// Second target
	if config.Targets[1].Host != "db.example.com" {
		t.Errorf("Expected second target host to be 'db.example.com', got %s", config.Targets[1].Host)
	}
	if config.Targets[1].Password != "password123" {
		t.Errorf("Expected second target password to be 'password123', got %s", config.Targets[1].Password)
	}

	// Verify jobs
	if len(config.Jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(config.Jobs))
	}

	// Setup job
	if config.Jobs[0].Name != "setup" {
		t.Errorf("Expected first job name to be 'setup', got %s", config.Jobs[0].Name)
	}
	if len(config.Jobs[0].Steps) != 2 {
		t.Errorf("Expected setup job to have 2 steps, got %d", len(config.Jobs[0].Steps))
	}
	if config.Jobs[0].Steps[0].Run != "mkdir -p /var/www" {
		t.Errorf("Expected first step command to be 'mkdir -p /var/www', got %s", config.Jobs[0].Steps[0].Run)
	}

	// Deploy job
	if config.Jobs[1].Name != "deploy" {
		t.Errorf("Expected second job name to be 'deploy', got %s", config.Jobs[1].Name)
	}
	if len(config.Jobs[1].Steps) != 2 {
		t.Errorf("Expected deploy job to have 2 steps, got %d", len(config.Jobs[1].Steps))
	}

	// Copy step
	copyStep := config.Jobs[1].Steps[0].Copy
	if copyStep == nil {
		t.Fatalf("Expected first step to be a copy step, but Copy is nil")
	}
	if copyStep.Src != "./config/nginx.conf" {
		t.Errorf("Expected copy step source to be './config/nginx.conf', got %s", copyStep.Src)
	}

	// Docker step
	dockerStep := config.Jobs[1].Steps[1].Docker
	if dockerStep == nil {
		t.Fatalf("Expected second step to be a docker step, but Docker is nil")
	}
	if dockerStep.Image != "nginx:latest" {
		t.Errorf("Expected docker image to be 'nginx:latest', got %s", dockerStep.Image)
	}
}

func TestReplaceEnvVariables(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("TEST_HOST", "test.example.com")
	os.Setenv("TEST_USER", "testuser")
	os.Setenv("TEST_PORT", "1234")

	// Test content with environment variables
	content := `
targets:
  - host: ${TEST_HOST}
    user: ${TEST_USER}
    port: ${TEST_PORT}
    password: secret
`

	// Apply environment variable substitution
	result := replaceEnvVariables(content)

	// Verify substitutions
	if !strings.Contains(result, "test.example.com") {
		t.Errorf("Expected content to contain 'test.example.com', but it doesn't")
	}
	if !strings.Contains(result, "testuser") {
		t.Errorf("Expected content to contain 'testuser', but it doesn't")
	}
	if !strings.Contains(result, "1234") {
		t.Errorf("Expected content to contain '1234', but it doesn't")
	}
	if strings.Contains(result, "${TEST_HOST}") {
		t.Errorf("Environment variable ${TEST_HOST} was not replaced")
	}
	if strings.Contains(result, "${TEST_USER}") {
		t.Errorf("Environment variable ${TEST_USER} was not replaced")
	}
	if strings.Contains(result, "${TEST_PORT}") {
		t.Errorf("Environment variable ${TEST_PORT} was not replaced")
	}

	// Test with non-existent environment variable
	content = "host: ${NONEXISTENT_VAR}"
	result = replaceEnvVariables(content)

	// Non-existent variables should be replaced with empty string
	if strings.Contains(result, "${NONEXISTENT_VAR}") {
		t.Errorf("Non-existent environment variable was not replaced")
	}
	if !strings.Contains(result, "host: ") {
		t.Errorf("Expected 'host: ' after replacement, got %s", result)
	}
}

// setupTestLoader creates a test loader with a mock command runner
func setupTestLoader(cmdOutput []byte, cmdErr error) *DefaultLoader {
	validate := validator.New()
	loader := &DefaultLoader{
		validator: validate,
		loaders:   make(map[string]func(string) (*Config, error)),
		cmdRunner: func(dir string, args ...string) ([]byte, error) {
			return cmdOutput, cmdErr
		},
	}

	// Register default loaders
	loader.loaders[".yaml"] = loader.loadYAMLConfig
	loader.loaders[".yml"] = loader.loadYAMLConfig
	loader.loaders[".ts"] = loader.loadTypeScriptConfig
	loader.loaders[".js"] = loader.loadJavaScriptConfig
	loader.loaders[".mjs"] = loader.loadJavaScriptConfig
	loader.loaders[".go"] = loader.loadGolangConfig

	return loader
}

// mockValidOutput creates mock output for command execution with valid config
func mockValidOutput(config *Config) []byte {
	// Create a valid config output as would be returned by a script
	jsonBytes, _ := json.Marshal(config)
	return []byte(string(jsonBytes) + "\n")
}

func TestLoadJavaScriptConfig(t *testing.T) {
	// Create a simple valid config for testing
	validConfig := &Config{
		Targets: []*target.Target{
			{
				Name:     "test-server",
				Host:     "test.example.com",
				User:     "testuser",
				Password: "testpass",
			},
		},
		Jobs: []*job.Job{
			{
				Name: "test-job",
				Steps: []*job.Step{
					{Run: "echo 'Hello, world!'"},
				},
			},
		},
	}

	tests := []struct {
		name        string
		cmdOutput   []byte
		cmdErr      error
		expectError bool
	}{
		{
			name:        "Valid JavaScript Config",
			cmdOutput:   mockValidOutput(validConfig),
			cmdErr:      nil,
			expectError: false,
		},
		{
			name:        "Command Error",
			cmdOutput:   []byte("Error: Cannot find module"),
			cmdErr:      fmt.Errorf("exit status 1"),
			expectError: true,
		},
		{
			name:        "Invalid JSON Output",
			cmdOutput:   []byte("not-json\n"),
			cmdErr:      nil,
			expectError: true,
		},
		{
			name:        "Empty Output",
			cmdOutput:   []byte(""),
			cmdErr:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := setupTestLoader(tt.cmdOutput, tt.cmdErr)

			// Create a temporary JS file path (won't be accessed, just used for name)
			tmpDir, _ := os.MkdirTemp("", "js-test")
			defer os.RemoveAll(tmpDir)
			jsPath := filepath.Join(tmpDir, "config.js")

			config, err := loader.loadJavaScriptConfig(jsPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify config was loaded correctly
				if config.Targets[0].Host != validConfig.Targets[0].Host {
					t.Errorf("Expected host %s, got %s", validConfig.Targets[0].Host, config.Targets[0].Host)
				}

				if config.Jobs[0].Name != validConfig.Jobs[0].Name {
					t.Errorf("Expected job name %s, got %s", validConfig.Jobs[0].Name, config.Jobs[0].Name)
				}
			}
		})
	}
}

func TestLoadTypeScriptConfig(t *testing.T) {
	// Create a simple valid config for testing
	validConfig := &Config{
		Targets: []*target.Target{
			{
				Name:     "ts-server",
				Host:     "typescript.example.com",
				User:     "tsuser",
				Password: "tspass",
			},
		},
		Jobs: []*job.Job{
			{
				Name: "ts-job",
				Steps: []*job.Step{
					{Run: "echo 'TypeScript Config'"},
				},
			},
		},
	}

	tests := []struct {
		name        string
		cmdOutput   []byte
		cmdErr      error
		expectError bool
	}{
		{
			name:        "Valid TypeScript Config",
			cmdOutput:   mockValidOutput(validConfig),
			cmdErr:      nil,
			expectError: false,
		},
		{
			name:        "Command Error",
			cmdOutput:   []byte("Error: TypeScript compilation failed"),
			cmdErr:      fmt.Errorf("exit status 1"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary TS file (won't be accessed by our mock)
			tmpDir, _ := os.MkdirTemp("", "ts-test")
			defer os.RemoveAll(tmpDir)
			tsPath := filepath.Join(tmpDir, "config.ts")

			// Write dummy content to the file
			err := os.WriteFile(tsPath, []byte("export default {}"), 0644)
			if err != nil {
				t.Fatalf("Failed to write test TS file: %v", err)
			}

			loader := setupTestLoader(tt.cmdOutput, tt.cmdErr)

			// Skip actual TypeScript compilation during testing
			oldJSLoader := loader.loaders[".js"]
			loader.loaders[".js"] = func(path string) (*Config, error) {
				// Instead of actually running JS, return the mock result
				if tt.cmdErr != nil {
					return nil, tt.cmdErr
				}
				// Parse the mock output
				var config Config
				if err := json.Unmarshal(tt.cmdOutput, &config); err != nil {
					return nil, err
				}
				return &config, nil
			}

			// Run the test
			config, err := loader.loadTypeScriptConfig(tsPath)

			// Restore the original JS loader
			loader.loaders[".js"] = oldJSLoader

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Check config values
				if config.Targets[0].Host != validConfig.Targets[0].Host {
					t.Errorf("Expected host %s, got %s", validConfig.Targets[0].Host, config.Targets[0].Host)
				}
			}
		})
	}
}

func TestLoadGolangConfig(t *testing.T) {
	// Create a simple valid config for testing
	validConfig := &Config{
		Targets: []*target.Target{
			{
				Name:     "go-server",
				Host:     "golang.example.com",
				User:     "gouser",
				Password: "gopass",
			},
		},
		Jobs: []*job.Job{
			{
				Name: "go-job",
				Steps: []*job.Step{
					{Run: "echo 'Go Config'"},
				},
			},
		},
	}

	// Setup test loader with mock output
	loader := setupTestLoader(mockValidOutput(validConfig), nil)

	// Create a temporary Go file path (won't be accessed, just used for name)
	tmpDir, _ := os.MkdirTemp("", "go-test")
	defer os.RemoveAll(tmpDir)
	goPath := filepath.Join(tmpDir, "config.go")

	// Load the config using our mocked function
	config, err := loader.loadGolangConfig(goPath)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if config.Targets[0].Host != validConfig.Targets[0].Host {
		t.Errorf("Expected host %s, got %s", validConfig.Targets[0].Host, config.Targets[0].Host)
	}

	if config.Jobs[0].Name != validConfig.Jobs[0].Name {
		t.Errorf("Expected job name %s, got %s", validConfig.Jobs[0].Name, config.Jobs[0].Name)
	}
}

func TestInvalidConfig(t *testing.T) {
	// Create temporary YAML config file with invalid configuration
	tmpDir, err := os.MkdirTemp("", "config-test-invalid")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid-config.yaml")
	invalidConfig := `
targets:
  - name: missing-required-fields
jobs:
  - name: empty-job
`

	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	loader := NewLoader()
	_, err = loader.Load(configPath)

	// Expect validation error
	if err == nil {
		t.Fatalf("Expected validation error but got nil")
	}

	// Check that error message contains validation failure
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected error to contain 'validation failed', got: %v", err)
	}
}

func TestAutoNaming(t *testing.T) {
	// Test that jobs and targets get default names if not specified
	config := &Config{
		Targets: []*target.Target{
			{Host: "unnamed.example.com", User: "user", Password: "password"}, // Add password to satisfy validation
		},
		Jobs: []*job.Job{
			{Steps: []*job.Step{{Run: "echo 'unnamed job'"}}},
		},
	}

	// Create loader for validation
	loader := &DefaultLoader{
		validator: validator.New(),
	}

	// Validate should set default names
	err := loader.validateConfig(config)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Check target naming
	if config.Targets[0].Name != "unnamed.example.com" {
		t.Errorf("Expected target name to default to host, got: %s", config.Targets[0].Name)
	}

	// Check job naming
	if config.Jobs[0].Name != "job-1" {
		t.Errorf("Expected job name to be 'job-1', got: %s", config.Jobs[0].Name)
	}
}

func TestUnsupportedExtension(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load("config.unsupported")

	if err == nil {
		t.Fatalf("Expected error for unsupported extension, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported config file extension") {
		t.Errorf("Expected error to mention unsupported extension, got: %v", err)
	}
}

// TestExecCommand tests the execCommand function with various scenarios
func TestExecCommand(t *testing.T) {
	// Capture stdout and stderr for verification
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "execcommand-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("SimpleCommand", func(t *testing.T) {
		// Capture stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r)
			outC <- buf.String()
		}()

		// Use different commands based on OS
		var cmd string
		var args []string
		expected := "Hello, World!"

		if runtime.GOOS == "windows" {
			cmd = "cmd"
			args = []string{"/c", "echo", expected}
		} else {
			cmd = "echo"
			args = []string{expected}
		}

		// Run command
		output, err := execCommand("", append([]string{cmd}, args...)...)

		// Close write end of pipe to get output
		w.Close()
		stdout := <-outC
		os.Stdout = originalStdout

		// Verify
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !strings.Contains(string(output), expected) {
			t.Errorf("Expected output to contain '%s', got: '%s'", expected, string(output))
		}

		if !strings.Contains(stdout, expected) {
			t.Errorf("Expected stdout to contain '%s', got: '%s'", expected, stdout)
		}
	})

	t.Run("CommandWithError", func(t *testing.T) {
		// Capture stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		errC := make(chan string)
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r)
			errC <- buf.String()
		}()

		// Run a command that will fail - use a command name that doesn't exist on any platform
		_, err := execCommand("", "command-that-does-not-exist")

		// Close write end of pipe to get error output
		w.Close()
		stderr := <-errC
		os.Stderr = originalStderr
		_ = stderr // This line prevents the "declared and not used" error for stderr

		// Verify
		if err == nil {
			t.Errorf("Expected an error, got nil")
		}

		// No assertion on stderr content as it may vary by OS
	})

	t.Run("CommandWithStderr", func(t *testing.T) {
		// Create a platform-specific script that outputs to both stdout and stderr
		var cmd string
		var args []string
		var scriptPath string

		if runtime.GOOS == "windows" {
			scriptPath = filepath.Join(tempDir, "stderr.bat")
			content := "@echo off\r\necho Standard output\r\necho Standard error 1>&2\r\nexit /b 0\r\n"
			if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
				t.Fatalf("Failed to create test batch file: %v", err)
			}
			cmd = "cmd"
			args = []string{"/c", scriptPath}
		} else {
			scriptPath = filepath.Join(tempDir, "stderr.sh")
			content := "#!/bin/sh\necho \"Standard output\"\necho \"Standard error\" >&2\nexit 0\n"
			if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
				t.Fatalf("Failed to create test script: %v", err)
			}
			if err := os.Chmod(scriptPath, 0755); err != nil {
				t.Fatalf("Failed to make script executable: %v", err)
			}
			cmd = scriptPath
			args = []string{}
		}

		// Capture stdout and stderr
		rOut, wOut, _ := os.Pipe()
		rErr, wErr, _ := os.Pipe()
		os.Stdout = wOut
		os.Stderr = wErr

		outC := make(chan string)
		errC := make(chan string)

		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, rOut)
			outC <- buf.String()
		}()

		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, rErr)
			errC <- buf.String()
		}()

		// Run the command
		output, err := execCommand("", append([]string{cmd}, args...)...)

		// Close write ends of pipes to get output
		wOut.Close()
		wErr.Close()
		stdout := <-outC
		stderr := <-errC
		os.Stdout = originalStdout
		os.Stderr = originalStderr

		// Verify
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Check combined output has both stdout and stderr
		if !strings.Contains(string(output), "Standard output") {
			t.Errorf("Expected output to contain 'Standard output', got: '%s'", string(output))
		}
		if !strings.Contains(string(output), "Standard error") {
			t.Errorf("Expected output to contain 'Standard error', got: '%s'", string(output))
		}

		// Check that stdout and stderr were captured correctly
		if !strings.Contains(stdout, "Standard output") {
			t.Errorf("Expected stdout to contain 'Standard output', got: '%s'", stdout)
		}
		if !strings.Contains(stderr, "Standard error") {
			t.Errorf("Expected stderr to contain 'Standard error', got: '%s'", stderr)
		}
	})

	t.Run("LongRunningCommand", func(t *testing.T) {
		// Create a platform-specific command that outputs with delays
		var cmd string
		var args []string
		var scriptPath string

		if runtime.GOOS == "windows" {
			scriptPath = filepath.Join(tempDir, "slow.bat")
			// Windows batch file using timeout instead of sleep
			content := "@echo off\r\necho Starting\r\ntimeout /t 1 /nobreak >nul\r\necho Step 1\r\ntimeout /t 1 /nobreak >nul\r\necho Step 2\r\ntimeout /t 1 /nobreak >nul\r\necho Finished\r\nexit /b 0\r\n"
			if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
				t.Fatalf("Failed to create test batch file: %v", err)
			}
			cmd = "cmd"
			args = []string{"/c", scriptPath}
		} else {
			scriptPath = filepath.Join(tempDir, "slow.sh")
			content := "#!/bin/sh\necho \"Starting\"\nsleep 0.5\necho \"Step 1\"\nsleep 0.5\necho \"Step 2\"\nsleep 0.5\necho \"Finished\"\nexit 0\n"
			if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
				t.Fatalf("Failed to create test script: %v", err)
			}
			if err := os.Chmod(scriptPath, 0755); err != nil {
				t.Fatalf("Failed to make script executable: %v", err)
			}
			cmd = scriptPath
			args = []string{}
		}

		// Capture stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r)
			outC <- buf.String()
		}()

		// Measure execution time to ensure we're not waiting for all output at once
		startTime := time.Now()

		// Run the command
		output, err := execCommand("", append([]string{cmd}, args...)...)
		execTime := time.Since(startTime)

		// Close write end of pipe to get output
		w.Close()
		stdout := <-outC
		os.Stdout = originalStdout

		// Verify
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Check that all output is present
		expectedLines := []string{"Starting", "Step 1", "Step 2", "Finished"}
		for _, line := range expectedLines {
			if !strings.Contains(string(output), line) {
				t.Errorf("Expected output to contain '%s', got: '%s'", line, string(output))
			}
			if !strings.Contains(stdout, line) {
				t.Errorf("Expected stdout to contain '%s', got: '%s'", line, stdout)
			}
		}

		// Verify that the execution time is reasonable
		if execTime < 50*time.Millisecond {
			t.Errorf("Execution time too short (%v), suggests output was not streamed in real-time", execTime)
		}
	})

	t.Run("WorkingDirectory", func(t *testing.T) {
		// Test that the working directory is respected
		var cmd string
		var args []string

		if runtime.GOOS == "windows" {
			cmd = "cmd"
			args = []string{"/c", "cd"}
		} else {
			cmd = "pwd"
			args = []string{}
		}

		output, err := execCommand(tempDir, append([]string{cmd}, args...)...)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// On Windows, normalize paths for comparison (remove extra whitespace and convert case)
		normalizedOutput := strings.TrimSpace(string(output))
		normalizedTempDir := strings.TrimSpace(tempDir)

		if runtime.GOOS == "windows" {
			normalizedOutput = strings.ToLower(normalizedOutput)
			normalizedTempDir = strings.ToLower(normalizedTempDir)
		}

		// Check that the output contains the temp directory path
		if !strings.Contains(normalizedOutput, normalizedTempDir) {
			t.Errorf("Expected output to contain temp directory '%s', got: '%s'", normalizedTempDir, normalizedOutput)
		}
	})
}
