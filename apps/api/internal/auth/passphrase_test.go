package auth

import (
	"bytes"
	"testing"
)

func TestWrapUnwrapRoundTrip(t *testing.T) {
	p := DefaultKDFParams
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("salt: %v", err)
	}
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	plaintext := []byte("the secret key bytes")
	cipher, err := Wrap(plaintext, "passphrase", salt, nonce, p)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	out, err := Unwrap(cipher, "passphrase", salt, nonce, p)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(out, plaintext) {
		t.Errorf("got %q, want %q", out, plaintext)
	}
}

func TestUnwrapWrongPassphrase(t *testing.T) {
	p := DefaultKDFParams
	salt, _ := GenerateSalt()
	nonce, _ := GenerateNonce()
	cipher, err := Wrap([]byte("secret"), "right", salt, nonce, p)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if _, err := Unwrap(cipher, "wrong", salt, nonce, p); err == nil {
		t.Errorf("expected error for wrong passphrase")
	}
}

func TestUnwrapTamperedCiphertext(t *testing.T) {
	p := DefaultKDFParams
	salt, _ := GenerateSalt()
	nonce, _ := GenerateNonce()
	cipher, err := Wrap([]byte("secret"), "pass", salt, nonce, p)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	cipher[0] ^= 0x01
	if _, err := Unwrap(cipher, "pass", salt, nonce, p); err == nil {
		t.Errorf("expected error for tampered ciphertext")
	}
}

func TestWrapDifferentSaltDifferentCiphertext(t *testing.T) {
	p := DefaultKDFParams
	saltA, _ := GenerateSalt()
	saltB, _ := GenerateSalt()
	nonce, _ := GenerateNonce()
	a, _ := Wrap([]byte("secret"), "pass", saltA, nonce, p)
	b, _ := Wrap([]byte("secret"), "pass", saltB, nonce, p)
	if bytes.Equal(a, b) {
		t.Errorf("different salts should produce different ciphertext")
	}
}

func TestSaltLen(t *testing.T) {
	s, err := GenerateSalt()
	if err != nil {
		t.Fatalf("salt: %v", err)
	}
	if len(s) != SaltLen {
		t.Errorf("salt length = %d, want %d", len(s), SaltLen)
	}
}

func TestNonceLen(t *testing.T) {
	n, err := GenerateNonce()
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	if len(n) != NonceLen {
		t.Errorf("nonce length = %d, want %d", len(n), NonceLen)
	}
}
