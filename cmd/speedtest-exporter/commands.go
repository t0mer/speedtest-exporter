package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/t0mer/speedtest-exporter/internal/api"
	"github.com/t0mer/speedtest-exporter/internal/config"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/metrics"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/notify"
	"github.com/t0mer/speedtest-exporter/internal/runner"
	"github.com/t0mer/speedtest-exporter/internal/scheduler"
	"github.com/t0mer/speedtest-exporter/internal/service"
)

// version is set at build time via -ldflags.
var version = "dev"

func rootCmd() *cobra.Command {
	var cfgFile string

	root := &cobra.Command{
		Use:     "speedtest-exporter",
		Short:   "Internet speed monitoring with Prometheus metrics",
		Version: version,
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "Path to config file (YAML)")
	root.AddCommand(newRunCmd(&cfgFile))
	root.AddCommand(newServeCmd(&cfgFile))
	return root
}

func newRunCmd(cfgFile *string) *cobra.Command {
	var outputJSON bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a single speed test and print the result",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgFile)
			if err != nil {
				return err
			}
			svc, err := buildService(cfg)
			if err != nil {
				return err
			}
			defer svc.Close()

			result, err := svc.Run(context.Background(), model.SourceManual)
			if err != nil {
				return err
			}

			if outputJSON {
				return json.NewEncoder(os.Stdout).Encode(result)
			}
			fmt.Printf("Download:  %.2f Mbps\n", result.DownloadMbps)
			fmt.Printf("Upload:    %.2f Mbps\n", result.UploadMbps)
			fmt.Printf("Ping:      %.2f ms\n", result.PingMs)
			fmt.Printf("Jitter:    %.2f ms\n", result.JitterMs)
			fmt.Printf("Server:    %s\n", result.ServerName)
			fmt.Printf("ISP:       %s\n", result.ISP)
			fmt.Printf("Duration:  %.1fs\n", result.DurationSec)
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Print result as JSON")
	return cmd
}

func newServeCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server (API, /metrics, Web UI) with optional scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgFile)
			if err != nil {
				return err
			}
			svc, err := buildService(cfg)
			if err != nil {
				return err
			}
			defer svc.Close()

			if cfg.Schedule != "" {
				sched, err := scheduler.New(svc, cfg.Schedule)
				if err != nil {
					return fmt.Errorf("scheduler: %w", err)
				}
				sched.Start()
				defer sched.Stop()
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-quit; os.Exit(0) }()

			return api.NewServer(svc, cfg).ListenAndServe()
		},
	}
}

// buildService assembles all dependencies from cfg.
func buildService(cfg *config.Config) (*service.Service, error) {
	db, err := database.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	var r runner.Runner
	switch model.Engine(cfg.Engine) {
	case model.EngineOokla:
		r = runner.NewOoklaRunner(cfg.OoklaPath)
	default:
		r = runner.NewGoRunner()
	}

	m := metrics.New()
	n := notify.New(notify.ThresholdConfig{
		MinDownloadMbps: cfg.Thresholds.MinDownloadMbps,
		MinUploadMbps:   cfg.Thresholds.MinUploadMbps,
		MaxPingMs:       cfg.Thresholds.MaxPingMs,
		MaxJitterMs:     cfg.Thresholds.MaxJitterMs,
		MaxPacketLoss:   cfg.Thresholds.MaxPacketLossRatio,
		CooldownMinutes: cfg.Thresholds.CooldownMinutes,
	}, cfg.Webhooks)

	return service.New(db, r, m, n), nil
}
