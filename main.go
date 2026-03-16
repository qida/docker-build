package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/github"

	"github.com/robfig/cron/v3"
)

func main() {
	configPath := flag.String("c", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	githubClient := github.NewClient(cfg.GitHub.Token)
	dockerClient, err := docker.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Docker client: %v\n", err)
		os.Exit(1)
	}

	dockerClient.SetProxyConfig(cfg.Proxy)

	ctx := context.Background()

	if err := dockerClient.EnsureBuildxBuilder(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		fmt.Println("[INFO] Proceeding without multi-platform support")
	}

	c := cron.New(cron.WithLocation(time.Local))
	for _, repo := range cfg.Repositories {
		if repo.Enabled != nil && !*repo.Enabled {
			fmt.Printf("[INFO] Skipping disabled repository: %s\n", repo.URL)
			continue
		}

		if repo.Cron == "" {
			fmt.Printf("[INFO] Building immediately (cron not set): %s\n", repo.URL)
			if err := buildRepository(ctx, cfg, githubClient, dockerClient, repo); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to build: %v\n", err)
			}
		} else {
			fmt.Printf("[INFO] Scheduled build with cron '%s': %s\n", repo.Cron, repo.URL)
			_, err := c.AddFunc(repo.Cron, func() {
				fmt.Printf("[INFO] Starting scheduled build for %s\n", repo.URL)
				if err := buildRepository(ctx, cfg, githubClient, dockerClient, repo); err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Scheduled build failed: %v\n", err)
				}
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to schedule build for %s: %v\n", repo.URL, err)
			}
		}
	}

	c.Start()
	fmt.Println("[INFO] Cron scheduler started. Press Ctrl+C to exit.")
	select {}
}

