package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

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
