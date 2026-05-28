// Package config loads and validates the speedtest-exporter runtime configuration.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the complete runtime configuration.
type Config struct {
	Engine     string           `mapstructure:"engine"`
	OoklaPath  string           `mapstructure:"ookla_path"`
	DataDir    string           `mapstructure:"data_dir"`
	LogLevel   string           `mapstructure:"log_level"`
	Schedule   string           `mapstructure:"schedule"`
	Server     ServerConfig     `mapstructure:"server"`
	Thresholds ThresholdsConfig `mapstructure:"thresholds"`
	Webhooks   []string         `mapstructure:"webhooks"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	Host         string `mapstructure:"host"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	EnableUI     bool   `mapstructure:"enable_ui"`
}

// ThresholdsConfig defines breach limits for notifications.
type ThresholdsConfig struct {
	MinDownloadMbps float64 `mapstructure:"min_download_mbps"`
	MinUploadMbps   float64 `mapstructure:"min_upload_mbps"`
	MaxPingMs       float64 `mapstructure:"max_ping_ms"`
	MaxJitterMs     float64 `mapstructure:"max_jitter_ms"`
	MaxPacketLossRatio float64 `mapstructure:"max_packet_loss_ratio"`
	CooldownMinutes int     `mapstructure:"cooldown_minutes"`
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		Engine:    "go",
		OoklaPath: "speedtest",
		DataDir:   "./data",
		LogLevel:  "info",
		Schedule:  "@every 1h",
		Server: ServerConfig{
			Port:         9090,
			Host:         "0.0.0.0",
			ReadTimeout:  10,
			WriteTimeout: 120,
			EnableUI:     true,
		},
		Thresholds: ThresholdsConfig{
			CooldownMinutes: 30,
		},
	}
}

// Load reads config from path (optional), then overlays SPEEDTEST_* env vars.
// Precedence: env > file > defaults.
func Load(path string) (*Config, error) {
	v := viper.New()

	def := Default()
	v.SetDefault("engine", def.Engine)
	v.SetDefault("ookla_path", def.OoklaPath)
	v.SetDefault("data_dir", def.DataDir)
	v.SetDefault("log_level", def.LogLevel)
	v.SetDefault("schedule", def.Schedule)
	v.SetDefault("server.port", def.Server.Port)
	v.SetDefault("server.host", def.Server.Host)
	v.SetDefault("server.read_timeout", def.Server.ReadTimeout)
	v.SetDefault("server.write_timeout", def.Server.WriteTimeout)
	v.SetDefault("server.enable_ui", def.Server.EnableUI)
	v.SetDefault("thresholds.cooldown_minutes", def.Thresholds.CooldownMinutes)
	v.SetDefault("thresholds.min_download_mbps", def.Thresholds.MinDownloadMbps)
	v.SetDefault("thresholds.min_upload_mbps", def.Thresholds.MinUploadMbps)
	v.SetDefault("thresholds.max_ping_ms", def.Thresholds.MaxPingMs)
	v.SetDefault("thresholds.max_jitter_ms", def.Thresholds.MaxJitterMs)
	v.SetDefault("thresholds.max_packet_loss_ratio", def.Thresholds.MaxPacketLossRatio)
	v.SetDefault("webhooks", def.Webhooks)

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
	}

	v.SetEnvPrefix("SPEEDTEST")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
