package job

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nickalie/nship/config"
)

type MockSSHClient struct {
}

func (m *MockSSHClient) ExecuteStep(_ *config.Step, _, _ int) error {
	return nil
}

func (m *MockSSHClient) Close() {}

func NewMockSSHClient(target *config.Target) (Client, error) {
	if target.Host == "invalidhost" {
		return nil, errors.New("SSH connection failed")
	}
	return &MockSSHClient{}, nil
}

func TestNewSSHClient(t *testing.T) {
	tests := []struct {
		name    string
		target  config.Target
		wantErr bool
	}{
		{
			name: "valid SSH client",
			target: config.Target{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpass",
			},
			wantErr: false,
		},
		{
			name: "invalid SSH client",
			target: config.Target{
				Host: "invalidhost",
				User: "testuser",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMockSSHClient(&tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSSHClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildDockerCommands(t *testing.T) {
	tests := []struct {
		name   string
		docker *config.DockerStep
		want   []string
	}{
		{
			name: "simple docker command",
			docker: &config.DockerStep{
				Name:  "test-container",
				Image: "nginx",
			},
			want: []string{
				"docker rm -f test-container 2>/dev/null || true",
				"docker create --name test-container nginx",
				"docker start test-container",
			},
		},
		{
			name: "docker command with networks",
			docker: &config.DockerStep{
				Name:     "test-container",
				Image:    "nginx",
				Networks: []string{"net1", "net2"},
			},
			want: []string{
				"docker rm -f test-container 2>/dev/null || true",
				"docker network create net1 2>/dev/null || true",
				"docker network create net2 2>/dev/null || true",
				"docker create --name test-container --network net1 --network net2 nginx",
				"docker network connect net1 test-container",
				"docker network connect net2 test-container",
				"docker start test-container",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDockerCommands(tt.docker)
			if len(got) != len(tt.want) {
				t.Errorf("buildDockerCommands() got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildDockerCommands()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPipeOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name:     "single line",
			input:    "hello world\n",
			wantText: "hello world\n",
		},
		{
			name:     "multiple lines",
			input:    "line 1\nline 2\nline 3\n",
			wantText: "line 1\nline 2\nline 3\n",
		},
		{
			name:     "empty string",
			input:    "",
			wantText: "",
		},
		{
			name:     "no newline at end",
			input:    "test",
			wantText: "test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			var w bytes.Buffer

			pipeOutput(r, &w)

			if got := w.String(); got != tt.wantText {
				t.Errorf("pipeOutput() output = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestEscapeCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{
			name: "simple command",
			cmd:  "echo hello",
			want: "'echo hello'",
		},
		{
			name: "command with single quotes",
			cmd:  "echo 'hello world'",
			want: "'echo '\\''hello world'\\'''",
		},
		{
			name: "command with backticks",
			cmd:  "echo `hostname`",
			want: "'echo \\`hostname\\`'",
		},
		{
			name: "complex command",
			cmd:  "echo 'hello' && hostname `date` 'test'",
			want: "'echo '\\''hello'\\'' && hostname \\`date\\` '\\''test'\\'''",
		},
		{
			name: "empty command",
			cmd:  "",
			want: "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeCommand(tt.cmd); got != tt.want {
				t.Errorf("escapeCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockSession implements SSHSession interface for testing
type mockSession struct {
	startErr   error
	waitErr    error
	stdout     string
	stderr     string
	startedCmd string
	stdoutPipe io.Reader
	stderrPipe io.Reader
	pipeErr    error
}

func (m *mockSession) Start(cmd string) error {
	m.startedCmd = cmd
	return m.startErr
}

func (m *mockSession) Wait() error {
	return m.waitErr
}

func (m *mockSession) StdoutPipe() (io.Reader, error) {
	if m.stdoutPipe != nil {
		return m.stdoutPipe, m.pipeErr
	}
	return strings.NewReader(m.stdout), m.pipeErr
}

func (m *mockSession) StderrPipe() (io.Reader, error) {
	if m.stderrPipe != nil {
		return m.stderrPipe, m.pipeErr
	}
	return strings.NewReader(m.stderr), m.pipeErr
}

func (m *mockSession) Close() error {
	return nil
}

func TestRunShellCommand(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		cmd         string
		session     *mockSession
		wantStdout  string
		wantStderr  string
		wantStarted string
		wantErr     bool
		errString   string
	}{
		{
			name:  "successful command",
			shell: "sh",
			cmd:   "echo hello",
			session: &mockSession{
				stdout: "hello\n",
			},
			wantStdout:  "hello\n",
			wantStarted: "sh -c 'echo hello'",
		},
		{
			name:  "command with stderr",
			shell: "bash",
			cmd:   "echo 'error' >&2",
			session: &mockSession{
				stderr: "error\n",
			},
			wantStderr:  "error\n",
			wantStarted: "bash -c 'echo '\\''error'\\'' >&2'",
		},
		{
			name:  "stdout pipe error",
			shell: "sh",
			cmd:   "test",
			session: &mockSession{
				pipeErr: errors.New("pipe error"),
			},
			wantErr:   true,
			errString: "failed to get stdout pipe: pipe error",
		},
		{
			name:  "start error",
			shell: "sh",
			cmd:   "test",
			session: &mockSession{
				startErr: errors.New("start error"),
			},
			wantStarted: "sh -c 'test'",
			wantErr:     true,
			errString:   "failed to start command: start error",
		},
		{
			name:  "wait error",
			shell: "sh",
			cmd:   "test",
			session: &mockSession{
				waitErr: errors.New("wait error"),
			},
			wantStarted: "sh -c 'test'",
			wantErr:     true,
			errString:   "wait error",
		},
		{
			name:  "complex command",
			shell: "bash",
			cmd:   "echo `hostname` && echo 'test'",
			session: &mockSession{
				stdout: "localhost\ntest\n",
			},
			wantStdout:  "localhost\ntest\n",
			wantStarted: "bash -c 'echo \\`hostname\\` && echo '\\''test'\\'''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			err := runShellCommand(tt.session, tt.shell, tt.cmd, &stdout, &stderr)

			if (err != nil) != tt.wantErr {
				t.Errorf("runShellCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errString != "" && err.Error() != tt.errString {
				t.Errorf("runShellCommand() error = %v, wantErrString %v", err, tt.errString)
			}

			if tt.wantStdout != "" && stdout.String() != tt.wantStdout {
				t.Errorf("runShellCommand() stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}

			if tt.wantStderr != "" && stderr.String() != tt.wantStderr {
				t.Errorf("runShellCommand() stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}

			if tt.wantStarted != "" && tt.session.startedCmd != tt.wantStarted {
				t.Errorf("runShellCommand() startedCmd = %q, want %q", tt.session.startedCmd, tt.wantStarted)
			}
		})
	}
}

func TestGetShell(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		want  string
	}{
		{
			name:  "empty shell",
			shell: "",
			want:  "sh",
		},
		{
			name:  "bash shell",
			shell: "bash",
			want:  "bash",
		},
		{
			name:  "zsh shell",
			shell: "zsh",
			want:  "zsh",
		},
		{
			name:  "custom shell",
			shell: "/bin/fish",
			want:  "/bin/fish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getShell(tt.shell); got != tt.want {
				t.Errorf("getShell() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecuteDocker(t *testing.T) {
	tests := []struct {
		name        string
		step        *config.Step
		stepNum     int
		totalSteps  int
		mockSession *mockSession
		wantCmd     string
		wantErr     bool
		errString   string
	}{
		{
			name: "simple docker container",
			step: &config.Step{
				Docker: &config.DockerStep{
					Name:  "test-container",
					Image: "nginx",
				},
			},
			stepNum:    1,
			totalSteps: 3,
			mockSession: &mockSession{
				stdout: "container started\n",
			},
			wantCmd: "sh -c 'docker rm -f test-container 2>/dev/null || true\ndocker create --name test-container nginx\ndocker start test-container'",
		},
		{
			name: "docker container with networks",
			step: &config.Step{
				Docker: &config.DockerStep{
					Name:     "test-container",
					Image:    "nginx",
					Networks: []string{"net1", "net2"},
				},
			},
			stepNum:    2,
			totalSteps: 3,
			mockSession: &mockSession{
				stdout: "container started\n",
			},
			wantCmd: "sh -c 'docker rm -f test-container 2>/dev/null || true\ndocker network create net1 2>/dev/null || true\ndocker network create net2 2>/dev/null || true\ndocker create --name test-container --network net1 --network net2 nginx\ndocker network connect net1 test-container\ndocker network connect net2 test-container\ndocker start test-container'",
		},
		{
			name: "docker container with custom shell",
			step: &config.Step{
				Shell: "bash",
				Docker: &config.DockerStep{
					Name:  "test-container",
					Image: "nginx",
				},
			},
			stepNum:    3,
			totalSteps: 3,
			mockSession: &mockSession{
				stdout: "container started\n",
			},
			wantCmd: "bash -c 'docker rm -f test-container 2>/dev/null || true\ndocker create --name test-container nginx\ndocker start test-container'",
		},
		{
			name: "session creation error",
			step: &config.Step{
				Docker: &config.DockerStep{
					Name:  "test-container",
					Image: "nginx",
				},
			},
			stepNum:    1,
			totalSteps: 1,
			mockSession: &mockSession{
				startErr: errors.New("failed to create session"),
			},
			wantErr:   true,
			errString: "failed to start command: failed to create session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &SSHClient{
				sshClient: &mockSSHClient{
					session: tt.mockSession,
				},
			}

			err := client.executeDocker(tt.step, tt.stepNum, tt.totalSteps)

			if (err != nil) != tt.wantErr {
				t.Errorf("executeDocker() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errString != "" && err.Error() != tt.errString {
				t.Errorf("executeDocker() error = %v, wantErrString %v", err, tt.errString)
			}

			if tt.wantCmd != "" && tt.mockSession.startedCmd != tt.wantCmd {
				t.Errorf("executeDocker() command = %q, want %q", tt.mockSession.startedCmd, tt.wantCmd)
			}
		})
	}
}

type mockSSHClient struct {
	session *mockSession
}

func (m *mockSSHClient) NewSession() (SSHSession, error) {
	return m.session, nil
}

func (m *mockSSHClient) Close() error {
	return nil
}

func TestExecuteCopy(t *testing.T) {
	tests := []struct {
		name       string
		copyStep   *config.CopyStep
		stepNum    int
		totalSteps int
		mockCopier *mockCopier
		wantErr    bool
		errString  string
	}{
		{
			name: "successful copy",
			copyStep: &config.CopyStep{
				Src: "source/path",
				Dst: "dest/path",
			},
			stepNum:    1,
			totalSteps: 3,
			mockCopier: &mockCopier{},
			wantErr:    false,
		},
		{
			name: "copy with exclude patterns",
			copyStep: &config.CopyStep{
				Src:     "source/path",
				Dst:     "dest/path",
				Exclude: []string{"*.tmp", "*.log"},
			},
			stepNum:    2,
			totalSteps: 3,
			mockCopier: &mockCopier{},
			wantErr:    false,
		},
		{
			name: "copy error",
			copyStep: &config.CopyStep{
				Src: "invalid/path",
				Dst: "dest/path",
			},
			stepNum:    3,
			totalSteps: 3,
			mockCopier: &mockCopier{
				err: errors.New("copy failed"),
			},
			wantErr:   true,
			errString: "copy failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &SSHClient{
				copier: tt.mockCopier,
			}

			err := client.executeCopy(tt.copyStep, tt.stepNum, tt.totalSteps)

			if (err != nil) != tt.wantErr {
				t.Errorf("executeCopy() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err.Error() != tt.errString {
				t.Errorf("executeCopy() error = %v, want %v", err, tt.errString)
			}

			if !tt.wantErr {
				if tt.mockCopier.calledSrc != tt.copyStep.Src {
					t.Errorf("executeCopy() called with src = %v, want %v", tt.mockCopier.calledSrc, tt.copyStep.Src)
				}
				if tt.mockCopier.calledDst != tt.copyStep.Dst {
					t.Errorf("executeCopy() called with dst = %v, want %v", tt.mockCopier.calledDst, tt.copyStep.Dst)
				}
				if !reflect.DeepEqual(tt.mockCopier.calledExclude, tt.copyStep.Exclude) {
					t.Errorf("executeCopy() called with exclude = %v, want %v", tt.mockCopier.calledExclude, tt.copyStep.Exclude)
				}
			}
		})
	}
}

// mockCopier implements file.Copier interface for testing
type mockCopier struct {
	calledSrc     string
	calledDst     string
	calledExclude []string
	err           error
}

func (m *mockCopier) CopyPath(src, dst string, exclude []string) error {
	m.calledSrc = src
	m.calledDst = dst
	m.calledExclude = exclude
	return m.err
}

func TestExecuteCommand(t *testing.T) {
	tests := []struct {
		name        string
		step        *config.Step
		stepNum     int
		totalSteps  int
		mockSession *mockSession
		wantCmd     string
		wantErr     bool
		errString   string
	}{
		{
			name: "successful command execution",
			step: &config.Step{
				Run: "echo 'hello world'",
			},
			stepNum:    1,
			totalSteps: 3,
			mockSession: &mockSession{
				stdout: "hello world\n",
			},
			wantCmd: "sh -c 'echo '\\''hello world'\\'''",
		},
		{
			name: "command with custom shell",
			step: &config.Step{
				Shell: "bash",
				Run:   "echo $SHELL",
			},
			stepNum:    2,
			totalSteps: 3,
			mockSession: &mockSession{
				stdout: "/bin/bash\n",
			},
			wantCmd: "bash -c 'echo $SHELL'",
		},
		{
			name: "command with backticks",
			step: &config.Step{
				Run: "echo `hostname`",
			},
			stepNum:    3,
			totalSteps: 3,
			mockSession: &mockSession{
				stdout: "testhost\n",
			},
			wantCmd: "sh -c 'echo \\`hostname\\`'",
		},
		{
			name: "session creation error",
			step: &config.Step{
				Run: "echo test",
			},
			stepNum:    1,
			totalSteps: 1,
			mockSession: &mockSession{
				startErr: errors.New("failed to start session"),
			},
			wantCmd:   "sh -c 'echo test'",
			wantErr:   true,
			errString: "failed to start command: failed to start session",
		},
		{
			name: "command execution error",
			step: &config.Step{
				Run: "invalid_command",
			},
			stepNum:    1,
			totalSteps: 2,
			mockSession: &mockSession{
				waitErr: errors.New("command not found"),
			},
			wantCmd:   "sh -c 'invalid_command'",
			wantErr:   true,
			errString: "command not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &SSHClient{
				sshClient: &mockSSHClient{
					session: tt.mockSession,
				},
			}

			err := client.executeCommand(tt.step, tt.stepNum, tt.totalSteps)

			if (err != nil) != tt.wantErr {
				t.Errorf("executeCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errString != "" && err.Error() != tt.errString {
				t.Errorf("executeCommand() error = %v, wantErrString %v", err, tt.errString)
			}

			if tt.wantCmd != "" && tt.mockSession.startedCmd != tt.wantCmd {
				t.Errorf("executeCommand() command = %q, want %q", tt.mockSession.startedCmd, tt.wantCmd)
			}
		})
	}
}

func TestLoadPrivateKey(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		keyContent  string
		setupFile   bool
		wantErr     bool
		errContains string
	}{
		{
			name: "valid_key",
			keyContent: `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqe+MpHOH+g94fmSJkfAAXm3sPa5/lsSkRGh4rNue13xVaW8F
lv5vSKwJz8zhvwyDbsWZ4LDbyIGNaluO8hFACoKgOtrb3jHFxLoZqNGojYy08JU/
AwU0opoeWlv+2BeFAFDKZCFhLB7BGeahDgI8naLRPnvbnUSvog4VYBAC4JfSuJub
XoNel9duXlKs9SrxrbcmlbigMGKIRyizsGkxBQ8fHj4aHMDHHreEL2yhDCi9DJaD
cD5WDZl7lw6OWDqTtIx0cjqPOWZEN6dttK6ssnF4R3R8PsfrBWttXi/K5qabKWmS
fgep3oZzhCcwHHYPSizyrUW2t11dcTTowjsHEQIDAQABAoIBAEgiSjYIYIDyDjjA
nnDXSqDK0kwAhWJPSFdNbWQauZtIyMy/dsT4be6QMH4Hvw+k1SmxMTdie3jqHUg3
Yz83uVJ8zl0NR+VShVuTj47MqFqljqzM/XlfvU8EUrwSbuP1X9yZbsAAN0pOJ7PB
5T2YD3jugxmd48QnQtJsXgBv63i/5sELFL+e109fokofzj5lueeEAvYIZHoEkF7Y
rWNayxzwcaejpYNYRZc8Ix7EJDPP0LfKab4j+//uaHweodMtyQNd18qCxcuOAv1i
Pbw6tjs9oFF/vId8GrwQYAZ2K/1qn3igB2lci9Vc1nO4XZNjf64LRUOeKypBWTSG
zjQwxdECgYEA7+A2IZQTLkd+hL0X/th/O2LZZZ3Dtk6J8SFV8+lUIEuu9SnqkwUp
Gnp/ii50LzS7BZFuZogcyGck65VqTXoBno6LiVDREHft1rWgDtTwfw/Ky2xVfKBR
ik4ba1DDaEkZnqjhz+ZncKHMsv1jDX4gInQXDtKY3JIusT1t6+Aj2KUCgYEAtVvO
sCR/A6vbHqM08/AAutFZnOnT7X61diT1MPwO+nBCBU9Vrf2UVaNKroI3ChEurQDU
J6/TDliEVN4HCCffV+pyhRwuEqJns7yQEgCjHCRjFDAarq6uKQ8Jl0K/yCrGAU2s
bl5sq5YcaX6mv9odBsNzgwmQ/UtW9yprHTYGfP0CgYBi/w54BytvUxQ05fFMPL8t
nBsKY/TMfVdSi6Z0dlxAw9td1MG5kUyoX9vZBFjwzntMzftZF12Bm4fSLiTj+rFG
ZZ/SuOa/PC+NCAIZfOoQFk1kbL5PI77jLF8GiBtNI7YOE7a13WndQvk++XHytJXA
glatyF5L0Yyxmx+NVECW/QKBgQCGlhIFn5/unoum6eEzIim4egHxs4kFl2GcwoJ/
Dp8i9UnZXO2tiCCbiOm0JYgo3WVxF8tZhF6xJ7lUrcw0HjrdqGvCIo6CX6lrtgSI
h5aEHPC2G5jBh3pRmAo7CVr/ddapQvYylbo5f9Wn6Ehg2cFusn83gFLr1gw8smr5
K42XFQKBgG9kGgAp4yaF2dStuaYbWPFqi1HoZTJxS3QOK+7Yz0UOOlIAlG986xra
gJodeaKvrqtHQ98amoIPxI69zpjXCQgdCcLRgCV+1fxzzwE256nK689hfDqH7MME
9hZrucqXVbdznECU5Q0hXudFuZtp5XwJkV50/KxVstLSsYD3Oa5U
-----END RSA PRIVATE KEY-----`,
			setupFile: true,
			wantErr:   false,
		},
		{
			name:        "nonexistent_key",
			setupFile:   false,
			wantErr:     true,
			errContains: "failed to read private key",
		},
		{
			name:        "invalid_key",
			keyContent:  "invalid key content",
			setupFile:   true,
			wantErr:     true,
			errContains: "failed to parse private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a key file path for this test
			keyPath := filepath.Join(tmpDir, fmt.Sprintf("key_%s.txt", tt.name))

			// Create the key file if needed
			if tt.setupFile {
				err := os.WriteFile(keyPath, []byte(tt.keyContent), 0600)
				if err != nil {
					t.Fatalf("Failed to create test key file: %v", err)
				}
			}

			// Test the loadPrivateKey function
			auth, err := loadPrivateKey(keyPath)

			// Check error cases
			if (err != nil) != tt.wantErr {
				t.Errorf("loadPrivateKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("loadPrivateKey() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			// For successful cases, verify we got a valid auth method
			if auth == nil {
				t.Error("loadPrivateKey() returned nil auth method for valid key")
			}
		})
	}
}

func TestGetPort(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantPort int
	}{
		{
			name:     "default_port",
			port:     0,
			wantPort: 22,
		},
		{
			name:     "custom_port",
			port:     2222,
			wantPort: 2222,
		},
		{
			name:     "negative_port",
			port:     -1,
			wantPort: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPort(tt.port); got != tt.wantPort {
				t.Errorf("getPort() = %v, want %v", got, tt.wantPort)
			}
		})
	}
}

func TestGetAuthMethods(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid private key file
	validKeyPath := filepath.Join(tmpDir, "valid_key.txt")
	err := os.WriteFile(validKeyPath, []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqe+MpHOH+g94fmSJkfAAXm3sPa5/lsSkRGh4rNue13xVaW8F
lv5vSKwJz8zhvwyDbsWZ4LDbyIGNaluO8hFACoKgOtrb3jHFxLoZqNGojYy08JU/
AwU0opoeWlv+2BeFAFDKZCFhLB7BGeahDgI8naLRPnvbnUSvog4VYBAC4JfSuJub
XoNel9duXlKs9SrxrbcmlbigMGKIRyizsGkxBQ8fHj4aHMDHHreEL2yhDCi9DJaD
cD5WDZl7lw6OWDqTtIx0cjqPOWZEN6dttK6ssnF4R3R8PsfrBWttXi/K5qabKWmS
fgep3oZzhCcwHHYPSizyrUW2t11dcTTowjsHEQIDAQABAoIBAEgiSjYIYIDyDjjA
nnDXSqDK0kwAhWJPSFdNbWQauZtIyMy/dsT4be6QMH4Hvw+k1SmxMTdie3jqHUg3
Yz83uVJ8zl0NR+VShVuTj47MqFqljqzM/XlfvU8EUrwSbuP1X9yZbsAAN0pOJ7PB
5T2YD3jugxmd48QnQtJsXgBv63i/5sELFL+e109fokofzj5lueeEAvYIZHoEkF7Y
rWNayxzwcaejpYNYRZc8Ix7EJDPP0LfKab4j+//uaHweodMtyQNd18qCxcuOAv1i
Pbw6tjs9oFF/vId8GrwQYAZ2K/1qn3igB2lci9Vc1nO4XZNjf64LRUOeKypBWTSG
zjQwxdECgYEA7+A2IZQTLkd+hL0X/th/O2LZZZ3Dtk6J8SFV8+lUIEuu9SnqkwUp
Gnp/ii50LzS7BZFuZogcyGck65VqTXoBno6LiVDREHft1rWgDtTwfw/Ky2xVfKBR
ik4ba1DDaEkZnqjhz+ZncKHMsv1jDX4gInQXDtKY3JIusT1t6+Aj2KUCgYEAtVvO
sCR/A6vbHqM08/AAutFZnOnT7X61diT1MPwO+nBCBU9Vrf2UVaNKroI3ChEurQDU
J6/TDliEVN4HCCffV+pyhRwuEqJns7yQEgCjHCRjFDAarq6uKQ8Jl0K/yCrGAU2s
bl5sq5YcaX6mv9odBsNzgwmQ/UtW9yprHTYGfP0CgYBi/w54BytvUxQ05fFMPL8t
nBsKY/TMfVdSi6Z0dlxAw9td1MG5kUyoX9vZBFjwzntMzftZF12Bm4fSLiTj+rFG
ZZ/SuOa/PC+NCAIZfOoQFk1kbL5PI77jLF8GiBtNI7YOE7a13WndQvk++XHytJXA
glatyF5L0Yyxmx+NVECW/QKBgQCGlhIFn5/unoum6eEzIim4egHxs4kFl2GcwoJ/
Dp8i9UnZXO2tiCCbiOm0JYgo3WVxF8tZhF6xJ7lUrcw0HjrdqGvCIo6CX6lrtgSI
h5aEHPC2G5jBh3pRmAo7CVr/ddapQvYylbo5f9Wn6Ehg2cFusn83gFLr1gw8smr5
K42XFQKBgG9kGgAp4yaF2dStuaYbWPFqi1HoZTJxS3QOK+7Yz0UOOlIAlG986xra
gJodeaKvrqtHQ98amoIPxI69zpjXCQgdCcLRgCV+1fxzzwE256nK689hfDqH7MME
9hZrucqXVbdznECU5Q0hXudFuZtp5XwJkV50/KxVstLSsYD3Oa5U
-----END RSA PRIVATE KEY-----`), 0600)
	if err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	// Create an invalid private key file
	invalidKeyPath := filepath.Join(tmpDir, "invalid_key.txt")
	err = os.WriteFile(invalidKeyPath, []byte("invalid key content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	tests := []struct {
		name        string
		target      config.Target
		wantMethods int
	}{
		{
			name: "valid_private_key",
			target: config.Target{
				PrivateKey: validKeyPath,
			},
			wantMethods: 1,
		},
		{
			name: "invalid_private_key_with_password_fallback",
			target: config.Target{
				PrivateKey: invalidKeyPath,
				Password:   "testpass",
			},
			wantMethods: 1,
		},
		{
			name: "password_only",
			target: config.Target{
				Password: "testpass",
			},
			wantMethods: 1,
		},
		{
			name:        "no_auth_methods",
			target:      config.Target{},
			wantMethods: 0,
		},
		{
			name: "nonexistent_key_with_password_fallback",
			target: config.Target{
				PrivateKey: "nonexistent.key",
				Password:   "testpass",
			},
			wantMethods: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methods := getAuthMethods(&tt.target)
			if len(methods) != tt.wantMethods {
				t.Errorf("getAuthMethods() got %v methods, want %v methods", len(methods), tt.wantMethods)
			}
		})
	}
}

func TestExecuteStep(t *testing.T) {
	tests := []struct {
		name        string
		step        *config.Step
		stepNum     int
		total       int
		mockSession *mockSession
		wantCmd     string
		wantErr     bool
		errString   string
	}{
		{
			name: "successful_command",
			step: &config.Step{
				Run: "echo hello",
			},
			stepNum: 1,
			total:   3,
			mockSession: &mockSession{
				stdout: "hello\n",
			},
			wantCmd: "sh -c 'echo hello'",
			wantErr: false,
		},
		{
			name: "successful_copy",
			step: &config.Step{
				Copy: &config.CopyStep{
					Src: "source/path",
					Dst: "dest/path",
				},
			},
			stepNum: 2,
			total:   3,
			wantErr: false,
		},
		{
			name: "successful_docker",
			step: &config.Step{
				Docker: &config.DockerStep{
					Name:  "test-container",
					Image: "nginx",
				},
			},
			stepNum: 3,
			total:   3,
			mockSession: &mockSession{
				stdout: "container started\n",
			},
			wantCmd: "sh -c 'docker rm -f test-container 2>/dev/null || true\ndocker create --name test-container nginx\ndocker start test-container'",
			wantErr: false,
		},
		{
			name: "command_error",
			step: &config.Step{
				Run: "invalid_command",
			},
			stepNum: 1,
			total:   3,
			mockSession: &mockSession{
				waitErr: fmt.Errorf("command failed"),
			},
			wantCmd:   "sh -c 'invalid_command'",
			wantErr:   true,
			errString: "command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSSHClient := &mockSSHClient{
				session: tt.mockSession,
			}
			mockCopier := &mockCopier{}

			client := &SSHClient{
				sshClient: mockSSHClient,
				copier:    mockCopier,
			}

			err := client.ExecuteStep(tt.step, tt.stepNum, tt.total)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteStep() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errString != "" && err != nil && !strings.Contains(err.Error(), tt.errString) {
				t.Errorf("ExecuteStep() error = %v, want error containing %q", err, tt.errString)
			}

			if tt.wantCmd != "" && tt.mockSession != nil && tt.mockSession.startedCmd != tt.wantCmd {
				t.Errorf("ExecuteStep() command = %q, want %q", tt.mockSession.startedCmd, tt.wantCmd)
			}

			// Check copy operations if applicable
			if tt.step.Copy != nil {
				if mockCopier.calledSrc != tt.step.Copy.Src {
					t.Errorf("ExecuteStep() copy src = %v, want %v", mockCopier.calledSrc, tt.step.Copy.Src)
				}
				if mockCopier.calledDst != tt.step.Copy.Dst {
					t.Errorf("ExecuteStep() copy dst = %v, want %v", mockCopier.calledDst, tt.step.Copy.Dst)
				}
			}
		})
	}
}

func TestRunJob(t *testing.T) {
	tests := []struct {
		name      string
		target    *config.Target
		job       *config.Job
		wantErr   bool
		errString string
	}{
		{
			name: "successful_job",
			target: &config.Target{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpass",
			},
			job: &config.Job{
				Steps: []*config.Step{
					{Run: "echo hello"},
					{Copy: &config.CopyStep{
						Src: "source",
						Dst: "dest",
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "ssh_connection_error",
			target: &config.Target{
				Host: "invalidhost",
				User: "testuser",
			},
			job: &config.Job{
				Steps: []*config.Step{
					{Run: "echo hello"},
				},
			},
			wantErr:   true,
			errString: "failed to create SSH client: SSH connection failed",
		},
		{
			name: "step_execution_error",
			target: &config.Target{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpass",
			},
			job: &config.Job{
				Steps: []*config.Step{
					{Run: "invalid_command"},
				},
			},
			wantErr:   true,
			errString: "step 1 failed",
		},
		{
			name: "empty_job",
			target: &config.Target{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpass",
			},
			job: &config.Job{
				Steps: []*config.Step{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origFactory := sshClientFactory
			defer func() { sshClientFactory = origFactory }()

			sshClientFactory = func(target *config.Target) (Client, error) {
				if target.Host == "invalidhost" {
					return nil, fmt.Errorf("SSH connection failed")
				}

				mockSession := &mockSession{}
				if tt.errString == "step 1 failed" {
					mockSession.waitErr = fmt.Errorf("command failed")
				}

				return &SSHClient{
					sshClient: &mockSSHClient{
						session: mockSession,
					},
					copier: &mockCopier{},
				}, nil
			}

			err := RunJob(tt.target, tt.job)

			if (err != nil) != tt.wantErr {
				t.Errorf("RunJob() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
				t.Errorf("RunJob() error = %v, want error containing %q", err, tt.errString)
			}
		})
	}
}
