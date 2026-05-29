package model

import (
	"encoding/json"
	"time"
)

// Engine identifies the speedtest backend.
type Engine string

// Source identifies what triggered a test.
type Source string

const (
	EngineGo    Engine = "go"
	EngineOokla Engine = "ookla"
)

const (
	SourceManual    Source = "manual"
	SourceScheduled Source = "scheduled"
	SourceAPI       Source = "api"
)

// Result holds one completed speed test measurement.
type Result struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Source       Source    `json:"source"`
	Engine       Engine    `json:"engine"`
	DownloadMbps float64   `json:"download_mbps"`
	UploadMbps   float64   `json:"upload_mbps"`
	PingMs       float64   `json:"ping_ms"`
	JitterMs     float64   `json:"jitter_ms"`
	PacketLoss   float64   `json:"packet_loss_ratio"`
	ServerName   string    `json:"server_name"`
	ServerID     string    `json:"server_id"`
	ISP          string    `json:"isp"`
	ExternalIP   string    `json:"external_ip"`
	DurationSec  float64   `json:"duration_sec"`
}

// Settings holds the runtime-editable configuration stored in the database.
// These fields override their equivalents from the YAML/env config at startup
// and take effect immediately when saved via the API.
type Settings struct {
	Engine             string   `json:"engine"`
	Schedule           string   `json:"schedule"`
	MinDownloadMbps    float64  `json:"min_download_mbps"`
	MinUploadMbps      float64  `json:"min_upload_mbps"`
	MaxPingMs          float64  `json:"max_ping_ms"`
	MaxJitterMs        float64  `json:"max_jitter_ms"`
	MaxPacketLossRatio float64  `json:"max_packet_loss_ratio"`
	CooldownMinutes    int      `json:"cooldown_minutes"`
	Webhooks           []string `json:"webhooks"`
	// PreferredServerID is the numeric Speedtest.net server ID to use for Go-engine
	// tests. Empty means "pick nearest". If the preferred server fails the test,
	// the runner falls back to the nearest available server automatically.
	PreferredServerID   string `json:"preferred_server_id"`
	PreferredServerName string `json:"preferred_server_name"`
	// DateFormat controls how dates are displayed in the UI.
	// Empty string means use the browser's locale default.
	// Valid values: "YYYY-MM-DD", "MM/DD/YYYY", "DD/MM/YYYY", "DD.MM.YYYY"
	DateFormat string `json:"date_format"`
	// TimeFormat controls how times are displayed in the UI.
	// Empty string means use the browser's locale default.
	// Valid values: "HH:mm", "HH:mm:ss", "hh:mm a", "hh:mm:ss a"
	TimeFormat string `json:"time_format"`
	// ExportPassphrase is the passphrase used to encrypt/decrypt settings export files.
	// Empty string means encrypted export is not available.
	// Never written into export files.
	ExportPassphrase string `json:"export_passphrase"`
}

// ExportDoc is the top-level structure of a settings export file.
type ExportDoc struct {
	Version   int             `json:"version"`
	Encrypted bool            `json:"encrypted"`
	Salt      string          `json:"salt"`
	Settings  Settings        `json:"settings"`
	Channels  []ExportChannel `json:"channels"`
}

// ExportChannel is the per-channel record inside an ExportDoc.
// Config holds the plain JSON config (unencrypted export).
// ConfigEncrypted holds the base64-encoded AES-256-GCM ciphertext (encrypted export).
type ExportChannel struct {
	Name            string          `json:"name"`
	Provider        string          `json:"provider"`
	Enabled         bool            `json:"enabled"`
	NotifyOnSuccess bool            `json:"notify_on_success"`
	NotifyOnFailure bool            `json:"notify_on_failure"`
	Config          json.RawMessage `json:"config,omitempty"`
	ConfigEncrypted string          `json:"config_encrypted,omitempty"`
}
