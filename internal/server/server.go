package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"docker-build/internal/api"
	"docker-build/internal/web"
)

type WebServer struct {
	addr       string
	handler    http.Handler
	apiHandler *api.APIHandler
}

func NewWebServer(addr string, apiHandler *api.APIHandler) *WebServer {
	mux := http.NewServeMux()

	mux.Handle("/api/", http.StripPrefix("/api", apiHandler))

	mux.Handle("/", http.FileServer(http.FS(web.Static)))

	return &WebServer{
		addr:       addr,
		handler:    mux,
		apiHandler: apiHandler,
	}
}

func (s *WebServer) Start() error {
	server := &http.Server{
		Addr:         s.addr,
		Handler:      s.handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[INFO] Web server starting on %s\n", s.addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start web server: %w", err)
	}
	return nil
}

func (s *WebServer) Shutdown() error {
	server := &http.Server{
		Addr: s.addr,
	}
	return server.Shutdown(nil)
}

func (s *WebServer) GetAddr() string {
	return s.addr
}
