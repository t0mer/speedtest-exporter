package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/metrics"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notify"
	"github.com/t0mer/speedtest-exporter/internal/service"
)

// fakeRunner is a test double for runner.Runner.
type fakeRunner struct {
	result *model.Result
	err    error
}

func (f *fakeRunner) Engine() model.Engine { return model.EngineGo }
func (f *fakeRunner) Run(_ context.Context) (*model.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	r := *f.result
	return &r, nil
}

func buildTestService(t *testing.T, r *fakeRunner) *service.Service {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return service.New(db, r, metrics.New(), notify.New(notify.ThresholdConfig{}, nil))
}

func TestRunSuccess(t *testing.T) {
	stub := &fakeRunner{result: &model.Result{
		DownloadMbps: 150, UploadMbps: 50, PingMs: 10,
	}}
	svc := buildTestService(t, stub)

	result, err := svc.Run(context.Background(), model.SourceManual)
	require.NoError(t, err)
	assert.Equal(t, model.SourceManual, result.Source)
	assert.Equal(t, model.EngineGo, result.Engine)
	assert.Positive(t, result.ID)
	assert.False(t, result.Timestamp.IsZero())
	assert.Positive(t, result.DurationSec)
	assert.InDelta(t, 150.0, result.DownloadMbps, 0.01)

	// Verify persisted.
	got, err := svc.DB().Latest(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, result.ID, got.ID)
}

func TestRunRunnerError(t *testing.T) {
	stub := &fakeRunner{err: errors.New("network unreachable")}
	svc := buildTestService(t, stub)

	_, err := svc.Run(context.Background(), model.SourceManual)
	assert.ErrorContains(t, err, "network unreachable")

	// Nothing should be persisted.
	latest, _ := svc.DB().Latest(context.Background())
	assert.Nil(t, latest)
}

func TestRunSetsTimestamp(t *testing.T) {
	before := time.Now().Add(-time.Second)
	stub := &fakeRunner{result: &model.Result{}}
	svc := buildTestService(t, stub)

	result, err := svc.Run(context.Background(), model.SourceScheduled)
	require.NoError(t, err)
	assert.True(t, result.Timestamp.After(before))
}
