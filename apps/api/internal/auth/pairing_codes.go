package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// PairingCode is the in-memory representation of a pairing code.
type PairingCode struct {
	Code           string
	IssuerDeviceID string
	ExpiresAt      time.Time
	UsedAt         *time.Time
	CreatedAt      time.Time
}

// PairingStore manages pairing codes in SQLite.
type PairingStore struct{ db *sql.DB }

// NewPairingStore wraps a database.
func NewPairingStore(db *sql.DB) *PairingStore { return &PairingStore{db: db} }

// GenerateCode returns an 8-character alphanumeric code.
func GenerateCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 31 chars (no 0/1/I/O)
	out := make([]byte, 8)
	n := big.NewInt(int64(len(alphabet)))
	for i := range out {
		v, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", fmt.Errorf("rand: %w", err)
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out), nil
}

// HashCode returns the SHA-256 of the code (used as the storage key).
func HashCode(code string) []byte {
	h := sha256.Sum256([]byte(code))
	return h[:]
}

// Issue creates a new pairing code.
func (s *PairingStore) Issue(code, issuerDeviceID string, ttl time.Duration) error {
	now := time.Now().UTC().Truncate(time.Second)
	hash := HashCode(code)
	_, err := s.db.Exec(`INSERT INTO pairing_codes(code, issuer_device_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		base64Encode(hash), issuerDeviceID, now.Add(ttl).Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("issue: %w", err)
	}
	return nil
}

// Get returns the pairing code by raw code (looked up by hash).
// Returns nil if missing or expired.
func (s *PairingStore) Get(code string) (*PairingCode, error) {
	hash := HashCode(code)
	row := s.db.QueryRow(`SELECT code, issuer_device_id, expires_at, created_at, used_at FROM pairing_codes WHERE code = ?`, base64Encode(hash))
	var pc PairingCode
	var exp, ca string
	var usedAt sql.NullString
	if err := row.Scan(&pc.Code, &pc.IssuerDeviceID, &exp, &ca, &usedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	pc.ExpiresAt, _ = time.Parse(time.RFC3339, exp)
	pc.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	if usedAt.Valid {
		v, _ := time.Parse(time.RFC3339, usedAt.String)
		pc.UsedAt = &v
	}
	if pc.ExpiresAt.Before(time.Now().UTC()) || pc.UsedAt != nil {
		return nil, nil
	}
	return &pc, nil
}

// MarkUsed sets used_at to now.
func (s *PairingStore) MarkUsed(code string) error {
	hash := HashCode(code)
	_, err := s.db.Exec(`UPDATE pairing_codes SET used_at = ? WHERE code = ?`, time.Now().UTC().Format(time.RFC3339), base64Encode(hash))
	return err
}

func base64Encode(b []byte) string {
	const tab = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := make([]byte, 0, ((len(b)+2)/3)*4)
	for i := 0; i < len(b); i += 3 {
		var v uint32
		switch len(b) - i {
		case 1:
			v = uint32(b[i]) << 16
			out = append(out, tab[(v>>18)&0x3F], tab[(v>>12)&0x3F], '=', '=')
		case 2:
			v = uint32(b[i])<<16 | uint32(b[i+1])<<8
			out = append(out, tab[(v>>18)&0x3F], tab[(v>>12)&0x3F], tab[(v>>6)&0x3F], '=')
		default:
			v = uint32(b[i])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
			out = append(out, tab[(v>>18)&0x3F], tab[(v>>12)&0x3F], tab[(v>>6)&0x3F], tab[v&0x3F])
		}
	}
	return string(out)
}
