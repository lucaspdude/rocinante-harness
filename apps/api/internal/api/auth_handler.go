package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
)

// AuthState holds the runtime dependencies for the auth endpoints.
type AuthState struct {
	Signer       *auth.Signer
	RefreshStore *auth.RefreshStore
	DeviceStore  *auth.DeviceStore
	PairingStore *auth.PairingStore
}

// LoginRequest is the body of POST /api/v1/login.
type LoginRequest struct {
	Passphrase string `json:"passphrase"`
	DeviceName string `json:"device_name"`
}

// LoginResponse is the 200 body.
type LoginResponse struct {
	DeviceID  string `json:"device_id"`
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
	ExpiresIn int    `json:"expires_in"`
}

// RefreshRequest is the body of POST /api/v1/refresh.
type RefreshRequest struct {
	Refresh string `json:"refresh"`
}

// RefreshResponse is the 200 body.
type RefreshResponse struct {
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
	ExpiresIn int    `json:"expires_in"`
}

// PairingInitRequest is the body of POST /api/v1/pairing/init.
type PairingInitRequest struct {
	DeviceName string `json:"device_name"`
}

// PairingInitResponse is the 200 body.
type PairingInitResponse struct {
	Code      string `json:"code"`
	ExpiresIn int    `json:"expires_in"`
}

// PairingRedeemRequest is the body of POST /api/v1/pairing/redeem.
type PairingRedeemRequest struct {
	Code       string `json:"code"`
	DeviceName string `json:"device_name"`
}

// PairingRedeemResponse is the 200 body.
type PairingRedeemResponse struct {
	DeviceID  string `json:"device_id"`
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
	ExpiresIn int    `json:"expires_in"`
}

// LoginHandler authenticates the user via passphrase and issues
// an access + refresh pair bound to a new device row.
func LoginHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if req.Passphrase == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "empty_passphrase", Message: "passphrase required"})
			return
		}
		if err := state.Signer.VerifyPassphrase(req.Passphrase); err != nil {
			if errors.Is(err, auth.ErrPassphraseMismatch) {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_invalid_passphrase"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		deviceName := req.DeviceName
		if deviceName == "" {
			deviceName = "unknown-device"
		}
		id, err := auth.GenerateDeviceID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		now := time.Now().UTC()
		dev := &auth.Device{
			ID:          id,
			Name:        deviceName,
			PublicKeyID: state.Signer.PublicKeyID(),
			CreatedAt:   now,
			LastSeenAt:  now,
		}
		if err := state.DeviceStore.Create(dev); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		access, refresh, err := mintTokens(state, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, LoginResponse{
			DeviceID:  id,
			Access:    access,
			Refresh:   refresh,
			ExpiresIn: int(state.Signer.AccessTTL.Seconds()),
		})
	}
}

// RefreshHandler rotates the refresh token.
func RefreshHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if req.Refresh == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "empty_refresh", Message: "refresh required"})
			return
		}
		raw, err := hex.DecodeString(req.Refresh)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_invalid_refresh"})
			return
		}
		hash := auth.HashRefreshToken(raw)
		id, err := state.RefreshStore.LookupByHash(hash)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		if id == nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_invalid_refresh"})
			return
		}
		if id.UsedAt != nil {
			_ = state.RefreshStore.RevokeFamily(id.FamilyID)
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "refresh_family_revoked"})
			return
		}
		if id.RevokedAt != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_invalid_refresh"})
			return
		}
		if time.Now().After(id.ExpiresAt) {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_invalid_refresh"})
			return
		}
		if err := state.RefreshStore.MarkUsed(id.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		access, refresh, err := mintTokensInFamily(state, id.DeviceID, id.FamilyID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, RefreshResponse{
			Access:    access,
			Refresh:   refresh,
			ExpiresIn: int(state.Signer.AccessTTL.Seconds()),
		})
	}
}

