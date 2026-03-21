package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"docker-build/internal/builder"
	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
	"docker-build/internal/notify"
	"docker-build/internal/scheduler"
)

const (
	DefaultLogFile = "docker-build.log"
)

type APIHandler struct {
	mu              sync.RWMutex
	cfg             *config.Config
	cfgPath         string
	logFilePath     string
	scheduler       *scheduler.Scheduler
	gitClients      map[string]git.GitClient
	dockerClient    *docker.Client
	notifier        *notify.Manager
	buildingRepos   map[int]bool
	buildingReposMu sync.Mutex
	buildContexts   map[int]context.CancelFunc
	buildContextsMu sync.Mutex
}

func NewAPIHandler(cfgPath string, sched *scheduler.Scheduler, gitClients map[string]git.GitClient, dockerClient *docker.Client, notifier *notify.Manager) *APIHandler {
	return &APIHandler{
		cfg:           nil,
		cfgPath:       cfgPath,
		logFilePath:   DefaultLogFile,
		scheduler:     sched,
		gitClients:    gitClients,
		dockerClient:  dockerClient,
		notifier:      notifier,
		buildingRepos: make(map[int]bool),
		buildContexts: make(map[int]context.CancelFunc),
	}
}

func (h *APIHandler) LoadConfig() error {
	cfg, err := config.LoadConfig(h.cfgPath)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cfg = cfg
	return nil
}

