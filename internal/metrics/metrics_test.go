package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/metrics"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

func TestMetricsUpdate(t *testing.T) {
	m := metrics.New()
	r := &model.Result{
		Timestamp:    time.Now(),
		DownloadMbps: 200,
		UploadMbps:   50,
		PingMs:       10,
		JitterMs:     1.5,
		PacketLoss:   0.01,
		ServerName:   "TestServer",
		ServerID:     "42",
		ISP:          "TestISP",
	}
	m.Update(r)

	// Verify metrics are exposed via the handler.
	srv := httptest.NewServer(m.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "speedtest_download_mbps 200")
	assert.Contains(t, string(body), "speedtest_upload_mbps 50")
	assert.Contains(t, string(body), "speedtest_ping_ms 10")
}

func TestMetricsCounters(t *testing.T) {
	m := metrics.New()
	m.IncrTests("manual", "success")
	m.IncrTests("manual", "success")
	m.IncrTests("api", "error")
	m.IncrBreaches("download_mbps")
	m.ObserveDuration(35.0)

	srv := httptest.NewServer(m.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Contains(t, string(body), `speedtest_tests_total{outcome="success",source="manual"} 2`)
	assert.Contains(t, string(body), `speedtest_threshold_breaches_total{metric="download_mbps"} 1`)
}
