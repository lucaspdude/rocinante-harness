package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Device represents a single sign-in client.
type Device struct {
	ID          string
	Name        string
	PublicKeyID string
	CreatedAt   time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
}

// GenerateDeviceID returns a fresh 16-byte random id as hex.
func GenerateDeviceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// DeviceStore manages devices in SQLite.
type DeviceStore struct{ db *sql.DB }

// NewDeviceStore wraps a database.
func NewDeviceStore(db *sql.DB) *DeviceStore { return &DeviceStore{db: db} }

// Create inserts a new device.
func (s *DeviceStore) Create(d *Device) error {
	_, err := s.db.Exec(`INSERT INTO devices(id, name, public_key_id, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.PublicKeyID,
		d.CreatedAt.UTC().Format(time.RFC3339),
		d.LastSeenAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("device create: %w", err)
	}
	return nil
}

// Get returns the device by id, or nil if missing.
func (s *DeviceStore) Get(id string) (*Device, error) {
	row := s.db.QueryRow(`SELECT id, name, public_key_id, created_at, last_seen_at, revoked_at FROM devices WHERE id = ?`, id)
	var d Device
	var ca, ls string
	var revokedAt sql.NullString
	if err := row.Scan(&d.ID, &d.Name, &d.PublicKeyID, &ca, &ls, &revokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get: %w", err)
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	d.LastSeenAt, _ = time.Parse(time.RFC3339, ls)
	if revokedAt.Valid {
		v, _ := time.Parse(time.RFC3339, revokedAt.String)
		d.RevokedAt = &v
	}
	return &d, nil
}

// List returns every device.
func (s *DeviceStore) List() ([]*Device, error) {
	rows, err := s.db.Query(`SELECT id, name, public_key_id, created_at, last_seen_at, revoked_at FROM devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	out := []*Device{}
	for rows.Next() {
		var d Device
		var ca, ls string
		var revokedAt sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &d.PublicKeyID, &ca, &ls, &revokedAt); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		d.LastSeenAt, _ = time.Parse(time.RFC3339, ls)
		if revokedAt.Valid {
			v, _ := time.Parse(time.RFC3339, revokedAt.String)
			d.RevokedAt = &v
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// Touch updates last_seen_at.
func (s *DeviceStore) Touch(id string) error {
	_, err := s.db.Exec(`UPDATE devices SET last_seen_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// Revoke marks the device as revoked.
func (s *DeviceStore) Revoke(id string) error {
	_, err := s.db.Exec(`UPDATE devices SET revoked_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}
