package ssh

import (
	"strings"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
)

func TestDockerCommandBuilder_BuildCommands(t *testing.T) {
	tests := []struct {
		name            string
		dockerStep      *job.DockerStep
		expectedCmds    []string
		expectedNotCmds []string
	}{
		{
			name: "simple docker container",
			dockerStep: &job.DockerStep{
				Image: "nginx:latest",
				Name:  "web-server",
			},
			expectedCmds: []string{
				"docker rm -f web-server",
				"docker create --name web-server",
				"nginx:latest",
				"docker start web-server",
			},
		},
		{
			name: "docker with environment variables",
			dockerStep: &job.DockerStep{
				Image: "postgres:13",
				Name:  "db",
				Environment: map[string]string{
					"POSTGRES_USER":     "admin",
					"POSTGRES_PASSWORD": "secret",
				},
			},
			expectedCmds: []string{
				"docker rm -f db",
				"-e POSTGRES_USER",
				"-e POSTGRES_PASSWORD",
				"postgres:13",
				"docker start db",
			},
		},
		{
			name: "docker with ports and volumes",
			dockerStep: &job.DockerStep{
				Image: "redis:alpine",
				Name:  "cache",
				Ports: []string{"6379:6379"},
				Volumes: []string{
					"redis-data:/data",
				},
			},
			expectedCmds: []string{
				"docker rm -f cache",
				"-p 6379:6379",
				"-v redis-data:/data",
				"redis:alpine",
				"docker start cache",
			},
		},
		{
			name: "docker with networks",
			dockerStep: &job.DockerStep{
				Image:    "app:latest",
				Name:     "backend",
				Networks: []string{"app-network", "db-network"},
			},
			expectedCmds: []string{
				"docker network create app-network",
				"docker network create db-network",
				"docker network connect app-network backend",
				"docker network connect db-network backend",
			},
		},
		{
			name: "docker with restart policy",
			dockerStep: &job.DockerStep{
				Image:   "monitor:latest",
				Name:    "monitoring",
				Restart: "always",
			},
			expectedCmds: []string{
				"--restart always",
			},
		},
		{
			name: "docker with commands",
			dockerStep: &job.DockerStep{
				Image:    "alpine:latest",
				Name:     "task",
				Commands: []string{"sh", "-c", "echo hello && sleep 10"},
			},
			expectedCmds: []string{
				// Update the expected command format
				"docker create --name task alpine:latest \"sh -c echo hello && sleep 10\"",
			},
		},
		{
			name: "docker with labels",
			dockerStep: &job.DockerStep{
				Image: "traefik:latest",
				Name:  "proxy",
				Labels: map[string]string{
					"traefik.enable":                                     "true",
					"traefik.http.routers.app.rule":                      "Host(`app.example.com`)",
					"traefik.http.services.app.loadbalancer.server.port": "80",
				},
			},
			expectedCmds: []string{
				"-l traefik.enable",
				"-l traefik.http.routers.app.rule",
				"-l traefik.http.services.app.loadbalancer.server.port",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewDockerCommandBuilder(tt.dockerStep)
			commands := builder.BuildCommands()

			// Join all commands to simplify testing
			allCmds := strings.Join(commands, " ")

			// Check that expected commands are present
			for _, expected := range tt.expectedCmds {
				if !strings.Contains(allCmds, expected) {
					t.Errorf("Expected commands to contain '%s', but didn't find it in: %s", expected, allCmds)
				}
			}

			// Check that unexpected commands are not present
			for _, unexpected := range tt.expectedNotCmds {
				if strings.Contains(allCmds, unexpected) {
					t.Errorf("Commands should not contain '%s', but found it in: %s", unexpected, allCmds)
				}
			}

			// Check command count - should have remove, create, and start at minimum
			minCmdCount := 3
			if len(tt.dockerStep.Networks) > 0 {
				// Add network create and connect commands
				minCmdCount += len(tt.dockerStep.Networks) * 2
			}

			if len(commands) < minCmdCount {
				t.Errorf("Expected at least %d commands, got %d: %v", minCmdCount, len(commands), commands)
			}
		})
	}
}

