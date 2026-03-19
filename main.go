package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
	"docker-build/internal/logx"
	"docker-build/internal/scheduler"
)

func main() {
	configPath := flag.String("c", "config.yaml", "path to config file")
	flag.Parse()

	if err := logx.SetupLogFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
		os.Exit(1)
	}
	defer logx.CloseLogFile()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Printf("Error creating Docker client: %v\n", err)
		os.Exit(1)
	}

	dockerClient.SetProxyConfig(cfg.Proxy)

	ctx := context.Background()

	if err := dockerClient.EnsureBuildxBuilder(ctx); err != nil {
		log.Printf("Warning: %v\n", err)
		log.Println("[INFO] Proceeding without multi-platform support")
	}

	giteaClient := git.NewGiteaClient(cfg.Gitea.Token, cfg.Gitea.Url)
	githubClient := git.NewGitHubClient(cfg.GitHub.Token)

	sched := scheduler.NewScheduler()
	sched.SetConfig(cfg)
	sched.SetClients(map[string]git.GitClient{"gitea": giteaClient, "github": githubClient}, dockerClient)
	sched.Start()

	configWatcher, err := config.NewConfigWatcher(*configPath, func() {
		log.Printf("[INFO] Config file changed, reloading...\n")
		newCfg, err := config.LoadConfig(*configPath)
		if err != nil {
			log.Printf("[ERROR] Failed to reload config: %v\n", err)
			return
		}
		sched.SetConfig(newCfg)
		sched.Restart()
	})
	if err != nil {
		log.Printf("[ERROR] Failed to create config watcher: %v\n", err)
	} else {
		configWatcher.Start()
	}

	log.Println("[INFO] Docker build scheduler started. Press Ctrl+C to exit.")
	select {}
}
