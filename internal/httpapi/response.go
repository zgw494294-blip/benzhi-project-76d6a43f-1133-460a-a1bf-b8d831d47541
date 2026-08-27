package httpapi

import (
	"net/http"
	"time"
)

type Envelope struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	At        string `json:"at"`
}

func respond(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, Envelope{Data: data, RequestID: w.Header().Get("X-Request-ID"), At: time.Now().UTC().Format(time.RFC3339Nano)})
}
func respondError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, Envelope{Error: msg, RequestID: w.Header().Get("X-Request-ID"), At: time.Now().UTC().Format(time.RFC3339Nano)})
}
