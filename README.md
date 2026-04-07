# NUT Web GUI — Go Edition

A lightweight web interface for [Network UPS Tools (NUT)](https://networkupstools.org/), fully implemented in Go.

## Features

- 📊 Monitors UPS variables with auto-refresh
- ⚡ Supports INSTCMD, SET VAR, and FSD from the web interface
- 🔌 JSON REST API for integration and automation
- 🌐 WebSocket `/events` endpoint for real-time UPS event streaming
- 🩺 Probe endpoints (`/probes/health`, `/probes/readiness`)
- 🔧 Multiple NUT server namespaces supported
- 🪶 Small binary – single executable with embedded assets

## Quick Start

### Docker / Podman

```bash
docker run -p 9000:9000 \
  -e UPSD_ADDR=10.0.0.1 \
  -e UPSD_USER=monuser \
  -e UPSD_PASS=secret \
  ghcr.io/ospfx/nut_webgui:latest
```

Open `http://localhost:9000` in your browser.

### Binary

```bash
UPSD_ADDR=10.0.0.1 UPSD_USER=monuser UPSD_PASS=secret ./nut-webgui
```

## Configuration

Configuration is loaded in priority order:

1. **CLI flags** (highest priority)
2. **Environment variables**
3. **Config file** (`/etc/nut_webgui/config.toml` by default)

### Environment Variables (Simple Setup)

| Variable        | Default   | Description                   |
|-----------------|-----------|-------------------------------|
| `UPSD_ADDR`     | —         | NUT server address            |
| `UPSD_PORT`     | `3493`    | NUT server port               |
| `UPSD_USER`     | —         | NUT username                  |
| `UPSD_PASS`     | —         | NUT password                  |
| `PORT`          | `9000`    | HTTP listen port              |
| `LISTEN`        | `0.0.0.0` | HTTP listen address           |
| `BASE_PATH`     | `/`       | Base path (for reverse proxy) |
| `POLL_FREQ`     | `30`      | Full sync interval (seconds)  |
| `POLL_INTERVAL` | `2`       | Status check interval (s)     |

### TOML Config File

```toml
[http_server]
listen    = "0.0.0.0"
port      = 9000
base_path = "/"

[upsd.default]
address       = "localhost"
port          = 3493
username      = "monuser"
password      = "secret"
poll_freq     = 30
poll_interval = 2

# Multiple NUT servers:
[upsd.secondary]
address  = "10.0.1.5"
username = "observer"
password = "pass"
```

### CLI Flags

```
--config-file    Config file path (default: /etc/nut_webgui/config.toml)
--listen         HTTP listen address
--port           HTTP port
--base-path      Base path for reverse proxy
--log-level      Log level
--allow-env      Load config from environment variables (default: true)
```

## Building from Source

**Requirements:** Go 1.22+

```bash
git clone https://github.com/ospfx/nut_webgui
cd nut_webgui
make build          # produces ./bin/nut-webgui
make test           # run tests
make run            # build and run (requires UPSD_ADDR etc.)
```

## JSON API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/` | List all namespaces |
| GET | `/api/{ns}` | Namespace details |
| GET | `/api/{ns}/devices` | List UPS devices |
| GET | `/api/{ns}/devices/{ups}` | UPS details |
| PATCH | `/api/{ns}/devices/{ups}` | Set variable (`{"name":"…","value":"…"}`) |
| POST | `/api/{ns}/devices/{ups}/instcmd` | Run instant command (`{"command":"…"}`) |
| POST | `/api/{ns}/devices/{ups}/fsd` | Force shutdown |

## Probes

| Endpoint | Description |
|----------|-------------|
| `GET /probes/health` | Always 200 OK |
| `GET /probes/readiness` | 200 when ≥1 namespace connected |
| `GET /probes/health/{ns}` | Namespace health |
| `GET /probes/readiness/{ns}` | Namespace readiness |

## WebSocket Events

```javascript
const ws = new WebSocket('ws://localhost:9000/events');
ws.onmessage = (ev) => {
  const { namespace, ups_name, var_name, old_value, new_value } = JSON.parse(ev.data);
};
```

## Project Structure

```
cmd/nut-webgui/     Entry point
internal/
  config/           Configuration loading
  nut/              NUT protocol TCP client
  poller/           Background polling service
  server/           HTTP server, API handlers, templates
web/
  templates/        HTML templates
  static/           CSS and JS assets
```

## License

Apache-2.0
