package scheduler

import (
	"log"
	"sync"
	"time"

	"docker-build/internal/config"
	"docker-build/internal/docker"
	"docker-build/internal/git"
	"docker-build/internal/notify"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	mu            sync.RWMutex
	cronScheduler *cron.Cron
	cfg           *config.Config
	gitClients    map[string]git.GitClient
	dockerClient  *docker.Client
	notifier      *notify.Manager
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		cronScheduler: cron.New(cron.WithLocation(time.Local)),
	}
}

func (s *Scheduler) SetNotifier(notifier *notify.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifier = notifier
}

func (s *Scheduler) SetConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

func (s *Scheduler) SetClients(gitClients map[string]git.GitClient, dockerClient *docker.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gitClients = gitClients
	s.dockerClient = dockerClient
}

func (s *Scheduler) Start() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, repo := range s.cfg.Repositories {
		if repo.Enabled != nil && !*repo.Enabled {
			log.Printf("[INFO] Skipping disabled repository: %s\n", repo.URL)
			continue
		}
		gitClient, ok := s.gitClients[repo.Auth]
		if !ok {
			log.Printf("[ERROR] Git client not found for auth %s: %s\n", repo.Auth, repo.URL)
			continue
		}
		if repo.Cron == "" {
			log.Printf("[INFO] Building immediately (cron not set): %s\n", repo.URL)
			go buildRepositoryTaskWithNotify(s.cfg, &gitClient, s.dockerClient, repo, s.notifier)
		} else {
			if repo.URL == "" {
				log.Printf("[INFO] Scheduled build with cron '%s': %s\n", repo.Cron, repo.DockerfileUser)
			} else {
				log.Printf("[INFO] Scheduled build with cron '%s': %s\n", repo.Cron, repo.URL)
			}
			_, err := s.cronScheduler.AddFunc(repo.Cron, func() {
				log.Printf("[INFO] Starting scheduled build for %s\n", repo.URL)
				go buildRepositoryTaskWithNotify(s.cfg, &gitClient, s.dockerClient, repo, s.notifier)
			})
			if err != nil {
				log.Printf("[ERROR] Failed to schedule build for %s: %v\n", repo.URL, err)
			}
		}
	}

	s.cronScheduler.Start()
	log.Println("[INFO] Cron scheduler started")
}

func (s *Scheduler) Restart() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cronScheduler != nil {
		s.cronScheduler.Stop()
	}

	s.cronScheduler = cron.New(cron.WithLocation(time.Local))

	for _, repo := range s.cfg.Repositories {
		if repo.Enabled != nil && !*repo.Enabled {
			log.Printf("[INFO] Skipping disabled repository: %s\n", repo.URL)
			continue
		}
		gitClient, ok := s.gitClients[repo.Auth]
		if !ok {
			log.Printf("[ERROR] Git client not found for auth %s: %s\n", repo.Auth, repo.URL)
			continue
		}
		if repo.Cron == "" {
			log.Printf("[INFO] Building immediately (cron not set): %s\n", repo.URL)
			go buildRepositoryTaskWithNotify(s.cfg, &gitClient, s.dockerClient, repo, s.notifier)
		} else {
			if repo.URL == "" {
				log.Printf("[INFO] Scheduled build with cron '%s': %s\n", repo.Cron, repo.DockerfileUser)
			} else {
				log.Printf("[INFO] Scheduled build with cron '%s': %s\n", repo.Cron, repo.URL)
			}
			_, err := s.cronScheduler.AddFunc(repo.Cron, func() {
				log.Printf("[INFO] Starting scheduled build for %s\n", repo.URL)
				go buildRepositoryTaskWithNotify(s.cfg, &gitClient, s.dockerClient, repo, s.notifier)
			})
			if err != nil {
				log.Printf("[ERROR] Failed to schedule build for %s: %v\n", repo.URL, err)
			}
		}
	}

	s.cronScheduler.Start()
	log.Println("[INFO] Cron scheduler restarted")
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cronScheduler != nil {
		s.cronScheduler.Stop()
	}
}
