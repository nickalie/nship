package cli

import (
	"errors"
	"testing"

	"github.com/nickalie/nship/internal/config"
	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEnvLoader implements EnvLoader for testing
type MockEnvLoader struct {
	mock.Mock
}

func (m *MockEnvLoader) Load(path, vaultPassword string) error {
	args := m.Called(path, vaultPassword)
	return args.Error(0)
}

// MockConfigLoader implements ConfigLoader for testing
type MockConfigLoader struct {
	mock.Mock
}

func (m *MockConfigLoader) Load(configPath string) (*config.Config, error) {
	args := m.Called(configPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*config.Config), args.Error(1)
}

// MockJobService implements JobService for testing
type MockJobService struct {
	mock.Mock
}

func (m *MockJobService) ExecuteJobs(targets []*target.Target, jobs []*job.Job) error {
	args := m.Called(targets, jobs)
	return args.Error(0)
}

func TestApp_Run(t *testing.T) {
	tests := []struct {
		name          string
		configPath    string
		jobName       string
		envPaths      []string
		vaultPassword string
		setupMocks    func(*MockEnvLoader, *MockConfigLoader, *MockJobService)
		wantErr       bool
		errContains   string
	}{
		{
			name:       "successful run",
			configPath: "config.yaml",
			jobName:    "",
			envPaths:   []string{},
			setupMocks: func(envLoader *MockEnvLoader, configLoader *MockConfigLoader, jobService *MockJobService) {
				// Setup default config
				defaultConfig := &config.Config{
					Targets: []*target.Target{
						{Name: "default-target", Host: "localhost", User: "user"},
					},
					Jobs: []*job.Job{
						{Name: "default-job", Steps: []*job.Step{{Run: "echo test"}}},
					},
				}

				configLoader.On("Load", "config.yaml").Return(defaultConfig, nil)
				jobService.On("ExecuteJobs", defaultConfig.Targets, defaultConfig.Jobs).Return(nil)
			},
			wantErr: false,
		},
		{
			name:       "environment loading error",
			configPath: "config.yaml",
			jobName:    "",
			envPaths:   []string{"env.yaml"},
			setupMocks: func(envLoader *MockEnvLoader, configLoader *MockConfigLoader, jobService *MockJobService) {
				envLoader.On("Load", "env.yaml", "").Return(errors.New("env load error"))
			},
			wantErr:     true,
			errContains: "environment loading failed",
		},
		{
			name:       "config loading error",
			configPath: "invalid-config.yaml",
			jobName:    "",
			envPaths:   []string{},
			setupMocks: func(envLoader *MockEnvLoader, configLoader *MockConfigLoader, jobService *MockJobService) {
				configLoader.On("Load", "invalid-config.yaml").Return(nil, errors.New("config load error"))
			},
			wantErr:     true,
			errContains: "config loading failed",
		},
		{
			name:       "job not found",
			configPath: "config.yaml",
			jobName:    "nonexistent-job",
			envPaths:   []string{},
			setupMocks: func(envLoader *MockEnvLoader, configLoader *MockConfigLoader, jobService *MockJobService) {
				// Setup config with no matching job
				cfg := &config.Config{
					Targets: []*target.Target{
						{Name: "default-target", Host: "localhost", User: "user"},
					},
					Jobs: []*job.Job{
						{Name: "different-job", Steps: []*job.Step{{Run: "echo test"}}},
					},
				}
				configLoader.On("Load", "config.yaml").Return(cfg, nil)
			},
			wantErr:     true,
			errContains: "job selection failed",
		},
		{
			name:       "job execution error",
			configPath: "config.yaml",
			jobName:    "",
			envPaths:   []string{},
			setupMocks: func(envLoader *MockEnvLoader, configLoader *MockConfigLoader, jobService *MockJobService) {
				// Setup config
				cfg := &config.Config{
					Targets: []*target.Target{
						{Name: "default-target", Host: "localhost", User: "user"},
					},
					Jobs: []*job.Job{
						{Name: "default-job", Steps: []*job.Step{{Run: "echo test"}}},
					},
				}
				configLoader.On("Load", "config.yaml").Return(cfg, nil)
				jobService.On("ExecuteJobs", cfg.Targets, cfg.Jobs).Return(errors.New("job execution error"))
			},
			wantErr:     true,
			errContains: "job execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockEnvLoader := new(MockEnvLoader)
			mockConfigLoader := new(MockConfigLoader)
			mockJobService := new(MockJobService)

			// Setup mocks
			tt.setupMocks(mockEnvLoader, mockConfigLoader, mockJobService)

			// Create app with mocked dependencies
			app := NewAppWithDeps(mockEnvLoader, mockConfigLoader, mockJobService)

			// Run the test
			err := app.Run(tt.configPath, tt.jobName, tt.envPaths, tt.vaultPassword)

			// Verify results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			mockEnvLoader.AssertExpectations(t)
			mockConfigLoader.AssertExpectations(t)
			mockJobService.AssertExpectations(t)
		})
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	assert.NotNil(t, app, "NewApp() returned nil")

	// Check that all dependencies are initialized
	assert.NotNil(t, app.envLoader, "App has nil envLoader")
	assert.NotNil(t, app.configLoader, "App has nil configLoader")
	assert.NotNil(t, app.jobService, "App has nil jobService")
}

func TestGetJobsToRun(t *testing.T) {
	allJobs := []*job.Job{
		{Name: "job1", Steps: []*job.Step{{Run: "echo job1"}}},
		{Name: "job2", Steps: []*job.Step{{Run: "echo job2"}}},
		{Name: "job3", Steps: []*job.Step{{Run: "echo job3"}}},
	}

	cfg := &config.Config{
		Jobs: allJobs,
	}

	app := &App{}

	tests := []struct {
		name      string
		jobName   string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "all jobs",
			jobName:   "",
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "specific job",
			jobName:   "job2",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "nonexistent job",
			jobName:   "job4",
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, err := app.getJobsToRun(cfg, tt.jobName)

			if (err != nil) != tt.wantErr {
				t.Errorf("getJobsToRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(jobs) != tt.wantCount {
				t.Errorf("getJobsToRun() returned %d jobs, want %d", len(jobs), tt.wantCount)
			}

			if tt.jobName != "" && !tt.wantErr {
				if jobs[0].Name != tt.jobName {
					t.Errorf("getJobsToRun() returned job with name %s, want %s", jobs[0].Name, tt.jobName)
				}
			}
		})
	}
}

