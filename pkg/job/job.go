// Package job provides functionality for executing deployment jobs on remote targets.
// It implements SSH-based deployment operations including command execution,
// file copying via SFTP, and Docker container management on remote hosts.
package job

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nickalie/nship/config"
	"github.com/nickalie/nship/pkg/file"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// CopierInterface defines the interface for file copying operations
type CopierInterface interface {
	CopyPath(src, dst string, exclude []string) error
}

// SSHSession represents an SSH session
type SSHSession interface {
	Start(string) error
	Wait() error
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
	Close() error
}

// SFTPAdapter adapts sftp.Client to our SFTPClient interface
type SFTPAdapter struct {
	*sftp.Client
}

// NewSFTPAdapter creates a new SFTPAdapter instance wrapping the provided sftp.Client
func NewSFTPAdapter(client *sftp.Client) *SFTPAdapter {
	return &SFTPAdapter{Client: client}
}

// Create implements SFTPClient interface
func (a *SFTPAdapter) Create(path string) (io.WriteCloser, error) {
	return a.Client.Create(path)
}

// MkdirAll implements SFTPClient interface
func (a *SFTPAdapter) MkdirAll(path string) error {
	return a.Client.MkdirAll(path)
}

// Runner defines a function type for executing jobs on targets
type Runner func(target *config.Target, job *config.Job) error

// Client defines the interface for executing deployment steps
type Client interface {
	// ExecuteStep executes a single step with progress information
	ExecuteStep(step *config.Step, stepNum, totalSteps int) error
	// Close releases all resources associated with the client
	Close()
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
	Close() error
}

// SSHAdapter adapts ssh.Client to our SSHClientInterface
type SSHAdapter struct {
	*ssh.Client
}

// NewSSHAdapter creates a new SSHAdapter instance
func NewSSHAdapter(client *ssh.Client) *SSHAdapter {
	return &SSHAdapter{Client: client}
}

// NewSession implements SSHClientInterface by adapting the underlying ssh.Client's NewSession method
func (a *SSHAdapter) NewSession() (SSHSession, error) {
	return a.Client.NewSession()
}

// Close implements SSHClientInterface
func (a *SSHAdapter) Close() error {
	return a.Client.Close()
}

// Close implements SFTPClientInterface
func (a *SFTPAdapter) Close() error {
	return a.Client.Close()
}

// SSHClient implements the Client interface using SSH connections
type SSHClient struct {
	sshClient  SSHClientInterface
	sftpClient SFTPClientInterface
	copier     CopierInterface
}

// NewSSHClient creates a new SSHClient instance with the provided target configuration
func NewSSHClient(target *config.Target) (Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            target.User,
		Auth:            getAuthMethods(target),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", target.Host, getPort(target.Port)), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("SFTP connection failed: %w", err)
	}

	sftpAdapter := NewSFTPAdapter(sftpClient)
	copier := file.NewCopier(&file.DefaultFileSystem{}, sftpAdapter)

	return &SSHClient{
		sshClient:  NewSSHAdapter(sshClient),
		sftpClient: sftpAdapter,
		copier:     copier,
	}, nil
}

// Close implements the Client interface by releasing both SFTP and SSH client resources.
// It ensures all connections are properly terminated.
func (c *SSHClient) Close() {
	if c.sftpClient != nil {
		_ = c.sftpClient.Close()
	}
	if c.sshClient != nil {
		_ = c.sshClient.Close()
	}
}

// Add this at the package level, before any tests
var sshClientFactory = NewSSHClient

// RunJob executes a job on a target using a new SSH client
func RunJob(target *config.Target, job *config.Job) error {
	client, err := sshClientFactory(target)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}
	defer client.Close()

	for i, step := range job.Steps {
		if err := client.ExecuteStep(step, i+1, len(job.Steps)); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}
	return nil
}

// ExecuteStep implements the Client interface by executing a single deployment step.
// It handles different step types (Run, Copy, Docker) and provides progress information.
func (c *SSHClient) ExecuteStep(step *config.Step, stepNum, totalSteps int) error {
	switch {
	case step.Run != "":
		return c.executeCommand(step, stepNum, totalSteps)
	case step.Copy != nil:
		return c.executeCopy(step.Copy, stepNum, totalSteps)
	case step.Docker != nil:
		return c.executeDocker(step, stepNum, totalSteps)
	default:
		return fmt.Errorf("invalid step configuration")
	}
}