func buildRepository(ctx context.Context, cfg *config.Config, githubClient *github.Client, dockerClient *docker.Client, repo config.RepositoryConfig) error {
	isLocalContext := repo.URL == ""

	var tempDir string
	var branch string
	var dockerfilePath string
	var repoName string

	if isLocalContext {
		if repo.DockerfileUser == "" {
			fmt.Fprintf(os.Stderr, "[ERROR] When url is empty, dockerfile_user must be specified\n")
			return fmt.Errorf("dockerfile_user is required for local context")
		}
		tempDir = filepath.Dir(repo.DockerfileUser)
		fmt.Printf("[INFO] Using local context: %s\n", tempDir)
		branch = repo.Branch
		if branch == "" {
			branch = "main"
		}
		dockerfilePath = "Dockerfile"
		repoName = filepath.Base(tempDir)
	} else {
		fmt.Printf("[INFO] Checking repository: %s\n", repo.URL)

		branch = repo.Branch
		if branch == "" {
			var err error
			branch, err = githubClient.GetDefaultBranch(repo.URL)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to get default branch for %s: %v\n", repo.URL, err)
				return err
			}
			fmt.Printf("[INFO] No branch specified, using default: %s\n", branch)
		} else {
			valid, err := githubClient.ValidateBranch(repo.URL, branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to validate branch %s for %s: %v\n", branch, repo.URL, err)
				return err
			}
			if !valid {
				fmt.Fprintf(os.Stderr, "[ERROR] Branch %s does not exist in %s\n", branch, repo.URL)
				return fmt.Errorf("branch %s does not exist", branch)
			}
			fmt.Printf("[INFO] Using branch: %s\n", branch)
		}

		dockerfilePath = repo.DockerfileProject
		if dockerfilePath == "" {
			dockerfilePath = "Dockerfile"
		}

		tempDir, err := os.MkdirTemp("", "build-")
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to create temp dir: %v\n", err)
			return err
		}
		defer os.RemoveAll(tempDir)

		if repo.DockerfileUser != "" {
			fmt.Printf("[INFO] Using user-provided Dockerfile: %s\n", repo.DockerfileUser)
			userDockerfileContent, err := os.ReadFile(repo.DockerfileUser)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to read user Dockerfile %s: %v\n", repo.DockerfileUser, err)
				return err
			}
			userDockerfilePath := filepath.Join(tempDir, "Dockerfile")
			if err := os.WriteFile(userDockerfilePath, userDockerfileContent, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to write Dockerfile to %s: %v\n", userDockerfilePath, err)
				return err
			}
			fmt.Printf("[INFO] Copied user Dockerfile to %s\n", userDockerfilePath)
			dockerfilePath = "Dockerfile"
		} else {
			hasDockerfile, err := githubClient.HasDockerfileAtPath(repo.URL, dockerfilePath, branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to check Dockerfile for %s (branch %s, path %s): %v\n", repo.URL, branch, dockerfilePath, err)
				return err
			}
			if !hasDockerfile {
				fmt.Printf("[WARN] No Dockerfile found at %s (branch %s, path %s), skipping...\n", repo.URL, branch, dockerfilePath)
				return fmt.Errorf("no Dockerfile found")
			}
		}

		fmt.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, tempDir)
		if err := cloneRepository(ctx, repo.URL, branch, repo.Tag, tempDir); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to clone %s (branch %s): %v\n", repo.URL, branch, err)
			return err
		}

		fmt.Printf("[DEBUG] Cloned files:\n")
		cmd := exec.CommandContext(ctx, "ls", "-la", tempDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to list files: %v\n", err)
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
		fmt.Printf("[INFO] Building for platforms: %s\n", strings.Join(platforms, ", "))
	} else {
		fmt.Printf("[INFO] Building for default platform\n")
	}

	buildArgs := repo.BuildArgs
	if len(buildArgs) > 0 {
		fmt.Printf("[INFO] Build args: %v\n", buildArgs)
	}

	if dockerfilePath != "Dockerfile" {
		fmt.Printf("[INFO] Dockerfile path: %s\n", dockerfilePath)
	}

	fmt.Printf("[INFO] Logging in to Docker Hub...\n")
	if err := dockerClient.Login(ctx, cfg.DockerHub.Username, cfg.DockerHub.Password); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to login to Docker Hub: %v\n", err)
		return err
	}

	fmt.Printf("[INFO] Verifying proxy settings...\n")
	if cfg.Proxy != nil && cfg.Proxy.Enabled {
		fmt.Printf("[INFO] Proxy is ENABLED:\n")
		fmt.Printf("    HTTP:  %s\n", cfg.Proxy.HTTP)
		fmt.Printf("    HTTPS: %s\n", cfg.Proxy.HTTPS)
		if cfg.Proxy.NoProxy != "" {
			fmt.Printf("    NO_PROXY: %s\n", cfg.Proxy.NoProxy)
		}
	} else {
		fmt.Printf("[INFO] Proxy is DISABLED\n")
	}

	fmt.Printf("[INFO] Building and pushing Docker image: %s...\n", imageName)
	var buildErr error
	if isLocalContext {
		buildErr = dockerClient.BuildImageWithProxy(ctx, tempDir, imageName, platforms, buildArgs, dockerfilePath, cfg.Proxy)
	} else {
		buildErr = dockerClient.BuildImageWithProxy(ctx, tempDir, imageName, platforms, buildArgs, dockerfilePath, cfg.Proxy)
	}
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to build image for %s (branch %s): %v\n", repo.URL, branch, buildErr)
		return buildErr
	}

	fmt.Printf("[SUCCESS] Successfully built and pushed %s\n", imageName)
	return nil
}

func cloneRepository(ctx context.Context, repoURL, branch, tag, destDir string) error {
	var cmd *exec.Cmd

	if tag != "" && tag != "latest" {
		fmt.Printf("[INFO] Cloning repository (tag: %s) to %s...\n", tag, destDir)
		cmd = exec.CommandContext(ctx, "git", "clone",
			"--branch", tag,
			"--depth", "1",
			"--single-branch",
			repoURL, destDir)
	} else {
		fmt.Printf("[INFO] Cloning repository (branch: %s) to %s...\n", branch, destDir)
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
