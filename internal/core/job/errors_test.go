package job

import (
	"errors"
	"strings"
	"testing"
)

func TestConnectionError(t *testing.T) {
	cause := errors.New("connection refused")
	target := "web-server"

	err := &ConnectionError{
		Target: target,
		Cause:  cause,
	}

	expected := "connection to target web-server failed: connection refused"
	if err.Error() != expected {
		t.Errorf("ConnectionError.Error() = %q, want %q", err.Error(), expected)
	}

	// Verify that the error message contains both the target and cause
	if !strings.Contains(err.Error(), target) {
		t.Errorf("ConnectionError.Error() does not contain target %q", target)
	}

	if !strings.Contains(err.Error(), cause.Error()) {
		t.Errorf("ConnectionError.Error() does not contain cause %q", cause.Error())
	}
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
	if err.Error() != expected {
		t.Errorf("StepError.Error() = %q, want %q", err.Error(), expected)
	}
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

			if err.Error() != tt.expected {
				t.Errorf("CommandError.Error() = %q, want %q", err.Error(), tt.expected)
			}
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
	if err.Error() != expected {
		t.Errorf("CopyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestDockerError(t *testing.T) {
	cause := errors.New("image not found")
	err := &DockerError{
		ContainerName: "web-app",
		Operation:     "create",
		Cause:         cause,
	}

	expected := "Docker operation 'create' on container 'web-app' failed: image not found"
	if err.Error() != expected {
		t.Errorf("DockerError.Error() = %q, want %q", err.Error(), expected)
	}
}
