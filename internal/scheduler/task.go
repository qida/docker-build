package scheduler

import (
	"context"
	"log"
	"time"

	"docker-build/internal/builder"
	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/github"
)

func buildRepositoryTask(cfg *config.Config, githubClient *github.Client, dockerClient *docker.Client, repo config.RepositoryConfig) {
	timeout := 30 * time.Minute
	if repo.Timeout != "" {
		if t, err := time.ParseDuration(repo.Timeout); err == nil {
			timeout = t
			log.Printf("[INFO] Build timeout for %s: %v\n", repo.URL, timeout)
		}
	}

	ctx := context.Background()
	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := builder.BuildRepository(taskCtx, cfg, githubClient, dockerClient, repo); err != nil {
		if taskCtx.Err() == context.DeadlineExceeded {
			log.Printf("[ERROR] Build timed out for %s after %v\n", repo.URL, timeout)
		} else {
			log.Printf("[ERROR] Failed to build: %v\n", err)
		}
	}
}
