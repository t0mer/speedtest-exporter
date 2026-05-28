package api

import (
	"net/http"

	"github.com/showwin/speedtest-go/speedtest"
)

// ServerInfo is the wire representation of a Speedtest.net server.
type ServerInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Country    string  `json:"country"`
	Sponsor    string  `json:"sponsor"`
	DistanceKm float64 `json:"distance_km"`
}

// handleListServers fetches the nearest Speedtest.net servers and returns them
// as JSON. The list is sorted by distance (closest first).
func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	client := speedtest.New()
	list, err := client.FetchServerListContext(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "failed to fetch server list: check internet connectivity")
		return
	}

	servers := make([]ServerInfo, 0, len(list))
	for _, srv := range list {
		servers = append(servers, ServerInfo{
			ID:         srv.ID,
			Name:       srv.Name,
			Country:    srv.Country,
			Sponsor:    srv.Sponsor,
			DistanceKm: srv.Distance,
		})
	}

	writeJSON(w, http.StatusOK, servers)
}
