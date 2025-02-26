// Package mocks provides mock implementations for testing.
package mocks

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/mock"
)

// MockClient mocks the job.Client interface for testing
type MockClient struct {
	mock.Mock
}

// ExecuteStep implements the job.Client.ExecuteStep method
func (m *MockClient) ExecuteStep(step *job.Step, stepNum, totalSteps int) error {
	args := m.Called(step, stepNum, totalSteps)
	return args.Error(0)
}

// Close implements the job.Client.Close method
func (m *MockClient) Close() {
	m.Called()
}

// MockClientFactory mocks the job.ClientFactory interface for testing
type MockClientFactory struct {
	mock.Mock
}

// NewClient implements the job.ClientFactory.NewClient method for testing purposes.
// It returns a mocked Client implementation with controlled behavior based on the
// arguments passed to the mock's On() method. The tgt parameter represents the target
// to connect to.
func (m *MockClientFactory) NewClient(tgt *target.Target) (job.Client, error) {
	args := m.Called(tgt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(job.Client), args.Error(1)
}

// MockSession mocks an SSH session
type MockSession struct {
	mock.Mock
	startErr   error
	waitErr    error
	stdout     string
	stderr     string
	startedCmd string
	stdoutPipe io.Reader
	stderrPipe io.Reader
	pipeErr    error
}

// Start implements session.Start for testing
func (m *MockSession) Start(cmd string) error {
	m.startedCmd = cmd
	return m.startErr
}

// Wait implements session.Wait for testing
func (m *MockSession) Wait() error {
	return m.waitErr
}

// StdoutPipe implements session.StdoutPipe for testing
func (m *MockSession) StdoutPipe() (io.Reader, error) {
	if m.stdoutPipe != nil {
		return m.stdoutPipe, m.pipeErr
	}
	return strings.NewReader(m.stdout), m.pipeErr
}

// StderrPipe implements session.StderrPipe for testing
func (m *MockSession) StderrPipe() (io.Reader, error) {
	if m.stderrPipe != nil {
		return m.stderrPipe, m.pipeErr
	}
	return strings.NewReader(m.stderr), m.pipeErr
}

// Close implements session.Close for testing
func (m *MockSession) Close() error {
	return nil
}

// MockFileSystem mocks the file system interface for testing
type MockFileSystem struct {
	mock.Mock
	files   map[string]*MockFile
	statErr error
	openErr error
	readErr error
	entries []os.DirEntry
}

// MockFile represents a mocked file in the file system
type MockFile struct {
	content string
	mode    os.FileMode
	size    int64
	isDir   bool
}

// Stat implements FileSystem.Stat for testing
func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	if file, exists := m.files[name]; exists {
		return &MockFileInfo{name: name, size: file.size, mode: file.mode, isDir: file.isDir}, nil
	}
	return nil, os.ErrNotExist
}

// Open implements FileSystem.Open for testing
func (m *MockFileSystem) Open(name string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	if file, exists := m.files[name]; exists {
		return io.NopCloser(strings.NewReader(file.content)), nil
	}
	return nil, os.ErrNotExist
}

// ReadDir implements FileSystem.ReadDir for testing
func (m *MockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	return m.entries, nil
}

// WriteFile implements FileSystem.WriteFile for testing
func (m *MockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	args := m.Called(name, data, perm)
	return args.Error(0)
}

// MkdirAll implements FileSystem.MkdirAll for testing
func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

// RemoveAll implements FileSystem.RemoveAll for testing
func (m *MockFileSystem) RemoveAll(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

// MockFileInfo mocks file info
type MockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	isDir   bool
	modTime time.Time
}

// Name returns the base name of the file
func (m *MockFileInfo) Name() string { return m.name }

// Size returns the size of the file in bytes
func (m *MockFileInfo) Size() int64 { return m.size }

// Mode returns the file mode bits
func (m *MockFileInfo) Mode() os.FileMode { return m.mode }

// ModTime returns the modification time
func (m *MockFileInfo) ModTime() time.Time { return m.modTime }

// IsDir returns if the file is a directory
func (m *MockFileInfo) IsDir() bool { return m.isDir }

// Sys returns the underlying data source
func (m *MockFileInfo) Sys() interface{} { return nil }

// MockEnvLoader mocks the environment loader
type MockEnvLoader struct {
	mock.Mock
}

// Load mocks the env.Loader.Load method
func (m *MockEnvLoader) Load(path, vaultPassword string) error {
	args := m.Called(path, vaultPassword)
	return args.Error(0)
}

// MockVaultDecrypter mocks the vault decrypter
type MockVaultDecrypter struct {
	mock.Mock
}

// Decrypt mocks the VaultDecrypter.Decrypt method
func (m *MockVaultDecrypter) Decrypt(content, password string) (string, error) {
	args := m.Called(content, password)
	return args.String(0), args.Error(1)
}

// MockCopier mocks the file copier
type MockCopier struct {
	mock.Mock
}

// CopyPath mocks the fs.Copier.CopyPath method
func (m *MockCopier) CopyPath(src, dst string, exclude []string) error {
	args := m.Called(src, dst, exclude)
	return args.Error(0)
}

// CopyFile mocks the fs.Copier.CopyFile method
func (m *MockCopier) CopyFile(src, dst string) error {
	args := m.Called(src, dst)
	return args.Error(0)
}

// CopyDir mocks the fs.Copier.CopyDir method
func (m *MockCopier) CopyDir(src, dst string, exclude []string) error {
	args := m.Called(src, dst, exclude)
	return args.Error(0)
}
