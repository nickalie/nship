package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/sosedoff/ansible-vault-go"
	"os"
	"strings"

	"golang.org/x/term"
	"ngdeploy/config"
	"ngdeploy/pkg/job"
	"syscall"
)

func main() {
	configPath := flag.String("config", "deploy.yaml", "Path to YAML configuration file")
	jobName := flag.String("job", "", "Name of specific job to run (runs all jobs if not specified)")
	envPath := flag.String("env", "", "Path to .env file")
	vaultPassword := flag.String("vault-password", "", "Password for Ansible Vault file")
	flag.Parse()

	vaultPassword, err := getVaultPassword(*vaultPassword, *envPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := loadEnvFile(*envPath, *vaultPassword); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	jobsToRun, err := getJobsToRun(cfg, *jobName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	runJobs(cfg, jobsToRun)
}

func loadEnvFile(envPath, vaultPassword string) error {
	if envPath == "" {
		return nil
	}

	if strings.HasSuffix(envPath, ".vault") {
		return loadAnsibleVaultFile(envPath, vaultPassword)
	}

	return godotenv.Load(envPath)
}

func loadAnsibleVaultFile(envPath, vaultPassword string) error {
	if vaultPassword == "" {
		return fmt.Errorf("vault password is required for Ansible Vault file")
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("failed to read vault file: %w", err)
	}

	decrypted, err := vault.Decrypt(string(data), vaultPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt vault file: %w", err)
	}

	envMap, err := godotenv.Unmarshal(decrypted)
	if err != nil {
		return fmt.Errorf("failed to unmarshal decrypted vault file: %w", err)
	}

	for key, value := range envMap {
		os.Setenv(key, value)
	}

	return nil
}

func getJobsToRun(cfg *config.Config, jobName string) ([]config.Job, error) {
	if jobName == "" {
		return cfg.Jobs, nil
	}

	for _, j := range cfg.Jobs {
		if j.Name == jobName {
			return []config.Job{j}, nil
		}
	}
	return nil, fmt.Errorf("job '%s' not found in configuration", jobName)
}

func runJobs(cfg *config.Config, jobsToRun []config.Job) {
	for _, target := range cfg.Targets {

		if target.Name == "" {
			target.Name = target.Host
		}

		for _, j := range jobsToRun {
			fmt.Printf("Running job '%s' on target '%s'\n", j.Name, target.Name)
			if err := job.RunJob(target, j); err != nil {
				fmt.Printf("Error running job '%s' on target '%s': %v\n", j.Name, target.Name, err)
				continue
			}
			fmt.Printf("Job '%s' completed successfully on target '%s'\n", j.Name, target.Name)
		}
	}
}

func promptVaultPassword() (string, error) {
	fmt.Print("Enter vault password: ")

	// Try secure password input first
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err == nil {
		fmt.Println()
		return string(password), nil
	}

	// Fallback to standard input if terminal reading fails
	reader := bufio.NewReader(os.Stdin)
	password, err = reader.ReadBytes('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	// Trim whitespace and newline
	return strings.TrimSpace(string(password)), nil
}

func getVaultPassword(passwordFlag string, envPath string) (*string, error) {
	if passwordFlag != "" {
		return &passwordFlag, nil
	}

	if envVar := os.Getenv("VAULT_PASSWORD"); envVar != "" {
		return &envVar, nil
	}

	if !strings.HasSuffix(envPath, ".vault") {
		return new(string), nil
	}

	promptedPassword, err := promptVaultPassword()
	if err != nil {
		return nil, fmt.Errorf("error reading vault password: %w", err)
	}
	return &promptedPassword, nil
}
