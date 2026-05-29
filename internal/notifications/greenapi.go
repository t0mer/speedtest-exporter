package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultGreenAPIURL = "https://api.green-api.com"

type greenAPISender struct {
	cfg GreenAPIConfig
}

func (s *greenAPISender) Send(ctx context.Context, message string) error {
	apiURL := strings.TrimSpace(s.cfg.APIURL)
	if apiURL == "" {
		apiURL = defaultGreenAPIURL
	}
	apiURL = strings.TrimRight(apiURL, "/")

	instanceID := strings.TrimSpace(s.cfg.InstanceID)
	token := strings.TrimSpace(s.cfg.Token)
	chatID := strings.TrimSpace(s.cfg.Phone)
	if !strings.Contains(chatID, "@") {
		chatID += "@c.us"
	}

	url := fmt.Sprintf("%s/waInstance%s/sendMessage/%s", apiURL, instanceID, token)

	body, _ := json.Marshal(map[string]string{
		"chatId":  chatID,
		"message": message,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := safeHTTPClient().Do(req)
	if err != nil {
		// Do not wrap err directly — it contains the credential-bearing URL.
		return fmt.Errorf("greenapi: send failed (network error)")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("greenapi: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
