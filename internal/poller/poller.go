// Package poller implements background polling of NUT servers.
package poller

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ospfx/nut_webgui/internal/config"
	"github.com/ospfx/nut_webgui/internal/nut"
)

// UPSDevice holds all data for a single UPS device.
type UPSDevice struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Vars        map[string]string `json:"vars"`
	Commands    []string          `json:"commands"`
	RWVars      map[string]string `json:"rw_vars"`
}

// NamespaceState is the in-memory state for a single NUT server namespace.
type NamespaceState struct {
	mu           sync.RWMutex
	Namespace    string            `json:"namespace"`
	Address      string            `json:"address"`
	Port         int               `json:"port"`
	Status       string            `json:"status"`
	Version      string            `json:"version"`
	ProtVersion  string            `json:"protocol_version"`
	LastSync     *time.Time        `json:"last_sync_time"`
	Devices      map[string]*UPSDevice `json:"devices"`
	PollFreq     int               `json:"poll_freq"`
	PollInterval int               `json:"poll_interval"`
}

func (ns *NamespaceState) RLock()   { ns.mu.RLock() }
func (ns *NamespaceState) RUnlock() { ns.mu.RUnlock() }

// Event represents a change detected during polling.
type Event struct {
	Namespace string `json:"namespace"`
	UPSName   string `json:"ups_name"`
	VarName   string `json:"var_name"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
}

// Poller polls one NUT server and updates its NamespaceState.
type Poller struct {
	cfg    config.UpsdConfig
	state  *NamespaceState
	events chan<- Event
}

// NewPoller creates a new Poller for the given UpsdConfig.
func NewPoller(cfg config.UpsdConfig, state *NamespaceState, events chan<- Event) *Poller {
	return &Poller{cfg: cfg, state: state, events: events}
}

const dialTimeout = 10 * time.Second

// Run starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	// Initial full sync
	p.fullSync()

	// Ticker for full sync (poll_freq)
	fullTicker := time.NewTicker(time.Duration(p.cfg.PollFreq) * time.Second)
	defer fullTicker.Stop()

	// Ticker for quick status check (poll_interval)
	quickTicker := time.NewTicker(time.Duration(p.cfg.PollInterval) * time.Second)
	defer quickTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fullTicker.C:
			p.fullSync()
		case <-quickTicker.C:
			p.quickSync()
		}
	}
}

// fullSync does a complete refresh of all UPS devices and their variables.
func (p *Poller) fullSync() {
	c, err := p.dial()
	if err != nil {
		p.setStatus("disconnected")
		log.Printf("[poller/%s] connect error: %v", p.cfg.Name, err)
		return
	}
	defer c.Close()

	// Get server version/protocol
	ver, _ := c.GetVer()
	netver, _ := c.GetNetVer()

	// List all UPS
	upsList, err := c.ListUPS()
	if err != nil {
		p.setStatus("error")
		log.Printf("[poller/%s] LIST UPS error: %v", p.cfg.Name, err)
		return
	}

	devices := make(map[string]*UPSDevice, len(upsList))
	for _, entry := range upsList {
		dev := &UPSDevice{
			Name:        entry.Name,
			Description: entry.Description,
			Vars:        make(map[string]string),
			RWVars:      make(map[string]string),
		}

		// Variables
		vars, err := c.ListVar(entry.Name)
		if err != nil {
			log.Printf("[poller/%s] LIST VAR %s: %v", p.cfg.Name, entry.Name, err)
		}
		for _, v := range vars {
			dev.Vars[v.Name] = v.Value
		}

		// Commands
		cmds, err := c.ListCmd(entry.Name)
		if err != nil {
			log.Printf("[poller/%s] LIST CMD %s: %v", p.cfg.Name, entry.Name, err)
		}
		dev.Commands = cmds

		// RW Vars
		rwVars, err := c.ListRW(entry.Name)
		if err != nil {
			log.Printf("[poller/%s] LIST RW %s: %v", p.cfg.Name, entry.Name, err)
		}
		for _, v := range rwVars {
			dev.RWVars[v.Name] = v.Value
		}

		devices[entry.Name] = dev
	}

	now := time.Now()
	p.state.mu.Lock()
	p.state.Status = "connected"
	p.state.Version = ver
	p.state.ProtVersion = netver
	p.state.LastSync = &now

	// Emit events for changed variables
	for name, dev := range devices {
		if old, ok := p.state.Devices[name]; ok {
			for varName, newVal := range dev.Vars {
				if oldVal, exists := old.Vars[varName]; exists && oldVal != newVal {
					select {
					case p.events <- Event{
						Namespace: p.cfg.Name,
						UPSName:   name,
						VarName:   varName,
						OldValue:  oldVal,
						NewValue:  newVal,
					}:
					default:
					}
				}
			}
		}
	}

	p.state.Devices = devices
	p.state.mu.Unlock()
}

// quickSync updates only the ups.status variable for each UPS (frequent lightweight poll).
func (p *Poller) quickSync() {
	p.state.mu.RLock()
	deviceNames := make([]string, 0, len(p.state.Devices))
	for n := range p.state.Devices {
		deviceNames = append(deviceNames, n)
	}
	p.state.mu.RUnlock()

	if len(deviceNames) == 0 {
		return
	}

	c, err := p.dial()
	if err != nil {
		p.setStatus("disconnected")
		return
	}
	defer c.Close()

	for _, name := range deviceNames {
		val, err := c.GetVar(name, "ups.status")
		if err != nil {
			continue
		}

		p.state.mu.Lock()
		if dev, ok := p.state.Devices[name]; ok {
			if old := dev.Vars["ups.status"]; old != val {
				select {
				case p.events <- Event{
					Namespace: p.cfg.Name,
					UPSName:   name,
					VarName:   "ups.status",
					OldValue:  old,
					NewValue:  val,
				}:
				default:
				}
				dev.Vars["ups.status"] = val
			}
		}
		p.state.mu.Unlock()
	}
}

func (p *Poller) setStatus(status string) {
	p.state.mu.Lock()
	p.state.Status = status
	p.state.mu.Unlock()
}

func (p *Poller) dial() (*nut.Client, error) {
	c, err := nut.Dial(p.cfg.Address, p.cfg.Port, dialTimeout)
	if err != nil {
		return nil, err
	}
	if p.cfg.Username != "" {
		if err := c.Auth(p.cfg.Username, p.cfg.Password); err != nil {
			c.Close()
			return nil, err
		}
	}
	return c, nil
}
