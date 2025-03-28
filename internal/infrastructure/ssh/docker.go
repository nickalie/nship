package ssh

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nickalie/nship/internal/core/job"
)

// DockerCommandBuilder constructs Docker commands
type DockerCommandBuilder struct {
	docker *job.DockerStep
}

// NewDockerCommandBuilder creates a new DockerCommandBuilder
func NewDockerCommandBuilder(docker *job.DockerStep) *DockerCommandBuilder {
	return &DockerCommandBuilder{docker: docker}
}

// BuildCommands builds a list of Docker commands
func (b *DockerCommandBuilder) BuildCommands() []string {
	commands := make([]string, 0)

	// Build image if build specification is provided
	if b.docker.Build != nil {
		buildCmd := b.buildDockerBuildCommand()
		commands = append(commands, buildCmd)
	}

	// Remove existing container if any
	if b.docker.Name != "" {
		commands = append(commands, fmt.Sprintf("docker rm -f %s 2>/dev/null || true", b.docker.Name))
	}

	// Create networks if any
	for _, network := range b.docker.Networks {
		commands = append(commands, fmt.Sprintf("docker network create %s 2>/dev/null || true", network))
	}

	// Create container
	commands = append(commands, b.buildDockerCreateCommand())

	// Connect networks
	for _, network := range b.docker.Networks {
		commands = append(commands, fmt.Sprintf("docker network connect %s %s", network, b.docker.Name))
	}

	// Start container
	commands = append(commands, fmt.Sprintf("docker start %s", b.docker.Name))

	return commands
}

// buildDockerBuildCommand builds a docker build command
func (b *DockerCommandBuilder) buildDockerBuildCommand() string {
	args := []string{"docker build"}

	// Add tag for the image
	args = append(args, "-t", b.docker.Image)

	// Add build arguments
	args = append(args, b.appendDockerBuildArgs("--build-arg", b.docker.Build.Args)...)

	// Add build context
	args = append(args, b.docker.Build.Context)

	return strings.Join(args, " ")
}

// appendDockerBuildArgs appends Docker build arguments
func (b *DockerCommandBuilder) appendDockerBuildArgs(flag string, args map[string]string) []string {
	buildArgs := make([]string, 0, len(args)*2)

	// Get keys and sort them for consistent order
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		buildArgs = append(buildArgs, flag, fmt.Sprintf("%s=%s", k, args[k]))
	}
	return buildArgs
}

// buildDockerCreateCommand builds a docker create command
func (b *DockerCommandBuilder) buildDockerCreateCommand() string {
	args := []string{"docker create"}
	if b.docker.Name != "" {
		args = append(args, "--name", b.docker.Name)
	}
	if b.docker.Restart != "" {
		args = append(args, "--restart", b.docker.Restart)
	}
	// Get env keys and sort them for consistent order
	envKeys := make([]string, 0, len(b.docker.Environment))
	for k := range b.docker.Environment {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	// Add environment variables in sorted order
	for _, k := range envKeys {
		args = append(args, "-e", fmt.Sprintf("%s=%q", k, b.docker.Environment[k]))
	}
	args = append(args, b.appendDockerArgs("-p", b.docker.Ports)...)
	args = append(args, b.appendDockerArgs("-v", b.docker.Volumes)...)
	args = append(args, b.appendDockerLabels("-l", b.docker.Labels)...)
	args = append(args, b.appendDockerArgs("--network", b.docker.Networks)...)
	args = append(args, b.docker.Image)
	args = append(args, b.docker.Command...)
	return strings.Join(args, " ")
}

// appendDockerArgs appends Docker arguments
func (b *DockerCommandBuilder) appendDockerArgs(flag string, values []string) []string {
	args := make([]string, 0, len(values)*2)
	for _, v := range values {
		args = append(args, flag, v)
	}
	return args
}

// appendDockerLabels appends Docker labels
func (b *DockerCommandBuilder) appendDockerLabels(flag string, labels map[string]string) []string {
	args := make([]string, 0, len(labels)*2)
	for k, v := range labels {
		args = append(args, flag, fmt.Sprintf("%s=%q", k, v))
	}
	return args
}

// executeDocker executes Docker commands on the remote host
func (c *SSHClient) executeDocker(step *job.Step, stepNum, totalSteps int) error {
	docker := step.Docker
	fmt.Printf("[%d/%d] Running Docker container '%s'...\n", stepNum, totalSteps, docker.Name)

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	builder := NewDockerCommandBuilder(docker)
	commands := builder.BuildCommands()
	err = runShellCommand(session, step.GetShell(), strings.Join(commands, "\n"), os.Stdout, os.Stderr)

	if err != nil {
		return &job.DockerError{
			ContainerName: docker.Name,
			Operation:     "create/start",
			Cause:         err,
		}
	}

	return nil
}
