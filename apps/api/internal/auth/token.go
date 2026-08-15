package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AccessTokenClaims is the JSON payload of the Ed25519-signed
// access token. Format matches D-FRAME-12.
type AccessTokenClaims struct {
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	JTI      string `json:"jti"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// IssueAccessToken produces a self-contained Ed25519-signed
// access token. payload is base64url(json(claims)) + "." +
// base64url(ed25519-sign(payload)). The token is verifier-only
// (no secret leak beyond the signature).
func IssueAccessToken(sk ed25519.PrivateKey, deviceID string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := AccessTokenClaims{
		Issuer:    "rh",
		Audience:  "rh-client",
		Subject:   deviceID,
		JTI:       randomJTI(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	sig := ed25519.Sign(sk, []byte(payload))
	sigPart := base64.RawURLEncoding.EncodeToString(sig)
	return payload + "." + sigPart, nil
}

// VerifyAccessToken checks the signature and expiry of a token.
func VerifyAccessToken(token string, pk ed25519.PublicKey) (*AccessTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("auth_invalid_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("auth_invalid_token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("auth_invalid_token")
	}
	if !ed25519.Verify(pk, []byte(parts[0]), sig) {
		return nil, errors.New("auth_invalid_token")
	}
	var claims AccessTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("auth_invalid_token")
	}
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return nil, errors.New("auth_token_expired")
	}
	return &claims, nil
}

func randomJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("jti-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
