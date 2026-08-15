package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
)

// EnvelopeV1 is the on-disk format of the passphrase-wrapped key.
type EnvelopeV1 struct {
	Version    int          `json:"version"`
	KDF        string       `json:"kdf"`
	KDFParams  *EnvelopeKDF `json:"kdf_params,omitempty"`
	Salt       string       `json:"salt,omitempty"`
	Nonce      string       `json:"nonce,omitempty"`
	Ciphertext string       `json:"ciphertext,omitempty"`
	PK         string       `json:"pk"`
	SK         string       `json:"sk,omitempty"`
}

// EnvelopeKDF mirrors KDFParams for JSON round-trip.
type EnvelopeKDF struct {
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
}

// NewKeyPair generates a fresh Ed25519 keypair.
func NewKeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("ed25519: %w", err)
	}
	return sk, pk, nil
}

// SaveKeyFileEncrypted writes an encrypted envelope to path.
func SaveKeyFileEncrypted(path string, sk ed25519.PrivateKey, pk ed25519.PublicKey, passphrase string, p KDFParams) error {
	salt, err := GenerateSalt()
	if err != nil {
		return err
	}
	nonce, err := GenerateNonce()
	if err != nil {
		return err
	}
	cipher, err := Wrap(sk, passphrase, salt, nonce, p)
	if err != nil {
		return err
	}
	env := EnvelopeV1{
		Version: 1,
		KDF:     "argon2id",
		KDFParams: &EnvelopeKDF{
			Time:      p.Time,
			MemoryKiB: p.MemoryKiB,
			Threads:   p.Threads,
		},
		Salt:       EncodeBase64(salt),
		Nonce:      EncodeBase64(nonce),
		Ciphertext: EncodeBase64(cipher),
		PK:         EncodeBase64(pk),
	}
	body, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// SaveKeyFilePlaintext writes a plaintext envelope (dev only).
func SaveKeyFilePlaintext(path string, sk ed25519.PrivateKey, pk ed25519.PublicKey) error {
	env := EnvelopeV1{
		Version: 1,
		KDF:     "none",
		PK:      EncodeBase64(pk),
		SK:      EncodeBase64(sk),
	}
	body, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// LoadKeyFile reads an envelope and returns the signing key. If
// passphrase is empty, the envelope must be the plaintext kind.
func LoadKeyFile(path string, passphrase string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read: %w", err)
	}
	var env EnvelopeV1
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, fmt.Errorf("unmarshal: %w", err)
	}
	if env.Version != 1 {
		return nil, nil, fmt.Errorf("unsupported envelope version: %d", env.Version)
	}
	pk, err := DecodeBase64(env.PK)
	if err != nil {
		return nil, nil, fmt.Errorf("pk b64: %w", err)
	}
	if env.KDF == "none" {
		sk, err := DecodeBase64(env.SK)
		if err != nil {
			return nil, nil, fmt.Errorf("sk b64: %w", err)
		}
		return ed25519.PrivateKey(sk), ed25519.PublicKey(pk), nil
	}
	if env.KDF != "argon2id" {
		return nil, nil, fmt.Errorf("unsupported kdf: %s", env.KDF)
	}
	if passphrase == "" {
		return nil, nil, errors.New("passphrase required")
	}
	params := DefaultKDFParams
	if env.KDFParams != nil {
		params = KDFParams{
			Time:      env.KDFParams.Time,
			MemoryKiB: env.KDFParams.MemoryKiB,
			Threads:   env.KDFParams.Threads,
		}
	}
	salt, err := DecodeBase64(env.Salt)
	if err != nil {
		return nil, nil, fmt.Errorf("salt b64: %w", err)
	}
	nonce, err := DecodeBase64(env.Nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("nonce b64: %w", err)
	}
	cipher, err := DecodeBase64(env.Ciphertext)
	if err != nil {
		return nil, nil, fmt.Errorf("cipher b64: %w", err)
	}
	sk, err := Unwrap(cipher, passphrase, salt, nonce, params)
	if err != nil {
		return nil, nil, fmt.Errorf("unwrap: %w", err)
	}
	return ed25519.PrivateKey(sk), ed25519.PublicKey(pk), nil
}

// PublicKeyID returns the hex fingerprint of the public key.
func PublicKeyID(pk ed25519.PublicKey) string {
	// Simple fingerprint: first 8 bytes hex.
	return EncodeBase64(pk[:8])
}

var _ = argon2.IDKey
