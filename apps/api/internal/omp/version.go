package omp

import (
	"encoding/json"
	"net/http"
)

// MetaResponse is the JSON body of /api/v1/meta.
type MetaResponse struct {
	APIVersion      string `json:"api_version"`
	OmpVersion      string `json:"omp_version"`
	ProtocolVersion int    `json:"protocol_version"`
	OmpBin          string `json:"omp_bin"`
}

// NewMetaHandler returns the http.HandlerFunc for /api/v1/meta.
// On success (omp_bin resolved), it returns 200. When the omp
// binary cannot be resolved, it returns 503 with a stable error
// code.
func NewMetaHandler(loader Loader, apiVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bin := loader.OmpBin()
		if bin == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"code":    "omp_not_found",
				"message": "omp binary not found on PATH or $OMP_BIN",
			})
			return
		}
		protocol, version := loader.OmpVersion()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(MetaResponse{
			APIVersion:      apiVersion,
			OmpVersion:      version,
			ProtocolVersion: protocol,
			OmpBin:          bin,
		})
	}
}
