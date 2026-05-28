// Package notifications manages multi-channel notification delivery.
package notifications

import (
	"encoding/json"
	"strings"
)

// Provider identifies the notification backend.
type Provider string

const (
	ProviderShoutrrr    Provider = "shoutrrr"
	ProviderGreenAPI    Provider = "greenapi"
	ProviderWhatsAppWeb Provider = "whatsapp_web"
)

// Channel is a single notification delivery target stored in the database.
type Channel struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Provider        Provider        `json:"provider"`
	Config          json.RawMessage `json:"-"` // decrypted; never sent over the wire
	Enabled         bool            `json:"enabled"`
	NotifyOnSuccess bool            `json:"notify_on_success"`
	NotifyOnFailure bool            `json:"notify_on_failure"`
}

// ShoutrrrConfig holds the Shoutrrr provider configuration.
type ShoutrrrConfig struct {
	URL string `json:"url"`
}

// GreenAPIConfig holds the GreenAPI (WhatsApp cloud) provider configuration.
type GreenAPIConfig struct {
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
	Phone      string `json:"phone"`
	APIURL     string `json:"api_url"`
}

// WhatsAppWebConfig holds the go-whatsapp-web-multidevice provider configuration.
type WhatsAppWebConfig struct {
	BaseURL  string `json:"base_url"`
	Phone    string `json:"phone"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ChannelRequest is the wire format for creating or updating a channel.
type ChannelRequest struct {
	Name            string          `json:"name"`
	Provider        Provider        `json:"provider"`
	Config          json.RawMessage `json:"config"`
	Enabled         bool            `json:"enabled"`
	NotifyOnSuccess bool            `json:"notify_on_success"`
	NotifyOnFailure bool            `json:"notify_on_failure"`
}

// ChannelView is the API response for a channel — credential fields are masked.
type ChannelView struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Provider        Provider        `json:"provider"`
	Config          json.RawMessage `json:"config"`
	Enabled         bool            `json:"enabled"`
	NotifyOnSuccess bool            `json:"notify_on_success"`
	NotifyOnFailure bool            `json:"notify_on_failure"`
}

// ToView converts a Channel to its API response form with masked credentials.
func (ch *Channel) ToView() ChannelView {
	return ChannelView{
		ID:              ch.ID,
		Name:            ch.Name,
		Provider:        ch.Provider,
		Config:          MaskConfig(ch.Provider, ch.Config),
		Enabled:         ch.Enabled,
		NotifyOnSuccess: ch.NotifyOnSuccess,
		NotifyOnFailure: ch.NotifyOnFailure,
	}
}

// MaskConfig replaces credential fields with "***" before sending to the client.
func MaskConfig(provider Provider, config json.RawMessage) json.RawMessage {
	switch provider {
	case ProviderShoutrrr:
		var c ShoutrrrConfig
		if err := json.Unmarshal(config, &c); err == nil {
			if idx := strings.Index(c.URL, "://"); idx >= 0 {
				c.URL = c.URL[:idx+3] + "***"
			} else {
				c.URL = "***"
			}
			if b, err := json.Marshal(c); err == nil {
				return b
			}
		}
	case ProviderGreenAPI:
		var c GreenAPIConfig
		if err := json.Unmarshal(config, &c); err == nil {
			c.Token = "***"
			if b, err := json.Marshal(c); err == nil {
				return b
			}
		}
	case ProviderWhatsAppWeb:
		var c WhatsAppWebConfig
		if err := json.Unmarshal(config, &c); err == nil {
			if c.Password != "" {
				c.Password = "***"
			}
			if b, err := json.Marshal(c); err == nil {
				return b
			}
		}
	}
	return json.RawMessage(`{}`)
}
