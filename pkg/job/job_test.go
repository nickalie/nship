package job

import (
	"errors"
	"github.com/nickalie/ngdeploy/pkg/file"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"testing"

	"github.com/nickalie/ngdeploy/config"
)

type MockSSHClient struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	copier     *file.Copier
}

func (m *MockSSHClient) ExecuteStep(step *config.Step, stepNum, totalSteps int) error {
	return nil
}

func (m *MockSSHClient) Close() {}

func NewMockSSHClient(target config.Target) (Client, error) {
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
			_, err := NewMockSSHClient(tt.target)
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
