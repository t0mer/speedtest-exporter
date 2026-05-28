// Package scheduler runs periodic speed tests via a cron schedule.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/robfig/cron/v3"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/service"
)

// Scheduler drives periodic test execution. It skips a run if one is in progress.
type Scheduler struct {
	cron    *cron.Cron
	service *service.Service
	running atomic.Bool
}

// New creates a Scheduler with the given cron spec (robfig/cron v3 syntax).
// Returns an error if spec is invalid.
func New(svc *service.Service, spec string) (*Scheduler, error) {
	s := &Scheduler{
		cron:    cron.New(),
		service: svc,
	}
	if _, err := s.cron.AddFunc(spec, s.runOnce); err != nil {
		return nil, fmt.Errorf("invalid schedule %q: %w", spec, err)
	}
	return s, nil
}

// Start begins the scheduler. Safe to call multiple times.
func (s *Scheduler) Start() { s.cron.Start() }

// Stop halts the scheduler, blocking until the running job (if any) completes.
func (s *Scheduler) Stop() { s.cron.Stop() }

// ValidateSpec returns an error if spec is not a valid cron expression.
// Accepts the same syntax as New (robfig/cron v3, including @every descriptors).
func ValidateSpec(spec string) error {
	c := cron.New()
	_, err := c.AddFunc(spec, func() {})
	return err
}

func (s *Scheduler) runOnce() {
	if !s.running.CompareAndSwap(false, true) {
		slog.Warn("scheduled test skipped: previous run still in progress")
		return
	}
	defer s.running.Store(false)

	if _, err := s.service.Run(context.Background(), model.SourceScheduled); err != nil {
		slog.Error("scheduled test failed", "error", err)
	}
}
