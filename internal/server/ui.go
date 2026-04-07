package server

import (
	"net/http"
	"strings"

	"github.com/ospfx/nut_webgui/internal/poller"
)

// handleHome serves the main dashboard.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := s.buildHomeData()
	s.render(w, r, "index.html", data)
}

// handleUPS serves a single UPS detail page.
// URL: /ups/{namespace}/{ups_name}
func (s *Server) handleUPS(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/ups/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	ns, upsName := parts[0], parts[1]

	state, ok := s.state.Namespaces[ns]
	if !ok {
		http.NotFound(w, r)
		return
	}

	state.RLock()
	dev, ok := state.Devices[upsName]
	var devCopy *poller.UPSDevice
	if ok {
		d := *dev
		d.Vars = copyStringMap(dev.Vars)
		d.RWVars = copyStringMap(dev.RWVars)
		d.Commands = append([]string(nil), dev.Commands...)
		devCopy = &d
	}
	pollInterval := state.PollInterval
	state.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	data := map[string]any{
		"Namespace":    ns,
		"Device":       devCopy,
		"PollInterval": pollInterval,
	}
	s.render(w, r, "ups.html", data)
}

// handleTopology serves the topology view.
func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	data := s.buildHomeData()
	s.render(w, r, "topology.html", data)
}

// handleConnection serves the connection status page.
func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	type nsView struct {
		Namespace       string
		Address         string
		Port            int
		Status          string
		Version         string
		ProtocolVersion string
		LastSync        interface{}
		DeviceCount     int
	}
	nsList := make([]nsView, 0, len(s.state.Namespaces))
	for _, ns := range s.state.Namespaces {
		ns.RLock()
		nsList = append(nsList, nsView{
			Namespace:       ns.Namespace,
			Address:         ns.Address,
			Port:            ns.Port,
			Status:          ns.Status,
			Version:         ns.Version,
			ProtocolVersion: ns.ProtVersion,
			LastSync:        ns.LastSync,
			DeviceCount:     len(ns.Devices),
		})
		ns.RUnlock()
	}
	s.render(w, r, "connection.html", map[string]any{"Namespaces": nsList})
}

// handleSystem serves the system info page.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	data := s.buildHomeData()
	s.render(w, r, "system.html", data)
}

// buildHomeData collects all UPS data for the home / topology views.
func (s *Server) buildHomeData() map[string]any {
	type nsView struct {
		Namespace string
		Status    string
		Devices   []*poller.UPSDevice
	}
	nsList := make([]nsView, 0, len(s.state.Namespaces))
	for _, ns := range s.state.Namespaces {
		ns.RLock()
		devices := make([]*poller.UPSDevice, 0, len(ns.Devices))
		for _, d := range ns.Devices {
			dc := *d
			dc.Vars = copyStringMap(d.Vars)
			dc.RWVars = copyStringMap(d.RWVars)
			dc.Commands = append([]string(nil), d.Commands...)
			devices = append(devices, &dc)
		}
		view := nsView{
			Namespace: ns.Namespace,
			Status:    ns.Status,
			Devices:   devices,
		}
		ns.RUnlock()
		nsList = append(nsList, view)
	}
	return map[string]any{"Namespaces": nsList}
}

// render executes the named template with data.
func (s *Server) render(w http.ResponseWriter, _ *http.Request, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func copyStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
