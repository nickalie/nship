// Package cli provides the command-line interface functionality for the nship application.
// It handles the integration between user input, configuration loading, environment setup,
// and job execution orchestration. This package serves as the main entry point for the
// application's functionality when used as a CLI tool.
package cli

import (
	"fmt"

	"github.com/nickalie/nship/internal/config"
	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/infrastructure/env"
	"github.com/nickalie/nship/internal/infrastructure/ssh"
)

// App represents the main application structure that handles
// configuration loading and job execution.
type App struct {
	envLoader    env.Loader
	configLoader config.Loader
	jobService   *job.Service
}

// NewApp creates and returns a new App instance with default implementations
// for all dependencies.
func NewApp() *App {
	envLoader := env.NewLoader()
	configLoader := config.NewLoader()
	clientFactory := ssh.NewClientFactory()
	jobService := job.NewService(clientFactory)

	return &App{
		envLoader:    envLoader,
		configLoader: configLoader,
		jobService:   jobService,
	}
}

// Run executes the application with the provided parameters
func Run(configPath, jobName string, envPaths []string, vaultPassword string) error {
	return NewApp().Run(configPath, jobName, envPaths, vaultPassword)
}

// Run executes the application with the provided configuration, job name,
// environment paths, and vault password.
func (a *App) Run(configPath, jobName string, envPaths []string, vaultPassword string) error {
	// Load environment variables
	if err := a.loadEnvironments(envPaths, vaultPassword); err != nil {
		return fmt.Errorf("environment loading failed: %w", err)
	}

	// Load configuration
	cfg, err := a.configLoader.Load(configPath)
	if err != nil {
		return fmt.Errorf("config loading failed: %w", err)
	}

	// Get list of jobs to run
	jobs, err := a.getJobsToRun(cfg, jobName)
	if err != nil {
		return fmt.Errorf("job selection failed: %w", err)
	}

	// Execute jobs
	if err := a.jobService.ExecuteJobs(cfg.Targets, jobs); err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	return nil
}

// GetJobService returns the job service
func (a *App) GetJobService() *job.Service {
	return a.jobService
}

// loadEnvironments loads all environment files
func (a *App) loadEnvironments(envPaths []string, vaultPassword string) error {
	for _, path := range envPaths {
		if err := a.envLoader.Load(path, vaultPassword); err != nil {
			return fmt.Errorf("failed to load environment file %s: %w", path, err)
		}
	}
	return nil
}

// getJobsToRun determines which jobs to run based on the config and job name
func (a *App) getJobsToRun(cfg *config.Config, jobName string) ([]*job.Job, error) {
	if jobName == "" {
		return cfg.Jobs, nil
	}

	for _, j := range cfg.Jobs {
		if j.Name == jobName {
			return []*job.Job{j}, nil
		}
	}

	return nil, fmt.Errorf("job '%s' not found", jobName)
}