// LogoutHandler revokes the calling device's refresh tokens.
func LogoutHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := auth.DeviceIDFromContext(r.Context())
		if deviceID == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_missing"})
			return
		}
		if err := state.DeviceStore.Revoke(deviceID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// PairingInitHandler creates a pairing code (requires auth).
func PairingInitHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := auth.DeviceIDFromContext(r.Context())
		if deviceID == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_missing"})
			return
		}
		code, err := auth.GenerateCode()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		if err := state.PairingStore.Issue(code, deviceID, 5*time.Minute); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, PairingInitResponse{
			Code:      code,
			ExpiresIn: 300,
		})
	}
}

// PairingRedeemHandler redeems a pairing code without auth.
func PairingRedeemHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PairingRedeemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if req.Code == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "empty_code", Message: "code required"})
			return
		}
		pc, err := state.PairingStore.Get(req.Code)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		if pc == nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "pairing_not_found"})
			return
		}
		if err := state.PairingStore.MarkUsed(req.Code); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		name := req.DeviceName
		if name == "" {
			name = "paired-device"
		}
		id, err := auth.GenerateDeviceID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		now := time.Now().UTC()
		dev := &auth.Device{
			ID:          id,
			Name:        name,
			PublicKeyID: state.Signer.PublicKeyID(),
			CreatedAt:   now,
			LastSeenAt:  now,
		}
		if err := state.DeviceStore.Create(dev); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		access, refresh, err := mintTokens(state, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, PairingRedeemResponse{
			DeviceID:  id,
			Access:    access,
			Refresh:   refresh,
			ExpiresIn: int(state.Signer.AccessTTL.Seconds()),
		})
	}
}

// DevicesHandler lists devices for the owner (single owner in MVP).
func DevicesHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := auth.DeviceIDFromContext(r.Context())
		if deviceID == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_missing"})
			return
		}
		all, err := state.DeviceStore.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		type deviceJSON struct {
			ID         string  `json:"id"`
			Name       string  `json:"name"`
			CreatedAt  string  `json:"created_at"`
			LastSeenAt string  `json:"last_seen_at"`
			Current    bool    `json:"current"`
			RevokedAt  *string `json:"revoked_at,omitempty"`
		}
		out := []deviceJSON{}
		for _, d := range all {
			row := deviceJSON{
				ID:         d.ID,
				Name:       d.Name,
				CreatedAt:  d.CreatedAt.Format(time.RFC3339),
				LastSeenAt: d.LastSeenAt.Format(time.RFC3339),
				Current:    d.ID == deviceID,
			}
			if d.RevokedAt != nil {
				s := d.RevokedAt.Format(time.RFC3339)
				row.RevokedAt = &s
			}
			out = append(out, row)
		}
		writeJSON(w, http.StatusOK, map[string]any{"devices": out})
	}
}

// DeleteDeviceHandler revokes a device by id.
func DeleteDeviceHandler(state *AuthState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller := auth.DeviceIDFromContext(r.Context())
		if caller == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "auth_missing"})
			return
		}
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "missing_id"})
			return
		}
		if err := state.DeviceStore.Revoke(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal", Message: err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// mintTokens creates a new (access, refresh) pair bound to a fresh
// family id.
func mintTokens(state *AuthState, deviceID string) (string, string, error) {
	familyID, err := randomID()
	if err != nil {
		return "", "", err
	}
	return mintTokensInFamily(state, deviceID, familyID)
}

func mintTokensInFamily(state *AuthState, deviceID, familyID string) (string, string, error) {
	access, err := state.Signer.IssueAccess(deviceID)
	if err != nil {
		return "", "", err
	}
	raw, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	id, err := randomID()
	if err != nil {
		return "", "", err
	}
	hash := auth.HashRefreshToken(raw)
	if err := state.RefreshStore.Issue(id, familyID, deviceID, hash, state.Signer.RefreshTTL); err != nil {
		return "", "", err
	}
	return access, hex.EncodeToString(raw), nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
