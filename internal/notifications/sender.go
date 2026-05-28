package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Sender delivers a single notification message.
type Sender interface {
	Send(ctx context.Context, message string) error
}

// NewSender returns the correct Sender for the channel's provider and config.
// It validates outbound URLs to prevent SSRF before returning a Sender.
func NewSender(ch *Channel) (Sender, error) {
	switch ch.Provider {
	case ProviderShoutrrr:
		var cfg ShoutrrrConfig
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("parse shoutrrr config: %w", err)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("shoutrrr: url is required")
		}
		// generic:// makes arbitrary HTTP requests — disallow to prevent SSRF.
		if strings.HasPrefix(strings.ToLower(cfg.URL), "generic://") {
			return nil, fmt.Errorf("shoutrrr: generic:// scheme is not allowed")
		}
		return &shoutrrrSender{url: cfg.URL}, nil

	case ProviderGreenAPI:
		var cfg GreenAPIConfig
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("parse greenapi config: %w", err)
		}
		if cfg.InstanceID == "" || cfg.Token == "" || cfg.Phone == "" {
			return nil, fmt.Errorf("greenapi: instance_id, token, and phone are required")
		}
		// Validate custom api_url to prevent SSRF; default is trusted.
		if cfg.APIURL != "" {
			if err := validateOutboundURL(cfg.APIURL); err != nil {
				return nil, fmt.Errorf("greenapi api_url: %w", err)
			}
		}
		return &greenAPISender{cfg: cfg}, nil

	case ProviderWhatsAppWeb:
		var cfg WhatsAppWebConfig
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("parse whatsapp config: %w", err)
		}
		if cfg.BaseURL == "" || cfg.Phone == "" {
			return nil, fmt.Errorf("whatsapp_web: base_url and phone are required")
		}
		if err := validateOutboundURL(cfg.BaseURL); err != nil {
			return nil, fmt.Errorf("whatsapp_web base_url: %w", err)
		}
		return &whatsAppWebSender{cfg: cfg}, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", ch.Provider)
	}
}
