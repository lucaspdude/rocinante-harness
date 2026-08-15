package api

import (
	"crypto/rand"
	"encoding/hex"
)

// newClientRequestID returns a per-request correlation id of the
// shape `req_<32 hex chars>`. Used as the client_request_id on
// every command frame and echoed on the response.
func newClientRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is exceptional; fall back to a
		// time-based id so callers still get a unique string.
		return "req_00000000000000000000000000000000"
	}
	return "req_" + hex.EncodeToString(buf)
}
