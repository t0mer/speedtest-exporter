# speedtest-exporter

Monitor your internet connection speed with automated tests, a live dashboard, Prometheus metrics, and multi-channel notifications. Built in pure Go — no CGO, no dependencies beyond the binary.

## Features

- **Live dashboard** — animated download gauge, real-time sparkline chart, paginated results table with ▲/▼ trend arrows
- **Live test progress** — watch ping, download, and upload update in real time via Server-Sent Events as each phase completes
- **Two test backends** — built-in pure-Go engine (no binary needed) or the official Ookla CLI
- **Preferred server** — pick a specific Speedtest.net server; automatic fallback to nearest if it fails
- **Scheduled tests** — cron-based automation (`@every 1h`, `0 */6 * * *`, etc.)
- **Prometheus metrics** — scrape `/metrics` for Grafana dashboards
- **Threshold notifications** — alert on slow speeds or high latency via Shoutrrr (Slack, Discord, Telegram, SMTP…), GreenAPI (WhatsApp cloud), or self-hosted WhatsApp Web
- **AES-256-GCM encrypted credentials** — notification secrets never stored in plaintext
- **Settings UI** — all runtime config editable in the browser without a restart
- **Responsive, mobile-friendly** — bottom navigation, full gauge layout on any screen size

---

## Screenshots

### Dashboard

![Dashboard](assets/screenshots/dashboard.png)

The dashboard shows the latest download speed on an animated arc gauge, secondary metrics (upload, ping, jitter, server), a 7-day summary, a dual-line speed history chart, and a paginated results table. Each row includes ▲/▼ arrows that compare the result to the previous test — green for improvement, red for regression.

### Mobile

![Mobile](assets/screenshots/mobile.png)

Fully responsive at 390 px wide. The bottom navigation bar provides one-thumb access to Dashboard, Settings, and the Run Test button.

---

## Quick Start

### Docker (recommended)

```bash
docker run -d \
  --name speedtest-exporter \
  --restart unless-stopped \
  -p 9090:9090 \
  -v $(pwd)/data:/data \
  -e SPEEDTEST_DATA_DIR=/data \
  -e SPEEDTEST_SCHEDULE="@every 1h" \
  techblog/speedtest-exporter:latest
```

Open **http://localhost:9090** in your browser.

### Docker Compose (with Prometheus + Grafana)

```bash
curl -O https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/docker-compose.yml
docker compose up -d
```

| Service | URL |
|---|---|
| speedtest-exporter | http://localhost:9090 |
| Prometheus | http://localhost:9091 |
| Grafana | http://localhost:3000 |

### Binary

