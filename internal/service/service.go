// Package service orchestrates the full speed test pipeline.
package service

import (
	"context"
	"fmt"
	"log/slog"
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

	result, err := s.runner.Run(ctx)
	if err != nil {
		s.metrics.IncrTests(string(source), "error")
		return nil, fmt.Errorf("run speed test: %w", err)
	}

	result.Source = source
	result.Engine = s.runner.Engine()
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
