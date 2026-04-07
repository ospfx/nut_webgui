// Package config handles loading and merging configuration for nut_webgui.
// Priority (highest to lowest): CLI flags > environment variables > config file.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// UpsdConfig holds connection settings for a single NUT server.
type UpsdConfig struct {
	Name          string `toml:"-"       json:"name"`
	Address       string `toml:"address" json:"address"`
	Port          int    `toml:"port"    json:"port"`
	Username      string `toml:"username" json:"-"`
	Password      string `toml:"password" json:"-"`
	PollFreq      int    `toml:"poll_freq"     json:"poll_freq"`
	PollInterval  int    `toml:"poll_interval" json:"poll_interval"`
	MaxConnection int    `toml:"max_connection" json:"max_connection"`
}

// HTTPServerConfig holds HTTP server configuration.
type HTTPServerConfig struct {
	Listen      string `toml:"listen"       json:"listen"`
	Port        int    `toml:"port"         json:"port"`
	BasePath    string `toml:"base_path"    json:"base_path"`
	WorkerCount int    `toml:"worker_count" json:"worker_count"`
}

// Config is the top-level application configuration.
type Config struct {
	LogLevel     string                `toml:"log_level"     json:"log_level"`
	DefaultTheme string                `toml:"default_theme" json:"default_theme"`
	HTTPServer   HTTPServerConfig      `toml:"http_server"   json:"http_server"`
	Upsd         map[string]UpsdConfig `toml:"upsd"          json:"-"`
	UpsdList     []UpsdConfig          `toml:"-"             json:"-"`
}

// defaults fills missing values with sensible defaults.
func (c *Config) defaults() {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.HTTPServer.Listen == "" {
		c.HTTPServer.Listen = "0.0.0.0"
	}
	if c.HTTPServer.Port == 0 {
		c.HTTPServer.Port = 9000
	}
	if c.HTTPServer.BasePath == "" {
		c.HTTPServer.BasePath = "/"
	}
	for i := range c.UpsdList {
		if c.UpsdList[i].Port == 0 {
			c.UpsdList[i].Port = 3493
		}
		if c.UpsdList[i].PollFreq == 0 {
			c.UpsdList[i].PollFreq = 30
		}
		if c.UpsdList[i].PollInterval == 0 {
			c.UpsdList[i].PollInterval = 2
		}
		if c.UpsdList[i].MaxConnection == 0 {
			c.UpsdList[i].MaxConnection = 4
		}
	}
}

// Load reads configuration from the config file, environment variables, and CLI flags.
func Load() (*Config, error) {
	// --- CLI flags ---
	var (
		flagConfigFile   = flag.String("config-file", "/etc/nut_webgui/config.toml", "Path to config.toml")
		flagListen       = flag.String("listen", "", "HTTP listen address")
		flagPort         = flag.Int("port", 0, "HTTP listen port")
		flagBasePath     = flag.String("base-path", "", "HTTP base path")
		flagLogLevel     = flag.String("log-level", "", "Log level (error|warn|info|debug)")
		flagDefaultTheme = flag.String("default-theme", "", "Default UI theme")
		flagAllowEnv     = flag.Bool("allow-env", true, "Allow environment variable configuration")
	)
	flag.Parse()

	cfg := &Config{
		Upsd: make(map[string]UpsdConfig),
	}

	// --- Config file ---
	configPath := *flagConfigFile
	if envPath := readEnvOrFile("NUTWG__CONFIG_FILE", "CONFIG_FILE"); envPath != "" {
		configPath = envPath
	}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", configPath, err)
		}
	}

	// Flatten toml upsd map into list
	for name, u := range cfg.Upsd {
		u.Name = name
		cfg.UpsdList = append(cfg.UpsdList, u)
	}

	// --- Environment variables (when allowed) ---
	if *flagAllowEnv {
		applyEnv(cfg)
	}

	// --- CLI flags (highest priority) ---
	if *flagListen != "" {
		cfg.HTTPServer.Listen = *flagListen
	}
	if *flagPort != 0 {
		cfg.HTTPServer.Port = *flagPort
	}
	if *flagBasePath != "" {
		cfg.HTTPServer.BasePath = *flagBasePath
	}
	if *flagLogLevel != "" {
		cfg.LogLevel = *flagLogLevel
	}
	if *flagDefaultTheme != "" {
		cfg.DefaultTheme = *flagDefaultTheme
	}

	// Ensure defaults for any missing fields
	cfg.defaults()

	// If no upsd configured, check for single-server env config
	if len(cfg.UpsdList) == 0 {
		u := buildDefaultUpsdFromEnv()
		if u.Address != "" {
			u.Name = "default"
			cfg.UpsdList = append(cfg.UpsdList, u)
		}
	}

	return cfg, nil
}

// applyEnv overlays environment variable settings.
func applyEnv(cfg *Config) {
	if v := readEnvOrFile("NUTWG__LOG_LEVEL", "LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := readEnvOrFile("NUTWG__DEFAULT_THEME", "DEFAULT_THEME"); v != "" {
		cfg.DefaultTheme = v
	}
	if v := readEnvOrFile("NUTWG__HTTP_SERVER__LISTEN", "LISTEN"); v != "" {
		cfg.HTTPServer.Listen = v
	}
	if v := readEnvOrFile("NUTWG__HTTP_SERVER__BASE_PATH", "BASE_PATH"); v != "" {
		cfg.HTTPServer.BasePath = v
	}
	if v := readEnvOrFile("NUTWG__HTTP_SERVER__PORT", "PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.HTTPServer.Port = p
		}
	}
}

// buildDefaultUpsdFromEnv builds a single UpsdConfig from the simple UPSD_* env vars.
func buildDefaultUpsdFromEnv() UpsdConfig {
	u := UpsdConfig{
		Port:         3493,
		PollFreq:     30,
		PollInterval: 2,
		MaxConnection: 4,
	}
	if v := readEnvOrFile("NUTWG__UPSD__ADDRESS", "UPSD_ADDR"); v != "" {
		u.Address = v
	}
	if v := readEnvOrFile("NUTWG__UPSD__PORT", "UPSD_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			u.Port = p
		}
	}
	if v := readEnvOrFile("NUTWG__UPSD__USERNAME", "UPSD_USER"); v != "" {
		u.Username = v
	}
	if v := readEnvOrFile("NUTWG__UPSD__PASSWORD", "UPSD_PASS"); v != "" {
		u.Password = v
	}
	if v := readEnvOrFile("NUTWG__UPSD__POLL_FREQ", "POLL_FREQ"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			u.PollFreq = n
		}
	}
	if v := readEnvOrFile("NUTWG__UPSD__POLL_INTERVAL", "POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			u.PollInterval = n
		}
	}
	return u
}

// readEnvOrFile reads an environment variable, falling back to an alias.
// If the value looks like an absolute file path, it reads the file contents.
func readEnvOrFile(key, alias string) string {
	v := os.Getenv(key)
	if v == "" && alias != "" {
		v = os.Getenv(alias)
	}
	if v == "" {
		return ""
	}
	// If value looks like a file path, read the file
	if strings.HasPrefix(v, "/") {
		if data, err := os.ReadFile(v); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return v
}
