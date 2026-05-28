package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/speedtest-exporter/internal/notifications"
)

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	if s.notifStore == nil {
		writeJSON(w, http.StatusOK, []notifications.ChannelView{})
		return
	}
	channels, err := s.notifStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]notifications.ChannelView, len(channels))
	for i, ch := range channels {
		views[i] = ch.ToView()
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	if s.notifStore == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications not configured")
		return
	}
	var req notifications.ChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := validateChannelRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ch := &notifications.Channel{
		Name:            req.Name,
		Provider:        req.Provider,
		Config:          req.Config,
		Enabled:         req.Enabled,
		NotifyOnSuccess: req.NotifyOnSuccess,
		NotifyOnFailure: req.NotifyOnFailure,
	}
	if err := s.notifStore.Save(r.Context(), ch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ch.ToView())
}

func (s *Server) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	if s.notifStore == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := s.notifStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	var req notifications.ChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := validateChannelRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// If config contains "***" the client is sending the masked value — keep the existing config.
	cfg := req.Config
	if isMasked(cfg) {
		cfg = existing.Config
	}
	ch := &notifications.Channel{
		ID:              id,
		Name:            req.Name,
		Provider:        req.Provider,
		Config:          cfg,
		Enabled:         req.Enabled,
		NotifyOnSuccess: req.NotifyOnSuccess,
		NotifyOnFailure: req.NotifyOnFailure,
	}
	if err := s.notifStore.Update(r.Context(), ch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ch.ToView())
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	if s.notifStore == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.notifStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       *int64                 `json:"id"`
		Provider notifications.Provider `json:"provider"`
		Config   json.RawMessage        `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	var ch *notifications.Channel
	if req.ID != nil && s.notifStore != nil {
		existing, err := s.notifStore.Get(r.Context(), *req.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if existing == nil {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		ch = existing
	} else {
		if req.Provider == "" {
			writeError(w, http.StatusBadRequest, "provider is required")
			return
		}
		ch = &notifications.Channel{Provider: req.Provider, Config: req.Config}
	}

	sender, err := notifications.NewSender(ch)
	if err != nil {
		writeError(w, http.StatusBadRequest, "build sender: "+err.Error())
		return
	}
	if err := sender.Send(r.Context(), "🔔 Speedtest Exporter — test notification"); err != nil {
		writeError(w, http.StatusBadGateway, "send failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func validateChannelRequest(req *notifications.ChannelRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch req.Provider {
	case notifications.ProviderShoutrrr, notifications.ProviderGreenAPI, notifications.ProviderWhatsAppWeb:
	default:
		return fmt.Errorf("provider must be shoutrrr, greenapi, or whatsapp_web")
	}
	return nil
}

func isMasked(config json.RawMessage) bool {
	return bytes.Contains(config, []byte("***"))
}
