package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nickalie/ngdeploy/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockEnvLoader struct {
	mock.Mock
}

func (m *MockEnvLoader) Load(filename string) error {
	args := m.Called(filename)
	return args.Error(0)
}

func (m *MockEnvLoader) Unmarshal(text string) (map[string]string, error) {
	args := m.Called(text)
	return args.Get(0).(map[string]string), args.Error(1)
}

type MockVaultDecrypter struct {
	mock.Mock
}

func (m *MockVaultDecrypter) Decrypt(content, password string) (string, error) {
	args := m.Called(content, password)
	return args.String(0), args.Error(1)
}

type MockJobRunner struct {
	mock.Mock
}

func (m *MockJobRunner) Run(target *config.Target, job *config.Job) error {
	args := m.Called(target, job)
	return args.Error(0)
}

func TestApp_Run(t *testing.T) {
	// Create test config file
	configContent := `
targets:
  - name: test-server
    host: localhost
    user: testuser
    password: testpass
jobs:
  - name: test-job
    steps:
      - run: echo "test"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		configPath    string
		jobName       string
		envPaths      []string
		vaultPassword string
		setupMocks    func(*MockEnvLoader, *MockVaultDecrypter, *MockJobRunner)
		expectedErr   string
	}{
		{
			name:       "successful run without env files",
			configPath: configPath,
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter, jr *MockJobRunner) {
				jr.On("Run", mock.Anything, mock.Anything).Return(nil)
			},
		},
		{
			name:       "successful run with multiple env files",
			configPath: configPath,
			envPaths:   []string{"file1.env", "file2.env"},
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter, jr *MockJobRunner) {
				el.On("Load", "file1.env").Return(nil)
				el.On("Load", "file2.env").Return(nil)
				jr.On("Run", mock.Anything, mock.Anything).Return(nil)
			},
		},
		{
			name:       "env file error",
			configPath: configPath,
			envPaths:   []string{"good.env", "bad.env"},
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter, jr *MockJobRunner) {
				el.On("Load", "good.env").Return(nil)
				el.On("Load", "bad.env").Return(fmt.Errorf("file not found"))
			},
			expectedErr: "environment loading failed",
		},
		{
			name:          "multiple vault files",
			configPath:    configPath,
			envPaths:      []string{"test1.vault", "test2.vault"},
			vaultPassword: "secret",
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter, jr *MockJobRunner) {
				vd.On("Decrypt", "encrypted content", "secret").Return("KEY=value", nil).Times(2)
				el.On("Unmarshal", "KEY=value").Return(map[string]string{"KEY": "value"}, nil).Times(2)
				jr.On("Run", mock.Anything, mock.Anything).Return(nil)
			},
		},
		// ... existing test cases ...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEnvLoader := new(MockEnvLoader)
			mockVaultDecrypter := new(MockVaultDecrypter)
			mockJobRunner := new(MockJobRunner)

			tt.setupMocks(mockEnvLoader, mockVaultDecrypter, mockJobRunner)

			app := &App{
				envLoader:      mockEnvLoader,
				vaultDecrypter: mockVaultDecrypter,
				jobRunner:      mockJobRunner.Run,
			}

			var password *string
			if tt.vaultPassword != "" {
				password = &tt.vaultPassword
			}

			// Create vault files if needed
			for _, path := range tt.envPaths {
				if strings.HasSuffix(path, ".vault") {
					if err := os.WriteFile(path, []byte("encrypted content"), 0644); err != nil {
						t.Fatal(err)
					}
					defer os.Remove(path)
				}
			}

			err := app.Run(tt.configPath, tt.jobName, tt.envPaths, password)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			mockEnvLoader.AssertExpectations(t)
			mockVaultDecrypter.AssertExpectations(t)
			mockJobRunner.AssertExpectations(t)
		})
	}
}

func TestResolveVaultPassword(t *testing.T) {
	tests := []struct {
		name          string
		passwordFlag  string
		envPaths      []string
		envVar        string
		expectedEmpty bool
		expectedErr   bool
	}{
		{
			name:         "password from flag",
			passwordFlag: "flagpass",
			envPaths:     []string{"test.vault", "other.env"},
		},
		{
			name:     "password from env var",
			envPaths: []string{"test.vault"},
			envVar:   "envpass",
		},
		{
			name:          "no password needed without vault files",
			envPaths:      []string{"test.env", "other.env"},
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				os.Setenv("VAULT_PASSWORD", tt.envVar)
				defer os.Unsetenv("VAULT_PASSWORD")
			}

			password, err := resolveVaultPassword(tt.passwordFlag, strings.Join(tt.envPaths, ""))

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, password)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, password)

				if tt.expectedEmpty {
					assert.Empty(t, *password)
				} else if tt.passwordFlag != "" {
					assert.Equal(t, tt.passwordFlag, *password)
				} else if tt.envVar != "" {
					assert.Equal(t, tt.envVar, *password)
				}
			}
		})
	}
}

func TestLoadVaultFile(t *testing.T) {
	tests := []struct {
		name         string
		vaultContent string
		password     string
		decrypted    string
		envVars      map[string]string
		setupMocks   func(*MockEnvLoader, *MockVaultDecrypter)
		expectedErr  string
	}{
		{
			name:         "successful vault decryption",
			vaultContent: "encrypted content",
			password:     "secret",
			decrypted:    "KEY=value",
			envVars:      map[string]string{"KEY": "value"},
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter) {
				vd.On("Decrypt", "encrypted content", "secret").Return("KEY=value", nil)
				el.On("Unmarshal", "KEY=value").Return(map[string]string{"KEY": "value"}, nil)
			},
		},
		{
			name:        "empty password",
			password:    "",
			expectedErr: "vault password is required",
			setupMocks:  func(el *MockEnvLoader, vd *MockVaultDecrypter) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEnvLoader := new(MockEnvLoader)
			mockVaultDecrypter := new(MockVaultDecrypter)

			tt.setupMocks(mockEnvLoader, mockVaultDecrypter)

			app := &App{
				envLoader:      mockEnvLoader,
				vaultDecrypter: mockVaultDecrypter,
			}

			tempFile, err := os.CreateTemp("", "vault-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tempFile.Name())

			if tt.vaultContent != "" {
				if _, err := tempFile.WriteString(tt.vaultContent); err != nil {
					t.Fatal(err)
				}
			}
			tempFile.Close()

			err = app.loadVaultFile(tempFile.Name(), tt.password)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				for k, v := range tt.envVars {
					assert.Equal(t, v, os.Getenv(k))
				}
			}

			mockEnvLoader.AssertExpectations(t)
			mockVaultDecrypter.AssertExpectations(t)
		})
	}
}

func TestGetJobsToRun(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		jobName     string
		wantJobs    int
		expectedErr string
	}{
		{
			name: "get specific job",
			config: &config.Config{
				Jobs: []*config.Job{
					{Name: "job1"},
					{Name: "job2"},
				},
			},
			jobName:  "job1",
			wantJobs: 1,
		},
		{
			name: "get all jobs",
			config: &config.Config{
				Jobs: []*config.Job{
					{Name: "job1"},
					{Name: "job2"},
				},
			},
			wantJobs: 2,
		},
		{
			name: "job not found",
			config: &config.Config{
				Jobs: []*config.Job{
					{Name: "job1"},
				},
			},
			jobName:     "nonexistent",
			expectedErr: "job 'nonexistent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{config: tt.config}

			jobs, err := app.getJobsToRun(tt.jobName)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, jobs, tt.wantJobs)
				if tt.jobName != "" {
					assert.Equal(t, tt.jobName, jobs[0].Name)
				}
			}
		})
	}
}

func TestExecuteJobs(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		jobs        []*config.Job
		setupMocks  func(*MockJobRunner)
		expectedErr string
	}{
		{
			name: "successful execution",
			config: &config.Config{
				Targets: []*config.Target{
					{Host: "host1", User: "user1"},
					{Host: "host2", User: "user2"},
				},
			},
			jobs: []*config.Job{
				{Name: "job1"},
				{Name: "job2"},
			},
			setupMocks: func(jr *MockJobRunner) {
				jr.On("Run", mock.Anything, mock.Anything).Return(nil).Times(4)
			},
		},
		{
			name: "execution failure",
			config: &config.Config{
				Targets: []*config.Target{
					{Host: "host1", User: "user1"},
				},
			},
			jobs: []*config.Job{
				{Name: "job1"},
			},
			setupMocks: func(jr *MockJobRunner) {
				jr.On("Run", mock.Anything, mock.Anything).Return(fmt.Errorf("execution failed"))
			},
			expectedErr: "execution failed",
		},
		{
			name: "target name defaulting",
			config: &config.Config{
				Targets: []*config.Target{
					{Host: "host1"},
				},
			},
			jobs: []*config.Job{
				{Name: "job1"},
			},
			setupMocks: func(jr *MockJobRunner) {
				jr.On("Run", mock.MatchedBy(func(target *config.Target) bool {
					return target.Name == "host1" && target.Host == "host1"
				}), mock.Anything).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockJobRunner := new(MockJobRunner)
			tt.setupMocks(mockJobRunner)

			app := &App{
				config:    tt.config,
				jobRunner: mockJobRunner.Run,
			}

			err := app.executeJobs(tt.jobs)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			mockJobRunner.AssertExpectations(t)
		})
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp()

	assert.NotNil(t, app)
	assert.NotNil(t, app.envLoader)
	assert.NotNil(t, app.vaultDecrypter)
	assert.NotNil(t, app.jobRunner)

	// Test type assertions
	_, ok := app.envLoader.(*godotenvWrapper)
	assert.True(t, ok)
	_, ok = app.vaultDecrypter.(*vaultWrapper)
	assert.True(t, ok)
}

func TestGodotenvWrapper(t *testing.T) {
	wrapper := &godotenvWrapper{}

	// Create a temporary .env file
	tmpFile, err := os.CreateTemp("", "env-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := "TEST_KEY=test_value"
	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test Load
	err = wrapper.Load(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "test_value", os.Getenv("TEST_KEY"))

	// Test Unmarshal
	envMap, err := wrapper.Unmarshal(content)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", envMap["TEST_KEY"])
}

func TestVaultWrapper(t *testing.T) {
	wrapper := &vaultWrapper{}
	encrypted := "$ANSIBLE_VAULT;1.1;AES256\n61366437333436..." // Use a valid encrypted content
	password := "secret"

	decrypted, err := wrapper.Decrypt(encrypted, password)
	// Due to vault dependency, we can only test the interface compliance
	assert.Error(t, err) // Expected error with invalid content
	assert.Empty(t, decrypted)
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		envVars     map[string]string
		expectedErr string
	}{
		{
			name: "successful load with env vars",
			content: `
targets:
  - host: ${TEST_HOST}
    user: ${TEST_USER}
    password: ${TEST_PASSWORD}
jobs:
  - name: test-job
    steps:
      - run: echo "test"
`,
			envVars: map[string]string{
				"TEST_HOST":     "localhost",
				"TEST_USER":     "testuser",
				"TEST_PASSWORD": "testpass",
			},
		},
		{
			name: "successful load with private key",
			content: `
targets:
  - host: ${TEST_HOST}
    user: ${TEST_USER}
    private_key: ${TEST_KEY}
jobs:
  - name: test-job
    steps:
      - run: echo "test"
`,
			envVars: map[string]string{
				"TEST_HOST": "localhost",
				"TEST_USER": "testuser",
			},
		},
		{
			name: "invalid yaml",
			content: `
invalid:
  - [
`,
			expectedErr: "failed to parse YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary private key file if needed
			if strings.Contains(tt.content, "private_key") {
				keyFile, err := os.CreateTemp("", "private-key-")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(keyFile.Name())
				tt.envVars["TEST_KEY"] = keyFile.Name()
			}

			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create temporary config file
			tmpFile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			if err := os.WriteFile(tmpFile.Name(), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			app := &App{}
			err = app.loadConfig(tmpFile.Name())

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app.config)
				assert.NotEmpty(t, app.config.Targets)
				assert.NotEmpty(t, app.config.Jobs)

				target := app.config.Targets[0]
				assert.Equal(t, tt.envVars["TEST_HOST"], target.Host)
				assert.Equal(t, tt.envVars["TEST_USER"], target.User)

				if tt.envVars["TEST_PASSWORD"] != "" {
					assert.Equal(t, tt.envVars["TEST_PASSWORD"], target.Password)
				}
				if tt.envVars["TEST_KEY"] != "" {
					assert.Equal(t, tt.envVars["TEST_KEY"], target.PrivateKey)
				}
			}
		})
	}
}

func TestLoadEnvironment(t *testing.T) {
	password := "secret"
	tests := []struct {
		name          string
		envPath       string
		vaultPassword *string
		setupMocks    func(*MockEnvLoader, *MockVaultDecrypter)
		expectedErr   string
	}{
		{
			name:          "vault file with password",
			envPath:       "test.vault",
			vaultPassword: &password,
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter) {
				vd.On("Decrypt", "encrypted content", password).Return("KEY=value", nil)
				el.On("Unmarshal", "KEY=value").Return(map[string]string{"KEY": "value"}, nil)
				// Remove this line since we're testing the vault path directly
				// el.On("Load", mock.AnythingOfType("string")).Return(nil)
			},
		},
		{
			name:          "vault file without password",
			envPath:       "test.vault",
			vaultPassword: nil,
			expectedErr:   "vault password is required",
			setupMocks:    func(el *MockEnvLoader, vd *MockVaultDecrypter) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEnvLoader := new(MockEnvLoader)
			mockVaultDecrypter := new(MockVaultDecrypter)

			if tt.setupMocks != nil {
				tt.setupMocks(mockEnvLoader, mockVaultDecrypter)
			}

			app := &App{
				envLoader:      mockEnvLoader,
				vaultDecrypter: mockVaultDecrypter,
			}

			var testPath string
			if strings.HasSuffix(tt.envPath, ".vault") && tt.vaultPassword != nil {
				tmpFile, err := os.CreateTemp("", "vault-*.vault")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpFile.Name())

				if err := os.WriteFile(tmpFile.Name(), []byte("encrypted content"), 0644); err != nil {
					t.Fatal(err)
				}
				testPath = tmpFile.Name()

				// Update the mock expectations to use the actual file path and content
				mockVaultDecrypter.ExpectedCalls = nil
				mockVaultDecrypter.On("Decrypt", "encrypted content", *tt.vaultPassword).Return("KEY=value", nil)
			} else {
				testPath = tt.envPath
			}

			err := app.loadEnvironment(testPath, tt.vaultPassword)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			mockEnvLoader.AssertExpectations(t)
			mockVaultDecrypter.AssertExpectations(t)
		})
	}
}

func TestLoadVaultFile_Errors(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		password    string
		fileContent string
		setupMocks  func(*MockEnvLoader, *MockVaultDecrypter)
		expectedErr string
	}{
		{
			name:        "empty password",
			path:        "test.vault",
			expectedErr: "vault password is required",
		},
		{
			name:        "file read error",
			path:        "nonexistent.vault",
			password:    "secret",
			expectedErr: "failed to read vault file",
		},
		{
			name:        "decrypt error",
			path:        "test.vault",
			password:    "secret",
			fileContent: "encrypted content",
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter) {
				vd.On("Decrypt", "encrypted content", "secret").Return("", fmt.Errorf("decrypt error"))
			},
			expectedErr: "vault decryption failed",
		},
		{
			name:        "unmarshal error",
			path:        "test.vault",
			password:    "secret",
			fileContent: "encrypted content",
			setupMocks: func(el *MockEnvLoader, vd *MockVaultDecrypter) {
				vd.On("Decrypt", "encrypted content", "secret").Return("decrypted content", nil)
				el.On("Unmarshal", "decrypted content").Return(map[string]string{}, fmt.Errorf("unmarshal error"))
			},
			expectedErr: "environment unmarshaling failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEnvLoader := new(MockEnvLoader)
			mockVaultDecrypter := new(MockVaultDecrypter)

			if tt.setupMocks != nil {
				tt.setupMocks(mockEnvLoader, mockVaultDecrypter)
			}

			app := &App{
				envLoader:      mockEnvLoader,
				vaultDecrypter: mockVaultDecrypter,
			}

			var path string
			if tt.fileContent != "" {
				tmpFile, err := os.CreateTemp("", "vault-")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpFile.Name())

				if err := os.WriteFile(tmpFile.Name(), []byte(tt.fileContent), 0644); err != nil {
					t.Fatal(err)
				}
				path = tmpFile.Name()
			} else {
				path = tt.path
			}

			err := app.loadVaultFile(path, tt.password)
			assert.ErrorContains(t, err, tt.expectedErr)

			mockEnvLoader.AssertExpectations(t)
			mockVaultDecrypter.AssertExpectations(t)
		})
	}
}

func TestPromptVaultPassword(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write test password to pipe
	go func() {
		defer w.Close()
		w.Write([]byte("testpassword\n"))
	}()

	// Test password prompt
	password, err := promptVaultPassword()
	assert.NoError(t, err)
	assert.Equal(t, "testpassword", password)
}
