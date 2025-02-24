package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Targets []Target `yaml:"targets"`
	Jobs    []Job    `yaml:"jobs"`
}

type Target struct {
	Name       string `yaml:"name"`
	Host       string `yaml:"host"`
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	PrivateKey string `yaml:"private_key,omitempty"`
	Port       int    `yaml:"port,omitempty"`
}

type Job struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

type Step struct {
	Run    string      `yaml:"run,omitempty"`
	Copy   *CopyStep   `yaml:"copy,omitempty"`
	Shell  string      `yaml:"shell,omitempty"`
	Docker *DockerStep `yaml:"docker,omitempty"`
}

type DockerStep struct {
	Image       string            `yaml:"image"`
	Name        string            `yaml:"name"`
	Environment map[string]string `yaml:"environment"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Labels      map[string]string `yaml:"labels"`
	Networks    []string          `yaml:"networks"`
	Commands    []string          `yaml:"commands"`
	Restart     string            `yaml:"restart"`
}

type CopyStep struct {
	Src     string   `yaml:"src"`
	Dst     string   `yaml:"dst"`
	Exclude []string `yaml:"exclude,omitempty"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Replace environment variables in the YAML content
	dataStr := string(data)
	dataStr = replaceEnvVariables(dataStr)

	var config Config
	if err := yaml.Unmarshal([]byte(dataStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
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