func TestBuildDockerCreateCommand(t *testing.T) {
	tests := []struct {
		name       string
		dockerStep *job.DockerStep
		expected   string
		unexpected []string
	}{
		{
			name: "basic create command",
			dockerStep: &job.DockerStep{
				Image: "nginx:latest",
				Name:  "web",
			},
			expected: "docker create --name web nginx:latest",
		},
		{
			name: "create with restart policy",
			dockerStep: &job.DockerStep{
				Image:   "redis:alpine",
				Name:    "cache",
				Restart: "unless-stopped",
			},
			expected: "docker create --name cache --restart unless-stopped redis:alpine",
		},
		{
			name: "create with environment variables",
			dockerStep: &job.DockerStep{
				Image: "mysql:8",
				Name:  "db",
				Environment: map[string]string{
					"MYSQL_ROOT_PASSWORD": "rootpass",
					"MYSQL_DATABASE":      "appdb",
				},
			},
			expected: "docker create --name db -e MYSQL_ROOT_PASSWORD=\"rootpass\" -e MYSQL_DATABASE=\"appdb\" mysql:8",
		},
		{
			name: "create with ports and volumes",
			dockerStep: &job.DockerStep{
				Image:   "wordpress:latest",
				Name:    "blog",
				Ports:   []string{"8080:80"},
				Volumes: []string{"wp-data:/var/www/html"},
			},
			expected: "docker create --name blog -p 8080:80 -v wp-data:/var/www/html wordpress:latest",
		},
		{
			name: "create with command",
			dockerStep: &job.DockerStep{
				Image:    "alpine:latest",
				Name:     "task",
				Commands: []string{"sh", "-c", "echo hello"},
			},
			expected: "docker create --name task alpine:latest \"sh -c echo hello\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewDockerCommandBuilder(tt.dockerStep)
			cmd := builder.buildDockerCreateCommand()

			if !strings.Contains(cmd, tt.expected) {
				t.Errorf("Expected command to contain '%s', got: %s", tt.expected, cmd)
			}

			for _, unexpected := range tt.unexpected {
				if strings.Contains(cmd, unexpected) {
					t.Errorf("Command should not contain '%s', but found it in: %s", unexpected, cmd)
				}
			}
		})
	}
}

func TestAppendDockerArgs(t *testing.T) {
	builder := NewDockerCommandBuilder(nil)

	// Test with empty values
	args := builder.appendDockerArgs("-p", []string{})
	if len(args) != 0 {
		t.Errorf("Expected empty args for empty values, got: %v", args)
	}

	// Test with single value
	args = builder.appendDockerArgs("-v", []string{"/host:/container"})
	if len(args) != 2 || args[0] != "-v" || args[1] != "/host:/container" {
		t.Errorf("Expected ['-v', '/host:/container'], got: %v", args)
	}

	// Test with multiple values
	args = builder.appendDockerArgs("--network", []string{"net1", "net2"})
	if len(args) != 4 {
		t.Errorf("Expected 4 args for 2 values, got: %v", args)
	}
	if args[0] != "--network" || args[1] != "net1" || args[2] != "--network" || args[3] != "net2" {
		t.Errorf("Expected ['--network', 'net1', '--network', 'net2'], got: %v", args)
	}
}

func TestAppendDockerLabels(t *testing.T) {
	builder := NewDockerCommandBuilder(nil)

	// Test with empty labels
	args := builder.appendDockerLabels("-l", map[string]string{})
	if len(args) != 0 {
		t.Errorf("Expected empty args for empty labels, got: %v", args)
	}

	// Test with single label
	args = builder.appendDockerLabels("-l", map[string]string{"app": "web"})
	if len(args) != 2 || args[0] != "-l" {
		t.Errorf("Expected ['-l', 'app=\"web\"'], got: %v", args)
	}
	if !strings.Contains(args[1], "app=") || !strings.Contains(args[1], "web") {
		t.Errorf("Expected label format 'app=\"web\"', got: %s", args[1])
	}

	// Test with multiple labels
	args = builder.appendDockerLabels("-l", map[string]string{
		"com.example.description": "Test container",
		"com.example.version":     "1.0",
	})
	if len(args) != 4 {
		t.Errorf("Expected 4 args for 2 labels, got: %v", args)
	}
}
