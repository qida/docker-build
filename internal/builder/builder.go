package builder

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
)

func BuildRepository(ctx context.Context, cfg *config.Config, gitClient git.GitClient, dockerClient *docker.Client, repo config.RepositoryConfig) error {
	var err error
	// 判断任务类型：1 本地上下文构建 2 远程仓库构建
	// 创建构建上下文目录
	var contextDir string
	contextDir, err = os.MkdirTemp("", "build-")
	if err != nil {
		log.Printf("[ERROR] Failed to create temp dir: %v\n", err)
		return err
	}
	defer os.RemoveAll(contextDir)
	//远程仓库就先克隆到上下文目录
	if err = CloneRepository(ctx, contextDir, &repo, gitClient, cfg); err != nil {
		log.Printf("[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, repo.TagBranch, err)
		return err
	}
	//在终端查看文件列表执行ls
	log.Printf("[DEBUG] Cloned files:\n")
	cmd := exec.CommandContext(ctx, "ls", "-la", contextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[DEBUG] Failed to list files: %v\n", err)
	}
	//准备Dockerfile,如果用户自定义了Dockerfile,则复制到上下文目录
	var dockerfilePath string
	dockerfilePath, err = copyDockerfile(repo, contextDir)
	if err != nil {
		log.Printf("[ERROR] Failed to get Dockerfile path: %v\n", err)
		return err
	}
	//执行构建
	imageName := getImageName(repo.TagBranch, cfg.DockerHub.Username, getRepoName(&repo), repo.TagDocker)
	log.Printf("[INFO] Building and pushing Docker image: %s...\n", imageName)
	var buildErr error
	buildErr = dockerClient.BuildImage(ctx, contextDir, dockerfilePath, imageName, repo.Platforms, repo.BuildArgs, cfg.Proxy)
	if buildErr != nil {
		log.Printf("[ERROR] Failed to build image for %s (branch %s): %v\n", repo.URL, repo.TagBranch, buildErr)
		return buildErr
	}
	log.Printf("[SUCCESS] Successfully built and pushed %s\n", imageName)

	return nil
}
func CloneRepository(ctx context.Context, context_dir string, repo *config.RepositoryConfig, gitClient git.GitClient, cfg *config.Config) error {
	if repo.URL == "" {
		//本地仓库
		return nil
	}
	//判断branch是否为空
	branch, exist := isBranchExist(*repo, gitClient)
	if !exist {
		return fmt.Errorf("branch %s does not exist", repo.TagBranch)
	}
	//clone repository
	if err := cloneRepository(ctx, repo.URL, branch, context_dir, repo.Auth, cfg); err != nil {
		log.Printf("[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, branch, err)
		return err
	}
	repo.TagBranch = branch
	return nil
}

func isBranchExist(repo config.RepositoryConfig, gitClient git.GitClient) (string, bool) {
	if gitClient == nil {
		log.Printf("[ERROR] Git client is nil for repository: %s\n", repo.URL)
		return "", false
	}
	valid, err := gitClient.ValidateBranch(repo.URL, repo.TagBranch)
	if err != nil {
		log.Printf("[ERROR] Failed to validate branch %s for %s: %v\n", repo.TagBranch, repo.URL, err)
		return "", false
	}
	if valid {
		return repo.TagBranch, true
	}
	//如果用户没有指定branch,则默认使用默认分支
	branch, err := gitClient.GetDefaultBranch(repo.URL)
	if err != nil {
		log.Printf("[ERROR] Failed to get default branch for %s: %v\n", repo.URL, err)
		return "", false
	}
	log.Printf("[INFO] No branch specified, using default: %s\n", branch)
	return branch, true
}
func cloneRepository(ctx context.Context, repoURL, branch, contextDir, auth string, cfg *config.Config) error {
	var cmd *exec.Cmd
	log.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, contextDir)
	switch auth {
	case "gitea":
		giteaUser := cfg.Gitea.Username
		giteaToken := cfg.Gitea.Token
		parsedURL, err := url.Parse(repoURL)
		if err != nil {
			return err
		}
		authURL := fmt.Sprintf("%s://%s:%s@%s%s", parsedURL.Scheme, giteaUser, giteaToken, parsedURL.Host, parsedURL.Path)
		cmd = exec.CommandContext(ctx, "git", "clone",
			"--branch", branch,
			"--depth", "1",
			"--single-branch",
			"--no-tags",
			authURL, contextDir)
	default:
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

func getImageName(tag_branch, username, repo_name, tag_name string) string {
	if tag_name == "" {
		tag_name = "latest"
	}
	if tag_branch == "main" || tag_branch == "master" {
		return strings.ToLower(fmt.Sprintf("%s/%s:%s", username, repo_name, tag_name))
	} else {
		return strings.ToLower(fmt.Sprintf("%s/%s-%s:%s", username, repo_name, tag_branch, tag_name))
	}
}

func copyDockerfile(repo config.RepositoryConfig, context_dir string) (string, error) {
	if repo.DockerfileUser == "" {
		//返回绝对路径
		return filepath.Join(context_dir, repo.DockerfileProject), nil
	}
	//将本地用户自定义Dockerfile文件路径复制到构建上下文目录
	userDockerfileContent, err := os.ReadFile(repo.DockerfileUser)
	if err != nil {
		return "", fmt.Errorf("failed to read user-provided Dockerfile: %w", err)
	}
	userDockerfilePath := filepath.Join(context_dir, "Dockerfile")
	if err := os.WriteFile(userDockerfilePath, userDockerfileContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write user-provided Dockerfile: %w", err)
	}
	return userDockerfilePath, nil
}