Download the latest release from the [Releases page](https://github.com/t0mer/speedtest-exporter/releases), then:

```bash
chmod +x speedtest-exporter-linux-amd64
./speedtest-exporter-linux-amd64 serve --config config.example.yaml
```

### Build from source

```bash
git clone https://github.com/t0mer/speedtest-exporter.git
cd speedtest-exporter
CGO_ENABLED=0 go build ./cmd/speedtest-exporter/
./speedtest-exporter serve
```

---

## CLI

```
speedtest-exporter [command] [flags]

Commands:
  run     Run a single speed test and print the result
  serve   Start the HTTP server (API, /metrics, Web UI, scheduler)

Flags:
  --config string   Path to YAML config file
  --version         Print version and exit
  --help            Show help
```

### Examples

```bash
# One-shot test, human-readable output
./speedtest-exporter run

# One-shot test, JSON output
./speedtest-exporter run --json

# Start the daemon with a config file
./speedtest-exporter serve --config /etc/speedtest-exporter/config.yaml
```

---

## Configuration

Configuration is layered: **built-in defaults → YAML file → environment variables**.

Environment variables use the `SPEEDTEST_` prefix with `_` for nesting, e.g. `SPEEDTEST_SERVER_PORT=9090`.

All runtime settings (engine, schedule, thresholds, notifications) can also be changed at any time through the **Settings** tab in the Web UI without restarting the server.

### config.yaml

```yaml
# Backend: "go" (no binary) or "ookla" (requires Speedtest CLI)
engine: go
ookla_path: speedtest

data_dir: ./data
log_level: info          # debug | info | warning | error
schedule: "@every 1h"    # cron expression; leave empty to disable

server:
  host: 0.0.0.0
  port: 9090
  read_timeout: 10       # seconds
  write_timeout: 120     # seconds (sized for a full 30–60 s test)
  enable_ui: true

thresholds:              # 0 = disabled
  min_download_mbps: 0
  min_upload_mbps: 0
  max_ping_ms: 0
  max_jitter_ms: 0
  max_packet_loss_ratio: 0
  cooldown_minutes: 30   # minimum gap between breach alerts

webhooks: []             # legacy webhook URLs for threshold alerts
```

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `SPEEDTEST_ENGINE` | `go` | `go` or `ookla` |
| `SPEEDTEST_DATA_DIR` | `./data` | SQLite database directory |
| `SPEEDTEST_LOG_LEVEL` | `info` | `debug` / `info` / `warning` / `error` |
| `SPEEDTEST_SCHEDULE` | `@every 1h` | Cron schedule (empty to disable) |
| `SPEEDTEST_SERVER_PORT` | `9090` | HTTP listen port |
| `SPEEDTEST_SERVER_HOST` | `0.0.0.0` | HTTP listen address |
| `SPEEDTEST_THRESHOLDS_MIN_DOWNLOAD_MBPS` | `0` | Minimum download (0 = off) |
| `SPEEDTEST_THRESHOLDS_MIN_UPLOAD_MBPS` | `0` | Minimum upload (0 = off) |
| `SPEEDTEST_THRESHOLDS_MAX_PING_MS` | `0` | Maximum ping (0 = off) |
| `SPEEDTEST_THRESHOLDS_MAX_JITTER_MS` | `0` | Maximum jitter (0 = off) |
| `SPEEDTEST_THRESHOLDS_MAX_PACKET_LOSS_RATIO` | `0` | Maximum packet loss ratio 0–1 (0 = off) |
| `SPEEDTEST_THRESHOLDS_COOLDOWN_MINUTES` | `30` | Minimum minutes between alerts |

---

## Prometheus Metrics

Scrape endpoint: `GET /metrics`

| Metric | Type | Description |
|---|---|---|
| `speedtest_download_mbps` | Gauge | Latest download speed (Mbps) |
| `speedtest_upload_mbps` | Gauge | Latest upload speed (Mbps) |
| `speedtest_ping_ms` | Gauge | Latest ping latency (ms) |
| `speedtest_jitter_ms` | Gauge | Latest jitter (ms) |
| `speedtest_packet_loss_ratio` | Gauge | Latest packet loss (0–1) |
| `speedtest_last_test_timestamp_seconds` | Gauge | Unix timestamp of last completed test |
| `speedtest_server_info` | Gauge | Constant 1, labelled with `server_name`, `server_id`, `isp` |
| `speedtest_tests_total` | Counter | Total tests, labelled `source` × `outcome` |
| `speedtest_test_duration_seconds` | Histogram | Test duration distribution |
| `speedtest_threshold_breaches_total` | Counter | Threshold breaches per metric |

### Example prometheus.yml scrape config

```yaml
scrape_configs:
  - job_name: speedtest
    static_configs:
      - targets: ['speedtest-exporter:9090']
    scrape_interval: 60s
```

---

## REST API

All endpoints return JSON.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/test` | Run a test synchronously, return result |
| `POST` | `/api/test/stream` | Run a test, stream SSE progress events |
| `GET` | `/api/results` | List results (`limit`, `offset`, `since`, `until`) |
| `GET` | `/api/results/latest` | Most recent result |
| `GET` | `/api/results/{id}` | Single result by ID |
| `GET` | `/api/summary?days=N` | Aggregate stats over N days |
| `GET` | `/api/compare?a=ID&b=ID` | Compare two results side by side |
| `GET` | `/api/servers` | List nearby Speedtest.net servers |
| `GET` | `/api/settings` | Read current runtime settings |
| `PUT` | `/api/settings` | Update runtime settings (applied immediately) |
| `GET` | `/api/notifications` | List notification channels |
| `POST` | `/api/notifications` | Add a channel |
| `PUT` | `/api/notifications/{id}` | Update a channel |
| `DELETE` | `/api/notifications/{id}` | Remove a channel |
| `POST` | `/api/notifications/test` | Send a test message |
| `GET` | `/metrics` | Prometheus exposition |
| `GET` | `/healthz` | Liveness probe (`{"status":"ok"}`) |

### SSE stream format (`/api/test/stream`)

```
data: {"phase":"connecting","server_name":"NYC Test","server_id":"12345"}

data: {"phase":"ping","ping_ms":12.5}

data: {"phase":"download","download_mbps":234.7}

data: {"phase":"upload","upload_mbps":48.2}

data: {"phase":"done","download_mbps":891.3,"upload_mbps":49.1,"ping_ms":11.8}
```

---

## Notifications

Channels are configured in the **Alerts** tab of the Web UI. Credentials are encrypted at rest with AES-256-GCM.

### Shoutrrr

Covers Slack, Discord, Telegram, Gotify, SMTP, ntfy, and more via a single URL scheme.

| Provider | URL format |
|---|---|
| Slack | `slack://token@channel` |
| Discord | `discord://token@webhook-id` |
| Telegram | `telegram://token@telegram?chats=@channel` |
| Gotify | `gotify://hostname/token` |
| SMTP | `smtp://user:pass@host:port/?from=a@b&to=c@d` |
| ntfy | `ntfy://topic@host` |

### GreenAPI (WhatsApp cloud)

Requires a [GreenAPI](https://green-api.com) account. Fields: Instance ID, Token, Recipient Phone (international format, no `+`).

### WhatsApp Web (self-hosted)

Requires a running [go-whatsapp-web-multidevice](https://github.com/aldinokemal/go-whatsapp-web-multidevice) instance. Fields: Base URL, Recipient Phone, optional Basic Auth.

> **Security:** Outbound URLs for GreenAPI and WhatsApp Web are validated at connection time using a dial-level IP check (defeating DNS rebinding). The Shoutrrr `generic://` scheme is blocked.

---

## Preferred Server

In **Settings → Engine → Browse**, pick any server from the distance-sorted list returned by `/api/servers`. The Go backend tries your preferred server first; if it is unreachable or the test fails at any phase, the nearest available server is used automatically and a warning is logged.

---

## Architecture

```
cmd/speedtest-exporter/     CLI (cobra): run | serve
internal/
  config/     Layered config: defaults < YAML < env vars
  database/   SQLite store (modernc.org/sqlite, CGO_ENABLED=0)
  runner/     Runner interface + Go backend + Ookla CLI backend
  metrics/    Prometheus collectors (private registry)
  notify/     Threshold evaluation + legacy webhook sender
  notifications/  Multi-channel store (AES-256-GCM) + Shoutrrr/GreenAPI/WhatsApp senders
  scheduler/  Cron scheduler with skip-if-running guard
  service/    Service.Run — single pipeline: test → persist → metrics → notify
  api/        chi HTTP server: REST endpoints, /metrics, SSE stream, Web UI
  crypto/     AES-256-GCM helpers + key-file management
web/          Embedded dashboard (go:embed), pure HTML/CSS/JS
```

`Service.Run()` is the single code path for all test triggers (CLI, API, scheduler). The same goroutine chain — runner → persist → metrics → evaluate → notify — executes regardless of what started the test.

---

## Development

```bash
# Install dependencies
go mod tidy

# Run a one-shot test
go run ./cmd/speedtest-exporter run

# Start the server (hot-reload not included; rebuild manually)
go run ./cmd/speedtest-exporter serve --config config.example.yaml

# Tests
go test -race ./...

# Cross-compile all targets
VERSION=2026.5.0 ./scripts/build.sh
```

---

## License

MIT — see [LICENSE](LICENSE).
