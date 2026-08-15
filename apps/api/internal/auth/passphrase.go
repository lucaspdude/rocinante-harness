// Package auth owns the passphrase-wrapped Ed25519 key and the
// auth middleware.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/nacl/secretbox"
)

// KDFParams controls the argon2id derivation.
type KDFParams struct {
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
	SaltLen   int    `json:"-"` // filled from layout
}

// DefaultKDFParams is the safe default for cloud installs.
var DefaultKDFParams = KDFParams{
	Time:      2,
	MemoryKiB: 65536,
	Threads:   1,
}

// HighMemoryKDFParams is used on machines with >= 8GB total RAM.
var HighMemoryKDFParams = KDFParams{
	Time:      2,
	MemoryKiB: 131072,
	Threads:   1,
}

// SaltLen is the size of the salt in bytes.
const SaltLen = 16

// NonceLen is the size of the secretbox nonce.
const NonceLen = 24

// KeyLen is the size of the symmetric key used by secretbox.
const KeyLen = 32

// DeriveKey runs argon2id over the passphrase and returns the
// symmetric key.
func DeriveKey(passphrase string, salt []byte, p KDFParams) [KeyLen]byte {
	var key [KeyLen]byte
	// Pre-mix the passphrase into a deterministic []byte (UTF-8).
	derived := argon2.IDKey([]byte(passphrase), salt, p.Time, p.MemoryKiB, p.Threads, KeyLen)
	copy(key[:], derived)
	return key
}

// Wrap encrypts plaintext under passphrase+salt+nonce and returns
// the ciphertext (the secretbox box). The nonce is stored
// alongside the ciphertext in the envelope and is *not* reused
// across Wrap calls.
func Wrap(plaintext []byte, passphrase string, salt []byte, nonce []byte, p KDFParams) ([]byte, error) {
	if len(salt) != SaltLen {
		return nil, fmt.Errorf("salt length: got %d, want %d", len(salt), SaltLen)
	}
	if len(nonce) != NonceLen {
		return nil, fmt.Errorf("nonce length: got %d, want %d", len(nonce), NonceLen)
	}
	key := DeriveKey(passphrase, salt, p)
	var n [NonceLen]byte
	copy(n[:], nonce)
	out := make([]byte, len(plaintext)+secretbox.Overhead)
	secretbox.Seal(out[:0], plaintext, &n, &key)
	return out, nil
}

// Unwrap decrypts ciphertext under passphrase+salt+nonce. Returns
// an error if the secretbox authenticator fails (wrong passphrase
// or tampered ciphertext).
func Unwrap(ciphertext []byte, passphrase string, salt []byte, nonce []byte, p KDFParams) ([]byte, error) {
	if len(salt) != SaltLen {
		return nil, fmt.Errorf("salt length: got %d, want %d", len(salt), SaltLen)
	}
	if len(nonce) != NonceLen {
		return nil, fmt.Errorf("nonce length: got %d, want %d", len(nonce), NonceLen)
	}
	if len(ciphertext) < secretbox.Overhead {
		return nil, errors.New("ciphertext too short")
	}
	key := DeriveKey(passphrase, salt, p)
	var n [NonceLen]byte
	copy(n[:], nonce)
	out := make([]byte, len(ciphertext)-secretbox.Overhead)
	ok := false
	out, ok = secretbox.Open(out[:0], ciphertext, &n, &key)
	if !ok {
		return nil, errors.New("decrypt failed: wrong passphrase or tampered ciphertext")
	}
	return out, nil
}

// GenerateSalt returns a fresh random salt.
func GenerateSalt() ([]byte, error) {
	buf := make([]byte, SaltLen)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}
	return buf, nil
}

// GenerateNonce returns a fresh random nonce.
func GenerateNonce() ([]byte, error) {
	buf := make([]byte, NonceLen)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return buf, nil
}

// EncodeBase64 is a small helper for envelope encoding.
func EncodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// DecodeBase64 decodes a base64 string used inside the envelope.
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
