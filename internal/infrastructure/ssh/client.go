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
	fileSystem fs.FileSystem
}

// NewClientFactory creates a new SSH client factory
func NewClientFactory() *ClientFactory {
	return &ClientFactory{
		fileSystem: fs.NewFileSystem(),
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

	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", tgt.Host, tgt.GetPort()), sshConfig)
	if err != nil {
		return nil, &job.ConnectionError{
			Target: tgt.GetName(),
			Cause:  err,
		}
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("SFTP connection failed: %w", err)
	}

	sftpAdapter := NewSFTPAdapter(sftpClient)
	copier := fs.NewCopier(f.fileSystem, sftpAdapter)

	return &SSHClient{
		sshClient:  NewSSHAdapter(sshClient),
		sftpClient: sftpAdapter,
		copier:     *copier,
		target:     tgt,
	}, nil
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
