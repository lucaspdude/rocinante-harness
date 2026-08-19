package ssh

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/ssh"
)

// Handler bundles the routes. Home is the absolute path to the
// user's $HOME directory (used to resolve ~/.ssh); it falls back
// to os.UserHomeDir() at runtime if left empty so existing tests
// that build a Handler literal without Home keep working.
type Handler struct {
	Keys    *KeyStore
	Servers *ServerStore
	AuthMW  func(http.Handler) http.Handler
	Home    string
}

// Routes returns the handlers wired with the auth middleware.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(h.AuthMW)
	r.Get("/api/v1/ssh/keys", h.listKeys)
	r.Post("/api/v1/ssh/keys", h.createKey)
	r.Delete("/api/v1/ssh/keys/{id}", h.deleteKey)
	r.Get("/api/v1/ssh/servers", h.listServers)
	r.Post("/api/v1/ssh/servers", h.createServer)
	r.Delete("/api/v1/ssh/servers/{id}", h.deleteServer)
	r.Post("/api/v1/ssh/servers/{id}/test", h.testServer)
	return r
}

type keyJSON struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Provider    string `json:"provider"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
	CreatedAt   string `json:"created_at"`
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type createKeyRequest struct {
	Label    string `json:"label"`
	Provider string `json:"provider"`
}

type createKeyResponse struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Provider    string `json:"provider"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
	PrivateKey  string `json:"private_key"`
	CreatedAt   string `json:"created_at"`
}

type serverJSON struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	KeyID     string `json:"key_id"`
	CreatedAt string `json:"created_at"`
}

