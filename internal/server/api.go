package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ospfx/nut_webgui/internal/config"
	"github.com/ospfx/nut_webgui/internal/nut"
	"github.com/ospfx/nut_webgui/internal/poller"
)

// handleAPI routes all /api/* requests to the JSON data API.
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	// Strip leading /api
	path := strings.TrimPrefix(r.URL.Path, "/api")
	path = strings.Trim(path, "/")
	parts := strings.SplitN(path, "/", 4)

	switch {
	// GET /api/ → list namespaces
	case path == "" && r.Method == http.MethodGet:
		s.apiListNamespaces(w, r)

	// GET /api/{namespace} → namespace info
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.apiGetNamespace(w, r, parts[0])

	// GET /api/{namespace}/devices → list UPS devices
	case len(parts) == 2 && parts[1] == "devices" && r.Method == http.MethodGet:
		s.apiListDevices(w, r, parts[0])

	// GET /api/{namespace}/devices/{ups} → UPS detail
	case len(parts) == 3 && parts[1] == "devices" && r.Method == http.MethodGet:
		s.apiGetDevice(w, r, parts[0], parts[2])

	// PATCH /api/{namespace}/devices/{ups} → set variable
	case len(parts) == 3 && parts[1] == "devices" && r.Method == http.MethodPatch:
		s.apiSetVar(w, r, parts[0], parts[2])

	// POST /api/{namespace}/devices/{ups}/instcmd
	case len(parts) == 4 && parts[1] == "devices" && parts[3] == "instcmd" && r.Method == http.MethodPost:
		s.apiInstCmd(w, r, parts[0], parts[2])

	// POST /api/{namespace}/devices/{ups}/fsd
	case len(parts) == 4 && parts[1] == "devices" && parts[3] == "fsd" && r.Method == http.MethodPost:
		s.apiFSD(w, r, parts[0], parts[2])

	default:
		jsonError(w, "not found", http.StatusNotFound)
	}
}

type namespaceEntry struct {
	Namespace       string     `json:"namespace"`
	Address         string     `json:"address"`
	Port            int        `json:"port"`
	Status          string     `json:"status"`
	Version         string     `json:"version"`
	ProtocolVersion string     `json:"protocol_version"`
	LastSyncTime    *time.Time `json:"last_sync_time"`
	DeviceCount     int        `json:"device_count"`
	PollFreq        int        `json:"poll_freq"`
	PollInterval    int        `json:"poll_interval"`
}

func (s *Server) nsEntry(ns *poller.NamespaceState) namespaceEntry {
	ns.RLock()
	defer ns.RUnlock()
	return namespaceEntry{
		Namespace:       ns.Namespace,
		Address:         ns.Address,
		Port:            ns.Port,
		Status:          ns.Status,
		Version:         ns.Version,
		ProtocolVersion: ns.ProtVersion,
		LastSyncTime:    ns.LastSync,
		DeviceCount:     len(ns.Devices),
		PollFreq:        ns.PollFreq,
		PollInterval:    ns.PollInterval,
	}
}

func (s *Server) apiListNamespaces(w http.ResponseWriter, _ *http.Request) {
	list := make([]namespaceEntry, 0, len(s.state.Namespaces))
	for _, ns := range s.state.Namespaces {
		list = append(list, s.nsEntry(ns))
	}
	jsonOK(w, list)
}

func (s *Server) apiGetNamespace(w http.ResponseWriter, _ *http.Request, ns string) {
	state, ok := s.state.Namespaces[ns]
	if !ok {
		jsonError(w, "namespace not found", http.StatusNotFound)
		return
	}
	jsonOK(w, s.nsEntry(state))
}

func (s *Server) apiListDevices(w http.ResponseWriter, _ *http.Request, ns string) {
	state, ok := s.state.Namespaces[ns]
	if !ok {
		jsonError(w, "namespace not found", http.StatusNotFound)
		return
	}
	state.RLock()
	defer state.RUnlock()
	devices := make([]*poller.UPSDevice, 0, len(state.Devices))
	for _, d := range state.Devices {
		devices = append(devices, d)
	}
	jsonOK(w, devices)
}

func (s *Server) apiGetDevice(w http.ResponseWriter, _ *http.Request, ns, upsName string) {
	state, ok := s.state.Namespaces[ns]
	if !ok {
		jsonError(w, "namespace not found", http.StatusNotFound)
		return
	}
	state.RLock()
	defer state.RUnlock()
	dev, ok := state.Devices[upsName]
	if !ok {
		jsonError(w, "device not found", http.StatusNotFound)
		return
	}
	jsonOK(w, dev)
}

type setVarRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (s *Server) apiSetVar(w http.ResponseWriter, r *http.Request, ns, upsName string) {
	cfg := s.getUpsdConfig(ns)
	if cfg == nil {
		jsonError(w, "namespace not found", http.StatusNotFound)
		return
	}
	var req setVarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	c, err := dialUpsd(cfg)
	if err != nil {
		jsonError(w, "cannot connect to NUT server", http.StatusServiceUnavailable)
		return
	}
	defer c.Close()
	if err := c.SetVar(upsName, req.Name, req.Value); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type instCmdRequest struct {
	Command string `json:"command"`
}

func (s *Server) apiInstCmd(w http.ResponseWriter, r *http.Request, ns, upsName string) {
	cfg := s.getUpsdConfig(ns)
	if cfg == nil {
		jsonError(w, "namespace not found", http.StatusNotFound)
		return
	}
	var req instCmdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	c, err := dialUpsd(cfg)
	if err != nil {
		jsonError(w, "cannot connect to NUT server", http.StatusServiceUnavailable)
		return
	}
	defer c.Close()
	if err := c.InstCmd(upsName, req.Command); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiFSD(w http.ResponseWriter, r *http.Request, ns, upsName string) {
	cfg := s.getUpsdConfig(ns)
	if cfg == nil {
		jsonError(w, "namespace not found", http.StatusNotFound)
		return
	}
	c, err := dialUpsd(cfg)
	if err != nil {
		jsonError(w, "cannot connect to NUT server", http.StatusServiceUnavailable)
		return
	}
	defer c.Close()
	if err := c.FSD(upsName); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getUpsdConfig returns the UpsdConfig for a given namespace name.
func (s *Server) getUpsdConfig(ns string) *config.UpsdConfig {
	for i := range s.state.Config.UpsdList {
		if s.state.Config.UpsdList[i].Name == ns {
			return &s.state.Config.UpsdList[i]
		}
	}
	return nil
}

// dialUpsd opens an authenticated connection to a NUT server.
func dialUpsd(cfg *config.UpsdConfig) (*nut.Client, error) {
	c, err := nut.Dial(cfg.Address, cfg.Port, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if cfg.Username != "" {
		if err := c.Auth(cfg.Username, cfg.Password); err != nil {
			c.Close()
			return nil, err
		}
	}
	return c, nil
}

// jsonOK writes a JSON 200 response.
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}
