package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/t0mer/speedtest-exporter/internal/crypto"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notifications"
)

func (s *Server) handleExportSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	encrypted := r.URL.Query().Get("encrypted") == "true"

	settings, err := s.service.DB().GetSettings(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings == nil {
		settings = s.defaultSettings()
	}

	if encrypted && settings.ExportPassphrase == "" {
		writeError(w, http.StatusBadRequest, "export_passphrase is not configured")
		return
	}

	var channels []notifications.Channel
	if s.notifStore != nil {
		channels, err = s.notifStore.List(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	exported := *settings
	exported.ExportPassphrase = ""

	doc := model.ExportDoc{
		Version:   1,
		Encrypted: encrypted,
		Settings:  exported,
		Channels:  make([]model.ExportChannel, 0, len(channels)),
	}

	var derivedKey []byte
	if encrypted {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate salt")
			return
		}
		doc.Salt = hex.EncodeToString(salt)
		derivedKey = crypto.DeriveKey(settings.ExportPassphrase, salt)
	}

	for _, ch := range channels {
		ec := model.ExportChannel{
			Name:            ch.Name,
			Provider:        string(ch.Provider),
			Enabled:         ch.Enabled,
			NotifyOnSuccess: ch.NotifyOnSuccess,
			NotifyOnFailure: ch.NotifyOnFailure,
		}
		if encrypted {
			ciphertext, err := crypto.Encrypt(derivedKey, ch.Config)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "encrypt channel config: "+err.Error())
				return
			}
			ec.ConfigEncrypted = base64.StdEncoding.EncodeToString(ciphertext)
		} else {
			ec.Config = ch.Config
		}
		doc.Channels = append(doc.Channels, ec)
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="speedtest-settings.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleImportSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var doc model.ExportDoc
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if doc.Version != 1 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported version: %d", doc.Version))
		return
	}

	if doc.Settings.Engine != "go" && doc.Settings.Engine != "ookla" {
		writeError(w, http.StatusBadRequest, "invalid engine in import file")
		return
	}

	var derivedKey []byte
	if doc.Encrypted {
		current, err := s.service.DB().GetSettings(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if current == nil {
			current = s.defaultSettings()
		}
		if current.ExportPassphrase == "" {
			writeError(w, http.StatusBadRequest, "export_passphrase is not configured; set it in Settings before importing an encrypted file")
			return
		}
		salt, err := hex.DecodeString(doc.Salt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid salt in export file")
			return
		}
		derivedKey = crypto.DeriveKey(current.ExportPassphrase, salt)
	}

	for i := range doc.Channels {
		if doc.Encrypted {
			ciphertext, err := base64.StdEncoding.DecodeString(doc.Channels[i].ConfigEncrypted)
			if err != nil {
				writeError(w, http.StatusBadRequest, "decryption failed: corrupt file")
				return
			}
			plaintext, err := crypto.Decrypt(derivedKey, ciphertext)
			if err != nil {
				writeError(w, http.StatusBadRequest, "decryption failed: wrong passphrase or corrupt file")
				return
			}
			doc.Channels[i].Config = plaintext
		}
	}

	if doc.Settings.Webhooks == nil {
		doc.Settings.Webhooks = []string{}
	}

	if err := s.service.DB().SaveSettings(ctx, &doc.Settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	channelsImported := 0
	if s.notifStore != nil {
		if err := s.notifStore.DeleteAll(ctx); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, ec := range doc.Channels {
			ch := &notifications.Channel{
				Name:            ec.Name,
				Provider:        notifications.Provider(ec.Provider),
				Config:          ec.Config,
				Enabled:         ec.Enabled,
				NotifyOnSuccess: ec.NotifyOnSuccess,
				NotifyOnFailure: ec.NotifyOnFailure,
			}
			if err := s.notifStore.Save(ctx, ch); err != nil {
				writeError(w, http.StatusInternalServerError, "save channel: "+err.Error())
				return
			}
			channelsImported++
		}
	}

	s.service.Apply(&doc.Settings, s.ooklaPath)
	if err := s.SetSchedule(doc.Settings.Schedule); err != nil {
		slog.Error("scheduler restart failed after import", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"channels_imported": channelsImported,
	})
}
