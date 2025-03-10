package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

func TestLoadYAMLConfig(t *testing.T) {
	// Create temporary YAML config file
	tmpDir, err := os.MkdirTemp("", "config-test")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	// Create a temporary private key file
	privateKeyPath := filepath.Join(tmpDir, "key.pem")
	err = os.WriteFile(privateKeyPath, []byte("dummy private key"), 0600)
	assert.NoError(t, err, "Failed to create private key file")

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

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	config, err := loader.Load(configPath)
	assert.NoError(t, err, "Failed to load config")

	// Verify targets
	assert.Len(t, config.Targets, 2, "Expected 2 targets")

	// First target
	assert.Equal(t, "web-server", config.Targets[0].Name, "Incorrect first target name")
	assert.Equal(t, "web.example.com", config.Targets[0].Host, "Incorrect first target host")
	assert.Equal(t, 2222, config.Targets[0].Port, "Incorrect first target port")
	assert.Equal(t, privateKeyPath, config.Targets[0].PrivateKey, "Incorrect first target private key")

	// Second target
	assert.Equal(t, "db.example.com", config.Targets[1].Host, "Incorrect second target host")
	assert.Equal(t, "password123", config.Targets[1].Password, "Incorrect second target password")

	// Verify jobs
	assert.Len(t, config.Jobs, 2, "Expected 2 jobs")

	// Setup job
	assert.Equal(t, "setup", config.Jobs[0].Name, "Incorrect first job name")
	assert.Len(t, config.Jobs[0].Steps, 2, "Expected setup job to have 2 steps")
	assert.Equal(t, "mkdir -p /var/www", config.Jobs[0].Steps[0].Run, "Incorrect first step command")

	// Deploy job
	assert.Equal(t, "deploy", config.Jobs[1].Name, "Incorrect second job name")
	assert.Len(t, config.Jobs[1].Steps, 2, "Expected deploy job to have 2 steps")

	// Copy step
	copyStep := config.Jobs[1].Steps[0].Copy
	assert.NotNil(t, copyStep, "Expected first step to be a copy step, but Copy is nil")
	assert.Equal(t, "./config/nginx.conf", copyStep.Src, "Incorrect copy step source")

	// Docker step
	dockerStep := config.Jobs[1].Steps[1].Docker
	assert.NotNil(t, dockerStep, "Expected second step to be a docker step, but Docker is nil")
	assert.Equal(t, "nginx:latest", dockerStep.Image, "Incorrect docker image")
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
	assert.Contains(t, result, "test.example.com", "Expected content to contain substituted host")
	assert.Contains(t, result, "testuser", "Expected content to contain substituted user")
	assert.Contains(t, result, "1234", "Expected content to contain substituted port")
	assert.NotContains(t, result, "${TEST_HOST}", "Environment variable ${TEST_HOST} was not replaced")
	assert.NotContains(t, result, "${TEST_USER}", "Environment variable ${TEST_USER} was not replaced")
	assert.NotContains(t, result, "${TEST_PORT}", "Environment variable ${TEST_PORT} was not replaced")

	// Test with non-existent environment variable
	content = "host: ${NONEXISTENT_VAR}"
	result = replaceEnvVariables(content)

	// Non-existent variables should be replaced with empty string
	assert.NotContains(t, result, "${NONEXISTENT_VAR}", "Non-existent environment variable was not replaced")
	assert.Contains(t, result, "host: ", "Expected 'host: ' after replacement")
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
				assert.Error(t, err, "Expected error but got nil")
			} else {
				assert.NoError(t, err, "Expected no error")

				// Verify config was loaded correctly
				assert.Equal(t, validConfig.Targets[0].Host, config.Targets[0].Host, "Incorrect host")
				assert.Equal(t, validConfig.Jobs[0].Name, config.Jobs[0].Name, "Incorrect job name")
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
			assert.NoError(t, err, "Failed to write test TS file")

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
				assert.Error(t, err, "Expected error but got nil")
			} else {
				assert.NoError(t, err, "Expected no error")

				// Check config values
				assert.Equal(t, validConfig.Targets[0].Host, config.Targets[0].Host, "Incorrect host")
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
	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, validConfig.Targets[0].Host, config.Targets[0].Host, "Incorrect host")
	assert.Equal(t, validConfig.Jobs[0].Name, config.Jobs[0].Name, "Incorrect job name")
}

func TestInvalidConfig(t *testing.T) {
	// Create temporary YAML config file with invalid configuration
	tmpDir, err := os.MkdirTemp("", "config-test-invalid")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid-config.yaml")
	invalidConfig := `
targets:
  - name: missing-required-fields
jobs:
  - name: empty-job
`

	err = os.WriteFile(configPath, []byte(invalidConfig), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	_, err = loader.Load(configPath)

	// Expect validation error
	assert.Error(t, err, "Expected validation error but got nil")
	assert.Contains(t, err.Error(), "validation failed", "Error message should mention validation failure")
}

// TestAutoNaming test needs to be updated
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
	assert.NoError(t, err, "Validation failed")

	// Check target naming
	assert.Equal(t, "unnamed.example.com", config.Targets[0].Name, "Target name should default to host")

	// Check job naming
	assert.Equal(t, "job-1", config.Jobs[0].Name, "Job name should be 'job-1'")
}

