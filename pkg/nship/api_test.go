package nship

import (
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/assert"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	assert.NotNil(t, builder, "NewBuilder() should not return nil")

	// Check that the builder can create a config
	c := builder.GetConfig()
	assert.NotNil(t, c, "Builder returned nil config")
}

func TestBuilderFunctions(t *testing.T) {
	// Test that the builder functions work correctly
	builder := NewBuilder()

	// Create a target
	tgt := &Target{
		Name: "test-target",
		Host: "example.com",
		User: "admin",
	}

	// Add the target to the builder
	builder.AddTarget(tgt)

	// Create a job
	builder.AddJob("test-job")

	// Add steps to the job
	builder.AddRunStep("echo hello")

	// Create a copy step
	builder.AddCopyStep("src", "dst")

	// Create a docker step
	docker := &DockerStep{
		Image: "nginx",
		Name:  "web",
	}
	builder.AddDockerStep(docker)

	// Get the final config
	cfg := builder.GetConfig()

	// Verify target
	assert.Len(t, cfg.Targets, 1, "Expected 1 target")
	assert.Equal(t, "test-target", cfg.Targets[0].Name, "Target was not correctly added to config")

	// Verify job
	assert.Len(t, cfg.Jobs, 1, "Expected 1 job")
	assert.Equal(t, "test-job", cfg.Jobs[0].Name, "Job was not correctly added to config")

	// Verify steps
	assert.Len(t, cfg.Jobs[0].Steps, 3, "Expected 3 steps")

	// Verify run step
	assert.Equal(t, "echo hello", cfg.Jobs[0].Steps[0].Run, "Run step was not correctly added")

	// Verify copy step
	assert.NotNil(t, cfg.Jobs[0].Steps[1].Copy, "Copy step was not correctly added")
	assert.Equal(t, "src", cfg.Jobs[0].Steps[1].Copy.Src, "Copy step source was not correctly set")
	assert.Equal(t, "dst", cfg.Jobs[0].Steps[1].Copy.Dst, "Copy step destination was not correctly set")

	// Verify docker step
	assert.NotNil(t, cfg.Jobs[0].Steps[2].Docker, "Docker step was not correctly added")
	assert.Equal(t, "nginx", cfg.Jobs[0].Steps[2].Docker.Image, "Docker step image was not correctly set")
	assert.Equal(t, "web", cfg.Jobs[0].Steps[2].Docker.Name, "Docker step container name was not correctly set")
}

func TestTypeAliases(t *testing.T) {
	// Test that the type aliases refer to the right types

	// Create instances of each type from both the api and internal packages
	apiTarget := &Target{Name: "test", Host: "example.com", User: "admin"}
	apiJob := &Job{Name: "test-job", Steps: []*Step{{Run: "echo test"}}}
	apiStep := &Step{Run: "echo test"}
	apiDockerStep := &DockerStep{Image: "nginx", Name: "web"}
	apiCopyStep := &CopyStep{Src: "src", Dst: "dst"}
	apiConfig := &Config{
		Targets: []*Target{apiTarget},
		Jobs:    []*Job{apiJob},
	}

	// Verify that the API types are indeed aliases to the internal types
	// This is a compile-time check, if it builds, the types are correct
	var _ *target.Target = apiTarget
	var _ *job.Job = apiJob
	var _ *job.Step = apiStep
	var _ *job.DockerStep = apiDockerStep
	var _ *job.CopyStep = apiCopyStep

	// Also check that the builder is exposed correctly
	apiBuilder := NewBuilder()

	// Type is expected to match the appropriate internal type
	assert.NotNil(t, apiBuilder, "Builder should not be nil")
	assert.NotNil(t, apiConfig, "Config should not be nil")
}

func TestLoadConfig(t *testing.T) {
	// This is a basic test to ensure LoadConfig doesn't panic
	// A full test would require creating a test config file

	// We expect this to fail since the file doesn't exist
	cfg, err := LoadConfig("nonexistent-config.yaml")

	// Check that we got an error and not a panic
	assert.Error(t, err, "Expected error when loading nonexistent config file")
	assert.Nil(t, cfg, "Expected nil config when loading fails")
}

func TestRun(t *testing.T) {
	// This test verifies that the Run function calls cli.Run without errors
	// We can't fully test the behavior, but we can ensure it doesn't panic

	// Call with nonexistent file - should return error but not panic
	err := Run("nonexistent-config.yaml", "", nil, "")

	// Check that we got an error and not a panic
	assert.Error(t, err, "Expected error when running with nonexistent config")
}

func TestRunConfig(t *testing.T) {
	// Create a minimal valid config
	cfg := &Config{
		Targets: []*Target{
			{
				Name: "test",
				Host: "localhost",
				User: "user",
				// Need either Password or PrivateKey, but we can't actually connect
				Password: "pass",
			},
		},
		Jobs: []*Job{
			{
				Name: "test-job",
				Steps: []*Step{
					{Run: "echo test"},
				},
			},
		},
	}

	// Call RunConfig with a specific job name that exists
	err := RunConfig(cfg, "test-job")

	// This will likely fail due to connection issues, but should not panic
	assert.Error(t, err, "Expected error when executing the job (since no real connection)")

	// Test with a job name that doesn't exist
	err = RunConfig(cfg, "nonexistent-job")

	// Should get a "job not found" error
	assert.Error(t, err, "Expected 'job not found' error")
	assert.Contains(t, err.Error(), "not found", "Error should indicate job not found")
}
