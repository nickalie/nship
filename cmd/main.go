package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/nickalie/ngdeploy/config"
	"github.com/nickalie/ngdeploy/pkg/job"
	"github.com/sosedoff/ansible-vault-go"
	"golang.org/x/term"
)

type godotenvWrapper struct{}

func (g *godotenvWrapper) Load(filename string) error {
	return godotenv.Load(filename)
}

func (g *godotenvWrapper) Unmarshal(text string) (map[string]string, error) {
	return godotenv.Unmarshal(text)
}

type vaultWrapper struct{}

func (v *vaultWrapper) Decrypt(content, password string) (string, error) {
	return vault.Decrypt(content, password)
}

type EnvLoader interface {
	Load(filename string) error
	Unmarshal(text string) (map[string]string, error)
}

type VaultDecrypter interface {
	Decrypt(content, password string) (string, error)
}

type App struct {
	envLoader      EnvLoader
	vaultDecrypter VaultDecrypter
	config         *config.Config
	jobRunner      job.Runner
}

func NewApp() *App {
	return &App{
		envLoader:      &godotenvWrapper{},
		vaultDecrypter: &vaultWrapper{},
		jobRunner:      job.RunJob,
	}
}

func (a *App) Run(configPath, jobName string, envPaths []string, vaultPassword *string) error {
	if err := a.loadEnvironments(envPaths, vaultPassword); err != nil {
		return fmt.Errorf("environment loading failed: %w", err)
	}

	if err := a.loadConfig(configPath); err != nil {
		return fmt.Errorf("config loading failed: %w", err)
	}

	jobs, err := a.getJobsToRun(jobName)
	if err != nil {
		return fmt.Errorf("job selection failed: %w", err)
	}

	return a.executeJobs(jobs)
}

func (a *App) loadEnvironments(envPaths []string, vaultPassword *string) error {
	for _, path := range envPaths {
		if err := a.loadEnvironment(path, vaultPassword); err != nil {
			return fmt.Errorf("failed to load environment file %s: %w", path, err)
		}
	}
	return nil
}

func (a *App) loadEnvironment(envPath string, vaultPassword *string) error {
	if envPath == "" {
		return nil
	}

	if strings.HasSuffix(envPath, ".vault") {
		if vaultPassword == nil {
			return fmt.Errorf("vault password is required")
		}
		return a.loadVaultFile(envPath, *vaultPassword)
	}

	return a.envLoader.Load(envPath)
}

func (a *App) loadVaultFile(path, password string) error {
	if password == "" {
		return fmt.Errorf("vault password is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read vault file: %w", err)
	}

	decrypted, err := a.vaultDecrypter.Decrypt(string(data), password)
	if err != nil {
		return fmt.Errorf("vault decryption failed: %w", err)
	}

	envMap, err := a.envLoader.Unmarshal(decrypted)
	if err != nil {
		return fmt.Errorf("environment unmarshaling failed: %w", err)
	}

	for k, v := range envMap {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", k, err)
		}
	}

	return nil
}

func (a *App) loadConfig(path string) error {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return err
	}
	a.config = cfg
	return nil
}

func (a *App) getJobsToRun(jobName string) ([]*config.Job, error) {
	if jobName == "" {
		return a.config.Jobs, nil
	}

	for _, job := range a.config.Jobs {
		if job.Name == jobName {
			return []*config.Job{job}, nil
		}
	}
	return nil, fmt.Errorf("job '%s' not found", jobName)
}

func (a *App) executeJobs(jobs []*config.Job) error {
	for _, target := range a.config.Targets {
		if target.Name == "" {
			target.Name = target.Host
		}

		for index, job := range jobs {
			if job.Name == "" {
				job.Name = strconv.Itoa(index + 1)
			}

			fmt.Printf("Running job '%s' on target '%s'\n", job.Name, target.Name)

			if err := a.jobRunner(target, job); err != nil {
				return fmt.Errorf("job '%s' failed on target '%s': %w", job.Name, target.Name, err)
			}
			fmt.Printf("Job '%s' completed successfully on target '%s'\n", job.Name, target.Name)
		}
	}
	return nil
}

func promptVaultPassword() (string, error) {
	fmt.Print("Enter vault password: ")

	if password, err := term.ReadPassword(int(syscall.Stdin)); err == nil {
		fmt.Println()
		return string(password), nil
	}

	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadBytes('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	return strings.TrimSpace(string(password)), nil
}

func resolveVaultPassword(passwordFlag, envPaths string) (*string, error) {
	if passwordFlag != "" {
		return &passwordFlag, nil
	}

	if envVar := os.Getenv("VAULT_PASSWORD"); envVar != "" {
		return &envVar, nil
	}

	if !strings.Contains(envPaths, ".vault") {
		return new(string), nil
	}

	password, err := promptVaultPassword()
	if err != nil {
		return nil, fmt.Errorf("password prompt failed: %w", err)
	}
	return &password, nil
}

func main() {
	configPath := flag.String("config", "deploy.yaml", "Path to configuration file")
	jobName := flag.String("job", "", "Name of specific job to run")
	envPaths := flag.String("env", "", "Comma-separated paths to environment files")
	vaultPassword := flag.String("vault-password", "", "Password for Ansible Vault file")
	flag.Parse()

	paths := []string{}
	if *envPaths != "" {
		paths = strings.Split(*envPaths, ",")
	}

	password, err := resolveVaultPassword(*vaultPassword, strings.Join(paths, ""))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	app := NewApp()
	if err := app.Run(*configPath, *jobName, paths, password); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
