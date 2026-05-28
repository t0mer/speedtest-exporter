package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/runner"
)

// handleStreamTest runs a speed test and streams Server-Sent Events to the
// client as each phase completes. The stream closes when the test finishes
// or the client disconnects.
//
// Event format: "data: <JSON>\n\n"
// Phases: connecting → ping → download → upload → done | error
func (s *Server) handleStreamTest(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported by this server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // tell nginx not to buffer SSE
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	progress := make(chan runner.ProgressEvent, 64)

	// Run the test in a goroutine; close progress when it finishes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(progress)
		s.service.RunWithProgress(r.Context(), model.SourceAPI, progress)
	}()

	// Stream every event to the client as it arrives.
	for ev := range progress {
		data, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	<-done
}
