package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nickalie/nship/internal/platform/cli"
)

func main() {
	// Define command-line flags
	configPath := flag.String("config", "nship.yaml", "Path to configuration file")
	jobName := flag.String("job", "", "Name of specific job to run")
	envPaths := flag.String("env", "", "Comma-separated paths to environment files")
	vaultPassword := flag.String("vault-password", "", "Password for Ansible Vault file")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	version := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version and exit if requested
	if *version {
		fmt.Println("nship version 1.0.0")
		os.Exit(0)
	}

	// Setup logging
	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(0)
	}

	// Parse environment file paths
	var envPathsList []string
	if *envPaths != "" {
		envPathsList = strings.Split(*envPaths, ",")
	}

	// Create and run application
	app := cli.NewApp()
	if err := app.Run(*configPath, *jobName, envPathsList, *vaultPassword); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
