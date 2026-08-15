package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Key is an SSH key pair stored in the api.
type Key struct {
	ID          string
	Label       string
	Provider    string
	Fingerprint string
	PublicKey   string
	CreatedAt   time.Time
}

// Server is a stored SSH connection target.
type Server struct {
	ID        string
	Label     string
	Host      string
	Port      int
	Username  string
	KeyID     string
	HostKeyFP string
	CreatedAt time.Time
}

// KeyStore is the SQLite-backed key store.
type KeyStore struct{ db *sql.DB }

// NewKeyStore wraps a database.
func NewKeyStore(db *sql.DB) *KeyStore { return &KeyStore{db: db} }

// Generate creates a new Ed25519 SSH key pair and stores the
// metadata. The private key is returned to the caller; P14 will
// wrap it with the master passphrase.
func (s *KeyStore) Generate(label, provider string) (Key, ed25519.PrivateKey, error) {
	if label == "" {
		return Key{}, nil, errors.New("label required")
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Key{}, nil, fmt.Errorf("ed25519: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return Key{}, nil, fmt.Errorf("ssh signer: %w", err)
	}
	authorized := ssh.MarshalAuthorizedKey(signer.PublicKey())
	id := randomID()
	now := time.Now().UTC()
	fp := fingerprintSHA256(signer.PublicKey())
	_, err = s.db.Exec(`INSERT INTO ssh_keys(id, label, provider, public_key, fingerprint, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, label, provider, string(authorized), fp, now.Format(time.RFC3339))
	if err != nil {
		return Key{}, nil, err
	}
	return Key{
		ID:          id,
		Label:       label,
		Provider:    provider,
		Fingerprint: fp,
		PublicKey:   strings.TrimSpace(string(authorized)),
		CreatedAt:   now,
	}, priv, nil
}

// List returns every key.
func (s *KeyStore) List() ([]Key, error) {
	rows, err := s.db.Query(`SELECT id, label, provider, public_key, fingerprint, created_at FROM ssh_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		var ca string
		if err := rows.Scan(&k.ID, &k.Label, &k.Provider, &k.PublicKey, &k.Fingerprint, &ca); err != nil {
			return nil, err
		}
		k.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		out = append(out, k)
	}
	return out, rows.Err()
}

// Delete removes the key by id.
func (s *KeyStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM ssh_keys WHERE id = ?`, id)
	return err
}

// ServerStore is the SSH server target store.
type ServerStore struct{ db *sql.DB }

// NewServerStore wraps a database.
func NewServerStore(db *sql.DB) *ServerStore { return &ServerStore{db: db} }

// Create inserts a server.
func (s *ServerStore) Create(label, host string, port int, username, keyID string) (Server, error) {
	if host == "" || username == "" || keyID == "" {
		return Server{}, errors.New("host, username, keyID required")
	}
	if port == 0 {
		port = 22
	}
	id := randomID()
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO ssh_servers(id, label, host, port, username, key_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, label, host, port, username, keyID, now.Format(time.RFC3339))
	if err != nil {
		return Server{}, err
	}
	return Server{ID: id, Label: label, Host: host, Port: port, Username: username, KeyID: keyID, CreatedAt: now}, nil
}

// List returns every server.
func (s *ServerStore) List() ([]Server, error) {
	rows, err := s.db.Query(`SELECT id, label, host, port, username, key_id, created_at FROM ssh_servers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		var s Server
		var ca string
		if err := rows.Scan(&s.ID, &s.Label, &s.Host, &s.Port, &s.Username, &s.KeyID, &ca); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Delete removes a server by id.
func (s *ServerStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM ssh_servers WHERE id = ?`, id)
	return err
}

// Get returns the server by id.
func (s *ServerStore) Get(id string) (Server, error) {
	row := s.db.QueryRow(`SELECT id, label, host, port, username, key_id, created_at FROM ssh_servers WHERE id = ?`, id)
	var srv Server
	var ca string
	if err := row.Scan(&srv.ID, &srv.Label, &srv.Host, &srv.Port, &srv.Username, &srv.KeyID, &ca); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Server{}, errors.New("server_not_found")
		}
		return Server{}, err
	}
	srv.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	return srv, nil
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func fingerprintSHA256(pub ssh.PublicKey) string {
	sum := sha256.Sum256(pub.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}
