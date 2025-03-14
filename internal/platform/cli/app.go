// Package cli provides the command-line interface functionality for the nship application.
// It handles the integration between user input, configuration loading, environment setup,
// and job execution orchestration. This package serves as the main entry point for the
// application's functionality when used as a CLI tool.
package cli

import (
	"fmt"

	"github.com/nickalie/nship/internal/config"
	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/nickalie/nship/internal/infrastructure/env"
	"github.com/nickalie/nship/internal/infrastructure/fs"
	"github.com/nickalie/nship/internal/infrastructure/ssh"
)

// EnvLoader defines the interface for loading environment variables
type EnvLoader interface {
	Load(path, vaultPassword string) error
}

// ConfigLoader defines the interface for loading configuration
type ConfigLoader interface {
	Load(configPath string) (*config.Config, error)
}

// JobService defines the interface for job execution
type JobService interface {
	ExecuteJobs(targets []*target.Target, jobs []*job.Job) error
}

// App represents the main application structure that handles
// configuration loading and job execution.
type App struct {
	envLoader    EnvLoader
	configLoader ConfigLoader
	jobService   JobService
}

// NewApp creates and returns a new App instance with default implementations
// for all dependencies.
func NewApp() *App {
	envLoader := env.NewLoader()
	configLoader := config.NewLoader()
	clientFactory := ssh.NewClientFactory()
	fileSystem := fs.NewFileSystem()
	jobService := job.NewService(clientFactory, job.WithFileSystem(fileSystem))

	return &App{
		envLoader:    envLoader,
		configLoader: configLoader,
		jobService:   jobService,
	}
}

// NewAppWithSkipUnchanged creates a new App instance with skip unchanged option
func NewAppWithSkipUnchanged(skipUnchanged bool) *App {
	envLoader := env.NewLoader()
	configLoader := config.NewLoader()
	clientFactory := ssh.NewClientFactory()
	hashStorage := fs.NewFileHashStorage()
	fileSystem := fs.NewFileSystem()

	jobService := job.NewService(
		clientFactory,
		job.WithHashStorage(hashStorage),
		job.WithSkipUnchanged(skipUnchanged),
		job.WithFileSystem(fileSystem),
	)

	return &App{
		envLoader:    envLoader,
		configLoader: configLoader,
		jobService:   jobService,
	}
}

// NewAppWithDeps creates and returns a new App instance with custom dependencies
func NewAppWithDeps(envLoader EnvLoader, configLoader ConfigLoader, jobService JobService) *App {
	return &App{
		envLoader:    envLoader,
		configLoader: configLoader,
		jobService:   jobService,
	}
}

// Run executes the application with the provided parameters (standard behavior without step skipping)
func Run(configPath, jobName string, envPaths []string, vaultPassword string) error {
	app := NewApp()
	return app.Run(configPath, jobName, envPaths, vaultPassword)
}

// RunWithSkipUnchanged executes the application with step skipping behavior
func RunWithSkipUnchanged(configPath, jobName string, envPaths []string, vaultPassword string, skipUnchanged bool) error {
	app := NewAppWithSkipUnchanged(skipUnchanged)
	return app.Run(configPath, jobName, envPaths, vaultPassword)
}

// RunWithOptions executes the application with the provided parameters and all options
func RunWithOptions(configPath, jobName string, envPaths []string, vaultPassword string, opts ...AppOption) error {
	app := NewAppWithOptions(opts...)
	return app.Run(configPath, jobName, envPaths, vaultPassword)
}

// AppOption is a function that modifies an App
type AppOption func(*App)

// WithSkipUnchanged returns an option that configures step skipping behavior
func WithSkipUnchanged(skipUnchanged bool) AppOption {
	return func(app *App) {
		hashStorage := fs.NewFileHashStorage()
		clientFactory := ssh.NewClientFactory()
		fileSystem := fs.NewFileSystem()
		app.jobService = job.NewService(
			clientFactory,
			job.WithHashStorage(hashStorage),
			job.WithSkipUnchanged(skipUnchanged),
			job.WithFileSystem(fileSystem),
		)
	}
}

// NewAppWithOptions creates a new App with the provided options
func NewAppWithOptions(opts ...AppOption) *App {
	app := NewApp()

	// Apply all options
	for _, opt := range opts {
		opt(app)
	}

	// Return the configured app
	return app
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

// GetJobService returns the job service for testing
func (a *App) GetJobService() JobService {
	return a.jobService
}

// GetConfigLoader returns the config loader for testing
func (a *App) GetConfigLoader() ConfigLoader {
	return a.configLoader
}

// GetEnvLoader returns the environment loader for testing
func (a *App) GetEnvLoader() EnvLoader {
	return a.envLoader
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
