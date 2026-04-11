package localserver

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type envelope struct {
	OK    bool    `json:"ok"`
	Data  any     `json:"data"`
	Error *errVal `json:"error"`
}

type errVal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{OK: true, Data: data})
}

func writeErr(w http.ResponseWriter, status int, code string, msg string) {
	writeJSON(w, status, envelope{OK: false, Error: &errVal{Code: code, Message: msg}})
}

var (
	startTime     time.Time
	startTimeOnce sync.Once
)

func initStartTime() {
	startTimeOnce.Do(func() {
		startTime = time.Now()
	})
}