// TestUnsupportedExtension needs to be updated
func TestUnsupportedExtension(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load("config.unsupported")

	assert.Error(t, err, "Expected error for unsupported extension")
	assert.Contains(t, err.Error(), "unsupported config file extension",
		"Error should mention unsupported extension")
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
		assert.NoError(t, err, "Command should execute without errors")
		assert.Contains(t, string(output), expected, "Output should contain expected text")
		assert.Contains(t, stdout, expected, "Stdout should contain expected text")
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
		assert.Error(t, err, "Command should produce an error")

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
		assert.NoError(t, err, "Command should execute without errors")

		// Check combined output has both stdout and stderr
		assert.Contains(t, string(output), "Standard output", "Output should contain stdout message")
		assert.Contains(t, string(output), "Standard error", "Output should contain stderr message")

		// Check that stdout and stderr were captured correctly
		assert.Contains(t, stdout, "Standard output", "Stdout should contain expected text")
		assert.Contains(t, stderr, "Standard error", "Stderr should contain expected text")
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
		assert.NoError(t, err, "Command should execute without errors")

		// Check that all output is present
		expectedLines := []string{"Starting", "Step 1", "Step 2", "Finished"}
		for _, line := range expectedLines {
			assert.Contains(t, string(output), line, "Output should contain expected line")
			assert.Contains(t, stdout, line, "Stdout should contain expected line")
		}

		// Verify that the execution time is reasonable
		assert.GreaterOrEqual(t, execTime, 30*time.Millisecond,
			"Execution time should be reasonable to ensure streaming output")
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
		assert.Contains(t, normalizedOutput, normalizedTempDir,
			"Output should contain the working directory path")
	})
}

func TestLoadJSONConfig(t *testing.T) {
	// Create temporary JSON config file
	tmpDir, err := os.MkdirTemp("", "config-test-json")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	// Create a temporary private key file
	privateKeyPath := filepath.Join(tmpDir, "key.pem")
	err = os.WriteFile(privateKeyPath, []byte("dummy private key"), 0600)
	assert.NoError(t, err, "Failed to create private key file")

	// Replace backslashes with forward slashes for JSON compatibility
	jsonSafePath := strings.ReplaceAll(privateKeyPath, "\\", "/")

	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
      "targets": [
       {
        "name": "web-server",
        "host": "web.example.com",
        "user": "admin",
        "port": 2222,
        "private_key": "` + jsonSafePath + `"
       },
       {
        "host": "db.example.com",
        "user": "admin",
        "password": "password123"
       }
      ],
      "jobs": [
       {
        "name": "setup",
        "steps": [
         {"run": "mkdir -p /var/www"},
         {"run": "chown www-data:www-data /var/www"}
        ]
       },
       {
        "name": "deploy",
        "steps": [
         {"copy": {"src": "./config/nginx.conf", "dst": "/etc/nginx/nginx.conf"}},
         {"docker": {"image": "nginx:latest", "name": "web"}}
        ]
       }
      ]
     }`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	config, err := loader.Load(configPath)
	assert.NoError(t, err, "Failed to load config")

	// Verify targets
	assert.Len(t, config.Targets, 2, "Expected 2 targets")

	// First target
	assert.Equal(t, "web-server", config.Targets[0].Name, "Incorrect first target name")
	assert.Equal(t, "web.example.com", config.Targets[0].Host, "Incorrect first target host")
	assert.Equal(t, 2222, config.Targets[0].Port, "Incorrect first target port")
	assert.Equal(t, jsonSafePath, config.Targets[0].PrivateKey, "Incorrect first target private key")

	// Second target
	assert.Equal(t, "db.example.com", config.Targets[1].Host, "Incorrect second target host")
	assert.Equal(t, "password123", config.Targets[1].Password, "Incorrect second target password")

	// Verify jobs
	assert.Len(t, config.Jobs, 2, "Expected 2 jobs")

	// Setup job
	assert.Equal(t, "setup", config.Jobs[0].Name, "Incorrect first job name")
	assert.Len(t, config.Jobs[0].Steps, 2, "Expected setup job to have 2 steps")
	assert.Equal(t, "mkdir -p /var/www", config.Jobs[0].Steps[0].Run, "Incorrect first step command")

	// Deploy job
	assert.Equal(t, "deploy", config.Jobs[1].Name, "Incorrect second job name")
}

func TestReplaceEnvVariablesInJSON(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("TEST_HOST", "test.example.com")
	os.Setenv("TEST_USER", "testuser")
	os.Setenv("TEST_PORT", "1234")

	// Create temporary JSON config file with environment variables
	tmpDir, err := os.MkdirTemp("", "config-test-json-env")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
  "targets": [
   {
    "host": "${TEST_HOST}",
    "user": "${TEST_USER}",
    "port": ${TEST_PORT},
    "password": "secret"
   }
  ],
  "jobs": [
   {
    "name": "test-job",
    "steps": [
     {
      "run": "echo 'Hello World'"
     }
    ]
   }
  ]
 }`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	config, err := loader.Load(configPath)
	assert.NoError(t, err, "Failed to load config")

	// Verify environment variable substitution
	assert.Equal(t, "test.example.com", config.Targets[0].Host, "Environment variable not substituted in host")
	assert.Equal(t, "testuser", config.Targets[0].User, "Environment variable not substituted in user")
	assert.Equal(t, 1234, config.Targets[0].Port, "Environment variable not substituted in port")
}

