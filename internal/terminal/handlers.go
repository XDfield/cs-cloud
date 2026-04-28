package terminal

import (
	"errors"
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

// @Summary      Create terminal session
// @Description  Creates a new PTY terminal session. Maximum 20 concurrent sessions.
// @Tags         Terminal
// @Accept       json
// @Produce      json
// @Param        body  body  createReq  true  "Terminal creation parameters"
// @Success      200  {object}  responseEnvelope{data=sessionResp}
// @Failure      400  {object}  responseEnvelope
// @Failure      409  {object}  responseEnvelope
// @Failure      500  {object}  responseEnvelope
// @Router       /terminal [post]
func (h *Handlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	logger.Debug("terminal: HandleCreate body=%q content-length=%s", string(body), r.Header.Get("Content-Length"))

	var req createReq
	if err := jsonUnmarshal(body, &req); err != nil {
		logger.Debug("terminal: HandleCreate json error: %v", err)
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
		logger.Error("terminal: HandleCreate failed cwd=%q rows=%d cols=%d err=%v", req.Cwd, req.Rows, req.Cols, err)
		switch {
		case errors.Is(err, ErrSessionLimit):
			writeErr(w, http.StatusConflict, "SESSION_LIMIT", err.Error())
		case IsSessionCreateError(err):
			writeErr(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED", err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	writeOK(w, sessionResp{SessionID: s.ID, Pid: s.Pid})
}

// @Summary      Kill terminal session
// @Description  Terminates the PTY process and removes the session.
// @Tags         Terminal
// @Produce      json
// @Param        id  path  string  true  "Terminal session ID"
// @Success      200  {object}  responseEnvelope{data=map[string]any}
// @Failure      404  {object}  responseEnvelope
// @Router       /terminal/{id} [delete]
func (h *Handlers) HandleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.mgr.Kill(id); err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, struct{}{})
}

// @Summary      Resize terminal
// @Description  Changes the PTY dimensions (rows and columns).
// @Tags         Terminal
// @Accept       json
// @Produce      json
// @Param        id    path  string    true  "Terminal session ID"
// @Param        body  body  resizeReq  true  "New dimensions"
// @Success      200  {object}  responseEnvelope{data=map[string]any}
// @Failure      400  {object}  responseEnvelope
// @Failure      404  {object}  responseEnvelope
// @Router       /terminal/{id}/resize [post]
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

// @Summary      Restart terminal session
// @Description  Restarts the PTY process, optionally with a new working directory.
// @Tags         Terminal
// @Accept       json
// @Produce      json
// @Param        id    path  string      true  "Terminal session ID"
// @Param        body  body  restartReq  false  "Restart parameters"
// @Success      200  {object}  responseEnvelope{data=sessionResp}
// @Failure      400  {object}  responseEnvelope
// @Failure      404  {object}  responseEnvelope
// @Router       /terminal/{id}/restart [post]
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

// @Summary      Send input to terminal
// @Description  Sends raw bytes (base64-encoded) to the PTY stdin.
// @Tags         Terminal
// @Accept       json
// @Produce      json
// @Param        id    path  string    true  "Terminal session ID"
// @Param        body  body  inputReq  true  "Base64-encoded input data"
// @Success      200  {object}  responseEnvelope{data=map[string]any}
// @Failure      400  {object}  responseEnvelope
// @Failure      404  {object}  responseEnvelope
// @Router       /terminal/{id}/input [post]
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

// @Summary      SSE terminal output stream
// @Description  Subscribes to terminal output as an SSE stream. Events: connected, data (base64-encoded), heartbeat, exit.
// @Tags         Terminal
// @Produce      text/event-stream
// @Param        id          path  int     true  "Terminal session ID"
// @Param        heartbeat   query  int    false  "Heartbeat interval in seconds"  minimum(1)  default(15)
// @Success      200  {string}  string  "SSE stream"
// @Failure      404  {object}  responseEnvelope
// @Router       /terminal/{id}/stream [get]
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
