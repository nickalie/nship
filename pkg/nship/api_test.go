package nship

import (
	"github.com/nickalie/nship/internal/config"
	"strings"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	if builder == nil {
		t.Fatal("NewBuilder() returned nil")
	}

	// Check that the builder can create a config
	config := builder.GetConfig()
	if config == nil {
		t.Error("Builder returned nil config")
	}
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
	if len(cfg.Targets) != 1 || cfg.Targets[0].Name != "test-target" {
		t.Error("Target was not correctly added to config")
	}

	// Verify job
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "test-job" {
		t.Error("Job was not correctly added to config")
	}

	// Verify steps
	if len(cfg.Jobs[0].Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(cfg.Jobs[0].Steps))
	}

	// Verify run step
	if cfg.Jobs[0].Steps[0].Run != "echo hello" {
		t.Errorf("Expected run step command to be 'echo hello', got %s", cfg.Jobs[0].Steps[0].Run)
	}

	// Verify copy step
	if cfg.Jobs[0].Steps[1].Copy == nil || cfg.Jobs[0].Steps[1].Copy.Src != "src" || cfg.Jobs[0].Steps[1].Copy.Dst != "dst" {
		t.Error("Copy step was not correctly added")
	}

	// Verify docker step
	if cfg.Jobs[0].Steps[2].Docker == nil || cfg.Jobs[0].Steps[2].Docker.Image != "nginx" || cfg.Jobs[0].Steps[2].Docker.Name != "web" {
		t.Error("Docker step was not correctly added")
	}
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
	var _ *config.Config = apiConfig

	// Also check that the builder is an alias
	apiBuilder := NewBuilder()
	var _ *config.Builder = apiBuilder

	// This test will pass if it compiles
	t.Log("API type aliases correctly reference internal types")
}

func TestLoadConfig(t *testing.T) {
	// This is a basic test to ensure LoadConfig doesn't panic
	// A full test would require creating a test config file

	// We expect this to fail since the file doesn't exist
	cfg, err := LoadConfig("nonexistent-config.yaml")

	// Check that we got an error and not a panic
	if err == nil {
		t.Error("Expected error when loading nonexistent config file, got nil")
	}

	// Config should be nil
	if cfg != nil {
		t.Errorf("Expected nil config when loading fails, got %v", cfg)
	}
}

func TestRun(t *testing.T) {
	// This test verifies that the Run function calls cli.Run without errors
	// We can't fully test the behavior, but we can ensure it doesn't panic

	// Call with nonexistent file - should return error but not panic
	err := Run("nonexistent-config.yaml", "", nil, "")

	// Check that we got an error and not a panic
	if err == nil {
		t.Error("Expected error when running with nonexistent config, got nil")
	}
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
	if err == nil {
		t.Error("Expected error when executing the job (since no real connection), got nil")
	}

	// Test with a job name that doesn't exist
	err = RunConfig(cfg, "nonexistent-job")

	// Should get a "job not found" error
	if err == nil {
		t.Error("Expected 'job not found' error, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'job not found' error, got: %v", err)
	}
}
