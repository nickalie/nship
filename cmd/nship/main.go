// file: cmd/nship/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nickalie/nship/internal/platform/cli"
)

var revision = "latest"

// Application encapsulates the nship CLI application
type Application struct {
	configPath    string
	jobName       string
	envPaths      []string
	vaultPassword string
	noSkip        bool
	version       bool
	versionString string
	// Internal field to store default config paths
	defaultConfigPaths []string
}

// NewApplication creates a new Application instance with default values
func NewApplication() *Application {
	return &Application{
		configPath:         "nship.yaml",
		versionString:      revision,
		defaultConfigPaths: []string{"nship.yaml", "nship.yml"},
	}
}

// ParseFlags parses the command-line flags and updates the Application fields accordingly.
// It sets the configuration file path, job name, environment file paths, vault password,
// verbosity, and version flag based on the provided command-line arguments.
func (app *Application) ParseFlags() {
	flag.StringVar(&app.configPath, "config", app.configPath, "Path to configuration file")
	flag.StringVar(&app.jobName, "job", app.jobName, "Name of specific job to run")

	// Use only a callback function to process each env-file flag
	flag.Func("env-file", "Path to environment file (can be specified multiple times)", func(value string) error {
		// Handle both comma-separated values and multiple flag occurrences
		if value != "" {
			app.envPaths = append(app.envPaths, strings.Split(value, ",")...)
		}
		return nil
	})

	flag.StringVar(&app.vaultPassword, "vault-password", app.vaultPassword, "Password for Ansible Vault file")
	flag.BoolVar(&app.noSkip, "no-skip", app.noSkip, "Disable skipping unchanged steps")
	flag.BoolVar(&app.version, "version", app.version, "Show version information")

	flag.Parse()
}

// Run executes the application
func (app *Application) Run() error {
	// Show version and exit if requested
	if app.version {
		fmt.Printf("nship version %s\n", app.versionString)
		return nil
	}

	// Find the appropriate config path
	configPath := app.findConfigPath()

	// Execute the application with the determined config path
	return app.executeWithConfig(configPath)
}

// findConfigPath determines which configuration file to use
func (app *Application) findConfigPath() string {
	// If user specified a config path directly, use that
	if app.configPath != app.defaultConfigPaths[0] {
		return app.configPath
	}

	// Try each default config path
	for _, configPath := range app.defaultConfigPaths {
		// Check if file exists
		_, err := os.Stat(configPath)
		if err == nil {
			// Found a config file, use it
			return configPath
		}
	}

	// If no config file found, use the first default path
	// This will likely lead to an error, but maintains backward compatibility
	return app.defaultConfigPaths[0]
}

// executeWithConfig runs the application with the given config path
func (app *Application) executeWithConfig(configPath string) error {
	if app.noSkip {
		cliApp := cli.NewApp()
		return cliApp.Run(configPath, app.jobName, app.envPaths, app.vaultPassword)
	}
	return cli.RunWithSkipUnchanged(configPath, app.jobName, app.envPaths, app.vaultPassword, !app.noSkip)
}

func main() {
	app := NewApplication()
	app.ParseFlags()

	if err := app.Run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
