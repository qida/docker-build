package config

import (
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type ConfigWatcher struct {
	configPath string
	callback   func()
	watcher    *fsnotify.Watcher
}

func NewConfigWatcher(configPath string, callback func()) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(configPath)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}

	return &ConfigWatcher{
		configPath: configPath,
		callback:   callback,
		watcher:    watcher,
	}, nil
}

func (cw *ConfigWatcher) Start() {
	log.Printf("[INFO] Watching config file: %s\n", cw.configPath)

	go func() {
		for {
			select {
			case event, ok := <-cw.watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Rename == fsnotify.Rename {
					if filepath.Base(event.Name) == filepath.Base(cw.configPath) {
						log.Printf("[INFO] Config file changed, reloading...\n")
						if cw.callback != nil {
							cw.callback()
						}
					}
				}
			case err, ok := <-cw.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[ERROR] Config watcher error: %v\n", err)
			}
		}
	}()
}

func (cw *ConfigWatcher) Stop() {
	if cw.watcher != nil {
		cw.watcher.Close()
	}
}