type createServerRequest struct {
	Label    string `json:"label"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	KeyID    string `json:"key_id"`
}

func (h *Handler) listKeys(w http.ResponseWriter, _ *http.Request) {
	keys, err := h.Keys.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorJSON{Code: "internal", Message: err.Error()})
		return
	}
	out := []keyJSON{}
	for _, k := range keys {
		out = append(out, keyJSON{
			ID:          k.ID,
			Label:       k.Label,
			Provider:    k.Provider,
			Fingerprint: k.Fingerprint,
			PublicKey:   k.PublicKey,
			CreatedAt:   k.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out})
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Code: "bad_request", Message: err.Error()})
		return
	}
	key, priv, err := h.Keys.Generate(req.Label, req.Provider)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Code: "bad_request", Message: err.Error()})
		return
	}
	privPEM := marshalED25519OpenSSH(priv)
	// PR-04: also write the private key file (chmod 0o600) and
	// append a matching `Host <alias>` block to ~/.ssh/config so
	// the user does not have to ssh-keygen + edit config by hand.
	// These side effects only run when sshDirFor returns a valid
	// path (i.e. the host has a writable $HOME) — otherwise we
	// return the metadata + private key as before so callers can
	// still import the key manually.
	sshDir, sshErr := sshDirFor(h.Home)
	if sshErr == nil {
		identity := identityPath(sshDir, key.Label)
		if err := WritePrivateKey(identity, privPEM); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorJSON{Code: "write_failed", Message: err.Error()})
			return
		}
		if block, ok := providerConfigBlock(req.Provider, identity); ok {
			if err := AppendConfigBlock(block); err != nil {
				writeJSON(w, http.StatusInternalServerError, errorJSON{Code: "config_write_failed", Message: err.Error()})
				return
			}
		}
	}
	writeJSON(w, http.StatusCreated, createKeyResponse{
		ID:          key.ID,
		Label:       key.Label,
		Provider:    key.Provider,
		Fingerprint: key.Fingerprint,
		PublicKey:   key.PublicKey,
		PrivateKey:  string(privPEM),
		CreatedAt:   key.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Keys.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorJSON{Code: "internal", Message: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listServers(w http.ResponseWriter, _ *http.Request) {
	servers, err := h.Servers.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorJSON{Code: "internal", Message: err.Error()})
		return
	}
	out := []serverJSON{}
	for _, s := range servers {
		out = append(out, serverJSON{
			ID:        s.ID,
			Label:     s.Label,
			Host:      s.Host,
			Port:      s.Port,
			Username:  s.Username,
			KeyID:     s.KeyID,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": out})
}

func (h *Handler) createServer(w http.ResponseWriter, r *http.Request) {
	var req createServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Code: "bad_request", Message: err.Error()})
		return
	}
	srv, err := h.Servers.Create(req.Label, req.Host, req.Port, req.Username, req.KeyID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Code: "bad_request", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, serverJSON{
		ID:        srv.ID,
		Label:     srv.Label,
		Host:      srv.Host,
		Port:      srv.Port,
		Username:  srv.Username,
		KeyID:     srv.KeyID,
		CreatedAt: srv.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) deleteServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Servers.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorJSON{Code: "internal", Message: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testServer tries to dial the host:port and returns the result.
// Actual SSH handshake is out of scope for the MVP — the goal is
// to surface DNS / port / reachability errors to the user.
func (h *Handler) testServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.Servers.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorJSON{Code: "server_not_found", Message: err.Error()})
		return
	}
	addr := net.JoinHostPort(srv.Host, strconv.Itoa(srv.Port))
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(r.Context(), "tcp", addr)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorJSON{Code: "connect_failed", Message: err.Error()})
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]any{"status": "tcp_reachable"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// sshDirFor resolves ~/.ssh for the current handler. If Home was
// injected at construction we honour it; otherwise we fall back to
// os.UserHomeDir(). On any failure the returned error is non-nil
// and createKey must skip the file-system side effects (returning
// only the key metadata + private key PEM to the caller).
func sshDirFor(handlerHome string) (string, error) {
	home := handlerHome
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = h
	}
	if home == "" {
		return "", errors.New("home dir is empty")
	}
	return filepath.Join(home, ".ssh"), nil
}

// identityPath joins sshDir with the canonical filename for a key
// label. The label is sanitized so a user-supplied label like
// "../etc/passwd" cannot escape the directory.
func identityPath(sshDir, label string) string {
	safe := sanitizeLabel(label)
	return filepath.Join(sshDir, "id_ed25519_"+safe)
}

// sanitizeLabel keeps alphanumerics, dash, underscore and dot; any
// other rune is replaced with an underscore. This is enough to
// neutralise path-traversal attempts without rejecting the common
// "github-key" or "azure.2024" shapes.
func sanitizeLabel(label string) string {
	out := make([]rune, 0, len(label))
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "key"
	}
	return string(out)
}

// providerConfigBlock maps a provider id (the same strings the web
// uses in its GitSshPanel: "github", "gitlab", "azureDevops") to
// the matching ConfigBlock. Unknown providers produce ok=false so
// createKey only writes the key file (the user can wire it up
// manually for custom hosts).
func providerConfigBlock(provider, identityFile string) (ConfigBlock, bool) {
	switch provider {
	case "github":
		return ConfigBlock{
			Aliases:               []string{"github.com"},
			HostName:              "github.com",
			User:                  "git",
			IdentityFile:          identityFile,
			IdentitiesOnly:        boolPtr(true),
			StrictHostKeyChecking: "accept-new",
			Port:                  "22",
		}, true
	case "gitlab":
		return ConfigBlock{
			Aliases:               []string{"gitlab.com"},
			HostName:              "gitlab.com",
			User:                  "git",
			IdentityFile:          identityFile,
			IdentitiesOnly:        boolPtr(true),
			StrictHostKeyChecking: "accept-new",
			Port:                  "22",
		}, true
	case "azureDevops":
		return ConfigBlock{
			Aliases:               []string{"dev.azure.com", "vs-ssh.visualstudio.com"},
			HostName:              "dev.azure.com",
			User:                  "git",
			IdentityFile:          identityFile,
			IdentitiesOnly:        boolPtr(true),
			StrictHostKeyChecking: "accept-new",
			Port:                  "22",
		}, true
	}
	return ConfigBlock{}, false
}

func boolPtr(b bool) *bool { return &b }

// marshalED25519OpenSSH encodes an ed25519 private key as the
// canonical OpenSSH-format PEM block. We implement the wire format
// directly because x/crypto/ssh.MarshalPrivateKey does not accept
// the wrapped signer we get from NewSignerFromKey (an oversight in
// the upstream library). The output is identical to
// `ssh-keygen -t ed25519`.
func marshalED25519OpenSSH(priv ed25519.PrivateKey) []byte {
	pub := priv.Public().(ed25519.PublicKey)
	privData := append([]byte{}, priv[:32]...)
	privData = append(privData, pub...)

	var buf bytes.Buffer
	buf.WriteString("openssh-key-v1\x00")
	writeOpenSSHString(&buf, "none")
	writeOpenSSHString(&buf, "none")
	writeOpenSSHString(&buf, "")
	writeOpenSSHUint32(&buf, 1)
	writeOpenSSHString(&buf, "ssh-ed25519")
	writeOpenSSHString(&buf, string(pub))
	writeOpenSSHString(&buf, "ssh-ed25519")
	writeOpenSSHString(&buf, string(privData))
	writeOpenSSHString(&buf, "rh@local")

	return pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: buf.Bytes(),
	})
}

func writeOpenSSHString(b *bytes.Buffer, s string) {
	n := uint32(len(s))
	b.WriteByte(byte(n >> 24))
	b.WriteByte(byte(n >> 16))
	b.WriteByte(byte(n >> 8))
	b.WriteByte(byte(n))
	b.WriteString(s)
}

func writeOpenSSHUint32(b *bytes.Buffer, v uint32) {
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

var _ = errors.New
var _ = ssh.MarshalAuthorizedKey
