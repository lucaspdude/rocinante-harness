package auth

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadKeyFileEncrypted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ed25519")
	sk, pk, err := NewKeyPair()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	if err := SaveKeyFileEncrypted(path, sk, pk, "open-sesame", DefaultKDFParams); err != nil {
		t.Fatalf("save: %v", err)
	}
	sk2, pk2, err := LoadKeyFile(path, "open-sesame")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !sk.Equal(sk2) {
		t.Errorf("sk mismatch")
	}
	if !pk.Equal(pk2) {
		t.Errorf("pk mismatch")
	}
}

func TestLoadKeyFileWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ed25519")
	sk, pk, _ := NewKeyPair()
	_ = SaveKeyFileEncrypted(path, sk, pk, "right", DefaultKDFParams)
	if _, _, err := LoadKeyFile(path, "wrong"); err == nil {
		t.Errorf("expected error for wrong passphrase")
	}
}

func TestSaveLoadKeyFilePlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ed25519")
	sk, pk, err := NewKeyPair()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	if err := SaveKeyFilePlaintext(path, sk, pk); err != nil {
		t.Fatalf("save: %v", err)
	}
	sk2, pk2, err := LoadKeyFile(path, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !sk.Equal(sk2) {
		t.Errorf("sk mismatch")
	}
	if !pk.Equal(pk2) {
		t.Errorf("pk mismatch")
	}
	body, _ := os.ReadFile(path)
	var env EnvelopeV1
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.KDF != "none" {
		t.Errorf("kdf = %q, want none", env.KDF)
	}
	if env.SK == "" {
		t.Errorf("sk empty")
	}
}

func TestPublicKeyID(t *testing.T) {
	pk, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	id := PublicKeyID(pk)
	if len(id) == 0 {
		t.Errorf("empty public key id")
	}
}
