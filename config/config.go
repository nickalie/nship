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

type Config struct {
	Targets []Target `yaml:"targets" json:"targets" validate:"required,dive"`
	Jobs    []Job    `yaml:"jobs" json:"jobs" validate:"required,dive"`
}

type Target struct {
	Name       string `yaml:"name" json:"name" validate:"required"`
	Host       string `yaml:"host" json:"host" validate:"required,hostname|ip"`
	User       string `yaml:"user" json:"user" validate:"required"`
	Password   string `yaml:"password" json:"password" validate:"required_without=PrivateKey"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty" validate:"required_without=Password,omitempty,file"`
	Port       int    `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
}

type Job struct {
	Name  string `yaml:"name" json:"name" validate:"required"`
	Steps []Step `yaml:"steps" json:"steps" validate:"required,dive"`
}

type Step struct {
	Run    string      `yaml:"run,omitempty" json:"run,omitempty" validate:"required_without_all=Copy Shell Docker"`
	Copy   *CopyStep   `yaml:"copy,omitempty" json:"copy,omitempty" validate:"required_without_all=Run Shell Docker"`
	Shell  string      `yaml:"shell,omitempty" json:"shell,omitempty" validate:"required_without_all=Run Copy Docker"`
	Docker *DockerStep `yaml:"docker,omitempty" json:"docker,omitempty" validate:"required_without_all=Run Copy Shell"`
}

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

type CopyStep struct {
	Src     string   `yaml:"src" json:"src" validate:"required"`
	Dst     string   `yaml:"dst" json:"dst" validate:"required"`
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty" validate:"omitempty,dive,required"`
}

var validate = validator.New()

func LoadConfig(configPath string) (*Config, error) {
	ext := strings.ToLower(filepath.Ext(configPath))

	var config *Config
	var err error

	switch ext {
	case ".yaml", ".yml":
		config, err = loadYAMLConfig(configPath)
	case ".ts":
		config, err = loadTypeScriptConfig(configPath)
	case ".js", ".mjs":
		config, err = loadJavaScriptConfig(configPath)
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}

	if err != nil {
		return nil, err
	}

	if err := validate.Struct(config); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return nil, fmt.Errorf("config validation failed: %s", formatValidationErrors(validationErrors))
		}
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func formatValidationErrors(errors validator.ValidationErrors) string {
	var errMsgs []string
	for _, err := range errors {
		errMsgs = append(errMsgs, fmt.Sprintf(
			"Field '%s' failed validation: %s",
			err.Field(),
			err.Tag(),
		))
	}
	return strings.Join(errMsgs, "; ")
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
	cmd := exec.Command("node", "-e", fmt.Sprintf("(async ()=>{const m=await import(\"./%s\");console.log(JSON.stringify(typeof m.default==='function'?await m.default():m.default));})();", filepath.Base(configPath)))
	cmd.Dir = filepath.Dir(configPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", nodeError(err), string(output))
	}

	parts := strings.Split(string(output), "\n")

	outputStr := parts[len(parts)-2]

	fmt.Println(outputStr)

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
		return errors.New("NodeJS (https://nodejs.org/) is not installed or not in PATH. NodeJS is required to load JavaScript and TypeScript configuration files")
	}

	return err
}
