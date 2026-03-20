package api

import (
	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
)

type BuildRequest struct {
	RepoIndex int `json:"repo_index"`
}

type BuildResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	RepoURL   string `json:"repo_url,omitempty"`
	RepoName  string `json:"repo_name,omitempty"`
	Branch    string `json:"branch,omitempty"`
	ImageName string `json:"image_name,omitempty"`
}

type SchedulerStatus struct {
	Running   bool       `json:"running"`
	CronCount int        `json:"cron_count"`
	Repos     []RepoInfo `json:"repos"`
}

type RepoInfo struct {
	Index     int    `json:"index"`
	URL       string `json:"url,omitempty"`
	NameTask  string `json:"name_task,omitempty"`
	Cron      string `json:"cron,omitempty"`
	TagBranch string `json:"tag_branch,omitempty"`
	TagDocker string `json:"tag_docker,omitempty"`
	Auth      string `json:"auth,omitempty"`
	Enabled   bool   `json:"enabled"`
}

type BuildManager interface {
	BuildRepository(cfg *config.Config, gitClient git.GitClient, dockerClient *docker.Client, repo config.RepositoryConfig) error
	GetSchedulerStatus() *SchedulerStatus
	TriggerBuild(repoIndex int) (*BuildResponse, error)
}
