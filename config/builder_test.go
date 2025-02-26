package config

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuilder(t *testing.T) {
	t.Run("building empty config", func(t *testing.T) {
		builder := NewBuilder()
		assert.NotNil(t, builder.config)
		assert.Empty(t, builder.config.Targets)
		assert.Empty(t, builder.config.Jobs)
	})

	t.Run("adding target", func(t *testing.T) {
		target := &Target{
			Name:     "test-server",
			Host:     "localhost",
			User:     "testuser",
			Password: "secret",
			Port:     22,
		}

		builder := NewBuilder()
		builder.AddTarget(target)

		assert.Len(t, builder.config.Targets, 1)
		assert.Equal(t, target, builder.config.Targets[0])
	})

	t.Run("adding multiple targets", func(t *testing.T) {
		target1 := &Target{
			Name:     "server1",
			Host:     "localhost",
			User:     "user1",
			Password: "secret1",
			Port:     22,
		}
		target2 := &Target{
			Name:     "server2",
			Host:     "127.0.0.1",
			User:     "user2",
			Password: "secret2",
			Port:     2222,
		}

		builder := NewBuilder()
		builder.AddTarget(target1).AddTarget(target2)

		assert.Len(t, builder.config.Targets, 2)
		assert.Equal(t, target1, builder.config.Targets[0])
		assert.Equal(t, target2, builder.config.Targets[1])
	})

	t.Run("adding job with steps", func(t *testing.T) {
		builder := NewBuilder()
		builder.AddJob("deploy")

		expectedCommand := "echo 'test'"
		builder.AddRunStep(expectedCommand)

		assert.Len(t, builder.config.Jobs, 1)
		assert.Equal(t, "deploy", builder.config.Jobs[0].Name)
		assert.Len(t, builder.config.Jobs[0].Steps, 1)
		assert.Equal(t, expectedCommand, builder.config.Jobs[0].Steps[0].Run)
	})

	t.Run("adding multiple jobs", func(t *testing.T) {
		builder := NewBuilder()
		builder.AddJob("deploy").AddRunStep("echo 'deploy'")
		builder.AddJob("test").AddRunStep("echo 'test'")

		assert.Len(t, builder.config.Jobs, 2)
		assert.Equal(t, "deploy", builder.config.Jobs[0].Name)
		assert.Equal(t, "test", builder.config.Jobs[1].Name)
	})

	t.Run("adding multiple steps to a job", func(t *testing.T) {
		builder := NewBuilder()
		builder.AddJob("deploy")
		builder.AddRunStep("echo 'step1'")
		builder.AddCopyStep("./src", "/dst")
		builder.AddDockerStep(&DockerStep{
			Image: "nginx",
			Name:  "web",
			Ports: []string{"80:80"},
		})

		assert.Len(t, builder.config.Jobs[0].Steps, 3)
		assert.NotNil(t, builder.config.Jobs[0].Steps[0].Run)
		assert.NotNil(t, builder.config.Jobs[0].Steps[1].Copy)
		assert.NotNil(t, builder.config.Jobs[0].Steps[2].Docker)
	})

	t.Run("adding copy step", func(t *testing.T) {
		builder := NewBuilder()
		builder.AddJob("copy-job")
		builder.AddCopyStep("./src", "/dst")

		assert.Len(t, builder.config.Jobs[0].Steps, 1)
		assert.NotNil(t, builder.config.Jobs[0].Steps[0].Copy)
		assert.Equal(t, "./src", builder.config.Jobs[0].Steps[0].Copy.Src)
		assert.Equal(t, "/dst", builder.config.Jobs[0].Steps[0].Copy.Dst)
	})

	t.Run("adding docker step", func(t *testing.T) {
		builder := NewBuilder()
		builder.AddJob("docker-job")

		dockerStep := &DockerStep{
			Image: "nginx",
			Name:  "web",
			Ports: []string{"80:80"},
		}
		builder.AddDockerStep(dockerStep)

		assert.Len(t, builder.config.Jobs[0].Steps, 1)
		assert.NotNil(t, builder.config.Jobs[0].Steps[0].Docker)
		assert.Equal(t, dockerStep, builder.config.Jobs[0].Steps[0].Docker)
	})

	t.Run("building complete config", func(t *testing.T) {
		builder := NewBuilder()

		target := &Target{
			Name:     "prod-server",
			Host:     "example.com",
			User:     "admin",
			Password: "secret",
		}
		builder.AddTarget(target)

		builder.AddJob("full-deploy")
		builder.AddRunStep("echo 'Starting deployment'")
		builder.AddCopyStep("./app", "/opt/app")
		builder.AddDockerStep(&DockerStep{
			Image: "nginx",
			Name:  "web",
			Ports: []string{"80:80"},
		})

		config := builder.config
		assert.Len(t, config.Targets, 1)
		assert.Len(t, config.Jobs, 1)
		assert.Len(t, config.Jobs[0].Steps, 3)
		assert.NotNil(t, config.Jobs[0].Steps[0].Run)
		assert.NotNil(t, config.Jobs[0].Steps[1].Copy)
		assert.NotNil(t, config.Jobs[0].Steps[2].Docker)
	})
}

func TestBuilderPrint(t *testing.T) {
	t.Run("printing empty config", func(t *testing.T) {
		builder := NewBuilder()
		// Redirect stdout to capture output
		stdout := captureOutput(t, func() {
			err := builder.Print()
			assert.NoError(t, err)
		})
		assert.Equal(t, `{"targets":null,"jobs":null}`, strings.TrimSpace(stdout))
	})

	t.Run("printing config with target", func(t *testing.T) {
		builder := NewBuilder()
		target := &Target{
			Name:     "test-server",
			Host:     "localhost",
			User:     "testuser",
			Password: "secret",
			Port:     22,
		}
		builder.AddTarget(target)

		stdout := captureOutput(t, func() {
			err := builder.Print()
			assert.NoError(t, err)
		})

		expected := `{"targets":[{"name":"test-server","host":"localhost","user":"testuser","password":"secret","port":22}],"jobs":null}`
		assert.Equal(t, expected, strings.TrimSpace(stdout))
	})
}

// Helper function to capture stdout
func captureOutput(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatal(err)
	}

	return buf.String()
}
