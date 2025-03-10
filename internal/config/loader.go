package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v2"
)

// CommandRunner is an interface for executing commands
type CommandRunner func(dir string, args ...string) ([]byte, error)

// Loader defines the interface for loading configuration.
type Loader interface {
	Load(configPath string) (*Config, error)
}

// DefaultLoader implements the Loader interface using file-based configuration.
type DefaultLoader struct {
	validator *validator.Validate
	loaders   map[string]func(string) (*Config, error)
	cmdRunner CommandRunner
}

// NewLoader creates a new configuration loader with default implementations.
func NewLoader() Loader {
	validate := validator.New()
	loader := &DefaultLoader{
		validator: validate,
		loaders:   make(map[string]func(string) (*Config, error)),
		cmdRunner: execCommand,
	}

	// Register default loaders
	loader.loaders[".yaml"] = loader.loadYAMLConfig
	loader.loaders[".yml"] = loader.loadYAMLConfig
	loader.loaders[".ts"] = loader.loadTypeScriptConfig
	loader.loaders[".js"] = loader.loadJavaScriptConfig
	loader.loaders[".mjs"] = loader.loadJavaScriptConfig
	loader.loaders[".go"] = loader.loadGolangConfig
	loader.loaders[".json"] = loader.loadJSONConfig
	loader.loaders[".toml"] = loader.loadTOMLConfig

	return loader
}

// execCommand executes a command and returns its output,
// streaming the output to stdout/stderr in real time.
func execCommand(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Buffer to collect all output for return
	var outputBuffer bytes.Buffer

	// Create wait groups to ensure all goroutines complete before we finish
	var wg sync.WaitGroup
	wg.Add(2)

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Read stdout in a goroutine and print to console while also collecting for return
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			outputBuffer.WriteString(line + "\n")
		}
	}()

	// Read stderr in a goroutine
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			outputBuffer.WriteString(line + "\n")
		}
	}()

	// Wait for all goroutines to complete
	wg.Wait()

	// Wait for the command to complete
	err = cmd.Wait()
	if err != nil {
		return outputBuffer.Bytes(), fmt.Errorf("%w\n%s", err, outputBuffer.String())
	}

	return outputBuffer.Bytes(), nil
}

// Load loads and validates configuration from the specified path.
func (l *DefaultLoader) Load(configPath string) (*Config, error) {
	config, err := l.loadConfigByExtension(configPath)
	if err != nil {
		return nil, err
	}

	if err := l.validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// loadConfigByExtension loads configuration based on file extension
func (l *DefaultLoader) loadConfigByExtension(configPath string) (*Config, error) {
	ext := strings.ToLower(filepath.Ext(configPath))

	loader, ok := l.loaders[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}

	return loader(configPath)
}

// validateConfig validates the configuration structure
func (l *DefaultLoader) validateConfig(config *Config) error {
	if err := l.validator.Struct(config); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("config validation failed: %s", formatValidationErrors(validationErrors))
		}
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Ensure job and target names are set
	for i, job := range config.Jobs {
		if job.Name == "" {
			job.Name = fmt.Sprintf("job-%d", i+1)
		}
	}

	for _, target := range config.Targets {
		if target.Name == "" {
			target.Name = target.Host
		}
	}

	return nil
}

// formatValidationErrors formats validation errors into a readable string
func formatValidationErrors(errs validator.ValidationErrors) string {
	errMsgs := make([]string, 0, len(errs))
	for _, err := range errs {
		errMsgs = append(errMsgs, fmt.Sprintf(
			"Field '%s' failed validation: %s (condition: %s)",
			err.Field(),
			err.Tag(),
			err.Param(),
		))
	}
	return strings.Join(errMsgs, "\n")
}

// loadJSONConfig loads configuration from JSON file
func (l *DefaultLoader) loadJSONConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	dataStr := replaceEnvVariables(string(data))

	var config Config
	if err := json.Unmarshal([]byte(dataStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &config, nil
}

// loadTOMLConfig loads configuration from TOML file
func (l *DefaultLoader) loadTOMLConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	dataStr := replaceEnvVariables(string(data))

	var config Config
	if err := toml.Unmarshal([]byte(dataStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	return &config, nil
}

// loadYAMLConfig loads configuration from YAML file
func (l *DefaultLoader) loadYAMLConfig(configPath string) (*Config, error) {
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

// loadTypeScriptConfig loads configuration from TypeScript file
func (l *DefaultLoader) loadTypeScriptConfig(configPath string) (*Config, error) {
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
	return l.loadJavaScriptConfig(jsFile)
}

// loadJavaScriptConfig loads configuration from JavaScript file
func (l *DefaultLoader) loadJavaScriptConfig(configPath string) (*Config, error) {
	return l.loadCmdConfig(
		filepath.Dir(configPath),
		"node",
		"-e",
		fmt.Sprintf(
			"(async ()=>{"+
				"const m=await import(\"./%s\");"+
				"console.log(JSON.stringify("+
				"typeof m.default==='function'?await m.default():m.default));"+
				"})();",
			filepath.Base(configPath),
		),
	)
}

// loadGolangConfig loads configuration from Go file
func (l *DefaultLoader) loadGolangConfig(configPath string) (*Config, error) {
	return l.loadCmdConfig("./", "go", "run", configPath)
}

// loadCmdConfig loads configuration by executing a command
func (l *DefaultLoader) loadCmdConfig(dir string, args ...string) (*Config, error) {
	output, err := l.cmdRunner(dir, args...)
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, string(output))
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

// replaceEnvVariables replaces environment variables in the content
func replaceEnvVariables(content string) string {
	re := regexp.MustCompile(`\${(\w+)}`)
	return re.ReplaceAllStringFunc(content, func(s string) string {
		key := re.FindStringSubmatch(s)[1]
		return os.Getenv(key)
	})
}
