// file: cmd/nship/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/nickalie/nship/internal/platform/cli"
)

// Application encapsulates the nship CLI application
type Application struct {
	configPath    string
	jobName       string
	envPaths      []string
	vaultPassword string
	noSkip        bool
	verbose       bool
	version       bool
	versionString string
}

// NewApplication creates a new Application instance with default values
func NewApplication() *Application {
	return &Application{
		configPath:    "nship.yaml",
		versionString: "1.0.0",
	}
}

// ParseFlags parses the command-line flags and updates the Application fields accordingly.
// It sets the configuration file path, job name, environment file paths, vault password,
// verbosity, and version flag based on the provided command-line arguments.
func (app *Application) ParseFlags() {
	flag.StringVar(&app.configPath, "config", app.configPath, "Path to configuration file")
	flag.StringVar(&app.jobName, "job", app.jobName, "Name of specific job to run")

	var envPathsStr string
	flag.StringVar(&envPathsStr, "env", "", "Comma-separated paths to environment files")
	flag.StringVar(&app.vaultPassword, "vault-password", app.vaultPassword, "Password for Ansible Vault file")
	flag.BoolVar(&app.noSkip, "no-skip", app.noSkip, "Disable skipping unchanged steps")
	flag.BoolVar(&app.verbose, "verbose", app.verbose, "Enable verbose logging")
	flag.BoolVar(&app.version, "version", app.version, "Show version information")

	flag.Parse()

	// Move this after flag.Parse() to access the parsed value
	if envPathsStr != "" {
		app.envPaths = strings.Split(envPathsStr, ",")
	}
}

// Run executes the application
func (app *Application) Run() error {
	// Show version and exit if requested
	if app.version {
		fmt.Printf("nship version %s\n", app.versionString)
		return nil
	}

	// Setup logging
	if app.verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(0)
	}

	// Create and run application
	cliApp := cli.NewApp()

	// If no-skip is enabled, use the standard cliApp
	if app.noSkip {
		return cliApp.Run(app.configPath, app.jobName, app.envPaths, app.vaultPassword)
	}
	return cli.RunWithSkipUnchanged(app.configPath, app.jobName, app.envPaths, app.vaultPassword, !app.noSkip)
}

func main() {
	app := NewApplication()
	app.ParseFlags()

	if err := app.Run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
