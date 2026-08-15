// Package health exposes the /api/v1/healthz endpoint.
package health

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	OK bool `json:"ok"`
}

func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{OK: true})
}
