package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/scheduler"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.service.DB().GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings == nil {
		settings = s.defaultSettings()
	}
	masked := *settings
	if masked.ExportPassphrase != "" {
		masked.ExportPassphrase = "***"
	}
	writeJSON(w, http.StatusOK, &masked)
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var settings model.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if settings.Engine != "go" && settings.Engine != "ookla" {
		writeError(w, http.StatusBadRequest, "engine must be 'go' or 'ookla'")
		return
	}

	if settings.Schedule != "" {
		if err := scheduler.ValidateSpec(settings.Schedule); err != nil {
			writeError(w, http.StatusBadRequest, "invalid schedule: "+err.Error())
			return
		}
	}

	if settings.ExportPassphrase == "***" {
		current, err := s.service.DB().GetSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if current != nil {
			settings.ExportPassphrase = current.ExportPassphrase
		} else {
			settings.ExportPassphrase = ""
		}
	}

	if settings.Webhooks == nil {
		settings.Webhooks = []string{}
	}

	if err := s.service.DB().SaveSettings(r.Context(), &settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.service.Apply(&settings, s.ooklaPath)

	if err := s.SetSchedule(settings.Schedule); err != nil {
		slog.Error("scheduler restart failed", "error", err)
	}

	writeJSON(w, http.StatusOK, &settings)
}

// defaultSettings returns settings derived from the startup config,
// used when no settings have been saved to the database yet.
func (s *Server) defaultSettings() *model.Settings {
	webhooks := s.cfg.Webhooks
	if webhooks == nil {
		webhooks = []string{}
	}
	return &model.Settings{
		Engine:             s.cfg.Engine,
		Schedule:           s.cfg.Schedule,
		MinDownloadMbps:    s.cfg.Thresholds.MinDownloadMbps,
		MinUploadMbps:      s.cfg.Thresholds.MinUploadMbps,
		MaxPingMs:          s.cfg.Thresholds.MaxPingMs,
		MaxJitterMs:        s.cfg.Thresholds.MaxJitterMs,
		MaxPacketLossRatio: s.cfg.Thresholds.MaxPacketLossRatio,
		CooldownMinutes:    s.cfg.Thresholds.CooldownMinutes,
		Webhooks:            webhooks,
		PreferredServerID:   "",
		PreferredServerName: "",
	}
}