func (h *APIHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.cfg == nil {
		http.Error(w, "config not loaded", http.StatusInternalServerError)
		return
	}

	cfgYAML, err := h.cfg.ToYAML()
	if err != nil {
		http.Error(w, "failed to marshal config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write(cfgYAML)
}

func (h *APIHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfgJSON map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&cfgJSON); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfgData, err := json.MarshalIndent(cfgJSON, "", "  ")
	if err != nil {
		http.Error(w, "failed to marshal config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newCfg, err := config.ConfigFromJSON(cfgData)
	if err != nil {
		http.Error(w, "invalid config: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := newCfg.Validate(); err != nil {
		http.Error(w, "invalid config: "+err.Error(), http.StatusBadRequest)
		return
	}

	yamlData, err := newCfg.ToYAML()
	if err != nil {
		http.Error(w, "failed to convert to YAML: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(h.cfgPath, yamlData, 0644); err != nil {
		http.Error(w, "failed to write config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.mu.Lock()
	h.cfg = newCfg
	h.mu.Unlock()

	if err := h.ReloadScheduler(); err != nil {
		log.Printf("[ERROR] Failed to reload scheduler: %v\n", err)
		http.Error(w, "failed to reload scheduler: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "config updated"})
}

func (h *APIHandler) ReloadScheduler() error {
	h.mu.RLock()
	cfg := h.cfg
	gitClients := h.gitClients
	dockerClient := h.dockerClient
	h.mu.RUnlock()

	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	h.scheduler.SetConfig(cfg)
	h.scheduler.SetClients(gitClients, dockerClient)

	return nil
}

func (h *APIHandler) GetSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	cfg := h.cfg
	h.mu.RUnlock()

	if cfg == nil {
		http.Error(w, "config not loaded", http.StatusInternalServerError)
		return
	}

	repos := make([]RepoInfo, len(cfg.Repositories))
	for i, repo := range cfg.Repositories {
		repos[i] = RepoInfo{
			Index:     i,
			URL:       repo.URL,
			NameTask:  repo.NameTask,
			Cron:      repo.Cron,
			TagBranch: repo.TagBranch,
			TagDocker: repo.TagDocker,
			Auth:      repo.Auth,

			Enabled: repo.Enabled == nil || *repo.Enabled,
		}
	}

	status := &SchedulerStatus{
		Running:   true,
		CronCount: len(cfg.Repositories),
		Repos:     repos,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *APIHandler) TriggerBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	cfg := h.cfg
	h.mu.RUnlock()

	if cfg == nil {
		http.Error(w, "config not loaded", http.StatusInternalServerError)
		return
	}

	if req.RepoIndex < 0 || req.RepoIndex >= len(cfg.Repositories) {
		http.Error(w, "invalid repo index", http.StatusBadRequest)
		return
	}

	repo := cfg.Repositories[req.RepoIndex]

	h.buildingReposMu.Lock()
	if h.buildingRepos[req.RepoIndex] {
		h.buildingReposMu.Unlock()
		http.Error(w, "build already in progress", http.StatusConflict)
		return
	}
	h.buildingRepos[req.RepoIndex] = true
	h.buildingReposMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	h.buildContextsMu.Lock()
	h.buildContexts[req.RepoIndex] = cancel
	h.buildContextsMu.Unlock()

	go func() {
		defer func() {
			h.buildingReposMu.Lock()
			delete(h.buildingRepos, req.RepoIndex)
			h.buildingReposMu.Unlock()

			h.buildContextsMu.Lock()
			delete(h.buildContexts, req.RepoIndex)
			h.buildContextsMu.Unlock()

			cancel()
		}()

		gitClient, ok := h.gitClients[repo.Auth]
		if !ok {
			log.Printf("[ERROR] Git client not found for auth %s\n", repo.Auth)
			return
		}

		// repoName := builder.GetRepoName(&repo)
		// imageName := builder.GetImageName(cfg.DockerHub.Username, repoName, repo.TagBranch, repo.TagDocker)

		if h.notifier != nil {
			err := h.notifier.SendBuildStart(repo.NameRepo, repo.TagBranch, repo.NameImage)
			if err != nil {
				log.Printf("[ERROR] Failed to send build start notification: %v\n", err)
				return
			}
		}

		err := builder.BuildRepository(ctx, cfg, gitClient, h.dockerClient, repo)

		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Printf("[INFO] Build canceled for %s\n", repo.URL)
				return
			}
			log.Printf("[ERROR] Build failed for %s: %v\n", repo.URL, err)
			if h.notifier != nil {
				h.notifier.SendBuildFailure(repo.NameRepo, repo.TagBranch, err.Error())
			}
			return
		}

		log.Printf("[SUCCESS] Build completed for %s\n", repo.URL)
		if h.notifier != nil {
			h.notifier.SendBuildSuccess(repo.NameRepo, repo.TagBranch, repo.NameImage)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "build started",
	})
}

func (h *APIHandler) StopBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	cfg := h.cfg
	h.mu.RUnlock()

	if cfg == nil {
		http.Error(w, "config not loaded", http.StatusInternalServerError)
		return
	}

	if req.RepoIndex < 0 || req.RepoIndex >= len(cfg.Repositories) {
		http.Error(w, "invalid repo index", http.StatusBadRequest)
		return
	}

	h.buildContextsMu.Lock()
	if cancelFunc, exists := h.buildContexts[req.RepoIndex]; exists {
		cancelFunc()
		delete(h.buildContexts, req.RepoIndex)
		h.buildContextsMu.Unlock()

		repo := cfg.Repositories[req.RepoIndex]
		// repoName := builder.GetRepoName(&repo)
		// imageName := builder.GetImageName(cfg.DockerHub.Username, repoName, repo.TagBranch, repo.TagDocker)

		if h.notifier != nil {
			h.notifier.SendBuildStop(repo.NameRepo, repo.TagBranch, repo.NameImage)
		}

		log.Printf("[INFO] Build stopped for repo index %d", req.RepoIndex)
	} else {
		h.buildContextsMu.Unlock()
		http.Error(w, "no build in progress for this repo", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "stopped",
		"message": "build stopped",
	})
}

func (h *APIHandler) GetBuildStatus(w http.ResponseWriter, r *http.Request) {
	h.buildingReposMu.Lock()
	building := make([]int, 0, len(h.buildingRepos))
	for idx := range h.buildingRepos {
		building = append(building, idx)
	}
	h.buildingReposMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]int{"building": building})
}

func (h *APIHandler) ReloadConfig(w http.ResponseWriter, r *http.Request) {
	if err := h.LoadConfig(); err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.ReloadScheduler(); err != nil {
		log.Printf("[ERROR] Failed to reload scheduler: %v\n", err)
		http.Error(w, "failed to reload scheduler: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "config reloaded"})
}

func (h *APIHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	tailLines := 100
	if lines := r.URL.Query().Get("lines"); lines != "" {
		fmt.Sscanf(lines, "%d", &tailLines)
	}

	file, err := os.Open(h.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(w, "等待日志文件创建...\n")
			flusher.Flush()
		} else {
			http.Error(w, "Failed to open log file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		defer file.Close()

		var buf [32 * 1024]byte
		var lastPos int64

		stats, err := file.Stat()
		if err == nil && stats.Size() > 0 {
			startPos := stats.Size() - int64(tailLines*200)
			if startPos < 0 {
				startPos = 0
			}
			file.Seek(startPos, 0)
			lastPos = startPos

			n, err := file.Read(buf[:])
			if err == nil && n > 0 {
				w.Write(buf[:n])
				flusher.Flush()
			}
			lastPos = startPos + int64(n)
		}

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				file, err := os.Open(h.logFilePath)
				if err != nil {
					continue
				}

				stats, err := file.Stat()
				if err != nil {
					file.Close()
					continue
				}

				if stats.Size() > lastPos {
					file.Seek(lastPos, 0)
					n, err := file.Read(buf[:])
					if err == nil && n > 0 {
						w.Write(buf[:n])
						flusher.Flush()
					}
					lastPos += int64(n)
				}

				file.Close()
			}
		}
	}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")

	switch path {
	case "/config":
		switch r.Method {
		case http.MethodGet:
			h.GetConfig(w, r)
		case http.MethodPost:
			h.UpdateConfig(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "/scheduler/status":
		h.GetSchedulerStatus(w, r)
	case "/build/trigger":
		h.TriggerBuild(w, r)
	case "/build/stop":
		h.StopBuild(w, r)
	case "/build/status":
		h.GetBuildStatus(w, r)
	case "/config/reload":
		h.ReloadConfig(w, r)
	case "/logs/stream":
		h.StreamLogs(w, r)
	default:
		http.NotFound(w, r)
	}
}
