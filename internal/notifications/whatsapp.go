package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type whatsAppWebSender struct {
	cfg WhatsAppWebConfig
}

func (s *whatsAppWebSender) Send(ctx context.Context, message string) error {
	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/api/send/message"

	body, _ := json.Marshal(map[string]string{
		"phone":   s.cfg.Phone,
		"message": message,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.Username != "" {
		req.SetBasicAuth(s.cfg.Username, s.cfg.Password)
	}

	resp, err := safeHTTPClient().Do(req)
	if err != nil {
		// Do not wrap err directly — it may contain the BaseURL revealing internal topology.
		return fmt.Errorf("whatsapp_web: send failed (network error)")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("whatsapp_web: HTTP %d", resp.StatusCode)
	}
	return nil
}
