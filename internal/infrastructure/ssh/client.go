// Package ssh provides SSH-based implementations of the job execution interfaces.
// It handles remote connections, command execution, file transfers via SFTP,
// and Docker container management on remote hosts. This package encapsulates
// all SSH-specific implementation details and adapts them to the core domain
// interfaces defined in the job package.
package ssh

import (
	"fmt"
	"os"
	"time"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/nickalie/nship/internal/infrastructure/fs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHClient implements the job.Client interface using SSH connections
type SSHClient struct {
	sshClient  SSHClientInterface
	sftpClient SFTPClientInterface
	copier     fs.Copier
	target     *target.Target
}

// ClientFactory implements job.ClientFactory using SSH
type ClientFactory struct {
	sshDialer     SSHDialer
	sftpConnector SFTPConnector
}

// SSHDialer defines an interface for creating SSH connections
type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

// SFTPConnector defines an interface for creating SFTP clients
type SFTPConnector interface {
	NewClient(sshClient *ssh.Client) (*sftp.Client, error)
}

// DefaultSSHDialer implements SSHDialer using ssh.Dial
type DefaultSSHDialer struct{}

// Dial creates a new SSH connection
func (d *DefaultSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return ssh.Dial(network, addr, config)
}

// DefaultSFTPConnector implements SFTPConnector using sftp.NewClient
type DefaultSFTPConnector struct{}

// NewClient creates a new SFTP client
func (c *DefaultSFTPConnector) NewClient(sshClient *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(sshClient)
}

// NewClientFactory creates a new SSH client factory with default implementations
func NewClientFactory() *ClientFactory {
	return &ClientFactory{
		sshDialer:     &DefaultSSHDialer{},
		sftpConnector: &DefaultSFTPConnector{},
	}
}

// NewClientFactoryWithDeps creates a new SSH client factory with custom dependencies
func NewClientFactoryWithDeps(dialer SSHDialer, connector SFTPConnector) *ClientFactory {
	return &ClientFactory{
		sshDialer:     dialer,
		sftpConnector: connector,
	}
}

// NewClient creates a new SSH client for the given target
func (f *ClientFactory) NewClient(tgt *target.Target) (job.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            tgt.User,
		Auth:            getAuthMethods(tgt),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	sshClient, err := f.sshDialer.Dial("tcp", fmt.Sprintf("%s:%d", tgt.Host, tgt.GetPort()), sshConfig)
	if err != nil {
		return nil, &job.ConnectionError{
			Target: tgt.GetName(),
			Cause:  err,
		}
	}

	sftpClient, err := f.sftpConnector.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("SFTP connection failed: %w", err)
	}

	sftpAdapter := NewSFTPAdapter(sftpClient)
	copier := fs.NewCopier(sftpAdapter)

	return &SSHClient{
		sshClient:  NewSSHAdapter(sshClient),
		sftpClient: sftpAdapter,
		copier:     *copier,
		target:     tgt,
	}, nil
}

// NewSSHClientWithDeps creates a new SSH client with provided dependencies
// This is primarily used for testing
func NewSSHClientWithDeps(sshClient SSHClientInterface, sftpClient SFTPClientInterface, copier fs.Copier, tgt *target.Target) *SSHClient {
	return &SSHClient{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		copier:     copier,
		target:     tgt,
	}
}

// ExecuteStep implements the Client interface by executing a single deployment step.
func (c *SSHClient) ExecuteStep(step *job.Step, stepNum, totalSteps int) error {
	switch step.GetType() {
	case job.RunStep:
		return c.executeCommand(step, stepNum, totalSteps)
	case job.CopyStepType:
		return c.executeCopy(step.Copy, stepNum, totalSteps)
	case job.DockerStepType:
		return c.executeDocker(step, stepNum, totalSteps)
	default:
		return fmt.Errorf("invalid step configuration")
	}
}

// Close implements the Client interface by releasing resources.
func (c *SSHClient) Close() {
	if c.sftpClient != nil {
		_ = c.sftpClient.Close()
	}
	if c.sshClient != nil {
		_ = c.sshClient.Close()
	}
}

func getAuthMethods(tgt *target.Target) []ssh.AuthMethod {
	methods := []ssh.AuthMethod{}

	if tgt.PrivateKey != "" {
		if key, err := loadPrivateKey(tgt.PrivateKey); err == nil {
			methods = append(methods, key)
		}
	}

	if tgt.Password != "" {
		methods = append(methods, ssh.Password(tgt.Password))
	}

	return methods
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
