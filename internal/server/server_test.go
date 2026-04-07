package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ospfx/nut_webgui/internal/config"
	"github.com/ospfx/nut_webgui/internal/poller"
)

// buildTestState creates a test AppState with one namespace and one device.
func buildTestState() *AppState {
	now := time.Now()
	ns := &poller.NamespaceState{
		Namespace:    "default",
		Address:      "127.0.0.1",
		Port:         3493,
		Status:       "connected",
		Version:      "2.8.0",
		ProtVersion:  "1.3",
		LastSync:     &now,
		PollFreq:     30,
		PollInterval: 2,
		Devices: map[string]*poller.UPSDevice{
			"myups": {
				Name:        "myups",
				Description: "Test UPS",
				Vars: map[string]string{
					"ups.status":      "OL",
					"battery.charge":  "100",
					"ups.load":        "20",
					"battery.runtime": "3600",
				},
				Commands: []string{"test.battery.start"},
				RWVars:   map[string]string{"ups.delay.shutdown": "20"},
			},
		},
	}
	return &AppState{
		Config: &config.Config{
			HTTPServer: config.HTTPServerConfig{
				BasePath: "/",
				Listen:   "0.0.0.0",
				Port:     9000,
			},
			UpsdList: []config.UpsdConfig{
				{Name: "default", Address: "127.0.0.1", Port: 3493},
			},
		},
		Namespaces: map[string]*poller.NamespaceState{"default": ns},
		Events:     make(chan poller.Event, 10),
	}
}

func TestAPIListNamespaces(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []namespaceEntry
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 namespace, got %d", len(list))
	}
	if list[0].Namespace != "default" {
		t.Errorf("want namespace=default, got %q", list[0].Namespace)
	}
	if list[0].DeviceCount != 1 {
		t.Errorf("want 1 device, got %d", list[0].DeviceCount)
	}
}

func TestAPIGetNamespace(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/default", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var entry namespaceEntry
	if err := json.NewDecoder(rec.Body).Decode(&entry); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if entry.Status != "connected" {
		t.Errorf("want status=connected, got %q", entry.Status)
	}
}

func TestAPIGetNamespaceNotFound(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestAPIListDevices(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/default/devices", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var devices []*poller.UPSDevice
	if err := json.NewDecoder(rec.Body).Decode(&devices); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("want 1 device, got %d", len(devices))
	}
	if devices[0].Name != "myups" {
		t.Errorf("want name=myups, got %q", devices[0].Name)
	}
}

func TestAPIGetDevice(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/default/devices/myups", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var dev poller.UPSDevice
	if err := json.NewDecoder(rec.Body).Decode(&dev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dev.Vars["ups.status"] != "OL" {
		t.Errorf("want ups.status=OL, got %q", dev.Vars["ups.status"])
	}
}

func TestAPIGetDeviceNotFound(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/default/devices/nosuchups", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestProbeHealth(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/probes/health", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestProbeReadiness(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestProbeNamespaceHealth(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/probes/health/default", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestHomePageRenders(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if len(body) == 0 {
		t.Error("empty body")
	}
}

func TestUPSDetailPageRenders(t *testing.T) {
	state := buildTestState()
	srv, err := New(state)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ups/default/myups", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpsStatusClass(t *testing.T) {
cases := []struct {
status string
want   string
}{
{"OL", "status-online"},
{"OL CHRG", "status-online"},
{"OB", "status-onbattery"},
{"OB LB", "status-onbattery"},
{"LB", "status-lowbattery"},
{"FSD", "status-fsd"},
{"", "status-unknown"},
}
for _, tc := range cases {
got := upsStatusClass(tc.status)
if got != tc.want {
t.Errorf("upsStatusClass(%q) = %q, want %q", tc.status, got, tc.want)
}
}
}
