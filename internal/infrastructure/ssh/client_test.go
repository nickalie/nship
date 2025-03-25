package ssh

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/assert"
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
					assert.Contains(t, cmd, "echo hello", "Command should contain 'echo hello'")
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
	assert.NoError(t, err, "ExecuteStep should not return an error")

	client.Close()
	assert.True(t, sshClient.closed, "SSH client was not closed")
	assert.True(t, sftpClient.closed, "SFTP client was not closed")
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
	assert.Error(t, err, "executeCommand should return an error when the command fails")

	commandErr, ok := err.(*job.CommandError)
	assert.True(t, ok, "Error should be of type *job.CommandError")
	assert.NotEmpty(t, commandErr.Command, "CommandError.Command should not be empty")
}

func TestNewClientFactory(t *testing.T) {
	factory := NewClientFactory()
	assert.NotNil(t, factory, "NewClientFactory should not return nil")
	assert.NotNil(t, factory.sshDialer, "ClientFactory.sshDialer should not be nil")
	assert.NotNil(t, factory.sftpConnector, "ClientFactory.sftpConnector should not be nil")
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
			if tt.expectMethods {
				assert.NotEmpty(t, methods, "Expected auth methods but got none")
			} else {
				assert.Empty(t, methods, "Expected no auth methods")
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
			if tt.expectError {
				assert.Error(t, err, "runShellCommand should return an error")
			} else {
				assert.NoError(t, err, "runShellCommand should not return an error")
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
			assert.Equal(t, tt.expected, result, "escapeCommand returned unexpected result")
		})
	}
}
