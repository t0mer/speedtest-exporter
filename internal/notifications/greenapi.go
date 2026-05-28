package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const defaultGreenAPIURL = "https://api.green-api.com"

type greenAPISender struct {
	cfg GreenAPIConfig
}

func (s *greenAPISender) Send(ctx context.Context, message string) error {
	apiURL := strings.TrimRight(s.cfg.APIURL, "/")
	if apiURL == "" {
		apiURL = defaultGreenAPIURL
	}
	url := fmt.Sprintf("%s/waInstance%s/sendMessage/%s", apiURL, s.cfg.InstanceID, s.cfg.Token)

	body, _ := json.Marshal(map[string]string{
		"chatId":  s.cfg.Phone + "@c.us",
		"message": message,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("greenapi: HTTP %d", resp.StatusCode)
	}
	return nil
}
