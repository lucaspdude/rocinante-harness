package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RefreshToken is the in-memory representation of a refresh token.
type RefreshToken struct {
	ID        string
	FamilyID  string
	DeviceID  string
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

// GenerateRefreshToken returns a fresh 32-byte opaque token.
func GenerateRefreshToken() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	return b, nil
}

// HashRefreshToken returns the SHA-256 of the opaque token bytes.
func HashRefreshToken(token []byte) []byte {
	h := sha256.Sum256(token)
	return h[:]
}

// RefreshStore manages refresh tokens in SQLite.
type RefreshStore struct {
	db *sql.DB
}

// NewRefreshStore wraps a database.
func NewRefreshStore(db *sql.DB) *RefreshStore { return &RefreshStore{db: db} }

// Issue creates a new refresh token. The tokenHash is what is
// stored; the caller hands the raw token to the client.
func (s *RefreshStore) Issue(id, familyID, deviceID string, tokenHash []byte, ttl time.Duration) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO refresh_tokens(id, family_id, device_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, familyID, deviceID, tokenHash, now.Add(ttl).Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("issue: %w", err)
	}
	return nil
}

// Lookup returns the token by id, or nil if missing.
func (s *RefreshStore) Lookup(id string) (*RefreshToken, error) {
	row := s.db.QueryRow(`SELECT id, family_id, device_id, token_hash, expires_at, created_at, used_at, revoked_at FROM refresh_tokens WHERE id = ?`, id)
	var t RefreshToken
	var exp, ca string
	var usedAt, revokedAt sql.NullString
	if err := row.Scan(&t.ID, &t.FamilyID, &t.DeviceID, &t.TokenHash, &exp, &ca, &usedAt, &revokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup: %w", err)
	}
	t.ExpiresAt, _ = time.Parse(time.RFC3339, exp)
	t.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	if usedAt.Valid {
		v, _ := time.Parse(time.RFC3339, usedAt.String)
		t.UsedAt = &v
	}
	if revokedAt.Valid {
		v, _ := time.Parse(time.RFC3339, revokedAt.String)
		t.RevokedAt = &v
	}
	return &t, nil
}

// LookupByHash returns the token by SHA-256 hash, or nil if missing.
func (s *RefreshStore) LookupByHash(hash []byte) (*RefreshToken, error) {
	row := s.db.QueryRow(`SELECT id, family_id, device_id, token_hash, expires_at, created_at, used_at, revoked_at FROM refresh_tokens WHERE token_hash = ?`, hash)
	var t RefreshToken
	var exp, ca string
	var usedAt, revokedAt sql.NullString
	if err := row.Scan(&t.ID, &t.FamilyID, &t.DeviceID, &t.TokenHash, &exp, &ca, &usedAt, &revokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup-by-hash: %w", err)
	}
	t.ExpiresAt, _ = time.Parse(time.RFC3339, exp)
	t.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	if usedAt.Valid {
		v, _ := time.Parse(time.RFC3339, usedAt.String)
		t.UsedAt = &v
	}
	if revokedAt.Valid {
		v, _ := time.Parse(time.RFC3339, revokedAt.String)
		t.RevokedAt = &v
	}
	return &t, nil
}

// MarkUsed sets used_at to now.
func (s *RefreshStore) MarkUsed(id string) error {
	_, err := s.db.Exec(`UPDATE refresh_tokens SET used_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// RevokeFamily sets revoked_at to now for every token in the family.
func (s *RefreshStore) RevokeFamily(familyID string) error {
	_, err := s.db.Exec(`UPDATE refresh_tokens SET revoked_at = ? WHERE family_id = ? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), familyID)
	return err
}

// ErrRefreshReuse is returned when a used token is presented again.
var ErrRefreshReuse = errors.New("refresh token reuse detected")
