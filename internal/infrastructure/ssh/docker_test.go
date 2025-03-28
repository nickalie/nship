package ssh

import (
	"strings"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/stretchr/testify/assert"
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
				Image:   "alpine:latest",
				Name:    "task",
				Command: []string{"sh", "-c", "echo hello && sleep 10"},
			},
			expectedCmds: []string{
				// Update the expected command format
				"docker create --name task alpine:latest sh -c echo hello && sleep 10",
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
		{
			name: "docker with build configuration",
			dockerStep: &job.DockerStep{
				Image: "myapp:latest",
				Name:  "custom-app",
				Build: &job.DockerBuildStep{
					Context: "./app",
					Args: map[string]string{
						"VERSION": "1.0.0",
						"ENV":     "production",
					},
				},
			},
			expectedCmds: []string{
				"docker build -t myapp:latest",
				"--build-arg ENV=production",
				"--build-arg VERSION=1.0.0",
				"./app",
				"docker rm -f custom-app",
				"docker create --name custom-app",
				"myapp:latest",
				"docker start custom-app",
			},
		},
		{
			name: "docker with build context only",
			dockerStep: &job.DockerStep{
				Image: "simple-app:dev",
				Name:  "simple",
				Build: &job.DockerBuildStep{
					Context: ".",
				},
			},
			expectedCmds: []string{
				"docker build -t simple-app:dev .",
				"docker rm -f simple",
				"docker create --name simple simple-app:dev",
				"docker start simple",
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
				assert.Contains(t, allCmds, expected,
					"Expected commands to contain '%s'", expected)
			}

			// Check that unexpected commands are not present
			for _, unexpected := range tt.expectedNotCmds {
				assert.NotContains(t, allCmds, unexpected,
					"Commands should not contain '%s'", unexpected)
			}

			// Check command count - should have remove, create, and start at minimum
			minCmdCount := 3
			if len(tt.dockerStep.Networks) > 0 {
				// Add network create and connect commands
				minCmdCount += len(tt.dockerStep.Networks) * 2
			}

			assert.GreaterOrEqual(t, len(commands), minCmdCount,
				"Expected at least %d commands, got %d", minCmdCount, len(commands))
		})
	}
}

func TestBuildDockerCreateCommand(t *testing.T) {
	tests := []struct {
		name          string
		dockerStep    *job.DockerStep
		expectedParts []string
		unexpected    []string
	}{
		{
			name: "basic create command",
			dockerStep: &job.DockerStep{
				Image: "nginx:latest",
				Name:  "web",
			},
			expectedParts: []string{"docker create", "--name web", "nginx:latest"},
		},
		{
			name: "create with restart policy",
			dockerStep: &job.DockerStep{
				Image:   "redis:alpine",
				Name:    "cache",
				Restart: "unless-stopped",
			},
			expectedParts: []string{"docker create", "--name cache", "--restart unless-stopped", "redis:alpine"},
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
			expectedParts: []string{
				"docker create",
				"--name db",
				"-e MYSQL_DATABASE=\"appdb\"",
				"-e MYSQL_ROOT_PASSWORD=\"rootpass\"",
				"mysql:8",
			},
		},
		{
			name: "create with ports and volumes",
			dockerStep: &job.DockerStep{
				Image:   "wordpress:latest",
				Name:    "blog",
				Ports:   []string{"8080:80"},
				Volumes: []string{"wp-data:/var/www/html"},
			},
			expectedParts: []string{
				"docker create",
				"--name blog",
				"-p 8080:80",
				"-v wp-data:/var/www/html",
				"wordpress:latest",
			},
		},
		{
			name: "create with command",
			dockerStep: &job.DockerStep{
				Image:   "alpine:latest",
				Name:    "task",
				Command: []string{"sh", "-c", "echo hello"},
			},
			expectedParts: []string{
				"docker create",
				"--name task",
				"alpine:latest",
				"sh",
				"-c",
				"echo hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewDockerCommandBuilder(tt.dockerStep)
			cmd := builder.buildDockerCreateCommand()

			// Check that all expected parts are in the command
			for _, expected := range tt.expectedParts {
				assert.Contains(t, cmd, expected, "Command should contain '%s'", expected)
			}

			// Check that unexpected parts are not in the command
			for _, unexpected := range tt.unexpected {
				assert.NotContains(t, cmd, unexpected, "Command should not contain '%s'", unexpected)
			}
		})
	}
}

