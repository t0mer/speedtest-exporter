package runner_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/runner"
)

func TestOoklaRunnerEngine(t *testing.T) {
	r := runner.NewOoklaRunner("speedtest")
	assert.Equal(t, model.EngineOokla, r.Engine())
}

func TestOoklaRunnerParseOutput(t *testing.T) {
	// Create a fake "speedtest" binary that emits the expected JSON.
	dir := t.TempDir()
	fakePath := filepath.Join(dir, "speedtest")

	fakeJSON := `{"download":{"bandwidth":18750000},"upload":{"bandwidth":6250000},"ping":{"latency":12.5,"jitter":1.2},"packetLoss":0.5,"server":{"name":"TestCity","id":12345},"interface":{"externalIp":"5.6.7.8"},"isp":"TestISP"}`
	script := "#!/bin/sh\necho '" + fakeJSON + "'"
	require.NoError(t, os.WriteFile(fakePath, []byte(script), 0o755))

	// Skip if we can't execute the fake binary (e.g., unusual OS restrictions).
	if _, err := exec.LookPath(fakePath); err != nil {
		t.Skip("cannot execute test binary:", err)
	}

	r := runner.NewOoklaRunner(fakePath)
	result, err := r.Run(context.Background())
	require.NoError(t, err)

	assert.InDelta(t, 150.0, result.DownloadMbps, 0.01) // 18750000 B/s * 8 / 1e6
	assert.InDelta(t, 50.0, result.UploadMbps, 0.01)    // 6250000 B/s * 8 / 1e6
	assert.InDelta(t, 12.5, result.PingMs, 0.01)
	assert.InDelta(t, 1.2, result.JitterMs, 0.01)
	assert.InDelta(t, 0.005, result.PacketLoss, 0.001) // 0.5% → 0.005 ratio
	assert.Equal(t, "TestCity", result.ServerName)
	assert.Equal(t, "12345", result.ServerID)
	assert.Equal(t, "TestISP", result.ISP)
	assert.Equal(t, "5.6.7.8", result.ExternalIP)
}
