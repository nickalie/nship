package config

import (
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

func TestBuilder(t *testing.T) {
	builder := NewBuilder()

	// Add targets
	target1 := &target.Target{
		Name: "web-server",
		Host: "web.example.com",
		User: "admin",
		Port: 2222,
	}
	target2 := &target.Target{
		Host: "db.example.com",
		User: "admin",
	}

	builder.AddTarget(target1)
	builder.AddTarget(target2)

	// Add job with run steps
	builder.AddJob("setup")
	builder.AddRunStep("mkdir -p /var/www")
	builder.AddRunStep("chown www-data:www-data /var/www")

	// Add job with copy and docker steps
	dockerConfig := &job.DockerStep{
		Image: "nginx:latest",
		Name:  "web",
		Ports: []string{"80:80"},
	}

	builder.AddJob("deploy")
	builder.AddCopyStep("./config/nginx.conf", "/etc/nginx/nginx.conf")
	builder.AddDockerStep(dockerConfig)

	// Test the resulting config
	config := builder.GetConfig()

	// Verify targets
	if len(config.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(config.Targets))
	}
	if config.Targets[0].Name != "web-server" {
		t.Errorf("Expected first target name to be 'web-server', got %s", config.Targets[0].Name)
	}
	if config.Targets[1].Host != "db.example.com" {
		t.Errorf("Expected second target host to be 'db.example.com', got %s", config.Targets[1].Host)
	}

	// Verify jobs
	if len(config.Jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(config.Jobs))
	}

	// Verify first job (setup)
	setupJob := config.Jobs[0]
	if setupJob.Name != "setup" {
		t.Errorf("Expected first job name to be 'setup', got %s", setupJob.Name)
	}
	if len(setupJob.Steps) != 2 {
		t.Errorf("Expected setup job to have 2 steps, got %d", len(setupJob.Steps))
	}
	if setupJob.Steps[0].Run != "mkdir -p /var/www" {
		t.Errorf("Expected first step command to be 'mkdir -p /var/www', got %s", setupJob.Steps[0].Run)
	}

	// Verify second job (deploy)
	deployJob := config.Jobs[1]
	if deployJob.Name != "deploy" {
		t.Errorf("Expected second job name to be 'deploy', got %s", deployJob.Name)
	}
	if len(deployJob.Steps) != 2 {
		t.Errorf("Expected deploy job to have 2 steps, got %d", len(deployJob.Steps))
	}

	// Verify copy step
	copyStep := deployJob.Steps[0].Copy
	if copyStep == nil {
		t.Fatalf("Expected first step to be a copy step, but Copy is nil")
	}
	if copyStep.Local != "./config/nginx.conf" {
		t.Errorf("Expected copy step source to be './config/nginx.conf', got %s", copyStep.Local)
	}
	if copyStep.Remote != "/etc/nginx/nginx.conf" {
		t.Errorf("Expected copy step destination to be '/etc/nginx/nginx.conf', got %s", copyStep.Remote)
	}

	// Verify docker step
	dockerStep := deployJob.Steps[1].Docker
	if dockerStep == nil {
		t.Fatalf("Expected second step to be a docker step, but Docker is nil")
	}
	if dockerStep.Image != "nginx:latest" {
		t.Errorf("Expected docker image to be 'nginx:latest', got %s", dockerStep.Image)
	}
	if dockerStep.Name != "web" {
		t.Errorf("Expected docker container name to be 'web', got %s", dockerStep.Name)
	}
	if len(dockerStep.Ports) != 1 || dockerStep.Ports[0] != "80:80" {
		t.Errorf("Expected docker ports to be ['80:80'], got %v", dockerStep.Ports)
	}
}

func TestAddStep(t *testing.T) {
	builder := NewBuilder()
	builder.AddJob("test-job")

	customStep := &job.Step{
		Run:   "custom command",
		Shell: "bash",
	}

	builder.AddStep(customStep)

	config := builder.GetConfig()
	if len(config.Jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(config.Jobs))
	}

	job := config.Jobs[0]
	if len(job.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(job.Steps))
	}

	step := job.Steps[0]
	if step.Run != "custom command" {
		t.Errorf("Expected step command to be 'custom command', got %s", step.Run)
	}
	if step.Shell != "bash" {
		t.Errorf("Expected step shell to be 'bash', got %s", step.Shell)
	}
}
