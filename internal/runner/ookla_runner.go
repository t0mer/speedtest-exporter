package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/t0mer/speedtest-exporter/internal/model"
)

// OoklaRunner shells out to the official Ookla speedtest CLI.
// Install the CLI from https://www.speedtest.net/apps/cli
type OoklaRunner struct {
	path string
}

// NewOoklaRunner creates an OoklaRunner using the binary at path.
func NewOoklaRunner(path string) *OoklaRunner { return &OoklaRunner{path: path} }

// Engine returns EngineOokla.
func (r *OoklaRunner) Engine() model.Engine { return model.EngineOokla }

type ooklaOutput struct {
	Download struct{ Bandwidth float64 } `json:"download"`
	Upload   struct{ Bandwidth float64 } `json:"upload"`
	Ping     struct {
		Latency float64 `json:"latency"`
		Jitter  float64 `json:"jitter"`
	} `json:"ping"`
	PacketLoss float64 `json:"packetLoss"`
	Server     struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	} `json:"server"`
	Interface struct{ ExternalIp string } `json:"interface"`
	ISP       string                      `json:"isp"`
}

// Run executes the Ookla CLI and parses its JSON output.
func (r *OoklaRunner) Run(ctx context.Context) (*model.Result, error) {
	cmd := exec.CommandContext(ctx, r.path,
		"--format=json",
		"--accept-license",
		"--accept-gdpr",
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("speedtest CLI: %w (stderr: %s)", err, errBuf.String())
	}

	var o ooklaOutput
	if err := json.Unmarshal(out.Bytes(), &o); err != nil {
		return nil, fmt.Errorf("parse CLI output: %w", err)
	}

	return &model.Result{
		Engine:       model.EngineOokla,
		DownloadMbps: o.Download.Bandwidth * 8 / 1_000_000,
		UploadMbps:   o.Upload.Bandwidth * 8 / 1_000_000,
		PingMs:       o.Ping.Latency,
		JitterMs:     o.Ping.Jitter,
		PacketLoss:   o.PacketLoss / 100,
		ServerName:   o.Server.Name,
		ServerID:     strconv.Itoa(o.Server.ID),
		ISP:          o.ISP,
		ExternalIP:   o.Interface.ExternalIp,
	}, nil
}
