package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"docker-build/internal/api"
	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
	"docker-build/internal/logx"
	"docker-build/internal/notify"
	"docker-build/internal/scheduler"
	"docker-build/internal/server"
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

	webAddr := fmt.Sprintf("%s:%d", cfg.WebHttp.Ip, cfg.WebHttp.Port)

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

	// 初始化通知管理器
	notifyManager := notify.NewManager(cfg.Notify)

	sched := scheduler.NewScheduler()
	sched.SetConfig(cfg)
	sched.SetClients(map[string]git.GitClient{"gitea": giteaClient, "github": githubClient}, dockerClient)
	sched.SetNotifier(notifyManager)
	sched.Start()

	apiHandler := api.NewAPIHandler(*configPath, sched, map[string]git.GitClient{"gitea": giteaClient, "github": githubClient}, dockerClient, notifyManager)
	if err := apiHandler.LoadConfig(); err != nil {
		log.Printf("Warning: failed to load config for API: %v\n", err)
	}

	webServer := server.NewWebServer(webAddr, apiHandler)

	go func() {
		if err := webServer.Start(); err != nil {
			log.Printf("Error starting web server: %v\n", err)
		}
	}()

	configWatcher, err := config.NewConfigWatcher(*configPath, func() {
		log.Printf("[INFO] Config file changed, reloading...\n")
		newCfg, err := config.LoadConfig(*configPath)
		if err != nil {
			log.Printf("[ERROR] Failed to reload config: %v\n", err)
			return
		}
		sched.SetConfig(newCfg)
		sched.Restart()
		apiHandler.LoadConfig()
	})
	if err != nil {
		log.Printf("[ERROR] Failed to create config watcher: %v\n", err)
	} else {
		configWatcher.Start()
	}

	log.Printf("[INFO] Web server available at http://%s:%d\n", cfg.WebHttp.Ip, cfg.WebHttp.Port)
	log.Println("[INFO] Docker build scheduler started. Press Ctrl+C to exit.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[INFO] Shutting down...")
	sched.Stop()
	if err := webServer.Shutdown(); err != nil {
		log.Printf("Error shutting down web server: %v\n", err)
	}
	log.Println("[INFO] Goodbye!")
}
