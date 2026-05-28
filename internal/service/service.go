// Package service orchestrates the full speed test pipeline.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/metrics"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notify"
	"github.com/t0mer/speedtest-exporter/internal/runner"
)

// Service orchestrates: run → persist → metrics → evaluate → notify.
type Service struct {
	db       *database.DB
	mu       sync.RWMutex
	runner   runner.Runner
	metrics  *metrics.Metrics
	notifier *notify.Notifier
}

// New assembles a Service from its dependencies.
func New(db *database.DB, r runner.Runner, m *metrics.Metrics, n *notify.Notifier) *Service {
	return &Service{db: db, runner: r, metrics: m, notifier: n}
}

// Run executes the full test cycle for the given source trigger.
// It is the single code path for CLI, API, and scheduler.
func (s *Service) Run(ctx context.Context, source model.Source) (*model.Result, error) {
	start := time.Now()
	s.metrics.IncrTests(string(source), "started")

	s.mu.RLock()
	r := s.runner
	s.mu.RUnlock()

	result, err := r.Run(ctx)
	if err != nil {
		s.metrics.IncrTests(string(source), "error")
		return nil, fmt.Errorf("run speed test: %w", err)
	}

	result.Source = source
	result.Engine = r.Engine()
	result.Timestamp = time.Now().UTC()
	result.DurationSec = time.Since(start).Seconds()

	s.metrics.ObserveDuration(result.DurationSec)
	s.metrics.Update(result)
	s.metrics.IncrTests(string(source), "success")

	if err := s.db.Save(ctx, result); err != nil {
		return result, fmt.Errorf("persist result: %w", err)
	}

	if err := s.notifier.Notify(ctx, result); err != nil {
		slog.Error("notification failed", "error", err)
	}

	for _, b := range s.notifier.Evaluate(result) {
		s.metrics.IncrBreaches(b.Metric)
	}

	return result, nil
}

// DB exposes the database for API queries.
func (s *Service) DB() *database.DB { return s.db }

// Metrics exposes the metrics for the /metrics handler.
func (s *Service) Metrics() *metrics.Metrics { return s.metrics }

// Close shuts down the database connection.
func (s *Service) Close() error { return s.db.Close() }

// SetEngine swaps the active runner to match the given engine and ooklaPath.
func (s *Service) SetEngine(engine model.Engine, ooklaPath string) {
	var r runner.Runner
	if engine == model.EngineOokla {
		r = runner.NewOoklaRunner(ooklaPath)
	} else {
		r = runner.NewGoRunner()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runner = r
}

// Apply applies DB-persisted settings to the live service components.
// It hot-swaps the runner and updates the notifier without a restart.
func (s *Service) Apply(settings *model.Settings, ooklaPath string) {
	s.notifier.Update(notify.ThresholdConfig{
		MinDownloadMbps: settings.MinDownloadMbps,
		MinUploadMbps:   settings.MinUploadMbps,
		MaxPingMs:       settings.MaxPingMs,
		MaxJitterMs:     settings.MaxJitterMs,
		MaxPacketLoss:   settings.MaxPacketLossRatio,
		CooldownMinutes: settings.CooldownMinutes,
	}, settings.Webhooks)
	s.SetEngine(model.Engine(settings.Engine), ooklaPath)
}