func getAuthMethods(target *config.Target) []ssh.AuthMethod {
	if target.PrivateKey != "" {
		if key, err := loadPrivateKey(target.PrivateKey); err == nil {
			return []ssh.AuthMethod{key}
		}
	}
	if target.Password != "" {
		return []ssh.AuthMethod{ssh.Password(target.Password)}
	}
	return nil
}

func loadPrivateKey(keyPath string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

func getPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func (c *SSHClient) executeCommand(step *config.Step, stepNum, totalSteps int) error {
	fmt.Printf("[%d/%d] Executing command...\n", stepNum, totalSteps)

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	return runShellCommand(session, getShell(step.Shell), step.Run, os.Stdout, os.Stderr)
}

func (c *SSHClient) executeCopy(copyStep *config.CopyStep, stepNum, totalSteps int) error {
	fmt.Printf("[%d/%d] Copying '%s' to '%s'...\n", stepNum, totalSteps, copyStep.Src, copyStep.Dst)
	return c.copier.CopyPath(copyStep.Src, copyStep.Dst, copyStep.Exclude)
}

func (c *SSHClient) executeDocker(step *config.Step, stepNum, totalSteps int) error {
	docker := step.Docker
	fmt.Printf("[%d/%d] Running Docker container '%s'...\n", stepNum, totalSteps, docker.Name)

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	commands := buildDockerCommands(docker)
	return runShellCommand(session, getShell(step.Shell), strings.Join(commands, "\n"), os.Stdout, os.Stderr)
}

func buildDockerCommands(docker *config.DockerStep) []string {
	commands := make([]string, 0)

	if docker.Name != "" {
		commands = append(commands, fmt.Sprintf("docker rm -f %s 2>/dev/null || true", docker.Name))
	}

	for _, network := range docker.Networks {
		commands = append(commands, fmt.Sprintf("docker network create %s 2>/dev/null || true", network))
	}

	commands = append(commands, buildDockerCreateCommand(docker))

	for _, network := range docker.Networks {
		commands = append(commands, fmt.Sprintf("docker network connect %s %s", network, docker.Name))
	}

	commands = append(commands, fmt.Sprintf("docker start %s", docker.Name))

	return commands
}

func buildDockerCreateCommand(docker *config.DockerStep) string {
	args := []string{"docker create"}

	if docker.Name != "" {
		args = append(args, "--name", docker.Name)
	}

	if docker.Restart != "" {
		args = append(args, "--restart", docker.Restart)
	}

	for k, v := range docker.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%q", k, v))
	}

	args = append(args,
		appendDockerArgs("-p", docker.Ports)...,
	)
	args = append(args,
		appendDockerArgs("-v", docker.Volumes)...,
	)
	args = append(args,
		appendDockerLabels("-l", docker.Labels)...,
	)
	args = append(args,
		appendDockerNetworks("--network", docker.Networks)...,
	)

	args = append(args, docker.Image)
	args = append(args, docker.Commands...)

	return strings.Join(args, " ")
}

func appendDockerArgs(flag string, values []string) []string {
	args := make([]string, 0, len(values)*2)
	for _, v := range values {
		args = append(args, flag, v)
	}
	return args
}

func appendDockerLabels(flag string, labels map[string]string) []string {
	args := make([]string, 0, len(labels)*2)
	for k, v := range labels {
		args = append(args, flag, fmt.Sprintf("%s=%q", k, v))
	}
	return args
}

func appendDockerNetworks(flag string, networks []string) []string {
	args := make([]string, 0, len(networks)*2)
	for _, n := range networks {
		args = append(args, flag, n)
	}
	return args
}

func getShell(shell string) string {
	if shell == "" {
		return "sh"
	}
	return shell
}

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

	return err
}

func escapeCommand(cmd string) string {
	cmd = "'" + strings.ReplaceAll(cmd, "'", "'\\''") + "'"
	return strings.ReplaceAll(cmd, "`", "\\`")
}

func pipeOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
	}
}
