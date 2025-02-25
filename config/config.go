// Package config provides functionality for creating and managing deployment configurations.
// It defines structures and methods for handling deployment targets, jobs, and steps,
// supporting both local and remote operations through various protocols including SSH and SFTP.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Config represents the main deployment configuration structure containing
// targets and jobs definitions.
type Config struct {
	Targets []*Target `yaml:"targets" json:"targets" validate:"required,dive"`
	Jobs    []*Job    `yaml:"jobs" json:"jobs" validate:"required,dive"`
}

// Target defines a deployment destination with connection details.
type Target struct {
	Name       string `yaml:"name" json:"name" validate:"omitempty"`
	Host       string `yaml:"host" json:"host" validate:"required,hostname|ip"`
	User       string `yaml:"user" json:"user" validate:"required"`
	Password   string `yaml:"password" json:"password" validate:"required_without=PrivateKey"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty" validate:"required_without=Password,omitempty,file"`
	Port       int    `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
}

// Job represents a collection of steps to be executed on targets.
type Job struct {
	Name  string  `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`
	Steps []*Step `yaml:"steps" json:"steps" validate:"required,dive"`
}

// Step defines a single deployment action that can be either
// a command execution, file copy operation, or Docker operation.
type Step struct {
	Run    string      `yaml:"run,omitempty" json:"run,omitempty" validate:"required_without_all=Copy Shell Docker"`
	Copy   *CopyStep   `yaml:"copy,omitempty" json:"copy,omitempty" validate:"required_without_all=Run Shell Docker"`
	Shell  string      `yaml:"shell,omitempty" json:"shell,omitempty"`
	Docker *DockerStep `yaml:"docker,omitempty" json:"docker,omitempty" validate:"required_without_all=Run Copy Shell"`
}

// DockerStep defines Docker container configuration and execution parameters.
type DockerStep struct {
	Image       string            `yaml:"image" json:"image" validate:"required"`
	Name        string            `yaml:"name" json:"name" validate:"required"`
	Environment map[string]string `yaml:"environment" json:"environment" validate:"omitempty"`
	Ports       []string          `yaml:"ports" json:"ports" validate:"omitempty,dive,required"`
	Volumes     []string          `yaml:"volumes" json:"volumes" validate:"omitempty,dive,required"`
	Labels      map[string]string `yaml:"labels" json:"labels" validate:"omitempty"`
	Networks    []string          `yaml:"networks" json:"networks" validate:"omitempty,dive,required"`
	Commands    []string          `yaml:"commands" json:"commands" validate:"omitempty,dive,required"`
	Restart     string            `yaml:"restart" json:"restart" validate:"omitempty,oneof=no on-failure always unless-stopped"`
}

// CopyStep defines source and destination paths for file copy operations.
type CopyStep struct {
	Src     string   `yaml:"src" json:"src" validate:"required"`
	Dst     string   `yaml:"dst" json:"dst" validate:"required"`
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty" validate:"omitempty,dive,required"`
}

var validate = validator.New()

// LoadConfig loads and validates configuration from the specified path.
func LoadConfig(configPath string) (*Config, error) {
	config, err := loadConfigByExtension(configPath)
	if err != nil {
		return nil, err
	}

	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func loadConfigByExtension(configPath string) (*Config, error) {
	ext := strings.ToLower(filepath.Ext(configPath))

	loaders := map[string]func(string) (*Config, error){
		".yaml": loadYAMLConfig,
		".yml":  loadYAMLConfig,
		".ts":   loadTypeScriptConfig,
		".js":   loadJavaScriptConfig,
		".mjs":  loadJavaScriptConfig,
		".go":   loadGolangConfig,
	}

	loader, ok := loaders[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}

	return loader(configPath)
}

func validateConfig(config *Config) error {
	if err := validate.Struct(config); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("config validation failed: %s", formatValidationErrors(validationErrors))
		}
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}

func loadGolangConfig(configPath string) (*Config, error) {
	return loadCmdConfig("", "go", "run", configPath)
}

func formatValidationErrors(errs validator.ValidationErrors) string {
	errMsgs := make([]string, 0, len(errs))
	for index, err := range errs {
		errMsgs[index] = fmt.Sprintf(
			"Field '%s' failed validation: %s (condition: %s)",
			err.Field(),
			err.Tag(),
			err.Param(),
		)
	}
	return strings.Join(errMsgs, "\n")
}

// loadYAMLConfig loads configuration from YAML file
func loadYAMLConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	dataStr := replaceEnvVariables(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(dataStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

func loadTypeScriptConfig(configPath string) (*Config, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "tsconfig")

	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{\"type\":\"module\"}"), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create package.json: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	jsFile := filepath.Join(tmpDir, "config.js")
	// Build TypeScript file using esbuild
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{configPath},
		Bundle:      true,
		Platform:    api.PlatformNode,
		Format:      api.FormatESModule,
		Write:       true,
		Outfile:     jsFile,
	})

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("failed to build TypeScript: %v", result.Errors[0].Text)
	}

	// Execute compiled JavaScript
	return loadJavaScriptConfig(jsFile)
}

func loadJavaScriptConfig(configPath string) (*Config, error) {
	return loadCmdConfig(
		filepath.Dir(configPath),
		"node",
		"-e",
		fmt.Sprintf(
			"(async ()=>{const m=await import(\"./%s\");console.log(JSON.stringify(typeof m.default==='function'?await m.default():m.default));})();", //nolint:lll //single line js code to wrap compiled TypeScript
			filepath.Base(configPath),
		),
	)
}

func loadCmdConfig(dir string, args ...string) (*Config, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", nodeError(err), string(output))
	}

	parts := strings.Split(string(output), "\n")

	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid output config cmd output")
	}

	outputStr := parts[len(parts)-2]

	var config Config

	if err := json.Unmarshal([]byte(outputStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config output: %w", err)
	}

	return &config, nil
}

func replaceEnvVariables(content string) string {
	re := regexp.MustCompile(`\${(\w+)}`)
	return re.ReplaceAllStringFunc(content, func(s string) string {
		key := re.FindStringSubmatch(s)[1]
		return os.Getenv(key)
	})
}

func nodeError(err error) error {
	if strings.Contains(err.Error(), "executable file not found") {
		return errors.New(
			"NodeJS (https://nodejs.org/) is not installed or not in PATH. NodeJS is required to load JavaScript and TypeScript configuration files",
		)
	}

	return err
}
