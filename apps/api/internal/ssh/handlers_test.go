package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func TestMarshalED25519OpenSSH(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	out := marshalED25519OpenSSH(priv)
	if len(out) == 0 {
		t.Fatalf("marshal returned empty")
	}
	if !strings.Contains(string(out), "OPENSSH PRIVATE KEY") {
		t.Errorf("missing PEM header:\n%s", string(out))
	}
}
