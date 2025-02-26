//go:build integration
// +build integration

package integration

import (
	"os"
	"strconv"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/nickalie/nship/internal/infrastructure/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test requires SSH access to a real server to run
// Set environment variables to configure the test:
// TEST_SSH_HOST - hostname to connect to
// TEST_SSH_PORT - SSH port (defaults to 22)
// TEST_SSH_USER - SSH username
// TEST_SSH_PASSWORD - SSH password or empty if using key authentication
// TEST_SSH_KEY - Path to private key file or empty if using password authentication

func TestSSHConnection(t *testing.T) {
	// Skip this test if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	// Get test configuration from environment
	host := os.Getenv("TEST_SSH_HOST")
	portStr := os.Getenv("TEST_SSH_PORT")
	user := os.Getenv("TEST_SSH_USER")
	password := os.Getenv("TEST_SSH_PASSWORD")
	privateKey := os.Getenv("TEST_SSH_KEY")

	// Validate required parameters
	if host == "" || user == "" {
		t.Fatal("TEST_SSH_HOST and TEST_SSH_USER environment variables must be set")
	}
	if password == "" && privateKey == "" {
		t.Fatal("Either TEST_SSH_PASSWORD or TEST_SSH_KEY must be set")
	}

	// Set default port if not specified
	port := 22
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		require.NoError(t, err, "Invalid TEST_SSH_PORT")
	}

	// Create target
	target := &target.Target{
		Host:       host,
		Port:       port,
		User:       user,
		Password:   password,
		PrivateKey: privateKey,
	}

	// Create client factory
	factory := ssh.NewClientFactory()

	// Test connection
	client, err := factory.NewClient(target)
	require.NoError(t, err, "Failed to connect to SSH server")
	defer client.Close()

	// Test command execution
	step := &job.Step{Run: "echo 'SSH connection test successful'"}
	err = client.ExecuteStep(step, 1, 1)
	assert.NoError(t, err, "Failed to execute command")
}
