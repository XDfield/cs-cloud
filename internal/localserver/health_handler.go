package localserver

import (
	"net/http"
	"time"
)

type healthData struct {
	Status  string `json:"status" example:"ok"`
	Uptime  int64  `json:"uptime" example:"12345"`
	Version string `json:"version" example:"1.0.0"`
}

// @Summary      Health check
// @Description  Returns server health status, uptime and version.
// @Tags         Runtime
// @Produce      json
// @Success      200  {object}  envelope{data=healthData}
// @Router       /runtime/health [get]
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, healthData{
		Status:  "ok",
		Uptime:  int64(time.Since(startTime).Seconds()),
		Version: s.version,
	})
}
