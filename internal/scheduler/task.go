package scheduler

import (
	"context"
	"log"
	"time"

	"docker-build/internal/builder"
	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
	"docker-build/internal/notify"
)

func buildRepositoryTask(cfg *config.Config, gitClient *git.GitClient, dockerClient *docker.Client, repo config.RepositoryConfig) {
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

	repoName := repo.NameTask
	if repoName == "" {
		repoName = repo.URL
	}

	err := builder.BuildRepository(taskCtx, cfg, *gitClient, dockerClient, repo)

	if err != nil {
		if taskCtx.Err() == context.DeadlineExceeded {
			log.Printf("[ERROR] Build timed out for %s after %v\n", repo.URL, timeout)
		} else {
			log.Printf("[ERROR] Failed to build: %v\n", err)
		}
	}
}

func buildRepositoryTaskWithNotify(cfg *config.Config, gitClient *git.GitClient, dockerClient *docker.Client, repo config.RepositoryConfig, notifier *notify.Manager) {
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

  
	if notifier != nil {
		notifier.SendBuildStart(repo.NameRepo, repo.TagBranch, repo.NameImage)
	}

	err := builder.BuildRepository(taskCtx, cfg, *gitClient, dockerClient, repo)

	if err != nil {
		if taskCtx.Err() == context.DeadlineExceeded {
			log.Printf("[ERROR] Build timed out for %s after %v\n", repo.URL, timeout)
			if notifier != nil {
				notifier.SendBuildFailure(repo.NameRepo, repo.TagBranch, "build timeout")
			}
		} else {
			log.Printf("[ERROR] Failed to build: %v\n", err)
			if notifier != nil {
				notifier.SendBuildFailure(repo.NameRepo, repo.TagBranch, err.Error())
			}
		}
	} else {
		if notifier != nil {
			notifier.SendBuildSuccess(repo.NameRepo, repo.TagBranch, repo.NameImage)
		}
	}
}
