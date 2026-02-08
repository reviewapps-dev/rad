package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/buildqueue"
	"github.com/reviewapps-dev/rad/internal/caddy"
	"github.com/reviewapps-dev/rad/internal/config"
	"github.com/reviewapps-dev/rad/internal/port"
)

type DeployFunc func(ctx context.Context, state *app.AppState, redeploy bool) error

type Server struct {
	cfg       *config.Config
	store     *app.Store
	ports     *port.Allocator
	queue     *buildqueue.Queue
	caddy     *caddy.Manager
	httpSrv   *http.Server
	startTime time.Time
	deployFn  DeployFunc
}

func New(cfg *config.Config, store *app.Store, ports *port.Allocator, queue *buildqueue.Queue, cm *caddy.Manager) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		ports:     ports,
		queue:     queue,
		caddy:     cm,
		startTime: time.Now(),
	}
}

func (s *Server) SetDeployFunc(fn DeployFunc) {
	s.deployFn = fn
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Unauthenticated
	mux.HandleFunc("GET /health", s.handleHealth)

	// Authenticated routes
	authed := http.NewServeMux()
	authed.HandleFunc("GET /apps", s.handleListApps)
	authed.HandleFunc("GET /apps/{app_id}/status", s.handleGetAppStatus)
	authed.HandleFunc("POST /apps/deploy", s.handleDeploy)
	authed.HandleFunc("DELETE /apps/{app_id}", s.handleTeardown)
	authed.HandleFunc("POST /apps/{app_id}/restart", s.handleRestart)
	authed.HandleFunc("POST /apps/{app_id}/exec", s.handleExec)
	authed.HandleFunc("GET /apps/{app_id}/logs", s.handleLogs)
	authed.HandleFunc("POST /update", s.handleUpdate)

	mux.Handle("/apps/", s.authMiddleware(authed))
	mux.Handle("/apps", s.authMiddleware(authed))
	mux.Handle("/update", s.authMiddleware(authed))

	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)

	return handler
}

func (s *Server) Start() error {
	s.httpSrv = &http.Server{
		Addr:         s.cfg.Server.Listen,
		Handler:      s.routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("rad listening on %s (dev=%v)", s.cfg.Server.Listen, s.cfg.Dev)
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
