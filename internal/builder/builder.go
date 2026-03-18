package builder

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/github"
)

func BuildRepository(ctx context.Context, cfg *config.Config, githubClient *github.Client, dockerClient *docker.Client, repo config.RepositoryConfig) error {
	var err error

	//判断任务类型：1 本地上下文构建 2 远程仓库构建
	// 创建构建上下文目录
	var contextDir string
	contextDir, err = os.MkdirTemp("", "build-")
	if err != nil {
		log.Printf("[ERROR] Failed to create temp dir: %v\n", err)
		return err
	}
	defer os.RemoveAll(contextDir)
	//远程仓库就先克隆到上下文目录
	if err = CloneRepository(ctx, contextDir, &repo, githubClient); err != nil {
		log.Printf("[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, repo.Branch, err)
		return err
	}
	//准备Dockerfile,如果用户自定义了Dockerfile,则复制到上下文目录
	var dockerfilePath string
	dockerfilePath, err = copyDockerfile(repo, contextDir)
	if err != nil {
		log.Printf("[ERROR] Failed to get Dockerfile path: %v\n", err)
		return err
	}
	//执行构建
	imageName := getImageName(repo.Branch, cfg.DockerHub.Username, getRepoName(&repo), repo.Tag)
	log.Printf("[INFO] Building and pushing Docker image: %s...\n", imageName)
	var buildErr error
	buildErr = dockerClient.BuildImage(ctx, contextDir, dockerfilePath, imageName, repo.Platforms, repo.BuildArgs, cfg.Proxy)
	if buildErr != nil {
		log.Printf("[ERROR] Failed to build image for %s (branch %s): %v\n", repo.URL, repo.Branch, buildErr)
		return buildErr
	}
	log.Printf("[SUCCESS] Successfully built and pushed %s\n", imageName)

	return nil
}
func CloneRepository(ctx context.Context, contextDir string, repo *config.RepositoryConfig, githubClient *github.Client) error {
	//判断是否是远程仓库
	if repo.URL == "" {
		return nil
	}
	//判断branch是否为空
	branch, exist := isBranchExist(ctx, *repo, githubClient)
	if !exist {
		return fmt.Errorf("branch %s does not exist", repo.Branch)
	}
	//clone repository
	if err := cloneRepository(ctx, repo.URL, branch, repo.Tag, contextDir); err != nil {
		log.Printf("[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, branch, err)
		return err
	}
	repo.Branch = branch
	return nil
}

func isBranchExist(ctx context.Context, repo config.RepositoryConfig, githubClient *github.Client) (string, bool) {
	if repo.Branch == "" {
		branch, err := githubClient.GetDefaultBranch(repo.URL)
		if err != nil {
			log.Printf("[ERROR] Failed to get default branch for %s: %v\n", repo.URL, err)
			return "", false
		}
		log.Printf("[INFO] No branch specified, using default: %s\n", branch)
		return branch, true
	}
	valid, err := githubClient.ValidateBranch(repo.URL, repo.Branch)
	if err != nil {
		log.Printf("[ERROR] Failed to validate branch %s for %s: %v\n", repo.Branch, repo.URL, err)
		return "", false
	}
	if !valid {
		log.Printf("[ERROR] Branch %s does not exist in %s\n", repo.Branch, repo.URL)
		return "", false
	}
	log.Printf("[INFO] Using branch: %s\n", repo.Branch)
	return repo.Branch, true
}
func cloneRepository(ctx context.Context, repoURL, branch, tag, contextDir string) error {
	var cmd *exec.Cmd

	if tag != "" && tag != "latest" {
		log.Printf("[INFO] Cloning repository (tag: %s) to %s...\n", tag, contextDir)
		cmd = exec.CommandContext(ctx, "git", "clone",
			"--branch", tag,
			"--depth", "1",
			"--single-branch",
			repoURL, contextDir)
	} else {
		log.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, contextDir)
		cmd = exec.CommandContext(ctx, "git", "clone",
			"--branch", branch,
			"--depth", "1",
			"--single-branch",
			"--no-tags",
			repoURL, contextDir)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getRepoName(repo *config.RepositoryConfig) string {
	if repo.URL != "" {
		parts := strings.Split(repo.URL, "/")
		name := parts[len(parts)-1]
		return strings.TrimSuffix(name, ".git")
	}
	if repo.DockerfileUser != "" {
		//如果用户自定义了Dockerfile文件路径，那么就使用用户自定义的Dockerfile的父级目录名作为仓库名
		return filepath.Base(filepath.Dir(repo.DockerfileUser))
	}
	return ""
}

func getImageName(branch, username, repoName, tag string) string {

	if branch == "main" || branch == "master" {
		return fmt.Sprintf("%s/%s:%s", username, repoName, tag)
	} else {
		return fmt.Sprintf("%s/%s-%s:%s", username, repoName, branch, tag)
	}
}

func copyDockerfile(repo config.RepositoryConfig, contextDir string) (string, error) {
	if repo.DockerfileUser == "" {
		return repo.DockerfileProject, nil
	}
	//将本地用户自定义Dockerfile文件路径复制到构建上下文目录
	userDockerfileContent, err := os.ReadFile(repo.DockerfileUser)
	if err != nil {
		return "", fmt.Errorf("failed to read user-provided Dockerfile: %w", err)
	}
	userDockerfilePath := filepath.Join(contextDir, "Dockerfile")
	if err := os.WriteFile(userDockerfilePath, userDockerfileContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write user-provided Dockerfile: %w", err)
	}
	return userDockerfilePath, nil
}
