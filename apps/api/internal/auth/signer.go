package auth

import (
	"crypto/ed25519"
	"errors"
	"time"
)

// Signer holds the private key and access/refresh TTLs.
type Signer struct {
	sk                 ed25519.PrivateKey
	pk                 ed25519.PublicKey
	publicKeyID        string
	AccessTTL          time.Duration
	RefreshTTL         time.Duration
	expectedPassphrase string
}

// NewSigner wraps a key pair with the standard TTLs.
func NewSigner(sk ed25519.PrivateKey, pk ed25519.PublicKey, passphrase string) *Signer {
	return &Signer{
		sk:                 sk,
		pk:                 pk,
		publicKeyID:        PublicKeyID(pk),
		AccessTTL:          1 * time.Hour,
		RefreshTTL:         30 * 24 * time.Hour,
		expectedPassphrase: passphrase,
	}
}

// PublicKey returns the public key for middleware verification.
func (s *Signer) PublicKey() ed25519.PublicKey { return s.pk }

// PublicKeyID returns the fingerprint of the public key.
func (s *Signer) PublicKeyID() string { return s.publicKeyID }

// IssueAccess mints a new access token for the device.
func (s *Signer) IssueAccess(deviceID string) (string, error) {
	return IssueAccessToken(s.sk, deviceID, s.AccessTTL)
}

// ErrPassphraseMismatch is returned when the login passphrase does
// not match the one used to unwrap the key.
var ErrPassphraseMismatch = errors.New("passphrase mismatch")

// VerifyPassphrase returns ErrPassphraseMismatch when the
// passphrase does not match the one used at startup.
func (s *Signer) VerifyPassphrase(passphrase string) error {
	if passphrase == "" || passphrase != s.expectedPassphrase {
		return ErrPassphraseMismatch
	}
	return nil
}

// StaticKeyLoader is the seam used by the middleware to expose the
// public key without exposing the signing key.
type StaticKeyLoader struct {
	Pk ed25519.PublicKey
}

// PublicKey implements PublicKeyLoader.
func (s *StaticKeyLoader) PublicKey() ed25519.PublicKey { return s.Pk }
