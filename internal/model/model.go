package model

import "time"

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
