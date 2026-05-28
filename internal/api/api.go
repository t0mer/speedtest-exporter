// Package api implements the chi HTTP server and all REST handlers.
package api

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/t0mer/speedtest-exporter/internal/config"
	"github.com/t0mer/speedtest-exporter/internal/notifications"
	"github.com/t0mer/speedtest-exporter/internal/scheduler"
	"github.com/t0mer/speedtest-exporter/internal/service"
	"github.com/t0mer/speedtest-exporter/web"
)

// Server is the HTTP API server.
type Server struct {
	service    *service.Service
	cfg        *config.Config
	ooklaPath  string
	router     *chi.Mux
	schedMu    sync.Mutex
	sched      *scheduler.Scheduler
	notifStore *notifications.Store // may be nil
}

// NewServer builds and wires up the chi router with all routes.
func NewServer(svc *service.Service, cfg *config.Config, ooklaPath string, notifStore *notifications.Store) *Server {
	s := &Server{service: svc, cfg: cfg, ooklaPath: ooklaPath, notifStore: notifStore, router: chi.NewRouter()}
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Timeout(time.Duration(cfg.Server.WriteTimeout) * time.Second))
	s.router.Use(middleware.Recoverer)

	s.router.Get("/healthz", s.handleHealth)
	s.router.Handle("/metrics", svc.Metrics().Handler())

	s.router.Route("/api", func(r chi.Router) {
		r.Post("/test", s.handleRunTest)
		r.Get("/results", s.handleListResults)
		r.Get("/results/latest", s.handleLatestResult)
		r.Get("/results/{id}", s.handleGetResult)
		r.Get("/summary", s.handleSummary)
		r.Get("/compare", s.handleCompare)
		r.Get("/settings", s.handleGetSettings)
		r.Put("/settings", s.handlePutSettings)
		r.Route("/notifications", func(r chi.Router) {
			r.Get("/", s.handleListChannels)
			r.Post("/", s.handleCreateChannel)
			r.Put("/{id}", s.handleUpdateChannel)
			r.Delete("/{id}", s.handleDeleteChannel)
			r.Post("/test", s.handleTestChannel)
		})
	})

	if cfg.Server.EnableUI {
		staticSub, _ := fs.Sub(web.FS, "static")
		s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
		tmpl := template.Must(template.ParseFS(web.FS, "templates/index.html"))
		s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = tmpl.Execute(w, nil)
		})
	}

	return s
}

// SetSchedule stops any running scheduler and starts a new one with spec.
// Passing an empty spec stops the scheduler without starting a new one.
func (s *Server) SetSchedule(spec string) error {
	s.schedMu.Lock()
	defer s.schedMu.Unlock()
	if s.sched != nil {
		s.sched.Stop()
		s.sched = nil
	}
	if spec == "" {
		return nil
	}
	sched, err := scheduler.New(s.service, spec)
	if err != nil {
		return fmt.Errorf("start scheduler %q: %w", spec, err)
	}
	sched.Start()
	s.sched = sched
	return nil
}

// StopScheduler stops the running scheduler, if any.
func (s *Server) StopScheduler() {
	s.schedMu.Lock()
	defer s.schedMu.Unlock()
	if s.sched != nil {
		s.sched.Stop()
		s.sched = nil
	}
}

// ServeHTTP implements http.Handler, used in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server and blocks until it returns an error.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.cfg.Server.WriteTimeout) * time.Second,
	}
	slog.Info("server listening", "addr", addr)
	return srv.ListenAndServe()
}
