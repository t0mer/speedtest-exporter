package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveAndGet(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	r := &model.Result{
		Timestamp:    time.Now().UTC().Truncate(time.Second),
		Source:       model.SourceManual,
		Engine:       model.EngineGo,
		DownloadMbps: 150.5,
		UploadMbps:   50.2,
		PingMs:       12.3,
		JitterMs:     2.1,
		PacketLoss:   0.0,
		ServerName:   "Test Server",
		ServerID:     "abc123",
		ISP:          "TestISP",
		ExternalIP:   "1.2.3.4",
		DurationSec:  35.2,
	}

	require.NoError(t, db.Save(ctx, r))
	assert.Positive(t, r.ID)

	got, err := db.Get(ctx, r.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, r.DownloadMbps, got.DownloadMbps)
	assert.Equal(t, r.ServerName, got.ServerName)
	assert.Equal(t, r.Source, got.Source)
}

func TestLatest(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	latest, err := db.Latest(ctx)
	require.NoError(t, err)
	assert.Nil(t, latest)

	r1 := &model.Result{Timestamp: time.Now().Add(-1 * time.Hour), Source: model.SourceManual, Engine: model.EngineGo}
	r2 := &model.Result{Timestamp: time.Now(), Source: model.SourceScheduled, Engine: model.EngineGo, DownloadMbps: 200}
	require.NoError(t, db.Save(ctx, r1))
	require.NoError(t, db.Save(ctx, r2))

	latest, err = db.Latest(ctx)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, r2.DownloadMbps, latest.DownloadMbps)
}

func TestList(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		r := &model.Result{
			Timestamp:    time.Now().Add(time.Duration(i) * time.Minute),
			Source:       model.SourceManual,
			Engine:       model.EngineGo,
			DownloadMbps: float64(100 + i),
		}
		require.NoError(t, db.Save(ctx, r))
	}

	results, err := db.List(ctx, database.ListOptions{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSummary(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		r := &model.Result{
			Timestamp:    time.Now(),
			Source:       model.SourceManual,
			Engine:       model.EngineGo,
			DownloadMbps: 100,
			UploadMbps:   50,
			PingMs:       10,
		}
		require.NoError(t, db.Save(ctx, r))
	}

	s, err := db.Summary(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 3, s.Count)
	assert.InDelta(t, 100.0, s.AvgDownload, 0.01)
	assert.InDelta(t, 50.0, s.AvgUpload, 0.01)
}