func TestLoadEnvironments(t *testing.T) {
	tests := []struct {
		name          string
		envPaths      []string
		vaultPassword string
		setupMock     func(*MockEnvLoader)
		wantErr       bool
	}{
		{
			name:     "no env files",
			envPaths: []string{},
			setupMock: func(m *MockEnvLoader) {
				// No expectations needed
			},
			wantErr: false,
		},
		{
			name:     "successful env loading",
			envPaths: []string{"env1.yaml", "env2.yaml"},
			setupMock: func(m *MockEnvLoader) {
				m.On("Load", "env1.yaml", "").Return(nil)
				m.On("Load", "env2.yaml", "").Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "env loading error",
			envPaths: []string{"env1.yaml", "error.yaml"},
			setupMock: func(m *MockEnvLoader) {
				m.On("Load", "env1.yaml", "").Return(nil)
				m.On("Load", "error.yaml", "").Return(errors.New("env load error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEnvLoader := new(MockEnvLoader)
			tt.setupMock(mockEnvLoader)

			app := &App{
				envLoader: mockEnvLoader,
			}

			err := app.loadEnvironments(tt.envPaths, tt.vaultPassword)

			if (err != nil) != tt.wantErr {
				t.Errorf("loadEnvironments() error = %v, wantErr %v", err, tt.wantErr)
			}

			mockEnvLoader.AssertExpectations(t)
		})
	}
}

func TestGetJobService(t *testing.T) {
	mockService := new(MockJobService)
	app := &App{
		jobService: mockService,
	}

	result := app.GetJobService()
	assert.Equal(t, mockService, result, "GetJobService() should return the jobService")
}

// TestGlobalRunFunction tests the global Run function indirectly
// by verifying that the NewApp() and app.Run() methods are called
func TestGlobalRunFunction(t *testing.T) {
	// Instead of trying to replace the global Run function (which can't be assigned to),
	// we can test it indirectly by creating a new app with mocked dependencies

	// Create mocks
	mockEnvLoader := new(MockEnvLoader)
	mockConfigLoader := new(MockConfigLoader)
	mockJobService := new(MockJobService)

	// Setup a test configuration
	testConfig := &config.Config{
		Targets: []*target.Target{
			{Name: "test-target", Host: "localhost", User: "user"},
		},
		Jobs: []*job.Job{
			{Name: "test-job", Steps: []*job.Step{{Run: "echo test"}}},
		},
	}

	// Setup expected behavior
	mockConfigLoader.On("Load", "config.yaml").Return(testConfig, nil)
	mockJobService.On("ExecuteJobs", testConfig.Targets, testConfig.Jobs).Return(nil)

	// Create app with mocked dependencies
	app := NewAppWithDeps(mockEnvLoader, mockConfigLoader, mockJobService)

	// Run the app directly (this is essentially what the global Run function does)
	err := app.Run("config.yaml", "", []string{}, "")

	// Verify
	assert.NoError(t, err)
	mockConfigLoader.AssertExpectations(t)
	mockJobService.AssertExpectations(t)

	// Note: We're not testing the global Run function directly,
	// but we're testing the core functionality it relies on
}
