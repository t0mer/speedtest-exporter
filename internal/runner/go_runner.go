package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/showwin/speedtest-go/speedtest"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

// GoRunner uses the pure-Go showwin/speedtest-go backend (no binary required).
// If PreferredServerID is set, it attempts that server first; on any failure it
// automatically retries with the nearest available server.
type GoRunner struct {
	preferredServerID string // numeric Speedtest.net server ID; empty = nearest
}

// NewGoRunner creates a GoRunner that selects the nearest available server.
func NewGoRunner() *GoRunner { return &GoRunner{} }

// NewGoRunnerWithPreferredServer creates a GoRunner that tries serverID first,
// falling back to nearest on failure.
func NewGoRunnerWithPreferredServer(serverID string) *GoRunner {
	return &GoRunner{preferredServerID: serverID}
}

// Engine returns EngineGo.
func (r *GoRunner) Engine() model.Engine { return model.EngineGo }

// Run performs a full speed test. It tries the preferred server if configured;
// if that server is unavailable or the test fails, it retries with the nearest server.
func (r *GoRunner) Run(ctx context.Context) (*model.Result, error) {
	client := speedtest.New()

	user, _ := client.FetchUserInfoContext(ctx)

	serverList, err := client.FetchServerListContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch servers: %w", err)
	}

	server, usingPreferred := r.pickServer(serverList)
	if server == nil {
		return nil, fmt.Errorf("no servers available")
	}

	result, err := r.runOnServer(ctx, server, user)
	if err != nil && usingPreferred {
		// Preferred server failed — log and fall back to nearest.
		slog.Warn("preferred server failed, retrying with nearest server",
			"preferred_id", r.preferredServerID,
			"server", server.Name,
			"error", err,
		)
		nearest, ferr := serverList.FindServer([]int{})
		if ferr == nil && len(nearest) > 0 && nearest[0].ID != server.ID {
			result, err = r.runOnServer(ctx, nearest[0], user)
		}
	}

	return result, err
}

// pickServer returns the server to use and whether it is the preferred one.
func (r *GoRunner) pickServer(serverList speedtest.Servers) (*speedtest.Server, bool) {
	if r.preferredServerID != "" {
		id, err := strconv.Atoi(r.preferredServerID)
		if err == nil {
			targets, err := serverList.FindServer([]int{id})
			if err == nil && len(targets) > 0 {
				return targets[0], true
			}
		}
		slog.Info("preferred server not found in list, using nearest",
			"preferred_id", r.preferredServerID)
	}
	targets, _ := serverList.FindServer([]int{})
	if len(targets) == 0 {
		return nil, false
	}
	return targets[0], false
}

// runOnServer executes ping, download, and upload tests on a single server.
func (r *GoRunner) runOnServer(ctx context.Context, server *speedtest.Server, user *speedtest.User) (*model.Result, error) {
	if err := server.PingTestContext(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping on %s: %w", server.Name, err)
	}
	if err := server.DownloadTestContext(ctx); err != nil {
		return nil, fmt.Errorf("download on %s: %w", server.Name, err)
	}
	if err := server.UploadTestContext(ctx); err != nil {
		return nil, fmt.Errorf("upload on %s: %w", server.Name, err)
	}

	pl := server.PacketLoss.Loss()
	if pl < 0 {
		pl = 0
	}

	result := &model.Result{
		Engine:       model.EngineGo,
		DownloadMbps: float64(server.DLSpeed.Mbps()),
		UploadMbps:   float64(server.ULSpeed.Mbps()),
		PingMs:       server.Latency.Seconds() * 1000,
		JitterMs:     server.Jitter.Seconds() * 1000,
		PacketLoss:   pl,
		ServerName:   server.Name,
		ServerID:     server.ID,
	}

	if user != nil {
		result.ISP = user.Isp
		result.ExternalIP = user.IP
	}

	return result, nil
}
