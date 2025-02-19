package job

import (
	"bufio"
	"fmt"
	"io"
	"ngdeploy/pkg/sync"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"ngdeploy/config"
	"ngdeploy/pkg/file"
)

func RunJob(target config.Target, job config.Job) error {
	var authMethods []ssh.AuthMethod

	if target.PrivateKey != "" {
		key, err := os.ReadFile(target.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if target.Password != "" {
		authMethods = append(authMethods, ssh.Password(target.Password))
	} else {
		return fmt.Errorf("no authentication method provided for target %s", target.Name)
	}

	config := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	port := target.Port
	if port == 0 {
		port = 22
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", target.Host, port), config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer client.Close()

	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	for i, step := range job.Steps {
		if step.Run != "" {
			fmt.Printf("[%d/%d] Executing command...\n", i+1, len(job.Steps))

			shell := "sh"
			if step.Shell != "" {
				shell = step.Shell
			}

			session, err := client.NewSession()
			if err != nil {
				return fmt.Errorf("failed to create SSH session: %w", err)
			}

			stdout, err := session.StdoutPipe()
			if err != nil {
				session.Close()
				return fmt.Errorf("failed to get stdout pipe: %w", err)
			}
			stderr, err := session.StderrPipe()
			if err != nil {
				session.Close()
				return fmt.Errorf("failed to get stderr pipe: %w", err)
			}

			err = session.Start(fmt.Sprintf("%s -c %s", shell, escapeCommand(step.Run)))
			if err != nil {
				session.Close()
				return fmt.Errorf("failed to start command: %w", err)
			}

			go printOutput(stdout, os.Stdout)
			go printOutput(stderr, os.Stderr)

			err = session.Wait()
			session.Close()
			if err != nil {
				return fmt.Errorf("command execution failed: %w", err)
			}

		} else if step.Copy != nil {
			fmt.Printf("[%d/%d] Copying '%s' to '%s'...\n", i+1, len(job.Steps), step.Copy.Src, step.Copy.Dst)
			err := file.CopyPath(sftpClient, step.Copy.Src, step.Copy.Dst, step.Copy.Exclude)
			if err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}
		} else if step.Sync != nil {
			fmt.Printf("[%d/%d] Syncing '%s' to '%s'...\n", i+1, len(job.Steps), step.Sync.Src, step.Sync.Dst)
			stats, err := sync.SyncDirectory(sftpClient, step.Sync.Src, step.Sync.Dst, step.Sync.Exclude)
			if err != nil {
				return fmt.Errorf("failed to sync: %w", err)
			}
			fmt.Printf("Sync completed: %d files transferred, %d files skipped, %d directories created\n",
				stats.FilesTransferred, stats.FilesSkipped, stats.DirsCreated)
		}
	}

	return nil
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
