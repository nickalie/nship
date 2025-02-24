package job

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"ngdeploy/config"
	"ngdeploy/pkg/file"
)

type SSHClient struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

func NewSSHClient(target config.Target) (*SSHClient, error) {
	sshClient, sftpClient, err := connectToTarget(target)
	if err != nil {
		return nil, err
	}
	return &SSHClient{
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}, nil
}

func (c *SSHClient) Close() {
	if c.sftpClient != nil {
		c.sftpClient.Close()
	}
	if c.sshClient != nil {
		c.sshClient.Close()
	}
}

func RunJob(target config.Target, job config.Job) error {
	client, err := NewSSHClient(target)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}
	defer client.Close()

	for i, step := range job.Steps {
		if err := executeStep(client, step, i+1, len(job.Steps)); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}
	return nil
}

func connectToTarget(target config.Target) (*ssh.Client, *sftp.Client, error) {
	authMethods, err := getAuthMethods(target)
	if err != nil {
		return nil, nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	port := getPort(target.Port)
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", target.Host, port), sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("SFTP connection failed: %w", err)
	}

	return client, sftpClient, nil
}

func getAuthMethods(target config.Target) ([]ssh.AuthMethod, error) {
	if target.PrivateKey != "" {
		return getPrivateKeyAuth(target.PrivateKey)
	}
	if target.Password != "" {
		return []ssh.AuthMethod{ssh.Password(target.Password)}, nil
	}
	return nil, fmt.Errorf("no authentication method provided for target %s", target.Name)
}

func getPrivateKeyAuth(keyPath string) ([]ssh.AuthMethod, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

func getPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func executeStep(client *SSHClient, step config.Step, stepNum, totalSteps int) error {
	switch {
	case step.Run != "":
		return executeCommand(client.sshClient, step, stepNum, totalSteps)
	case step.Copy != nil:
		return executeCopy(client.sftpClient, step.Copy, stepNum, totalSteps)
	default:
		return fmt.Errorf("invalid step configuration")
	}
}

func executeCommand(client *ssh.Client, step config.Step, stepNum, totalSteps int) error {
	fmt.Printf("[%d/%d] Executing command...\n", stepNum, totalSteps)

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	shell := getShell(step.Shell)
	return runCommand(session, shell, step.Run)
}

func executeCopy(client *sftp.Client, copyStep *config.CopyStep, stepNum, totalSteps int) error {
	fmt.Printf("[%d/%d] Copying '%s' to '%s'...\n", stepNum, totalSteps, copyStep.Src, copyStep.Dst)
	return file.CopyPath(client, copyStep.Src, copyStep.Dst, copyStep.Exclude)
}

func runCommand(session *ssh.Session, shell, cmd string) error {
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := session.Start(fmt.Sprintf("%s -c %s", shell, escapeCommand(cmd))); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	go printOutput(stdout, os.Stdout)
	go printOutput(stderr, os.Stderr)

	return session.Wait()
}

func getShell(shell string) string {
	if shell == "" {
		return "sh"
	}
	return shell
}

func escapeCommand(cmd string) string {
	return "'" + strings.Replace(cmd, "'", "'\\''", -1) + "'"
}

func printOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
	}
}
