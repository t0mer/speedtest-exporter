package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
	"github.com/t0mer/speedtest-exporter/internal/model"
)

// GoRunner uses the pure-Go showwin/speedtest-go backend (no binary required).
// It implements ProgressRunner: live speed samples are sent every 250 ms while
// download/upload tests are running.
type GoRunner struct {
	preferredServerID string // numeric Speedtest.net server ID; empty = nearest
}

// NewGoRunner creates a GoRunner that picks the nearest available server.
func NewGoRunner() *GoRunner { return &GoRunner{} }

// NewGoRunnerWithPreferredServer creates a GoRunner that tries serverID first,
// falling back to the nearest server on failure.
func NewGoRunnerWithPreferredServer(serverID string) *GoRunner {
	return &GoRunner{preferredServerID: serverID}
}

// Engine returns EngineGo.
func (r *GoRunner) Engine() model.Engine { return model.EngineGo }

// Run delegates to RunWithProgress with a nil channel (no progress streaming).
func (r *GoRunner) Run(ctx context.Context) (*model.Result, error) {
	return r.RunWithProgress(ctx, nil)
}

// RunWithProgress runs the full test and streams ProgressEvents to progress.
// Passing a nil channel is safe and equivalent to Run.
// Phase sequence: connecting → ping (multiple) → download (polled) → upload (polled) → done.
// If the preferred server fails at any stage, the nearest server is retried automatically.
func (r *GoRunner) RunWithProgress(ctx context.Context, progress chan<- ProgressEvent) (*model.Result, error) {
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

	SendEvent(progress, ProgressEvent{
		Phase: PhaseConnecting, ServerName: server.Name, ServerID: server.ID,
	})

	result, err := r.measureOnServer(ctx, server, user, progress)
	if err != nil && usingPreferred {
		slog.Warn("preferred server failed, retrying with nearest",
			"preferred_id", r.preferredServerID, "server", server.Name, "error", err)
		nearest, ferr := serverList.FindServer([]int{})
		if ferr == nil && len(nearest) > 0 && nearest[0].ID != server.ID {
			SendEvent(progress, ProgressEvent{
				Phase: PhaseConnecting, ServerName: nearest[0].Name, ServerID: nearest[0].ID,
			})
			result, err = r.measureOnServer(ctx, nearest[0], user, progress)
		}
	}
	return result, err
}

// pickServer returns the server to use and whether it is the user's preferred one.
func (r *GoRunner) pickServer(serverList speedtest.Servers) (*speedtest.Server, bool) {
	if r.preferredServerID != "" {
		if id, err := strconv.Atoi(r.preferredServerID); err == nil {
			if targets, err := serverList.FindServer([]int{id}); err == nil && len(targets) > 0 {
				return targets[0], true
			}
		}
		slog.Info("preferred server not in list, using nearest", "preferred_id", r.preferredServerID)
	}
	targets, _ := serverList.FindServer([]int{})
	if len(targets) == 0 {
		return nil, false
	}
	return targets[0], false
}

// measureOnServer runs ping → download → upload on a single server, streaming
// ProgressEvents to progress throughout.
func (r *GoRunner) measureOnServer(
	ctx context.Context,
	server *speedtest.Server,
	user *speedtest.User,
	progress chan<- ProgressEvent,
) (*model.Result, error) {
	// Ping — callback fires per attempt so the UI shows live latency updates.
	if err := server.PingTestContext(ctx, func(d time.Duration) {
		SendEvent(progress, ProgressEvent{
			Phase: PhasePing, PingMs: float64(d.Milliseconds()),
			ServerName: server.Name, ServerID: server.ID,
		})
	}); err != nil {
		return nil, fmt.Errorf("ping on %s: %w", server.Name, err)
	}
	// Send final averaged ping result.
	SendEvent(progress, ProgressEvent{
		Phase: PhasePing, PingMs: server.Latency.Seconds() * 1000,
		ServerName: server.Name, ServerID: server.ID,
	})

	// Download — poll DLSpeed every 250 ms while the test blocks.
	if err := r.runPhase(ctx, progress, PhaseDownload,
		func(c context.Context) error { return server.DownloadTestContext(c) },
		func() float64 { return server.DLSpeed.Mbps() },
	); err != nil {
		return nil, fmt.Errorf("download on %s: %w", server.Name, err)
	}
	SendEvent(progress, ProgressEvent{Phase: PhaseDownload, DownloadMbps: server.DLSpeed.Mbps()})

	// Upload — poll ULSpeed every 250 ms.
	if err := r.runPhase(ctx, progress, PhaseUpload,
		func(c context.Context) error { return server.UploadTestContext(c) },
		func() float64 { return server.ULSpeed.Mbps() },
	); err != nil {
		return nil, fmt.Errorf("upload on %s: %w", server.Name, err)
	}
	SendEvent(progress, ProgressEvent{Phase: PhaseUpload, UploadMbps: server.ULSpeed.Mbps()})

	pl := server.PacketLoss.Loss()
	if pl < 0 {
		pl = 0
	}
	result := &model.Result{
		Engine:       model.EngineGo,
		DownloadMbps: server.DLSpeed.Mbps(),
		UploadMbps:   server.ULSpeed.Mbps(),
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

// runPhase runs testFn (a blocking download or upload call) while concurrently
// polling speedFn every 250 ms and streaming the reading to progress.
func (r *GoRunner) runPhase(
	ctx context.Context,
	progress chan<- ProgressEvent,
	phase ProgressPhase,
	testFn func(context.Context) error,
	speedFn func() float64,
) error {
	if progress == nil {
		// No streaming — just run.
		return testFn(ctx)
	}

	pollCtx, stopPoll := context.WithCancel(ctx)
	defer stopPoll()

	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				if mbps := speedFn(); mbps > 0 {
					ev := ProgressEvent{Phase: phase}
					if phase == PhaseDownload {
						ev.DownloadMbps = mbps
					} else {
						ev.UploadMbps = mbps
					}
					SendEvent(progress, ev)
				}
			case <-pollCtx.Done():
				return
			}
		}
	}()

	return testFn(ctx)
}
