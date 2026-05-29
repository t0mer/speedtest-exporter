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

func buildTestServerWithNotifAndKey(t *testing.T, key []byte) (http.Handler, *database.DB) {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	store := notifications.NewStore(db.SQL(), key)
	svc := service.New(db, &alwaysOKRunner{}, metrics.New(), notify.New(notify.ThresholdConfig{}, nil))
	cfg := config.Default()
	cfg.Server.EnableUI = false
	return api.NewServer(svc, &cfg, "speedtest", store), db
}

func TestExportSettingsUnencrypted(t *testing.T) {
	key := make([]byte, 32)
	srv, _ := buildTestServerWithNotifAndKey(t, key)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/export?encrypted=false", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "speedtest-settings.json")

	var doc model.ExportDoc
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&doc))
	assert.Equal(t, 1, doc.Version)
	assert.False(t, doc.Encrypted)
	assert.Empty(t, doc.Salt)
	assert.NotEmpty(t, doc.Settings.Engine)
	assert.Empty(t, doc.Settings.ExportPassphrase, "passphrase must not be exported")
}

func TestExportSettingsEncryptedNeedPassphrase(t *testing.T) {
	key := make([]byte, 32)
	srv, _ := buildTestServerWithNotifAndKey(t, key)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/export?encrypted=true", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestExportImportRoundtripUnencrypted(t *testing.T) {
	key := make([]byte, 32)
	srv, db := buildTestServerWithNotifAndKey(t, key)
	ctx := context.Background()

	store := notifications.NewStore(db.SQL(), key)
	ch := &notifications.Channel{
		Name:            "Slack",
		Provider:        notifications.ProviderShoutrrr,
		Config:          json.RawMessage(`{"url":"slack://token@channel"}`),
		Enabled:         true,
		NotifyOnSuccess: true,
		NotifyOnFailure: false,
	}
	require.NoError(t, store.Save(ctx, ch))

	exportReq := httptest.NewRequest(http.MethodGet, "/api/settings/export?encrypted=false", nil)
	exportRec := httptest.NewRecorder()
	srv.ServeHTTP(exportRec, exportReq)
	require.Equal(t, http.StatusOK, exportRec.Code)

	require.NoError(t, store.DeleteAll(ctx))
	importReq := httptest.NewRequest(http.MethodPost, "/api/settings/import", exportRec.Body)
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()
	srv.ServeHTTP(importRec, importReq)
	require.Equal(t, http.StatusOK, importRec.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(importRec.Body).Decode(&result))
	assert.Equal(t, float64(1), result["channels_imported"])

	channels, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, "Slack", channels[0].Name)
}

func TestExportImportRoundtripEncrypted(t *testing.T) {
	key := make([]byte, 32)
	srv, db := buildTestServerWithNotifAndKey(t, key)
	ctx := context.Background()

	store := notifications.NewStore(db.SQL(), key)
	settings := &model.Settings{Engine: "go", Schedule: "", Webhooks: []string{}, ExportPassphrase: "testpass"}
	require.NoError(t, db.SaveSettings(ctx, settings))
	ch := &notifications.Channel{
		Name:     "Discord",
		Provider: notifications.ProviderShoutrrr,
		Config:   json.RawMessage(`{"url":"discord://tok@id"}`),
		Enabled:  true,
	}
	require.NoError(t, store.Save(ctx, ch))

	exportReq := httptest.NewRequest(http.MethodGet, "/api/settings/export?encrypted=true", nil)
	exportRec := httptest.NewRecorder()
	srv.ServeHTTP(exportRec, exportReq)
	require.Equal(t, http.StatusOK, exportRec.Code)

	var doc model.ExportDoc
	exportBody := exportRec.Body.Bytes()
	require.NoError(t, json.Unmarshal(exportBody, &doc))
	assert.True(t, doc.Encrypted)
	assert.NotEmpty(t, doc.Salt)
	assert.Empty(t, doc.Settings.ExportPassphrase)
	require.Len(t, doc.Channels, 1)
	assert.Empty(t, doc.Channels[0].Config)
	assert.NotEmpty(t, doc.Channels[0].ConfigEncrypted)

	require.NoError(t, store.DeleteAll(ctx))
	importReq := httptest.NewRequest(http.MethodPost, "/api/settings/import",
		strings.NewReader(string(exportBody)))
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()
	srv.ServeHTTP(importRec, importReq)
	require.Equal(t, http.StatusOK, importRec.Code)

	channels, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, "Discord", channels[0].Name)
	assert.Contains(t, string(channels[0].Config), "discord://")
}

func TestImportBadVersion(t *testing.T) {
	srv := buildTestServer(t)
	body := `{"version":99,"encrypted":false,"salt":"","settings":{"engine":"go","webhooks":[]},"channels":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestImportEncryptedNoPassphrase(t *testing.T) {
	srv := buildTestServer(t)
	body := `{"version":1,"encrypted":true,"salt":"aabbccdd","settings":{"engine":"go","webhooks":[]},"channels":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
