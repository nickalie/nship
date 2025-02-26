// Package nship provides a public API for deployment automation across multiple targets.
// It exposes simplified interfaces for configuration management, job execution, and
// deployment orchestration while hiding implementation details. This package allows
// users to integrate nship's deployment capabilities into their own applications
// or build custom deployment workflows on top of the core functionality.
package nship

import (
	"fmt"

	"github.com/nickalie/nship/internal/config"
	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/nickalie/nship/internal/infrastructure/ssh"
	"github.com/nickalie/nship/internal/platform/cli"
)

// Target represents a deployment target
type Target = target.Target

// Job represents a deployment job
type Job = job.Job

// Step represents a deployment step
type Step = job.Step

// DockerStep represents a Docker container configuration
type DockerStep = job.DockerStep

// CopyStep represents a file copy operation
type CopyStep = job.CopyStep

// Config represents a deployment configuration
type Config = config.Config

// Builder represents a configuration builder
type Builder = config.Builder

// NewBuilder creates a new configuration builder
func NewBuilder() *Builder {
	return config.NewBuilder()
}

// Run executes a deployment with the specified parameters
func Run(configPath, jobName string, envPaths []string, vaultPassword string) error {
	return cli.Run(configPath, jobName, envPaths, vaultPassword)
}

// LoadConfig loads a configuration file
func LoadConfig(configPath string) (*Config, error) {
	loader := config.NewLoader()
	return loader.Load(configPath)
}

// RunConfig executes the deployment based on the provided configuration
func RunConfig(cfg *Config, jobName string) error {
	var jobs []*job.Job
	var err error

	// Filter jobs by name if specified
	if jobName != "" {
		for _, j := range cfg.Jobs {
			if j.Name == jobName {
				jobs = []*job.Job{j}
				break
			}
		}
		if jobs == nil {
			return fmt.Errorf("job '%s' not found", jobName)
		}
	} else {
		jobs = cfg.Jobs
	}

	// Create service and execute jobs
	clientFactory := ssh.NewClientFactory()
	jobService := job.NewService(clientFactory)

	err = jobService.ExecuteJobs(cfg.Targets, jobs)
	if err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	return nil
}
