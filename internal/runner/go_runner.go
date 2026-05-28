package runner

import (
	"context"
	"fmt"

	"github.com/showwin/speedtest-go/speedtest"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

// GoRunner uses the pure-Go showwin/speedtest-go backend (no binary required).
type GoRunner struct{}

// NewGoRunner creates a GoRunner.
func NewGoRunner() *GoRunner { return &GoRunner{} }

// Engine returns EngineGo.
func (r *GoRunner) Engine() model.Engine { return model.EngineGo }

// Run performs a full speed test and returns a partial Result.
// The service layer fills in Timestamp, Source, and DurationSec.
func (r *GoRunner) Run(ctx context.Context) (*model.Result, error) {
	client := speedtest.New()

	user, _ := client.FetchUserInfo()

	serverList, err := client.FetchServers()
	if err != nil {
		return nil, fmt.Errorf("fetch servers: %w", err)
	}
	targets, err := serverList.FindServer([]int{})
	if err != nil {
		return nil, fmt.Errorf("find nearest server: %w", err)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no servers available")
	}

	server := targets[0]

	if err := server.PingTestContext(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping test: %w", err)
	}
	if err := server.DownloadTestContext(ctx); err != nil {
		return nil, fmt.Errorf("download test: %w", err)
	}
	if err := server.UploadTestContext(ctx); err != nil {
		return nil, fmt.Errorf("upload test: %w", err)
	}

	result := &model.Result{
		Engine:       model.EngineGo,
		DownloadMbps: float64(server.DLSpeed.Mbps()),
		UploadMbps:   float64(server.ULSpeed.Mbps()),
		PingMs:       server.Latency.Seconds() * 1000,
		JitterMs:     server.Jitter.Seconds() * 1000,
		PacketLoss:   server.PacketLoss.Loss() / 100,
		ServerName:   server.Name,
		ServerID:     server.ID,
	}

	if user != nil {
		result.ISP = user.Isp
		result.ExternalIP = user.IP
	}

	return result, nil
}
