package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ospfx/nut_webgui/internal/config"
	"github.com/ospfx/nut_webgui/internal/poller"
	"github.com/ospfx/nut_webgui/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	events := make(chan poller.Event, 512)

	// Build namespace states and start pollers
	namespaces := make(map[string]*poller.NamespaceState, len(cfg.UpsdList))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, upsd := range cfg.UpsdList {
		ns := &poller.NamespaceState{
			Namespace:    upsd.Name,
			Address:      upsd.Address,
			Port:         upsd.Port,
			Status:       "initializing",
			Devices:      make(map[string]*poller.UPSDevice),
			PollFreq:     upsd.PollFreq,
			PollInterval: upsd.PollInterval,
		}
		namespaces[upsd.Name] = ns

		p := poller.NewPoller(upsd, ns, events)
		go p.Run(ctx)
	}

	if len(namespaces) == 0 {
		log.Println("WARNING: no UPSD servers configured. Set UPSD_ADDR / UPSD_USER / UPSD_PASS environment variables or provide a config.toml")
	}

	appState := &server.AppState{
		Config:     cfg,
		Namespaces: namespaces,
		Events:     events,
	}

	srv, err := server.New(appState)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	if err := srv.ListenAndServe(ctx); err != nil {
		log.Fatalf("server: %v", err)
	}
}
