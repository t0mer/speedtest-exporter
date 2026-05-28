package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRunTest(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.Run(r.Context(), model.SourceAPI)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListResults(w http.ResponseWriter, r *http.Request) {
	opts := database.ListOptions{
		Limit:  queryInt(r, "limit", 100),
		Offset: queryInt(r, "offset", 0),
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			opts.Since = t
		}
	}
	if v := r.URL.Query().Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			opts.Until = t
		}
	}

	results, err := s.service.DB().List(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []model.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleLatestResult(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.DB().Latest(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "no results yet")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := s.service.DB().Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 7)
	if days <= 0 {
		days = 7
	}
	summary, err := s.service.DB().Summary(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	aID, err := strconv.ParseInt(r.URL.Query().Get("a"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid a")
		return
	}
	bID, err := strconv.ParseInt(r.URL.Query().Get("b"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid b")
		return
	}
	a, err := s.service.DB().Get(r.Context(), aID)
	if err != nil || a == nil {
		writeError(w, http.StatusNotFound, "result a not found")
		return
	}
	b, err := s.service.DB().Get(r.Context(), bID)
	if err != nil || b == nil {
		writeError(w, http.StatusNotFound, "result b not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]*model.Result{"a": a, "b": b})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
