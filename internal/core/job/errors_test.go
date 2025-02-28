package job

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnectionError(t *testing.T) {
	cause := errors.New("connection refused")
	target := "web-server"

	err := &ConnectionError{
		Target: target,
		Cause:  cause,
	}

	expected := "connection to target web-server failed: connection refused"
	assert.Equal(t, expected, err.Error(), "ConnectionError message doesn't match expected format")

	// Verify that the error message contains both the target and cause
	assert.Contains(t, err.Error(), target, "ConnectionError should contain the target name")
	assert.Contains(t, err.Error(), cause.Error(), "ConnectionError should contain the cause")
}

func TestStepError(t *testing.T) {
	cause := errors.New("command not found")
	err := &StepError{
		JobName:  "deploy",
		Target:   "api-server",
		StepNum:  2,
		TotalNum: 5,
		Cause:    cause,
	}

	expected := "job 'deploy' step 2/5 on 'api-server' failed: command not found"
	assert.Equal(t, expected, err.Error(), "StepError message doesn't match expected format")
}

func TestCommandError(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		output   string
		cause    error
		expected string
	}{
		{
			name:     "with output",
			command:  "ls -la /nonexistent",
			output:   "ls: cannot access '/nonexistent': No such file or directory",
			cause:    errors.New("exit status 2"),
			expected: "command 'ls -la /nonexistent' failed: exit status 2\nOutput: ls: cannot access '/nonexistent': No such file or directory",
		},
		{
			name:     "without output",
			command:  "ssh unreachable",
			output:   "",
			cause:    errors.New("connection timeout"),
			expected: "command 'ssh unreachable' failed: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CommandError{
				Command: tt.command,
				Output:  tt.output,
				Cause:   tt.cause,
			}

			assert.Equal(t, tt.expected, err.Error(), "CommandError message doesn't match expected format")
		})
	}
}

func TestCopyError(t *testing.T) {
	cause := errors.New("permission denied")
	err := &CopyError{
		Source:      "./config.yml",
		Destination: "/etc/app/config.yml",
		Cause:       cause,
	}

	expected := "copying './config.yml' to '/etc/app/config.yml' failed: permission denied"
	assert.Equal(t, expected, err.Error(), "CopyError message doesn't match expected format")
}

func TestDockerError(t *testing.T) {
	cause := errors.New("image not found")
	err := &DockerError{
		ContainerName: "web-app",
		Operation:     "create",
		Cause:         cause,
	}

	expected := "Docker operation 'create' on container 'web-app' failed: image not found"
	assert.Equal(t, expected, err.Error(), "DockerError message doesn't match expected format")
}
