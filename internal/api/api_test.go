package api_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/t0mer/speedtest-exporter/internal/notifications"
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
	return api.NewServer(svc, &cfg, "speedtest", nil)
}

func buildTestServerWithNotif(t *testing.T) http.Handler {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	key := make([]byte, 32)
	store := notifications.NewStore(db.SQL(), key)

	svc := service.New(db, &alwaysOKRunner{}, metrics.New(), notify.New(notify.ThresholdConfig{}, nil))
	cfg := config.Default()
	cfg.Server.EnableUI = false
	return api.NewServer(svc, &cfg, "speedtest", store)
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

func TestListChannelsEmpty(t *testing.T) {
	srv := buildTestServerWithNotif(t)
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var channels []notifications.ChannelView
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&channels))
	assert.Empty(t, channels)
}

func TestCreateChannel(t *testing.T) {
	srv := buildTestServerWithNotif(t)
	body := `{"name":"Slack","provider":"shoutrrr","config":{"url":"slack://token@channel"},"enabled":true,"notify_on_success":true,"notify_on_failure":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var ch notifications.ChannelView
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&ch))
	assert.Positive(t, ch.ID)
	assert.Equal(t, "Slack", ch.Name)
	assert.NotContains(t, string(ch.Config), "token@channel")
}

func TestCreateChannelBadProvider(t *testing.T) {
	srv := buildTestServerWithNotif(t)
	body := `{"name":"X","provider":"invalid","config":{},"enabled":true,"notify_on_success":true,"notify_on_failure":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteChannel(t *testing.T) {
	srv := buildTestServerWithNotif(t)

	body := `{"name":"X","provider":"shoutrrr","config":{"url":"slack://t@c"},"enabled":true,"notify_on_success":false,"notify_on_failure":true}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	var ch notifications.ChannelView
	json.NewDecoder(createRec.Body).Decode(&ch)

	delReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/notifications/%d", ch.ID), nil)
	delRec := httptest.NewRecorder()
	srv.ServeHTTP(delRec, delReq)
	assert.Equal(t, http.StatusNoContent, delRec.Code)
}
