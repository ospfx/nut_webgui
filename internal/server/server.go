// Package server wires the HTTP server with all routes.
package server

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/ospfx/nut_webgui/internal/config"
	"github.com/ospfx/nut_webgui/internal/poller"
	"github.com/ospfx/nut_webgui/web"
)

// AppState holds all shared runtime state.
type AppState struct {
	Config     *config.Config
	Namespaces map[string]*poller.NamespaceState
	Events     chan poller.Event
}

// Server is the HTTP server.
type Server struct {
	state     *AppState
	templates map[string]*template.Template
	mux       *http.ServeMux
	upgrader  websocket.Upgrader
	hub       *wsHub
}

// New creates a new Server instance.
func New(state *AppState) (*Server, error) {
	tmpls, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("server: parse templates: %w", err)
	}

	s := &Server{
		state:     state,
		templates: tmpls,
		mux:       http.NewServeMux(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		hub: newWSHub(),
	}

	// Start WebSocket hub
	go s.hub.run()

	// Forward events to WebSocket hub
	go func() {
		for ev := range state.Events {
			s.hub.broadcast(ev)
		}
	}()

	s.registerRoutes()
	return s, nil
}

// Handler returns the HTTP handler, applying base-path nesting.
func (s *Server) Handler() http.Handler {
	base := strings.TrimSuffix(s.state.Config.HTTPServer.BasePath, "/")
	if base == "" {
		return s.mux
	}
	outer := http.NewServeMux()
	outer.Handle(base+"/", http.StripPrefix(base, s.mux))
	return outer
}

func (s *Server) registerRoutes() {
	// Static assets from embedded FS
	staticSub, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Fatalf("server: sub static fs: %v", err)
	}
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Probes
	s.mux.HandleFunc("/probes/health", s.handleHealth)
	s.mux.HandleFunc("/probes/readiness", s.handleReadiness)
	s.mux.HandleFunc("/probes/health/", s.handleNamespaceHealth)
	s.mux.HandleFunc("/probes/readiness/", s.handleNamespaceReadiness)

	// JSON API
	s.mux.HandleFunc("/api/", s.handleAPI)
	s.mux.HandleFunc("/api", s.handleAPI)

	// WebSocket events
	s.mux.HandleFunc("/events", s.handleWS)

	// UI pages
	s.mux.HandleFunc("/", s.handleHome)
	s.mux.HandleFunc("/ups/", s.handleUPS)
	s.mux.HandleFunc("/topology", s.handleTopology)
	s.mux.HandleFunc("/connection", s.handleConnection)
	s.mux.HandleFunc("/system", s.handleSystem)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.state.Config.HTTPServer.Listen, s.state.Config.HTTPServer.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: withLogging(s.Handler()),
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()

	log.Printf("nut_webgui listening on http://%s%s", addr, s.state.Config.HTTPServer.BasePath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// withLogging wraps the handler with basic request logging.
func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
