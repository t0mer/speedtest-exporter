package runner

// ProgressPhase identifies the current stage of a live speed test.
type ProgressPhase string

const (
	PhaseConnecting ProgressPhase = "connecting" // server selected, starting
	PhasePing       ProgressPhase = "ping"        // per-ping result or final avg
	PhaseDownload   ProgressPhase = "download"    // live download speed sample
	PhaseUpload     ProgressPhase = "upload"      // live upload speed sample
	PhaseDone       ProgressPhase = "done"        // test complete
	PhaseError      ProgressPhase = "error"       // test failed
)

// ProgressEvent is a single update streamed to the client during a live test.
type ProgressEvent struct {
	Phase        ProgressPhase `json:"phase"`
	ServerName   string        `json:"server_name,omitempty"`
	ServerID     string        `json:"server_id,omitempty"`
	PingMs       float64       `json:"ping_ms,omitempty"`
	DownloadMbps float64       `json:"download_mbps,omitempty"`
	UploadMbps   float64       `json:"upload_mbps,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// SendEvent sends ev to ch without blocking. Safe to call with a nil channel.
func SendEvent(ch chan<- ProgressEvent, ev ProgressEvent) {
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}
