package ssh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHSession represents an SSH session
type SSHSession interface {
	Start(string) error
	Wait() error
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
	Close() error
}

// SSHClientInterface represents SSH client functionality
type SSHClientInterface interface {
	NewSession() (SSHSession, error)
	Close() error
}

// SFTPClientInterface represents SFTP client functionality
type SFTPClientInterface interface {
	Create(path string) (io.WriteCloser, error)
	MkdirAll(path string) error
	Chmod(path string, mode os.FileMode) error
	Stat(path string) (os.FileInfo, error)
	Close() error
}

// SSHAdapter adapts ssh.Client to our SSHClientInterface
type SSHAdapter struct {
	*ssh.Client
}

// NewSSHAdapter creates a new SSHAdapter instance
func NewSSHAdapter(client *ssh.Client) SSHClientInterface {
	return &SSHAdapter{Client: client}
}

// NewSession implements SSHClientInterface by adapting the underlying ssh.Client's NewSession method
func (a *SSHAdapter) NewSession() (SSHSession, error) {
	return a.Client.NewSession()
}

// SFTPAdapter adapts sftp.Client to our SFTPClientInterface
type SFTPAdapter struct {
	*sftp.Client
}

// NewSFTPAdapter creates a new SFTPAdapter instance wrapping the provided sftp.Client
func NewSFTPAdapter(client *sftp.Client) SFTPClientInterface {
	return &SFTPAdapter{Client: client}
}

// Create implements SFTPClientInterface
func (a *SFTPAdapter) Create(path string) (io.WriteCloser, error) {
	return a.Client.Create(path)
}

// MkdirAll implements SFTPClientInterface
func (a *SFTPAdapter) MkdirAll(path string) error {
	return a.Client.MkdirAll(path)
}

// executeCommand executes a command on the remote host
func (c *SSHClient) executeCommand(step *job.Step, stepNum, totalSteps int) error {
	fmt.Printf("[%d/%d] Executing command...\n", stepNum, totalSteps)

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	return runShellCommand(session, step.GetShell(), step.Run, os.Stdout, os.Stderr)
}

// executeCopy copies files to the remote host
func (c *SSHClient) executeCopy(copyStep *job.CopyStep, stepNum, totalSteps int) error {
	fmt.Printf("[%d/%d] Copying '%s' to '%s'...\n", stepNum, totalSteps, copyStep.Local, copyStep.Remote)
	err := c.copier.CopyPath(copyStep.Local, copyStep.Remote, copyStep.Exclude)
	if err != nil {
		return &job.CopyError{
			Source:      copyStep.Local,
			Destination: copyStep.Remote,
			Cause:       err,
		}
	}
	return nil
}

// runShellCommand runs a shell command and pipes output to the provided writers
func runShellCommand(session SSHSession, shell, cmd string, stdout, stderr io.Writer) error {
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	cmd = fmt.Sprintf("%s -c %s", shell, escapeCommand(cmd))
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Use WaitGroup to ensure output is fully processed
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		pipeOutput(stdoutPipe, stdout)
		wg.Done()
	}()
	go func() {
		pipeOutput(stderrPipe, stderr)
		wg.Done()
	}()

	// Wait for command to finish
	err = session.Wait()

	// Wait for output processing to complete
	wg.Wait()

	if err != nil {
		return &job.CommandError{
			Command: cmd,
			Cause:   err,
		}
	}

	return nil
}

// escapeCommand escapes special characters in shell commands
func escapeCommand(cmd string) string {
	cmd = "'" + strings.ReplaceAll(cmd, "'", "'\\''") + "'"
	return strings.ReplaceAll(cmd, "`", "\\`")
}

// pipeOutput pipes output from a reader to a writer
func pipeOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
	}
}
