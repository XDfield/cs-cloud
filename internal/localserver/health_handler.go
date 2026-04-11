package localserver

import (
	"net/http"
	"time"
)

type healthData struct {
	Status  string `json:"status"`
	Uptime  int64  `json:"uptime"`
	Version string `json:"version"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, healthData{
		Status:  "ok",
		Uptime:  int64(time.Since(startTime).Seconds()),
		Version: s.version,
	})
}
