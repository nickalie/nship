package ssh

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

// MockSSHSession for testing
type MockSSHSession struct {
	StartFunc      func(string) error
	WaitFunc       func() error
	StdoutPipeFunc func() (io.Reader, error)
	StderrPipeFunc func() (io.Reader, error)
	CloseFunc      func() error
}

func (m *MockSSHSession) Start(cmd string) error {
	if m.StartFunc != nil {
		return m.StartFunc(cmd)
	}
	return nil
}

func (m *MockSSHSession) Wait() error {
	if m.WaitFunc != nil {
		return m.WaitFunc()
	}
	return nil
}

func (m *MockSSHSession) StdoutPipe() (io.Reader, error) {
	if m.StdoutPipeFunc != nil {
		return m.StdoutPipeFunc()
	}
	return &MockReader{}, nil
}

func (m *MockSSHSession) StderrPipe() (io.Reader, error) {
	if m.StderrPipeFunc != nil {
		return m.StderrPipeFunc()
	}
	return &MockReader{}, nil
}

func (m *MockSSHSession) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockSSHClient for testing
type MockSSHClient struct {
	NewSessionFunc func() (SSHSession, error)
	CloseFunc      func() error
	closed         bool
}

func (m *MockSSHClient) NewSession() (SSHSession, error) {
	if m.NewSessionFunc != nil {
		return m.NewSessionFunc()
	}
	return &MockSSHSession{}, nil
}

func (m *MockSSHClient) Close() error {
	m.closed = true
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockSFTPClient for testing
type MockSFTPClient struct {
	CloseFunc func() error
	closed    bool
}

func (m *MockSFTPClient) Close() error {
	m.closed = true
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockSFTPClient) Create(path string) (io.WriteCloser, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSFTPClient) MkdirAll(path string) error {
	return errors.New("not implemented")
}

func (m *MockSFTPClient) Chmod(path string, mode os.FileMode) error {
	return errors.New("not implemented")
}

func (m *MockSFTPClient) Stat(path string) (os.FileInfo, error) {
	return nil, errors.New("not implemented")
}

// MockReader implements io.Reader for testing
type MockReader struct {
	ReadFunc func(p []byte) (n int, err error)
}

func (m *MockReader) Read(p []byte) (n int, err error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(p)
	}
	return 0, io.EOF
}

func TestExecuteStep_RunStep(t *testing.T) {
	sshClient := &MockSSHClient{
		NewSessionFunc: func() (SSHSession, error) {
			return &MockSSHSession{
				StartFunc: func(cmd string) error {
					if !strings.Contains(cmd, "echo hello") {
						return errors.New("unexpected command")
					}
					return nil
				},
				StdoutPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
				StderrPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
			}, nil
		},
	}

	sftpClient := &MockSFTPClient{}

	client := &SSHClient{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		target:     &target.Target{Name: "test-target"},
	}

	step := &job.Step{
		Run: "echo hello",
	}

	err := client.ExecuteStep(step, 1, 3)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	client.Close()
	if !sshClient.closed {
		t.Error("SSH client was not closed")
	}
	if !sftpClient.closed {
		t.Error("SFTP client was not closed")
	}
}

func TestExecuteCommand_Error(t *testing.T) {
	sshClient := &MockSSHClient{
		NewSessionFunc: func() (SSHSession, error) {
			return &MockSSHSession{
				StartFunc: func(cmd string) error {
					return nil
				},
				WaitFunc: func() error {
					return errors.New("command failed")
				},
				StdoutPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
				StderrPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
			}, nil
		},
	}

	client := &SSHClient{
		sshClient: sshClient,
		target:    &target.Target{Name: "test-target"},
	}

	step := &job.Step{
		Run: "failing-command",
	}

	err := client.executeCommand(step, 1, 3)
	if err == nil {
		t.Error("Expected error but got nil")
	}

	if commandErr, ok := err.(*job.CommandError); ok {
		if commandErr.Command == "" {
			t.Error("Expected CommandError with command, got empty command")
		}
	} else {
		t.Errorf("Expected CommandError, got: %T", err)
	}
}

func TestNewClientFactory(t *testing.T) {
	factory := NewClientFactory()
	if factory == nil {
		t.Fatal("NewClientFactory returned nil")
	}

	if factory.fileSystem == nil {
		t.Error("ClientFactory has nil fileSystem")
	}
}

func TestGetAuthMethods(t *testing.T) {
	// Cannot fully test without mocking ssh.AuthMethod, but can test basic logic
	tests := []struct {
		name          string
		target        *target.Target
		expectMethods bool
	}{
		{
			name: "with password",
			target: &target.Target{
				User:     "user",
				Password: "pass",
			},
			expectMethods: true,
		},
		{
			name: "with invalid private key path",
			target: &target.Target{
				User:       "user",
				PrivateKey: "/nonexistent/path/to/key",
			},
			expectMethods: false,
		},
		{
			name: "with neither password nor key",
			target: &target.Target{
				User: "user",
			},
			expectMethods: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methods := getAuthMethods(tt.target)
			if tt.expectMethods && len(methods) == 0 {
				t.Error("Expected auth methods but got none")
			}
		})
	}
}

func TestRunShellCommand(t *testing.T) {
	tests := []struct {
		name        string
		session     *MockSSHSession
		expectError bool
	}{
		{
			name: "successful command",
			session: &MockSSHSession{
				StartFunc: func(cmd string) error {
					return nil
				},
				WaitFunc: func() error {
					return nil
				},
				StdoutPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
				StderrPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
			},
			expectError: false,
		},
		{
			name: "start error",
			session: &MockSSHSession{
				StartFunc: func(cmd string) error {
					return errors.New("start failed")
				},
				StdoutPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
				StderrPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
			},
			expectError: true,
		},
		{
			name: "wait error",
			session: &MockSSHSession{
				StartFunc: func(cmd string) error {
					return nil
				},
				WaitFunc: func() error {
					return errors.New("wait failed")
				},
				StdoutPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
				StderrPipeFunc: func() (io.Reader, error) {
					return &MockReader{}, nil
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runShellCommand(tt.session, "sh", "echo test", io.Discard, io.Discard)
			if (err != nil) != tt.expectError {
				t.Errorf("runShellCommand() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestEscapeCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected string
	}{
		{
			name:     "simple command",
			cmd:      "echo hello",
			expected: "'echo hello'",
		},
		{
			name:     "command with single quotes",
			cmd:      "echo 'hello world'",
			expected: "'echo '\\''hello world'\\'''",
		},
		{
			name:     "command with backticks",
			cmd:      "echo `date`",
			expected: "'echo \\`date\\`'",
		},
		{
			name:     "complex command",
			cmd:      "grep -i 'error' `find /var/log -name '*.log'`",
			expected: "'grep -i '\\''error'\\'' \\`find /var/log -name '\\''*.log'\\''\\`'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeCommand(tt.cmd)
			if result != tt.expected {
				t.Errorf("escapeCommand(%q) = %q, want %q", tt.cmd, result, tt.expected)
			}
		})
	}
}
