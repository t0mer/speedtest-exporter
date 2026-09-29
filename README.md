# speedtest-exporter

Monitor your internet connection with scheduled speed tests, a live web dashboard, Prometheus metrics, and multi-channel notifications. It is a single, statically linked Go binary (no CGO) with an embedded web UI and a local SQLite database. It is built for homelabs and self-hosters who want to track ISP performance over time and get told when a test fails.

[![Release](https://img.shields.io/github/v/release/t0mer/speedtest-exporter)](https://github.com/t0mer/speedtest-exporter/releases)
[![Docker Hub](https://img.shields.io/docker/v/techblog/speedtest-exporter?label=docker%20hub&sort=date)](https://hub.docker.com/r/techblog/speedtest-exporter)
[![License](https://img.shields.io/github/license/t0mer/speedtest-exporter)](LICENSE)

## Table of Contents

- [Features](#features)
- [Screenshots](#screenshots)
- [How It Works](#how-it-works)
- [Requirements](#requirements)
- [Installation](#installation)
- [CLI](#cli)
- [Configuration](#configuration)
- [Settings (Web UI)](#settings-web-ui)
- [Prometheus Metrics](#prometheus-metrics)
- [REST API](#rest-api)
- [Notifications](#notifications)
- [Preferred Server](#preferred-server)
- [Data Storage](#data-storage)
- [Security Notes](#security-notes)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Live dashboard**: animated download gauge, 7-day summary, speed history chart, and a paginated results table with ▲/▼ trend arrows. The dashboard refreshes every 60 seconds.
- **Live test progress**: ping, download, and upload update in real time over a Server-Sent Events stream while a test runs.
- **Two test backends**: a built-in pure-Go engine ([showwin/speedtest-go](https://github.com/showwin/speedtest-go), no external binary) or the official [Ookla Speedtest CLI](https://www.speedtest.net/apps/cli). Both test against Speedtest.net servers.
- **Preferred server**: pick a specific Speedtest.net server from a distance-sorted list (Go engine only). If it fails, the test falls back to the nearest server.
- **Scheduled tests**: cron-based automation (`@every 1h`, `0 */6 * * *`, and so on). A scheduled run is skipped if the previous one is still in progress.
- **Prometheus metrics**: scrape `/metrics` for Grafana dashboards and alerting.
- **Notification channels**: send a message after every successful or failed test through Shoutrrr (Slack, Discord, Telegram, SMTP, ntfy, Gotify…), Green-API (WhatsApp cloud), or a self-hosted WhatsApp Web gateway.
- **Threshold webhooks**: POST a JSON payload to your own webhook URLs when download, upload, ping, jitter, or packet loss breach a limit.
- **Encrypted credentials**: notification channel configs are stored with AES-256-GCM.
- **Date and time format**: choose how timestamps are shown across the dashboard (ISO, US, EU, 12h/24h).
- **Settings export / import**: back up all settings and notification channels to a JSON file, with optional passphrase-based encryption of channel credentials.
- **Settings UI**: all runtime settings are editable in the browser without a restart.
- **Responsive, mobile-friendly**: bottom navigation bar and a full gauge layout on any screen size.

---

## Screenshots

### Dashboard

![Dashboard](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/dashboard.png)

The dashboard shows the latest download speed on an animated arc gauge, secondary metrics (upload, ping, jitter, server), a 7-day summary, a dual-line speed history chart, and a paginated results table. Each row includes ▲/▼ arrows that compare the result to the previous test: green for an improvement, red for a regression.

### Settings

![Settings](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/settings.png)

All runtime configuration is editable in the browser without a restart. Settings are grouped into **Display** (date/time format), **Engine** (test backend and preferred server), **Schedule** (cron expression), **Thresholds** (breach limits), **Webhooks**, and **Export / Import**.

### Export / Import

![Export / Import](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/export-import.png)

Back up all settings and notification channels to a JSON file. Choose **Export (Encrypted)** to protect channel credentials with PBKDF2 + AES-256-GCM using a stored passphrase, or **Export (Unencrypted)** for plaintext. The export passphrase is never written into the file. Use **Import** to restore on any machine; set the same passphrase first when importing an encrypted file.

### Server Picker

![Server Picker](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/server-picker.png)

Browse and search nearby Speedtest.net servers sorted by distance. Selecting a preferred server pins future Go-engine tests to that host; if it fails, the nearest available server is used automatically.

### Alerts (Notification Channels)

![Notifications](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/notifications.png)

The Alerts tab manages notification channels. Each channel uses Shoutrrr (Slack, Discord, Telegram, SMTP, ntfy, Gotify…), Green-API (WhatsApp cloud), or a self-hosted WhatsApp Web gateway.

### Add Channel

![Add Channel](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/add-channel.png)

The Add Channel dialog supports all three providers. Toggle **Enabled**, **Notify on success**, and **Notify on failure** independently, and use **Send Test** to send a real message before saving.

### Mobile

![Mobile](https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/assets/screenshots/mobile.png)

Fully responsive at 390 px wide. The bottom navigation bar gives one-thumb access to Home (dashboard), Settings, the Run Test button, and Alerts.

---

## How It Works

```mermaid
flowchart LR
    CLI["CLI: run"] --> SVC
    API["REST API / Web UI"] --> SVC
    SCHED["Cron scheduler"] --> SVC
    SVC["Service.Run"] --> RUN["Runner<br/>Go engine or Ookla CLI"]
    RUN --> ST[("Speedtest.net servers")]
    SVC --> DB[("SQLite<br/>results.db")]
    SVC --> MET["Prometheus gauges<br/>/metrics"]
    SVC --> THR["Threshold check<br/>→ webhooks"]
    SVC --> CH["Notification channels<br/>Shoutrrr / Green-API / WhatsApp Web"]
```

Every test, whether started from the CLI, the API, the web UI, or the scheduler, goes through the same pipeline: **run → persist → update metrics → evaluate thresholds → notify**.

- The result is saved to SQLite and the Prometheus gauges are updated.
- Thresholds are evaluated. Breaches increment `speedtest_threshold_breaches_total` and, subject to the cooldown, are POSTed to the configured webhook URLs.
- Notification channels are called asynchronously so they never block the test. A successful test notifies channels with **Notify on success**; a failed test (runner error) notifies channels with **Notify on failure**. Channel notifications are not tied to thresholds.

Channel notifications are wired up by `serve` only. A one-shot `run` from the CLI stores the result and fires threshold webhooks, but does not send channel notifications.

### Project layout

```
cmd/speedtest-exporter/     CLI (cobra): run | serve
internal/
  config/         Layered config: defaults < YAML < env vars (viper)
  database/       SQLite store (modernc.org/sqlite, CGO_ENABLED=0)
  runner/         Runner interface + Go backend + Ookla CLI backend + progress events
  metrics/        Prometheus collectors (private registry)
  notify/         Threshold evaluation + webhook sender
  notifications/  Channel store (AES-256-GCM) + Shoutrrr / Green-API / WhatsApp Web senders + URL checks
  scheduler/      Cron scheduler (robfig/cron v3) with skip-if-running guard
  service/        Service.Run: test → persist → metrics → notify
  api/            chi HTTP server: REST endpoints, /metrics, SSE stream, web UI
  crypto/         AES-256-GCM helpers, PBKDF2 key derivation, key-file management
  model/          Shared types (results, settings, export format)
web/              Embedded dashboard (go:embed), plain HTML/CSS/JS
```

---

## Requirements

- Outbound internet access to Speedtest.net (server list and test servers).
- **Go engine** (default): nothing else. The binary is self-contained.
- **Ookla engine**: the [Ookla Speedtest CLI](https://www.speedtest.net/apps/cli) installed on the host and reachable at `ookla_path`. The CLI is **not** included in the Docker image. speedtest-exporter runs it with `--accept-license --accept-gdpr`, so using this engine means you accept Ookla's license and GDPR terms.
- To build from source: Go 1.25 or newer.

---

## Installation

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

The image is published to Docker Hub as [`techblog/speedtest-exporter`](https://hub.docker.com/r/techblog/speedtest-exporter) (tags `latest` and versioned tags such as `2026.5.2`) for `linux/amd64`, `linux/arm64`, and `linux/arm/v7`. It is built `FROM scratch`: it contains only the binary and CA certificates, its entrypoint is `speedtest-exporter serve`, and `/data` is declared as a volume. Configure it with the `SPEEDTEST_*` environment variables below, or mount a YAML file and pass `--config`:

```bash
docker run -d -p 9090:9090 -v $(pwd)/data:/data \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  techblog/speedtest-exporter:latest --config /config.yaml
```

### Docker Compose (with Prometheus + Grafana)

The bundled [`docker-compose.yml`](docker-compose.yml) starts speedtest-exporter, Prometheus, and Grafana. Prometheus reads its scrape config from [`deploy/prometheus.yml`](deploy/prometheus.yml), so fetch both files:

```bash
mkdir -p speedtest-exporter/deploy && cd speedtest-exporter
curl -O https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/docker-compose.yml
curl -o deploy/prometheus.yml https://raw.githubusercontent.com/t0mer/speedtest-exporter/main/deploy/prometheus.yml
docker compose up -d
```

| Service | URL |
|---|---|
| speedtest-exporter | http://localhost:9090 |
| Prometheus | http://localhost:9091 |
| Grafana | http://localhost:3000 |

Grafana starts empty: add a Prometheus data source pointing at `http://prometheus:9090` and build a dashboard from the [metrics](#prometheus-metrics) below. No dashboard is shipped with the project.

### Binary

Download the binary for your platform from the [Releases page](https://github.com/t0mer/speedtest-exporter/releases). Each release ships:

| OS | Assets |
|---|---|
| Linux | `speedtest-exporter-linux-amd64`, `-linux-arm64`, `-linux-arm-v7`, `-linux-arm-v6`, `-linux-386` |
| macOS | `speedtest-exporter-darwin-amd64`, `-darwin-arm64` |
| Windows | `speedtest-exporter-windows-amd64.exe`, `-windows-arm64.exe` |

```bash
chmod +x speedtest-exporter-linux-amd64
./speedtest-exporter-linux-amd64 serve
# or with a config file (see config.example.yaml in this repo)
./speedtest-exporter-linux-amd64 serve --config config.yaml
```

### Build from source

```bash
git clone https://github.com/t0mer/speedtest-exporter.git
cd speedtest-exporter
CGO_ENABLED=0 go build -o speedtest-exporter ./cmd/speedtest-exporter/
./speedtest-exporter serve
```

---

## CLI

```
speedtest-exporter [command] [flags]

Commands:
  run         Run a single speed test and print the result
  serve       Start the HTTP server (API, /metrics, Web UI) with optional scheduler
  completion  Generate a shell completion script
  help        Help about any command

Global flags:
  --config string   Path to config file (YAML)
  -v, --version     Print version and exit
  -h, --help        Show help

run flags:
  --json            Print result as JSON
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

`run` uses the YAML/env configuration only (engine, thresholds, webhooks). It does not read the settings saved in the web UI, and it stores the result in the same database with `source` = `manual`.

---

## Configuration

Configuration is layered. From lowest to highest priority:

1. Built-in defaults
2. YAML file passed with `--config`
3. Environment variables (`SPEEDTEST_` prefix, nested keys joined with `_`, e.g. `SPEEDTEST_SERVER_PORT=9090`)
4. **Settings saved in the web UI** (only for `serve`, and only for the runtime fields listed in [Settings](#settings-web-ui))

Once settings have been saved from the web UI (or imported), they are stored in the database and take precedence over YAML and environment values for engine, schedule, thresholds, and webhooks. Server options (`host`, `port`, timeouts, `enable_ui`), `data_dir`, `log_level`, and `ookla_path` always come from YAML/env.

### config.yaml

See [`config.example.yaml`](config.example.yaml):

```yaml
# Backend: "go" (no binary) or "ookla" (requires Speedtest CLI)
engine: go
ookla_path: speedtest

data_dir: ./data
log_level: info          # debug | info | warning | error
schedule: "@every 1h"    # cron expression; set to "" to disable

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
  cooldown_minutes: 30   # minimum gap between webhook breach alerts

webhooks: []             # URLs that receive a JSON POST on threshold breach
```

### All options

| YAML key | Environment variable | Default | Description |
|---|---|---|---|
| `engine` | `SPEEDTEST_ENGINE` | `go` | `go` (built-in) or `ookla` (Ookla CLI) |
| `ookla_path` | `SPEEDTEST_OOKLA_PATH` | `speedtest` | Path to the Ookla CLI binary (used when `engine: ookla`) |
| `data_dir` | `SPEEDTEST_DATA_DIR` | `./data` | Directory for the SQLite database and the encryption key |
| `log_level` | `SPEEDTEST_LOG_LEVEL` | `info` | `debug` / `info` / `warning` / `error` |
| `schedule` | `SPEEDTEST_SCHEDULE` | `@every 1h` | Cron schedule; `""` in YAML disables it |
| `server.host` | `SPEEDTEST_SERVER_HOST` | `0.0.0.0` | HTTP listen address |
| `server.port` | `SPEEDTEST_SERVER_PORT` | `9090` | HTTP listen port |
| `server.read_timeout` | `SPEEDTEST_SERVER_READ_TIMEOUT` | `10` | HTTP read timeout (seconds) |
| `server.write_timeout` | `SPEEDTEST_SERVER_WRITE_TIMEOUT` | `120` | HTTP write timeout and per-request timeout (seconds) |
| `server.enable_ui` | `SPEEDTEST_SERVER_ENABLE_UI` | `true` | Serve the web dashboard at `/` |
| `thresholds.min_download_mbps` | `SPEEDTEST_THRESHOLDS_MIN_DOWNLOAD_MBPS` | `0` | Minimum download in Mbps (0 = off) |
| `thresholds.min_upload_mbps` | `SPEEDTEST_THRESHOLDS_MIN_UPLOAD_MBPS` | `0` | Minimum upload in Mbps (0 = off) |
| `thresholds.max_ping_ms` | `SPEEDTEST_THRESHOLDS_MAX_PING_MS` | `0` | Maximum ping in ms (0 = off) |
| `thresholds.max_jitter_ms` | `SPEEDTEST_THRESHOLDS_MAX_JITTER_MS` | `0` | Maximum jitter in ms (0 = off) |
| `thresholds.max_packet_loss_ratio` | `SPEEDTEST_THRESHOLDS_MAX_PACKET_LOSS_RATIO` | `0` | Maximum packet loss ratio 0–1 (0 = off) |
| `thresholds.cooldown_minutes` | `SPEEDTEST_THRESHOLDS_COOLDOWN_MINUTES` | `30` | Minimum minutes between webhook breach alerts |
| `webhooks` | `SPEEDTEST_WEBHOOKS` | `[]` | Webhook URLs; comma-separated in the env var |

An environment variable set to an empty string is ignored, so `SPEEDTEST_SCHEDULE=""` does **not** disable the schedule. Use `schedule: ""` in YAML or clear the schedule in the web UI instead (see [Troubleshooting](#troubleshooting)).

---

## Settings (Web UI)

The fields below are editable in the **Settings** tab (or via `PUT /api/settings`) and take effect immediately without a restart. They are persisted in the SQLite database. Until you save for the first time, the UI shows the values from YAML/env.

### Display

| Field | Options | Description |
|---|---|---|
| Date Format | Default, `YYYY-MM-DD`, `MM/DD/YYYY`, `DD/MM/YYYY`, `DD.MM.YYYY` | How dates are shown across the dashboard. Default uses the browser locale. |
| Time Format | Default, `HH:mm`, `HH:mm:ss`, `hh:mm AM/PM`, `hh:mm:ss AM/PM` | How times are shown across the dashboard. Default uses the browser locale. |

### Engine

| Field | Options | Description |
|---|---|---|
| Test Backend | `go`, `ookla` | `go` uses the built-in pure-Go engine (no binary required). `ookla` runs the official Speedtest CLI. |
| Preferred Server | Browse / Clear | Pin Go-engine tests to a specific Speedtest.net server by ID. Falls back to the nearest server if it fails. Ignored by the Ookla engine. |

### Schedule

Cron expressions use [robfig/cron v3](https://pkg.go.dev/github.com/robfig/cron/v3) syntax: standard 5-field cron (minute, hour, day of month, month, day of week) or descriptors.

| Expression | Meaning |
|---|---|
| `@every 1h` | Every hour, counted from when the scheduler starts |
| `@hourly` | At the start of every hour |
| `@daily` | Once a day at midnight |
| `0 */6 * * *` | Every 6 hours, on the hour |
| *(empty)* | Disabled: manual and API tests only |

Invalid expressions are rejected with HTTP 400.

### Thresholds

A result breaches a threshold when it is below a minimum or above a maximum. Set a field to `0` to disable it.

| Field | Unit | Description |
|---|---|---|
| Min Download | Mbps | Breach if download falls below this value |
| Min Upload | Mbps | Breach if upload falls below this value |
| Max Ping | ms | Breach if ping exceeds this value |
| Max Jitter | ms | Breach if jitter exceeds this value |
| Max Packet Loss | ratio 0–1 | Breach if packet loss exceeds this value |
| Cooldown | minutes | Minimum gap between webhook alerts (one shared cooldown for all metrics) |

Every breach increments `speedtest_threshold_breaches_total`. Webhooks are called at most once per cooldown window, with all breaches of that test in one payload. Thresholds do not trigger the notification channels in the Alerts tab.

> The Go engine does not measure packet loss and always reports `0`, so **Max Packet Loss** is only useful with the Ookla engine.

### Webhooks

One URL per line. On a threshold breach (outside the cooldown), each URL receives a `POST` with `Content-Type: application/json`:

```json
{
  "timestamp": "2026-05-28T17:44:32Z",
  "breaches": [
    { "metric": "download_mbps", "value": 42.1, "limit": 100 }
  ],
  "result": { "id": 27, "download_mbps": 42.1, "upload_mbps": 20.3, "ping_ms": 13.0, "...": "..." }
}
```

`metric` is one of `download_mbps`, `upload_mbps`, `ping_ms`, `jitter_ms`, `packet_loss`. `result` is the full result object (see the [REST API](#rest-api)). Delivery is best-effort: errors and non-2xx responses are ignored.

### Export / Import

| Field / Button | Description |
|---|---|
| Export Passphrase | Passphrase used to encrypt channel credentials on export. Set the same passphrase on the destination machine before importing an encrypted file. Returned as `***` by the API. |
| Export (Encrypted) | Downloads `speedtest-settings.json` with channel credentials encrypted via PBKDF2-SHA256 (100,000 iterations, 16-byte random salt) + AES-256-GCM. Requires a saved passphrase. |
| Export (Unencrypted) | Downloads `speedtest-settings.json` with channel credentials in **plaintext**. |
| Import | Opens a file picker. Accepts a file produced by either export variant (up to 4 MB). The local export passphrase is kept. The import as a whole is **not** atomic: settings are saved first, then providers are validated and all existing notification channels are replaced in a single transaction. A file with an invalid provider returns HTTP 400 after the settings have already been overwritten. |

The export file format:

```json
{
  "version": 1,
  "encrypted": false,
  "salt": "",
  "settings": { "engine": "go", "schedule": "@every 1h", "...": "..." },
  "channels": [
    {
      "name": "Team Slack",
      "provider": "shoutrrr",
      "enabled": true,
      "notify_on_success": true,
      "notify_on_failure": false,
      "config": { "url": "slack://token@channel" }
    }
  ]
}
```

For encrypted exports, each channel has `"config_encrypted": "<base64>"` instead of `"config"`, and `"salt"` contains the hex-encoded PBKDF2 salt. Results history is not part of the export.

---

## Prometheus Metrics

Scrape endpoint: `GET /metrics`. Metrics are served from a private registry, so only the `speedtest_*` metrics below are exposed (no Go runtime or process metrics). The value gauges and `speedtest_last_test_timestamp_seconds` report `0` from startup until the first test completes. `speedtest_server_info` has no series until the first test, and `speedtest_threshold_breaches_total` has none until the first breach.

| Metric | Type | Labels | Description |
|---|---|---|---|
| `speedtest_download_mbps` | Gauge | | Latest download speed (Mbps) |
| `speedtest_upload_mbps` | Gauge | | Latest upload speed (Mbps) |
| `speedtest_ping_ms` | Gauge | | Latest ping latency (ms) |
| `speedtest_jitter_ms` | Gauge | | Latest jitter (ms) |
| `speedtest_packet_loss_ratio` | Gauge | | Latest packet loss (0–1) |
| `speedtest_last_test_timestamp_seconds` | Gauge | | Unix timestamp of the last completed test |
| `speedtest_server_info` | Gauge | `server_name`, `server_id`, `isp` | Constant 1 for the server used by the latest test |
| `speedtest_tests_total` | Counter | `source` (`manual`, `scheduled`, `api`), `outcome` (`started`, `success`, `error`) | Number of tests |
| `speedtest_test_duration_seconds` | Histogram | | Test duration; buckets 10, 20, 30, 45, 60, 90, 120 s |
| `speedtest_threshold_breaches_total` | Counter | `metric` (`download_mbps`, `upload_mbps`, `ping_ms`, `jitter_ms`, `packet_loss`) | Threshold breaches per metric |

Tests started from the web UI are counted with `source="api"`.

### Example prometheus.yml scrape config

This is equivalent to [`deploy/prometheus.yml`](deploy/prometheus.yml):

```yaml
scrape_configs:
  - job_name: speedtest
    static_configs:
      - targets: ['speedtest-exporter:9090']
    scrape_interval: 60s
```

### Example queries

```promql
# Latest download / upload
speedtest_download_mbps
speedtest_upload_mbps

# Failed tests in the last 24 hours
sum(increase(speedtest_tests_total{outcome="error"}[24h]))

# No successful test in the last 2 hours
time() - speedtest_last_test_timestamp_seconds > 7200
```

---

## REST API

All endpoints return JSON unless noted. There is no authentication (see [Security Notes](#security-notes)).

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/test` | Run a test synchronously and return the result |
| `POST` | `/api/test/stream` | Run a test and stream progress as Server-Sent Events |
| `GET` | `/api/results` | List results, newest first. Query: `limit` (default 100; values ≤ 0 or > 1000 fall back to 100), `offset`, `since` and `until` (RFC 3339) |
| `GET` | `/api/results/latest` | Most recent result (404 if none) |
| `GET` | `/api/results/{id}` | Single result by ID |
| `GET` | `/api/summary?days=N` | Count, average, min, and max over the last N days (default 7) |
| `GET` | `/api/compare?a=ID&b=ID` | Two results side by side: `{"a": {...}, "b": {...}}` |
| `GET` | `/api/servers` | Nearby Speedtest.net servers sorted by distance (`id`, `name`, `country`, `sponsor`, `distance_km`) |
| `GET` | `/api/settings` | Current runtime settings |
| `PUT` | `/api/settings` | Replace runtime settings (applied immediately) |
| `GET` | `/api/settings/export?encrypted=true\|false` | Download settings + channels as a JSON file |
| `POST` | `/api/settings/import` | Restore settings + channels from an export file; returns `{"ok": true, "channels_imported": N}` |
| `GET` | `/api/notifications` | List notification channels (credentials masked) |
| `POST` | `/api/notifications` | Add a channel |
| `PUT` | `/api/notifications/{id}` | Update a channel |
| `DELETE` | `/api/notifications/{id}` | Remove a channel |
| `POST` | `/api/notifications/test` | Send a test message |
| `GET` | `/metrics` | Prometheus exposition format |
| `GET` | `/healthz` | Liveness probe (`{"status":"ok"}`) |
| `GET` | `/` | Web UI (when `server.enable_ui` is `true`) |

Errors are returned as `{"error": "message"}`. Every request is bounded by `server.write_timeout` (120 s by default), which also caps a synchronous `POST /api/test`.

### Result object

```json
{
  "id": 27,
  "timestamp": "2026-05-28T17:44:32Z",
  "source": "scheduled",
  "engine": "go",
  "download_mbps": 781.2,
  "upload_mbps": 45.2,
  "ping_ms": 13.0,
  "jitter_ms": 2.7,
  "packet_loss_ratio": 0,
  "server_name": "Example City",
  "server_id": "12345",
  "isp": "Example ISP",
  "external_ip": "203.0.113.10",
  "duration_sec": 28.4
}
```

### Settings fields (`GET` / `PUT /api/settings`)

```json
{
  "engine": "go",
  "schedule": "@every 1h",
  "min_download_mbps": 0,
  "min_upload_mbps": 0,
  "max_ping_ms": 0,
  "max_jitter_ms": 0,
  "max_packet_loss_ratio": 0,
  "cooldown_minutes": 30,
  "webhooks": [],
  "preferred_server_id": "",
  "preferred_server_name": "",
  "date_format": "",
  "time_format": "",
  "export_passphrase": "***"
}
```

`PUT` replaces the whole object, so send every field. `engine` must be `go` or `ookla`, and `schedule` must be empty or a valid cron expression.
`date_format` accepts `""` (browser locale), `"YYYY-MM-DD"`, `"MM/DD/YYYY"`, `"DD/MM/YYYY"`, `"DD.MM.YYYY"`.
`time_format` accepts `""` (browser locale), `"HH:mm"`, `"HH:mm:ss"`, `"hh:mm a"`, `"hh:mm:ss a"`.
`export_passphrase` is returned as `"***"` when set (never in plaintext); send `"***"` in a `PUT` to keep the stored value unchanged.

### Notification channels

Create or update a channel with:

```json
{
  "name": "Team Slack",
  "provider": "shoutrrr",
  "config": { "url": "slack://token@channel" },
  "enabled": true,
  "notify_on_success": true,
  "notify_on_failure": true
}
```

`provider` is `shoutrrr`, `greenapi`, or `whatsapp_web`; the `config` fields per provider are listed under [Notifications](#notifications). Responses mask secrets: the Shoutrrr URL becomes `scheme://***`, the Green-API `token` and the WhatsApp Web `password` become `***`. On `PUT`, if `config` contains `***` anywhere, the stored config is kept unchanged.

`POST /api/notifications/test` accepts either `{"id": 3}` to test a saved channel, or `{"provider": "...", "config": {...}}` to test unsaved values. It returns `{"status": "sent"}`, HTTP 400 (`build sender: ...`) if the config is invalid or fails the [outbound URL check](#outbound-url-checks), or HTTP 502 with the error if delivery fails.

### SSE stream format (`POST /api/test/stream`)

The stream is `text/event-stream` in response to a `POST`, so use `fetch` with a streaming reader (or `curl -N -X POST`) rather than `EventSource`. Each event is one `data:` line with a JSON object:

```
data: {"phase":"connecting","server_name":"Example City","server_id":"12345"}

data: {"phase":"ping","server_name":"Example City","server_id":"12345","ping_ms":12.5}

data: {"phase":"download","download_mbps":234.7}

data: {"phase":"upload","upload_mbps":48.2}

data: {"phase":"done","server_name":"Example City","server_id":"12345","download_mbps":891.3,"upload_mbps":49.1,"ping_ms":11.8}
```

Phases are `connecting` → `ping` → `download` → `upload` → `done`, or `error` (with an `error` field) if the test fails. With the Go engine, `ping`, `download`, and `upload` events repeat as live samples (download and upload every 250 ms). The Ookla engine sends only `connecting` and then `done` or `error`. Zero-valued fields are omitted. The `done` event is sent after the result has been saved.

---

## Notifications

Channels are configured in the **Alerts** tab of the web UI (or via the [API](#notification-channels)). Their configs are encrypted at rest with AES-256-GCM.

After each test, every enabled channel is notified if its toggle matches the outcome:

- **Notify on success**: the test completed. The message contains download, upload, ping, server name, and time.
- **Notify on failure**: the test runner returned an error. The message contains the error and time.

Delivery is best-effort; failures are logged and never affect the test. Channel notifications are sent by `serve` only.

### Shoutrrr

Covers Slack, Discord, Telegram, Gotify, SMTP, ntfy, and [many more](https://containrrr.dev/shoutrrr/) via a single URL. Config: `{"url": "..."}`.

| Provider | URL format |
|---|---|
| Slack | `slack://token@channel` |
| Discord | `discord://token@webhook-id` |
| Telegram | `telegram://token@telegram?chats=@channel` |
| Gotify | `gotify://hostname/token` |
| SMTP | `smtp://user:pass@host:port/?from=a@b&to=c@d` |
| ntfy | `ntfy://[user:pass@]host/topic` (e.g. `ntfy://ntfy.sh/mytopic`) |

The `generic://` scheme is rejected.

### Green-API (WhatsApp cloud)

Requires a [Green-API](https://green-api.com) account. Config keys: `instance_id`, `token`, `phone`, `api_url`.

| Field | Description |
|---|---|
| Instance ID | From the Green-API console (e.g. `1234567890`) |
| Token | API token from the console. Copy it exactly; leading and trailing spaces are trimmed. |
| Recipient Phone | International format, digits only, **no** `+` or spaces (e.g. `972501234567`) |
| API URL | Leave blank for `https://api.green-api.com`. Set it to your cluster URL if the Green-API console shows one (e.g. `https://7103.api.greenapi.com`) |

The sender calls `POST {api_url}/waInstance{instance_id}/sendMessage/{token}` and appends `@c.us` to the phone number if it contains no `@`.

### WhatsApp Web (self-hosted)

Requires a running [go-whatsapp-web-multidevice](https://github.com/aldinokemal/go-whatsapp-web-multidevice) instance. Config keys: `base_url`, `phone`, and optional `username` / `password` for HTTP Basic Auth. Messages are sent with `POST {base_url}/api/send/message`.

> **Note:** the outbound URL check below blocks private and loopback addresses, so a WhatsApp Web gateway reachable only on your LAN, on `localhost`, or on a Docker network with a private IP is rejected when a message is sent (the channel itself can still be saved). The gateway must resolve to a public address.

### Outbound URL checks

To prevent server-side request forgery (SSRF):

- The checks run whenever a message is built for sending (Send Test and every post-test notification), not when a channel is saved; creating or updating a channel only validates its name and provider.
- The WhatsApp Web `base_url` and a custom Green-API `api_url` must be `http` or `https` and must not resolve to a private, loopback, link-local, CGNAT (`100.64.0.0/10`), or multicast address.
- Requests to Green-API and WhatsApp Web also check the resolved IP again at connection time (which defeats DNS rebinding), refuse HTTP redirects, and time out after 30 seconds.
- Shoutrrr URLs are passed to Shoutrrr as-is, apart from the `generic://` block.
- Threshold [webhooks](#webhooks) are not checked; they may point at any address.

---

## Preferred Server

In **Settings → Engine → Browse**, pick any server from the distance-sorted list returned by `/api/servers`. The Go engine tries your preferred server first. If it is not in the current server list, the nearest server is used. If the test fails on the preferred server at any phase, it is retried once on the nearest server and a warning is logged. The Ookla engine ignores this setting and lets the CLI pick a server.

---

## Data Storage

Everything lives in `data_dir` (`/data` in Docker):

| File | Contents |
|---|---|
| `results.db` | SQLite database: test results, runtime settings (including the export passphrase), and encrypted notification channels |
| `.encryption_key` | 32-byte AES-256 key, hex-encoded, created on the first `serve` with file mode `0600`. Used to encrypt channel configs. |

Back up both files together. Results are kept indefinitely; there is no automatic retention or pruning. To move channels to a new machine without copying the key, use [Export / Import](#export--import).

---

## Security Notes

- **No authentication.** The web UI and every API endpoint, including settings, export, and test runs, are open to anyone who can reach the port. Bind to a trusted interface (`server.host`), keep it on a private network, or put it behind a reverse proxy that adds authentication and TLS. Do not expose it directly to the internet.
- **Sensitive data in the API.** Results include your public IP address (`external_ip`) and ISP. An unencrypted export contains channel credentials in plaintext.
- **Secrets at rest.** Notification channel configs are encrypted with AES-256-GCM using the key in `data_dir/.encryption_key`. Anyone with both the key file and `results.db` can decrypt them, so protect the data directory. The export passphrase and webhook URLs are stored unencrypted in `results.db`.
- **Encrypted exports** use a key derived from the export passphrase with PBKDF2-SHA256 (100,000 iterations) and a random salt per export. The passphrase itself is never written to the file.
- **Credential masking.** Channel secrets and the export passphrase are masked in API responses and are never returned in plaintext after saving.
- **Outbound requests.** Green-API and WhatsApp Web URLs are checked against private and internal addresses when a message is sent (not when the channel is saved) and again at connection time, and redirects are refused. Shoutrrr URLs are only checked for the `generic://` scheme, and webhook URLs are not checked. See [Outbound URL checks](#outbound-url-checks).
- **Container.** The Docker image is built `FROM scratch` and runs as root (no `USER` is set).

---

## Troubleshooting

**A WhatsApp Web channel fails with "private/internal address (SSRF protection)".**
The gateway URL resolves to a private or loopback address, which is blocked. See the note under [WhatsApp Web](#whatsapp-web-self-hosted).

**Packet loss is always 0.**
The Go engine does not measure packet loss. Switch to the Ookla engine if you need it.

**The Ookla engine fails in Docker.**
The image does not include the Ookla CLI. Use the Go engine in Docker, or run the binary on a host with the CLI installed.

**The schedule comes back after a restart although I cleared it in the UI.**
When the saved schedule is empty, `serve` falls back to the YAML/env schedule at startup, which defaults to `@every 1h`. To disable scheduled tests permanently, also set `schedule: ""` in your YAML config. An empty `SPEEDTEST_SCHEDULE` environment variable is ignored. <!-- TODO: verify whether this fallback is intended -->

**Changing YAML or environment values has no effect.**
After settings have been saved in the web UI, the database values win for engine, schedule, thresholds, and webhooks. Change them in the Settings tab instead.

**Notification channels stopped working after moving the data directory.**
Channel configs can only be decrypted with the original `.encryption_key`. If the key is lost or replaced, the channel list cannot be read. Restore the key file, or remove the channels and restore them from an export.

**Prometheus does not start with Docker Compose.**
Make sure `deploy/prometheus.yml` exists next to `docker-compose.yml` before running `docker compose up`. Otherwise Docker creates an empty directory in its place.

**A synchronous `POST /api/test` times out.**
Requests are capped by `server.write_timeout` (120 s by default). Raise it for slow links, or use `POST /api/test/stream`.

---

## Development

```bash
# Download dependencies
go mod download

# Run a one-shot test
go run ./cmd/speedtest-exporter run

# Start the server (no hot reload; restart after changes)
go run ./cmd/speedtest-exporter serve --config config.example.yaml

# Tests
go test ./...
go test -race ./...        # the race detector needs CGO and a C toolchain

# Cross-compile all release targets into dist/
VERSION=2026.5.0 ./scripts/build.sh
```

The version string is injected at build time with `-ldflags "-X main.version=<version>"` and shown by `--version`.

### Releases and CI

- **Release** (`.github/workflows/release.yml`, manual): computes the next `YYYY.M.PATCH` version with `scripts/next-version.sh` (or uses the version you enter), builds all targets with `scripts/build.sh`, tags the commit, and creates a GitHub Release with the binaries.
- **Docker** (`.github/workflows/docker.yml`): runs after a successful Release (or manually) and pushes a multi-arch image (`linux/amd64`, `linux/arm64`, `linux/arm/v7`) tagged `latest` and with the latest git tag.
- **Publish to GHCR** (`.github/workflows/publish-ghcr.yml`, manual): builds the same image for GitHub Container Registry. <!-- TODO: verify: no image has been published to ghcr.io yet -->

---

## Contributing

Issues and pull requests are welcome. Please run `go vet ./...` and `go test ./...` before opening a pull request, and keep the build CGO-free.

---

## License

Apache License 2.0. See [LICENSE](LICENSE).
