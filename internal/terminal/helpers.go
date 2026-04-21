package terminal

import (
	"encoding/json"
	"io"
	"net/http"
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

type responseEnvelope = envelope

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func jsonEncode(w http.ResponseWriter, v any) {
	json.NewEncoder(w).Encode(v)
}
