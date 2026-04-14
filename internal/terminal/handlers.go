package terminal

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cs-cloud/internal/logger"
)

type Handlers struct {
	mgr *TerminalManager
}

func NewHandlers(mgr *TerminalManager) *Handlers {
	return &Handlers{mgr: mgr}
}

type createReq struct {
	Cwd  string `json:"cwd"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type sessionResp struct {
	SessionID string `json:"sessionId"`
	Pid       int    `json:"pid"`
}

type resizeReq struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type restartReq struct {
	Cwd string `json:"cwd"`
}

type inputReq struct {
	Data string `json:"data"`
}

func (h *Handlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	if req.Rows == 0 {
		req.Rows = 24
	}
	if req.Cols == 0 {
		req.Cols = 80
	}

	s, err := h.mgr.Create(req.Cwd, req.Rows, req.Cols)
	if err != nil {
		writeErr(w, http.StatusConflict, "SESSION_LIMIT", err.Error())
		return
	}

	writeOK(w, sessionResp{SessionID: s.ID, Pid: s.Pid})
}

func (h *Handlers) HandleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.mgr.Kill(id); err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, struct{}{})
}

func (h *Handlers) HandleResize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req resizeReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := h.mgr.Resize(id, req.Rows, req.Cols); err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, struct{}{})
}

func (h *Handlers) HandleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req restartReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	s, err := h.mgr.Restart(id, req.Cwd)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, sessionResp{SessionID: s.ID, Pid: s.Pid})
}

func (h *Handlers) HandleInput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req inputReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid base64 data")
		return
	}

	if err := h.mgr.Write(id, data); err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, struct{}{})
}

func (h *Handlers) HandleStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ch, unsub, err := h.mgr.Subscribe(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, canFlush := w.(http.Flusher)

	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	if canFlush {
		flusher.Flush()
	}

	heartbeatSec := 15
	if v := r.URL.Query().Get("heartbeat"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			heartbeatSec = n
		}
	}
	heartbeat := time.NewTicker(time.Duration(heartbeatSec) * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			logger.Debug("terminal: SSE client disconnected id=%s", id)
			return

		case data, ok := <-ch:
			if !ok || data == nil {
				fmt.Fprintf(w, "event: exit\ndata: {\"exitCode\":0}\n\n")
				if canFlush {
					flusher.Flush()
				}
				return
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(w, "event: data\ndata: %s\n\n", encoded)
			if canFlush {
				flusher.Flush()
			}

		case <-heartbeat.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

func readJSON(r *http.Request, v any) error {
	body, err := readBody(r)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	return jsonUnmarshal(body, v)
}

func writeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonEncode(w, envelope{OK: true, Data: data})
}

func writeErr(w http.ResponseWriter, status int, code string, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	jsonEncode(w, envelope{OK: false, Error: &errVal{Code: code, Message: msg}})
}
