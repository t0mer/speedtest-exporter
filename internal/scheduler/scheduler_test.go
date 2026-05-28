package scheduler_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/metrics"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notify"
	"github.com/t0mer/speedtest-exporter/internal/scheduler"
	"github.com/t0mer/speedtest-exporter/internal/service"
)

type countingRunner struct{ count atomic.Int32 }

func (r *countingRunner) Engine() model.Engine { return model.EngineGo }
func (r *countingRunner) Run(_ context.Context) (*model.Result, error) {
	r.count.Add(1)
	return &model.Result{}, nil
}

func buildSvc(t *testing.T, r *countingRunner) *service.Service {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return service.New(db, r, metrics.New(), notify.New(notify.ThresholdConfig{}, nil))
}

func TestSchedulerRunsOnSchedule(t *testing.T) {
	r := &countingRunner{}
	svc := buildSvc(t, r)

	// robfig/cron v3 has a minimum resolution of 1 second for @every specs.
	sched, err := scheduler.New(svc, "@every 1s")
	require.NoError(t, err)

	sched.Start()
	time.Sleep(2500 * time.Millisecond)
	sched.Stop()

	count := r.count.Load()
	assert.GreaterOrEqual(t, count, int32(2))
}

func TestSchedulerInvalidSpec(t *testing.T) {
	r := &countingRunner{}
	svc := buildSvc(t, r)
	_, err := scheduler.New(svc, "not a valid cron spec")
	assert.Error(t, err)
}
