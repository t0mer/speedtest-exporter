package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/t0mer/speedtest-exporter/internal/model"
)

// Manager dispatches notifications to all matching enabled channels after each test.
type Manager struct {
	store *Store
}

// NewManager creates a Manager backed by the given store.
func NewManager(store *Store) *Manager {
	return &Manager{store: store}
}

// NotifySuccess fires success notifications to channels with notify_on_success=true.
// Best-effort: errors are logged and never propagated to the caller.
func (m *Manager) NotifySuccess(ctx context.Context, result *model.Result) {
	m.dispatch(ctx, true, formatSuccess(result))
}

// NotifyFailure fires failure notifications to channels with notify_on_failure=true.
// Best-effort: errors are logged and never propagated to the caller.
func (m *Manager) NotifyFailure(ctx context.Context, runErr error) {
	m.dispatch(ctx, false, formatFailure(runErr))
}

func (m *Manager) dispatch(ctx context.Context, success bool, message string) {
	channels, err := m.store.List(ctx)
	if err != nil {
		slog.Error("notifications: list channels", "error", err)
		return
	}
	for _, ch := range channels {
		ch := ch // capture loop variable
		if !ch.Enabled {
			continue
		}
		if success && !ch.NotifyOnSuccess {
			continue
		}
		if !success && !ch.NotifyOnFailure {
			continue
		}
		sender, err := NewSender(&ch)
		if err != nil {
			slog.Error("notifications: build sender", "channel", ch.Name, "error", err)
			continue
		}
		if err := sender.Send(ctx, message); err != nil {
			slog.Error("notifications: send failed", "channel", ch.Name, "provider", ch.Provider, "error", err)
		}
	}
}

func formatSuccess(r *model.Result) string {
	return fmt.Sprintf(
		"✅ Speedtest Complete\nDownload: %.1f Mbps\nUpload: %.1f Mbps\nPing: %.1f ms\nServer: %s\nTime: %s",
		r.DownloadMbps, r.UploadMbps, r.PingMs, r.ServerName,
		r.Timestamp.UTC().Format(time.RFC1123),
	)
}

func formatFailure(err error) string {
	return fmt.Sprintf(
		"❌ Speedtest Failed\nError: %s\nTime: %s",
		err.Error(),
		time.Now().UTC().Format(time.RFC1123),
	)
}
