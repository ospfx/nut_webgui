package server

import (
	"net/http"
	"strings"
)

// handleHealth handles GET /probes/health and GET /probes/health/{namespace}
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok")) //nolint:errcheck
}

// handleReadiness handles GET /probes/readiness
func (s *Server) handleReadiness(w http.ResponseWriter, _ *http.Request) {
	// Ready when at least one namespace is connected
	for _, ns := range s.state.Namespaces {
		ns.RLock()
		status := ns.Status
		ns.RUnlock()
		if status == "connected" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready")) //nolint:errcheck
			return
		}
	}
	if len(s.state.Namespaces) == 0 {
		// No namespaces configured – still return 200 for readiness
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready")) //nolint:errcheck
		return
	}
	http.Error(w, "not ready", http.StatusServiceUnavailable)
}

// handleNamespaceHealth handles GET /probes/health/{namespace}
func (s *Server) handleNamespaceHealth(w http.ResponseWriter, r *http.Request) {
	ns := strings.TrimPrefix(r.URL.Path, "/probes/health/")
	if _, ok := s.state.Namespaces[ns]; !ok {
		http.Error(w, "namespace not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok")) //nolint:errcheck
}

// handleNamespaceReadiness handles GET /probes/readiness/{namespace}
func (s *Server) handleNamespaceReadiness(w http.ResponseWriter, r *http.Request) {
	ns := strings.TrimPrefix(r.URL.Path, "/probes/readiness/")
	state, ok := s.state.Namespaces[ns]
	if !ok {
		http.Error(w, "namespace not found", http.StatusNotFound)
		return
	}
	state.RLock()
	status := state.Status
	state.RUnlock()
	if status == "connected" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready")) //nolint:errcheck
		return
	}
	http.Error(w, "not ready", http.StatusServiceUnavailable)
}
