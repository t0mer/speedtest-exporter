// Package runner defines the Runner interface and its two backends.
package runner

import (
	"context"

	"github.com/t0mer/speedtest-exporter/internal/model"
)

// Runner executes a single speed test and returns the raw measurement.
// The service layer adds Timestamp, Source, and DurationSec.
type Runner interface {
	Run(ctx context.Context) (*model.Result, error)
	Engine() model.Engine
}

// ProgressRunner is an optional interface for runners that support live progress
// events. Runners that do not implement it fall back to a plain Run() call.
type ProgressRunner interface {
	Runner
	RunWithProgress(ctx context.Context, progress chan<- ProgressEvent) (*model.Result, error)
}
