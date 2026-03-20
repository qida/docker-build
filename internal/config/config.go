package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type WebHttp struct {
	Ip   string `yaml:"ip" json:"ip"`
	Port int    `yaml:"port" json:"port"`
}

type DockerHubConfig struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type GitHubConfig struct {
	Username string `yaml:"username" json:"username"`
	Token    string `yaml:"token" json:"token"`
}

type GiteaConfig struct {
	Url      string `yaml:"url" json:"url"`
	Username string `yaml:"username" json:"username"`
	Token    string `yaml:"token" json:"token"`
}

type ProxyConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Http    string `yaml:"http,omitempty" json:"http"`
	Https   string `yaml:"https,omitempty" json:"https"`
	NoProxy string `yaml:"no_proxy,omitempty" json:"no_proxy"`
}

type RepositoryConfig struct {
	NameTask          string            `yaml:"name_task,omitempty" json:"name_task,omitempty"`
	Enabled           *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	URL               string            `yaml:"url,omitempty" json:"url,omitempty"`
	Auth              string            `yaml:"auth,omitempty" json:"auth,omitempty"`
	TagBranch         string            `yaml:"tag_branch,omitempty" json:"tag_branch,omitempty"`
	TagDocker         string            `yaml:"tag_docker,omitempty" json:"tag_docker,omitempty"`
	Platforms         []string          `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	BuildArgs         map[string]string `yaml:"build_args,omitempty" json:"build_args,omitempty"`
	DockerfileProject string            `yaml:"dockerfile_project,omitempty" json:"dockerfile_project,omitempty"`
	DockerfileUser    string            `yaml:"dockerfile_user,omitempty" json:"dockerfile_user,omitempty"`
	Cron              string            `yaml:"cron,omitempty" json:"cron,omitempty"`
	Timeout           string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

type Config struct {
	WebHttp      WebHttp            `yaml:"web_http" json:"web_http"`
	DockerHub    DockerHubConfig    `yaml:"docker_hub" json:"docker_hub"`
	GitHub       GitHubConfig       `yaml:"github" json:"github"`
	Gitea        *GiteaConfig       `yaml:"gitea" json:"gitea,omitempty"`
	Proxy        *ProxyConfig       `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Repositories []RepositoryConfig `yaml:"repositories" json:"repositories"`
}

func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	if cfg.WebHttp.Ip == "" {
		cfg.WebHttp.Ip = "0.0.0.0"
	}
	
	if cfg.WebHttp.Port == 0 {
		cfg.WebHttp.Port = 8080
	}

	if cfg.Gitea != nil {
		if cfg.Gitea.Url == "" {
			return nil, errMissingField("gitea.url")
		}
	}

	if cfg.DockerHub.Username == "" {
		return nil, errMissingField("docker_hub.username")
	}
	if cfg.DockerHub.Password == "" {
		return nil, errMissingField("docker_hub.password")
	}
	if cfg.GitHub.Token == "" {
		return nil, errMissingField("github.token")
	}
	if cfg.GitHub.Username == "" {
		return nil, errMissingField("github.username")
	}
	if len(cfg.Repositories) == 0 {
		return nil, errMissingField("repositories")
	}

	for i, repo := range cfg.Repositories {
		if repo.Auth == "" {
			if strings.Contains(repo.URL, "gitea") {
				cfg.Repositories[i].Auth = "gitea"
			} else {
				cfg.Repositories[i].Auth = "github"
			}
		}
		if repo.TagBranch == "" {
			cfg.Repositories[i].TagBranch = "main"
		}
		if repo.Platforms == nil {
			cfg.Repositories[i].Platforms = []string{"linux/amd64"}
		}
		if repo.DockerfileProject == "" {
			cfg.Repositories[i].DockerfileProject = "Dockerfile"
		}

		cfg.Repositories[i].URL = strings.TrimRight(repo.URL, ".git")

		if repo.URL == "" && repo.DockerfileUser == "" {
			return nil, errMissingFieldAt("repositories", i, "url or dockerfile_user")
		}
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.DockerHub.Username == "" {
		return errMissingField("docker_hub.username")
	}
	if c.DockerHub.Password == "" {
		return errMissingField("docker_hub.password")
	}
	if c.GitHub.Token == "" {
		return errMissingField("github.token")
	}
	if c.GitHub.Username == "" {
		return errMissingField("github.username")
	}
	if len(c.Repositories) == 0 {
		return errMissingField("repositories")
	}

	if c.Gitea != nil && c.Gitea.Url == "" {
		return errMissingField("gitea.url")
	}

	for i, repo := range c.Repositories {
		if repo.URL == "" && repo.DockerfileUser == "" {
			return errMissingFieldAt("repositories", i, "url or dockerfile_user")
		}
	}

	return nil
}

func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (c *Config) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

func ConfigFromJSON(data []byte) (*Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
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
