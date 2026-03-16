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

type Builder struct {
	ctx          context.Context
	cfg          *config.Config
	githubClient *github.Client
	dockerClient *docker.Client
}

func NewBuilder(ctx context.Context, cfg *config.Config, githubClient *github.Client, dockerClient *docker.Client) *Builder {
	return &Builder{
		ctx:          ctx,
		cfg:          cfg,
		githubClient: githubClient,
		dockerClient: dockerClient,
	}
}

func (b *Builder) BuildRepository(repo config.RepositoryConfig) error {
	isLocalContext := repo.URL == ""

	var tempDir string
	var branch string
	var dockerfilePath string
	var repoName string

	if isLocalContext {
		if repo.DockerfileUser == "" {
			log.Printf("[ERROR] When url is empty, dockerfile_user must be specified\n")
			return fmt.Errorf("dockerfile_user is required for local context")
		}
		tempDir = filepath.Dir(repo.DockerfileUser)
		log.Printf("[INFO] Using local context: %s\n", tempDir)
		branch = repo.Branch
		if branch == "" {
			branch = "main"
		}
		dockerfilePath = "Dockerfile"
		repoName = filepath.Base(tempDir)
	} else {
		log.Printf("[INFO] Checking repository: %s\n", repo.URL)

		branch = repo.Branch
		if branch == "" {
			var err error
			branch, err = b.githubClient.GetDefaultBranch(repo.URL)
			if err != nil {
				log.Printf("[ERROR] Failed to get default branch for %s: %v\n", repo.URL, err)
				return err
			}
			log.Printf("[INFO] No branch specified, using default: %s\n", branch)
		} else {
			valid, err := b.githubClient.ValidateBranch(repo.URL, branch)
			if err != nil {
				log.Printf("[ERROR] Failed to validate branch %s for %s: %v\n", branch, repo.URL, err)
				return err
			}
			if !valid {
				log.Printf("[ERROR] Branch %s does not exist in %s\n", branch, repo.URL)
				return fmt.Errorf("branch %s does not exist", branch)
			}
			log.Printf("[INFO] Using branch: %s\n", branch)
		}

		dockerfilePath = repo.DockerfileProject
		if dockerfilePath == "" {
			dockerfilePath = "Dockerfile"
		}

		tempDir, err := os.MkdirTemp("", "build-")
		if err != nil {
			log.Printf("[ERROR] Failed to create temp dir: %v\n", err)
			return err
		}
		defer os.RemoveAll(tempDir)

		if repo.DockerfileUser != "" {
			log.Printf("[INFO] Using user-provided Dockerfile: %s\n", repo.DockerfileUser)
			userDockerfileContent, err := os.ReadFile(repo.DockerfileUser)
			if err != nil {
				log.Printf("[ERROR] Failed to read user Dockerfile %s: %v\n", repo.DockerfileUser, err)
				return err
			}
			userDockerfilePath := filepath.Join(tempDir, "Dockerfile")
			if err := os.WriteFile(userDockerfilePath, userDockerfileContent, 0644); err != nil {
				log.Printf("[ERROR] Failed to write Dockerfile to %s: %v\n", userDockerfilePath, err)
				return err
			}
			log.Printf("[INFO] Copied user Dockerfile to %s\n", userDockerfilePath)
			dockerfilePath = "Dockerfile"
		} else {
			hasDockerfile, err := b.githubClient.HasDockerfileAtPath(repo.URL, dockerfilePath, branch)
			if err != nil {
				log.Printf("[ERROR] Failed to check Dockerfile for %s (branch %s, path %s): %v\n", repo.URL, branch, dockerfilePath, err)
				return err
			}
			if !hasDockerfile {
				log.Printf("[WARN] No Dockerfile found at %s (branch %s, path %s), skipping...\n", repo.URL, branch, dockerfilePath)
				return fmt.Errorf("no Dockerfile found")
			}
		}

		log.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, tempDir)
		if err := cloneRepository(b.ctx, repo.URL, branch, repo.Tag, tempDir); err != nil {
			log.Printf("[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, branch, err)
			return err
		}

		log.Printf("[DEBUG] Cloned files:\n")
		cmd := exec.CommandContext(b.ctx, "ls", "-la", tempDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("[DEBUG] Failed to list files: %v\n", err)
		}

		repoName = getRepoName(repo.URL)
	}

	var imageName string
	if branch == "main" || branch == "master" {
		imageName = fmt.Sprintf("%s/%s:%s", b.cfg.DockerHub.Username, repoName, repo.Tag)
	} else {
		imageName = fmt.Sprintf("%s/%s-%s:%s", b.cfg.DockerHub.Username, repoName, branch, repo.Tag)
	}

	platforms := repo.Platforms
	if len(platforms) > 0 {
		log.Printf("[INFO] Building for platforms: %s\n", strings.Join(platforms, ", "))
	} else {
		log.Printf("[INFO] Building for default platform\n")
	}

	buildArgs := repo.BuildArgs
	if len(buildArgs) > 0 {
		log.Printf("[INFO] Build args: %v\n", buildArgs)
	}

	if dockerfilePath != "Dockerfile" {
		log.Printf("[INFO] Dockerfile path: %s\n", dockerfilePath)
	}

	log.Printf("[INFO] Logging in to Docker Hub...\n")
	if err := b.dockerClient.Login(b.ctx, b.cfg.DockerHub.Username, b.cfg.DockerHub.Password); err != nil {
		log.Printf("[ERROR] Failed to login to Docker Hub: %v\n", err)
		return err
	}

	log.Printf("[INFO] Verifying proxy settings...\n")
	if b.cfg.Proxy != nil && b.cfg.Proxy.Enabled {
		log.Printf("[INFO] Proxy is ENABLED:\n")
		log.Printf("    HTTP:  %s\n", b.cfg.Proxy.HTTP)
		log.Printf("    HTTPS: %s\n", b.cfg.Proxy.HTTPS)
		if b.cfg.Proxy.NoProxy != "" {
			log.Printf("    NO_PROXY: %s\n", b.cfg.Proxy.NoProxy)
		}
	} else {
		log.Printf("[INFO] Proxy is DISABLED\n")
	}

	log.Printf("[INFO] Building and pushing Docker image: %s...\n", imageName)
	var buildErr error
	if isLocalContext {
		buildErr = b.dockerClient.BuildImageWithProxy(b.ctx, tempDir, imageName, platforms, buildArgs, dockerfilePath, b.cfg.Proxy)
	} else {
		buildErr = b.dockerClient.BuildImageWithProxy(b.ctx, tempDir, imageName, platforms, buildArgs, dockerfilePath, b.cfg.Proxy)
	}
	if buildErr != nil {
		log.Printf("[ERROR] Failed to build image for %s (branch %s): %v\n", repo.URL, branch, buildErr)
		return buildErr
	}

	log.Printf("[SUCCESS] Successfully built and pushed %s\n", imageName)
	return nil
}

