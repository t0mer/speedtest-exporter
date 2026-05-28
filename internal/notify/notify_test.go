package notify_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notify"
)

func TestEvaluateNoBreaches(t *testing.T) {
	n := notify.New(notify.ThresholdConfig{
		MinDownloadMbps: 100,
		MaxPingMs:       50,
	}, nil)

	r := &model.Result{DownloadMbps: 200, PingMs: 10}
	breaches := n.Evaluate(r)
	assert.Empty(t, breaches)
}

func TestEvaluateBreaches(t *testing.T) {
	n := notify.New(notify.ThresholdConfig{
		MinDownloadMbps: 100,
		MinUploadMbps:   30,
		MaxPingMs:       20,
	}, nil)

	r := &model.Result{DownloadMbps: 50, UploadMbps: 10, PingMs: 100}
	breaches := n.Evaluate(r)
	assert.Len(t, breaches, 3)
	assert.Equal(t, "download_mbps", breaches[0].Metric)
}

func TestWebhookFired(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	n := notify.New(notify.ThresholdConfig{MinDownloadMbps: 100}, []string{srv.URL})

	r := &model.Result{
		Timestamp:    time.Now(),
		DownloadMbps: 50, // breach
	}
	require.NoError(t, n.Notify(context.Background(), r))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(received, &payload))
	assert.NotNil(t, payload["breaches"])
}

func TestCooldown(t *testing.T) {
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer srv.Close()

	n := notify.New(notify.ThresholdConfig{
		MinDownloadMbps: 100,
		CooldownMinutes: 60,
	}, []string{srv.URL})

	r := &model.Result{DownloadMbps: 50}
	require.NoError(t, n.Notify(context.Background(), r))
	require.NoError(t, n.Notify(context.Background(), r)) // should be blocked by cooldown

	assert.Equal(t, 1, callCount)
}
