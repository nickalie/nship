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
	"github.com/nickalie/nship/internal/infrastructure/fs"
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

// HashStorage represents a storage for step hashes
type HashStorage = job.HashStorage

// Builder represents a configuration builder
type Builder = config.Builder

// NewBuilder creates a new configuration builder
func NewBuilder() *Builder {
	return config.NewBuilder()
}

// NewFileHashStorage creates a new file-based hash storage
func NewFileHashStorage() HashStorage {
	return fs.NewFileHashStorage()
}

// Run executes a deployment with the specified parameters (with default behavior)
func Run(configPath, jobName string, envPaths []string, vaultPassword string) error {
	return cli.Run(configPath, jobName, envPaths, vaultPassword)
}

// RunWithSkipUnchanged executes a deployment with the specified parameters
// and controls whether unchanged steps should be skipped
func RunWithSkipUnchanged(configPath, jobName string, envPaths []string, vaultPassword string, skipUnchanged bool) error {
	return cli.RunWithSkipUnchanged(configPath, jobName, envPaths, vaultPassword, skipUnchanged)
}

// LoadConfig loads a configuration file
func LoadConfig(configPath string) (*Config, error) {
	loader := config.NewLoader()
	return loader.Load(configPath)
}

// RunConfigWithOptions executes the deployment with options for skipping unchanged steps
func RunConfigWithOptions(cfg *Config, jobName string, skipUnchanged bool, hashStorage HashStorage) error {
	return runConfigInternal(cfg, jobName, skipUnchanged, hashStorage)
}

// RunConfig executes the deployment (runs all steps regardless of change status)
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
	return runConfigInternal(cfg, jobName, false, nil)
}

// runConfigInternal is the internal implementation of RunConfig and RunConfigWithOptions
func runConfigInternal(cfg *Config, jobName string, skipUnchanged bool, hashStorage HashStorage) error {
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

	clientFactory := ssh.NewClientFactory()
	jobService := job.NewService(clientFactory, job.WithSkipUnchanged(skipUnchanged),
		job.WithHashStorage(hashStorage))
	err = jobService.ExecuteJobs(cfg.Targets, jobs)
	if err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	return nil
}
