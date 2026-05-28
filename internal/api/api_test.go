package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/api"
	"github.com/t0mer/speedtest-exporter/internal/config"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/metrics"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notify"
	"github.com/t0mer/speedtest-exporter/internal/service"
)

type alwaysOKRunner struct{}

func (r *alwaysOKRunner) Engine() model.Engine { return model.EngineGo }
func (r *alwaysOKRunner) Run(_ context.Context) (*model.Result, error) {
	return &model.Result{DownloadMbps: 100, UploadMbps: 50, PingMs: 15}, nil
}

func buildTestServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	svc := service.New(db, &alwaysOKRunner{}, metrics.New(), notify.New(notify.ThresholdConfig{}, nil))
	cfg := config.Default()
	cfg.Server.EnableUI = false
	return api.NewServer(svc, &cfg, "speedtest")
}

func TestHealthz(t *testing.T) {
	srv := buildTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRunTestEndpoint(t *testing.T) {
	srv := buildTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var result model.Result
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.InDelta(t, 100.0, result.DownloadMbps, 0.01)
	assert.Positive(t, result.ID)
}

func TestLatestNotFound(t *testing.T) {
	srv := buildTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/results/latest", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListResults(t *testing.T) {
	srv := buildTestServer(t)

	// Seed one result.
	runReq := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	srv.ServeHTTP(httptest.NewRecorder(), runReq)

	req := httptest.NewRequest(http.MethodGet, "/api/results", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var results []model.Result
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	assert.Len(t, results, 1)
}

func TestMetricsEndpoint(t *testing.T) {
	srv := buildTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "speedtest_tests_total")
}

func TestGetSettingsDefault(t *testing.T) {
	srv := buildTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var s model.Settings
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&s))
	assert.Equal(t, "go", s.Engine) // default engine
}

func TestPutSettings(t *testing.T) {
	srv := buildTestServer(t)

	body := `{"engine":"go","schedule":"","min_download_mbps":50,"min_upload_mbps":10,"max_ping_ms":200,"max_jitter_ms":0,"max_packet_loss_ratio":0,"cooldown_minutes":30,"webhooks":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify persisted
	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	var s model.Settings
	require.NoError(t, json.NewDecoder(getRec.Body).Decode(&s))
	assert.InDelta(t, 50.0, s.MinDownloadMbps, 0.001)
}

func TestPutSettingsBadEngine(t *testing.T) {
	srv := buildTestServer(t)
	body := `{"engine":"invalid","schedule":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPutSettingsBadSchedule(t *testing.T) {
	srv := buildTestServer(t)
	body := `{"engine":"go","schedule":"not a valid spec"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