func TestInvalidJSONConfig(t *testing.T) {
	// Create temporary invalid JSON config file
	tmpDir, err := os.MkdirTemp("", "config-test-invalid-json")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid.json")
	invalidConfig := `{
		"targets": [
			{
				"name": "missing-required-fields"
			}
		],
		"jobs": [
			{
				"name": "invalid-json
				"steps": [{"run": "echo test"}]
			}
		]
	}`

	err = os.WriteFile(configPath, []byte(invalidConfig), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	_, err = loader.Load(configPath)

	// Expect parsing error
	assert.Error(t, err, "Expected parsing error but got nil")
	assert.Contains(t, err.Error(), "failed to parse JSON", "Error message does not mention JSON parsing failure")
}

func TestLoadTOMLConfig(t *testing.T) {
	// Create temporary TOML config file
	tmpDir, err := os.MkdirTemp("", "config-test-toml")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	// Create a temporary private key file
	privateKeyPath := filepath.Join(tmpDir, "key.pem")
	err = os.WriteFile(privateKeyPath, []byte("dummy private key"), 0600)
	assert.NoError(t, err, "Failed to create private key file")

	// Replace backslashes with forward slashes for TOML compatibility
	tomlSafePath := strings.ReplaceAll(privateKeyPath, "\\", "/")

	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
# TOML Configuration

[[targets]]
name = "web-server"
host = "web.example.com"
user = "admin"
port = 2222
private_key = "` + tomlSafePath + `"

[[targets]]
host = "db.example.com"
user = "admin"
password = "password123"

[[jobs]]
name = "setup"
steps = [
  { run = "mkdir -p /var/www" },
  { run = "chown www-data:www-data /var/www" }
]

[[jobs]]
name = "deploy"
steps = [
  { copy = { src = "./config/nginx.conf", dst = "/etc/nginx/nginx.conf" } },
  { docker = { image = "nginx:latest", name = "web", ports = ["80:80"] } }
]
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	config, err := loader.Load(configPath)
	assert.NoError(t, err, "Failed to load config")

	// Verify targets
	assert.Len(t, config.Targets, 2, "Expected 2 targets")

	// First target
	assert.Equal(t, "web-server", config.Targets[0].Name, "Incorrect first target name")
	assert.Equal(t, "web.example.com", config.Targets[0].Host, "Incorrect first target host")
	assert.Equal(t, 2222, config.Targets[0].Port, "Incorrect first target port")
	assert.Equal(t, tomlSafePath, config.Targets[0].PrivateKey, "Incorrect first target private key")

	// Second target
	assert.Equal(t, "db.example.com", config.Targets[1].Host, "Incorrect second target host")
	assert.Equal(t, "password123", config.Targets[1].Password, "Incorrect second target password")

	// Verify jobs
	assert.Len(t, config.Jobs, 2, "Expected 2 jobs")

	// Setup job
	assert.Equal(t, "setup", config.Jobs[0].Name, "Incorrect first job name")
	assert.Len(t, config.Jobs[0].Steps, 2, "Expected setup job to have 2 steps")
	assert.Equal(t, "mkdir -p /var/www", config.Jobs[0].Steps[0].Run, "Incorrect first step command")

	// Deploy job
	assert.Equal(t, "deploy", config.Jobs[1].Name, "Incorrect second job name")
}

func TestReplaceEnvVariablesInTOML(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("TEST_HOST", "test.example.com")
	os.Setenv("TEST_USER", "testuser")
	os.Setenv("TEST_PORT", "1234")

	// Create temporary TOML config file with environment variables
	tmpDir, err := os.MkdirTemp("", "config-test-toml-env")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
[[targets]]
host = "${TEST_HOST}"
user = "${TEST_USER}"
port = ${TEST_PORT}
password = "secret"

[[jobs]]
name = "test-job"
steps = [
  { run = "echo 'Hello World'" }
]
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write config file")

	// Load the config
	loader := NewLoader()
	config, err := loader.Load(configPath)
	assert.NoError(t, err, "Failed to load config")

	// Verify environment variable substitution
	assert.Equal(t, "test.example.com", config.Targets[0].Host, "Environment variable not substituted in host")
	assert.Equal(t, "testuser", config.Targets[0].User, "Environment variable not substituted in user")
	assert.Equal(t, 1234, config.Targets[0].Port, "Environment variable not substituted in port")
}
