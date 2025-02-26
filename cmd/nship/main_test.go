package main

import (
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	// Save original os.Args and flag.CommandLine
	oldArgs := os.Args
	oldFlagCommandLine := flag.CommandLine

	defer func() {
		// Restore original values
		os.Args = oldArgs
		flag.CommandLine = oldFlagCommandLine
	}()

	tests := []struct {
		name         string
		args         []string
		wantConfig   string
		wantJob      string
		wantEnv      []string
		wantPassword string
		wantVerbose  bool
		wantVersion  bool
	}{
		{
			name:         "default values",
			args:         []string{"nship"},
			wantConfig:   "nship.yaml",
			wantJob:      "",
			wantEnv:      nil,
			wantPassword: "",
			wantVerbose:  false,
			wantVersion:  false,
		},
		{
			name:         "all flags set",
			args:         []string{"nship", "-config", "custom.yaml", "-job", "deploy", "-env", "prod.env", "-vault-password", "secret", "-verbose"},
			wantConfig:   "custom.yaml",
			wantJob:      "deploy",
			wantEnv:      []string{"prod.env"},
			wantPassword: "secret",
			wantVerbose:  true,
			wantVersion:  false,
		},
		{
			name:         "version flag",
			args:         []string{"nship", "-version"},
			wantConfig:   "nship.yaml", // Default value should still be set
			wantJob:      "",
			wantEnv:      nil,
			wantPassword: "",
			wantVerbose:  false,
			wantVersion:  true,
		},
		{
			name:         "multiple env files",
			args:         []string{"nship", "-env", "dev.env,prod.env,secrets.env"},
			wantConfig:   "nship.yaml",
			wantEnv:      []string{"dev.env", "prod.env", "secrets.env"},
			wantJob:      "",
			wantPassword: "",
			wantVerbose:  false,
			wantVersion:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag.CommandLine for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			// Set up the args for this test
			os.Args = tt.args

			// Create and initialize the app
			app := NewApplication()
			app.ParseFlags()

			// Check values
			if app.configPath != tt.wantConfig {
				t.Errorf("configPath = %v, want %v", app.configPath, tt.wantConfig)
			}

			if app.jobName != tt.wantJob {
				t.Errorf("jobName = %v, want %v", app.jobName, tt.wantJob)
			}

			if !reflect.DeepEqual(app.envPaths, tt.wantEnv) {
				t.Errorf("envPaths = %v, want %v", app.envPaths, tt.wantEnv)
			}

			if app.vaultPassword != tt.wantPassword {
				t.Errorf("vaultPassword = %v, want %v", app.vaultPassword, tt.wantPassword)
			}

			if app.verbose != tt.wantVerbose {
				t.Errorf("verbose = %v, want %v", app.verbose, tt.wantVerbose)
			}

			if app.version != tt.wantVersion {
				t.Errorf("version = %v, want %v", app.version, tt.wantVersion)
			}
		})
	}
}

func TestEnvPathsParsing(t *testing.T) {
	tests := []struct {
		name      string
		envPaths  string
		wantPaths []string
	}{
		{
			name:      "empty env paths",
			envPaths:  "",
			wantPaths: nil,
		},
		{
			name:      "single env path",
			envPaths:  "dev.env",
			wantPaths: []string{"dev.env"},
		},
		{
			name:      "multiple env paths",
			envPaths:  "dev.env,prod.env,secrets.env",
			wantPaths: []string{"dev.env", "prod.env", "secrets.env"},
		},
		{
			name:      "path with spaces",
			envPaths:  "dev.env, prod.env",
			wantPaths: []string{"dev.env", " prod.env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the app
			app := NewApplication()

			// Set the env paths directly
			if tt.envPaths != "" {
				app.envPaths = strings.Split(tt.envPaths, ",")
			}

			if !reflect.DeepEqual(app.envPaths, tt.wantPaths) {
				t.Errorf("envPathsList = %v, want %v", app.envPaths, tt.wantPaths)
			}
		})
	}
}

func TestApplicationRun(t *testing.T) {
	tests := []struct {
		name        string
		app         *Application
		expectError bool
	}{
		{
			name: "version flag set",
			app: &Application{
				version:       true,
				versionString: "1.0.0-test",
			},
			expectError: false,
		},
		// Note: For more comprehensive tests, we would need to mock the cli.App
		// which would require refactoring the Run method to accept a cli.App factory
		// or implement a function to override the cli.NewApp for testing.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.app.Run()

			if (err != nil) != tt.expectError {
				t.Errorf("app.Run() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// TestMain tests the main function
// Note: Testing main directly is challenging, so we test the Application struct instead
func TestMainComponents(t *testing.T) {
	// Create an application with test version flag
	app := &Application{
		version:       true,
		versionString: "test-version",
	}

	// This should just print the version and return
	err := app.Run()
	if err != nil {
		t.Errorf("Expected no error for version flag, got: %v", err)
	}

	// Testing error cases would require mocking the cli.App dependency
}
