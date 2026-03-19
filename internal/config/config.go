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

type GiteaConfig struct {
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
	Enabled           *bool             `yaml:"enabled,omitempty"`
	URL               string            `yaml:"url,omitempty"`
	Auth              string            `yaml:"auth,omitempty"`
	TagBranch         string            `yaml:"tag_branch,omitempty"` // 用于clone repository的branch
	TagDocker         string            `yaml:"tag_docker,omitempty"` // 仅用于镜像名称的tag
	Platforms         []string          `yaml:"platforms,omitempty"`
	BuildArgs         map[string]string `yaml:"build_args,omitempty"`
	DockerfileProject string            `yaml:"dockerfile_project,omitempty"`
	DockerfileUser    string            `yaml:"dockerfile_user,omitempty"`

	Cron    string `yaml:"cron,omitempty"`
	Timeout string `yaml:"timeout,omitempty"`
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

		// if repo.TagDocker == "" {
		// 	config.Repositories[i].TagDocker = "latest"
		// }
		if repo.Auth == "" {
			config.Repositories[i].Auth = "github"
		}
		if repo.TagBranch == "" {
			config.Repositories[i].TagBranch = "main"
		}
		if repo.Platforms == nil {
			config.Repositories[i].Platforms = []string{"linux/amd64"}
		}
		if repo.DockerfileProject == "" {
			config.Repositories[i].DockerfileProject = "Dockerfile"
		}
		if repo.URL == "" && repo.DockerfileUser == "" {
			return nil, errMissingFieldAt("repositories", i, "url or dockerfile_user")
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
