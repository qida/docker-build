package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DockerHubConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type GitHubConfig struct {
	Username string `yaml:"username"`
	Token    string `yaml:"token"`
}

type ProxyConfig struct {
	Enabled bool   `yaml:"enabled"`
	HTTP    string `yaml:"http,omitempty"`
	HTTPS   string `yaml:"https,omitempty"`
	NoProxy string `yaml:"no_proxy,omitempty"`
}

type RepositoryConfig struct {
	URL               string            `yaml:"url,omitempty"`
	Branch            string            `yaml:"branch,omitempty"`
	Tag               string            `yaml:"tag,omitempty"`
	Platforms         []string          `yaml:"platforms,omitempty"`
	BuildArgs         map[string]string `yaml:"build_args,omitempty"`
	DockerfileProject string            `yaml:"dockerfile_project,omitempty"`
	DockerfileUser    string            `yaml:"dockerfile_user,omitempty"`
	Enabled           *bool             `yaml:"enabled,omitempty"`
}

type Config struct {
	DockerHub    DockerHubConfig    `yaml:"docker_hub"`
	GitHub       GitHubConfig       `yaml:"github"`
	Proxy        *ProxyConfig       `yaml:"proxy,omitempty"`
	Repositories []RepositoryConfig `yaml:"repositories"`
}

func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	if config.DockerHub.Username == "" {
		return nil, errMissingField("docker_hub.username")
	}
	if config.DockerHub.Password == "" {
		return nil, errMissingField("docker_hub.password")
	}
	if config.GitHub.Token == "" {
		return nil, errMissingField("github.token")
	}
	if config.GitHub.Username == "" {
		return nil, errMissingField("github.username")
	}
	if len(config.Repositories) == 0 {
		return nil, errMissingField("repositories")
	}

	for i, repo := range config.Repositories {
		if repo.Tag == "" {
			config.Repositories[i].Tag = "latest"
		}
	}

	return &config, nil
}

func errMissingField(field string) error {
	return &ConfigError{Message: "missing required field: " + field}
}

func errMissingFieldAt(field string, index int, subField string) error {
	return &ConfigError{Message: fmt.Sprintf("missing required field: %s[%d].%s", field, index, subField)}
}

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}