func BuildRepository(ctx context.Context, cfg *config.Config, githubClient *github.Client, dockerClient *docker.Client, repo config.RepositoryConfig) error {
	isLocalContext := repo.URL == ""

	var tempDir string
	var branch string
	var dockerfilePath string
	var repoName string

	if isLocalContext {
		if repo.DockerfileUser == "" {
			log.Printf("[ERROR] When url is empty, dockerfile_user must be specified\n")
			return fmt.Errorf("dockerfile_user is required for local context")
		}
		tempDir = filepath.Dir(repo.DockerfileUser)
		log.Printf("[INFO] Using local context: %s\n", tempDir)
		branch = repo.Branch
		if branch == "" {
			branch = "main"
		}
		dockerfilePath = "Dockerfile"
		repoName = filepath.Base(tempDir)
	} else {
		log.Printf("[INFO] Checking repository: %s\n", repo.URL)

		branch = repo.Branch
		if branch == "" {
			var err error
			branch, err = githubClient.GetDefaultBranch(repo.URL)
			if err != nil {
				log.Printf("[ERROR] Failed to get default branch for %s: %v\n", repo.URL, err)
				return err
			}
			log.Printf("[INFO] No branch specified, using default: %s\n", branch)
		} else {
			valid, err := githubClient.ValidateBranch(repo.URL, branch)
			if err != nil {
				log.Printf("[ERROR] Failed to validate branch %s for %s: %v\n", branch, repo.URL, err)
				return err
			}
			if !valid {
				log.Printf("[ERROR] Branch %s does not exist in %s\n", branch, repo.URL)
				return fmt.Errorf("branch %s does not exist", branch)
			}
			log.Printf("[INFO] Using branch: %s\n", branch)
		}

		dockerfilePath = repo.DockerfileProject
		if dockerfilePath == "" {
			dockerfilePath = "Dockerfile"
		}

		tempDir, err := os.MkdirTemp("", "build-")
		if err != nil {
			log.Printf("[ERROR] Failed to create temp dir: %v\n", err)
			return err
		}
		defer os.RemoveAll(tempDir)

		if repo.DockerfileUser != "" {
			log.Printf("[INFO] Using user-provided Dockerfile: %s\n", repo.DockerfileUser)
			userDockerfileContent, err := os.ReadFile(repo.DockerfileUser)
			if err != nil {
				log.Printf("[ERROR] Failed to read user Dockerfile %s: %v\n", repo.DockerfileUser, err)
				return err
			}
			userDockerfilePath := filepath.Join(tempDir, "Dockerfile")
			if err := os.WriteFile(userDockerfilePath, userDockerfileContent, 0644); err != nil {
				log.Printf("[ERROR] Failed to write Dockerfile to %s: %v\n", userDockerfilePath, err)
				return err
			}
			log.Printf("[INFO] Copied user Dockerfile to %s\n", userDockerfilePath)
			dockerfilePath = "Dockerfile"
		} else {
			hasDockerfile, err := githubClient.HasDockerfileAtPath(repo.URL, dockerfilePath, branch)
			if err != nil {
				log.Printf("[ERROR] Failed to check Dockerfile for %s (branch %s, path %s): %v\n", repo.URL, branch, dockerfilePath, err)
				return err
			}
			if !hasDockerfile {
				log.Printf("[WARN] No Dockerfile found at %s (branch %s, path %s), skipping...\n", repo.URL, branch, dockerfilePath)
				return fmt.Errorf("no Dockerfile found")
			}
		}

		log.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, tempDir)
		if err := cloneRepository(ctx, repo.URL, branch, repo.Tag, tempDir); err != nil {
			log.Printf("[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, branch, err)
			return err
		}

		log.Printf("[DEBUG] Cloned files:\n")
		cmd := exec.CommandContext(ctx, "ls", "-la", tempDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("[DEBUG] Failed to list files: %v\n", err)
		}

		repoName = getRepoName(repo.URL)
	}

	var imageName string
	if branch == "main" || branch == "master" {
		imageName = fmt.Sprintf("%s/%s:%s", cfg.DockerHub.Username, repoName, repo.Tag)
	} else {
		imageName = fmt.Sprintf("%s/%s-%s:%s", cfg.DockerHub.Username, repoName, branch, repo.Tag)
	}

	platforms := repo.Platforms
	if len(platforms) > 0 {
		log.Printf("[INFO] Building for platforms: %s\n", strings.Join(platforms, ", "))
	} else {
		log.Printf("[INFO] Building for default platform\n")
	}

	buildArgs := repo.BuildArgs
	if len(buildArgs) > 0 {
		log.Printf("[INFO] Build args: %v\n", buildArgs)
	}

	if dockerfilePath != "Dockerfile" {
		log.Printf("[INFO] Dockerfile path: %s\n", dockerfilePath)
	}

	log.Printf("[INFO] Logging in to Docker Hub...\n")
	if err := dockerClient.Login(ctx, cfg.DockerHub.Username, cfg.DockerHub.Password); err != nil {
		log.Printf("[ERROR] Failed to login to Docker Hub: %v\n", err)
		return err
	}

	log.Printf("[INFO] Verifying proxy settings...\n")
	if cfg.Proxy != nil && cfg.Proxy.Enabled {
		log.Printf("[INFO] Proxy is ENABLED:\n")
		log.Printf("    HTTP:  %s\n", cfg.Proxy.HTTP)
		log.Printf("    HTTPS: %s\n", cfg.Proxy.HTTPS)
		if cfg.Proxy.NoProxy != "" {
			log.Printf("    NO_PROXY: %s\n", cfg.Proxy.NoProxy)
		}
	} else {
		log.Printf("[INFO] Proxy is DISABLED\n")
	}

	log.Printf("[INFO] Building and pushing Docker image: %s...\n", imageName)
	var buildErr error
	if isLocalContext {
		buildErr = dockerClient.BuildImageWithProxy(ctx, tempDir, imageName, platforms, buildArgs, dockerfilePath, cfg.Proxy)
	} else {
		buildErr = dockerClient.BuildImageWithProxy(ctx, tempDir, imageName, platforms, buildArgs, dockerfilePath, cfg.Proxy)
	}
	if buildErr != nil {
		log.Printf("[ERROR] Failed to build image for %s (branch %s): %v\n", repo.URL, branch, buildErr)
		return buildErr
	}

	log.Printf("[SUCCESS] Successfully built and pushed %s\n", imageName)
	return nil
}

func cloneRepository(ctx context.Context, repoURL, branch, tag, destDir string) error {
	var cmd *exec.Cmd

	if tag != "" && tag != "latest" {
		log.Printf("[INFO] Cloning repository (tag: %s) to %s...\n", tag, destDir)
		cmd = exec.CommandContext(ctx, "git", "clone",
			"--branch", tag,
			"--depth", "1",
			"--single-branch",
			repoURL, destDir)
	} else {
		log.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, destDir)
		cmd = exec.CommandContext(ctx, "git", "clone",
			"--branch", branch,
			"--depth", "1",
			"--single-branch",
			"--no-tags",
			repoURL, destDir)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getRepoName(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	name := parts[len(parts)-1]
	return strings.TrimSuffix(name, ".git")
}
