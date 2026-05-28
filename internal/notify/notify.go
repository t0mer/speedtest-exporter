// Package notify evaluates speed test results against thresholds and fires webhook notifications.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/t0mer/speedtest-exporter/internal/model"
)

// ThresholdConfig defines the breach limits. Zero means disabled for that metric.
type ThresholdConfig struct {
	MinDownloadMbps float64
	MinUploadMbps   float64
	MaxPingMs       float64
	MaxJitterMs     float64
	MaxPacketLoss   float64
	CooldownMinutes int
}

// Breach represents a single threshold violation.
type Breach struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Limit  float64 `json:"limit"`
}

type payload struct {
	Timestamp time.Time     `json:"timestamp"`
	Breaches  []Breach      `json:"breaches"`
	Result    *model.Result `json:"result"`
}

// Notifier evaluates results and sends webhook notifications on breach.
type Notifier struct {
	cfg      ThresholdConfig
	webhooks []string
	mu       sync.Mutex
	lastSent time.Time
}

// New creates a Notifier with the given config and webhook URLs.
func New(cfg ThresholdConfig, webhooks []string) *Notifier {
	return &Notifier{cfg: cfg, webhooks: webhooks}
}

// Evaluate returns all threshold violations for the given result.
func (n *Notifier) Evaluate(r *model.Result) []Breach {
	var breaches []Breach
	if n.cfg.MinDownloadMbps > 0 && r.DownloadMbps < n.cfg.MinDownloadMbps {
		breaches = append(breaches, Breach{"download_mbps", r.DownloadMbps, n.cfg.MinDownloadMbps})
	}
	if n.cfg.MinUploadMbps > 0 && r.UploadMbps < n.cfg.MinUploadMbps {
		breaches = append(breaches, Breach{"upload_mbps", r.UploadMbps, n.cfg.MinUploadMbps})
	}
	if n.cfg.MaxPingMs > 0 && r.PingMs > n.cfg.MaxPingMs {
		breaches = append(breaches, Breach{"ping_ms", r.PingMs, n.cfg.MaxPingMs})
	}
	if n.cfg.MaxJitterMs > 0 && r.JitterMs > n.cfg.MaxJitterMs {
		breaches = append(breaches, Breach{"jitter_ms", r.JitterMs, n.cfg.MaxJitterMs})
	}
	if n.cfg.MaxPacketLoss > 0 && r.PacketLoss > n.cfg.MaxPacketLoss {
		breaches = append(breaches, Breach{"packet_loss", r.PacketLoss, n.cfg.MaxPacketLoss})
	}
	return breaches
}

// Notify evaluates r, and if breaches exist and cooldown has passed, POSTs
// a JSON payload to each configured webhook. Webhook errors are non-fatal.
func (n *Notifier) Notify(ctx context.Context, r *model.Result) error {
	breaches := n.Evaluate(r)
	if len(breaches) == 0 {
		return nil
	}

	n.mu.Lock()
	cooldown := time.Duration(n.cfg.CooldownMinutes) * time.Minute
	if cooldown > 0 && !n.lastSent.IsZero() && time.Since(n.lastSent) < cooldown {
		n.mu.Unlock()
		return nil
	}
	n.lastSent = time.Now()
	n.mu.Unlock()

	body, err := json.Marshal(payload{
		Timestamp: r.Timestamp,
		Breaches:  breaches,
		Result:    r,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	for _, url := range n.webhooks {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	return nil
}