func TestBuildDockerBuildCommand(t *testing.T) {
	tests := []struct {
		name          string
		dockerStep    *job.DockerStep
		expectedParts []string
		unexpected    []string
	}{
		{
			name: "basic build command",
			dockerStep: &job.DockerStep{
				Image: "app:v1",
				Build: &job.DockerBuildStep{
					Context: "./src",
				},
			},
			expectedParts: []string{"docker build", "-t app:v1", "./src"},
		},
		{
			name: "build with args",
			dockerStep: &job.DockerStep{
				Image: "web:latest",
				Build: &job.DockerBuildStep{
					Context: "/path/to/code",
					Args: map[string]string{
						"NODE_ENV": "production",
						"API_URL":  "https://api.example.com",
					},
				},
			},
			expectedParts: []string{
				"docker build",
				"-t web:latest",
				"--build-arg API_URL=https://api.example.com",
				"--build-arg NODE_ENV=production",
				"/path/to/code",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewDockerCommandBuilder(tt.dockerStep)
			cmd := builder.buildDockerBuildCommand()

			// Check that all expected parts are in the command
			for _, expected := range tt.expectedParts {
				assert.Contains(t, cmd, expected, "Command should contain '%s'", expected)
			}

			// Check that unexpected parts are not in the command
			for _, unexpected := range tt.unexpected {
				assert.NotContains(t, cmd, unexpected, "Command should not contain '%s'", unexpected)
			}
		})
	}
}

func TestAppendDockerArgs(t *testing.T) {
	builder := NewDockerCommandBuilder(nil)

	// Test with empty values
	args := builder.appendDockerArgs("-p", []string{})
	assert.Empty(t, args, "Expected empty args for empty values")

	// Test with single value
	args = builder.appendDockerArgs("-v", []string{"/host:/container"})
	assert.Len(t, args, 2, "Expected 2 args for a single value")
	assert.Equal(t, "-v", args[0], "First arg should be the flag")
	assert.Equal(t, "/host:/container", args[1], "Second arg should be the value")

	// Test with multiple values
	args = builder.appendDockerArgs("--network", []string{"net1", "net2"})
	assert.Len(t, args, 4, "Expected 4 args for 2 values")
	assert.Equal(t, "--network", args[0], "First arg should be the flag")
	assert.Equal(t, "net1", args[1], "Second arg should be the first value")
	assert.Equal(t, "--network", args[2], "Third arg should be the flag again")
	assert.Equal(t, "net2", args[3], "Fourth arg should be the second value")
}

func TestAppendDockerLabels(t *testing.T) {
	builder := NewDockerCommandBuilder(nil)

	// Test with empty labels
	args := builder.appendDockerLabels("-l", map[string]string{})
	assert.Empty(t, args, "Expected empty args for empty labels")

	// Test with single label
	args = builder.appendDockerLabels("-l", map[string]string{"app": "web"})
	assert.Len(t, args, 2, "Expected 2 args for a single label")
	assert.Equal(t, "-l", args[0], "First arg should be the flag")
	assert.Contains(t, args[1], "app=", "Label should contain key with equals sign")
	assert.Contains(t, args[1], "web", "Label should contain value")

	// Test with multiple labels
	args = builder.appendDockerLabels("-l", map[string]string{
		"com.example.description": "Test container",
		"com.example.version":     "1.0",
	})
	assert.Len(t, args, 4, "Expected 4 args for 2 labels")
}
