CREATE TABLE IF NOT EXISTS schema_version(version INTEGER PRIMARY KEY);

CREATE TABLE devices (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  public_key_id TEXT NOT NULL,
  created_at    TEXT NOT NULL,
  last_seen_at  TEXT NOT NULL,
  revoked_at    TEXT
);

CREATE TABLE refresh_tokens (
  id              TEXT PRIMARY KEY,
  family_id       TEXT NOT NULL,
  device_id       TEXT NOT NULL REFERENCES devices(id),
  token_hash      BLOB NOT NULL,
  expires_at      TEXT NOT NULL,
  created_at      TEXT NOT NULL,
  used_at         TEXT,
  revoked_at      TEXT
);

CREATE INDEX family_idx ON refresh_tokens (family_id);
CREATE INDEX device_idx ON refresh_tokens (device_id);

CREATE TABLE audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  ts          TEXT NOT NULL,
  device_id   TEXT,
  event       TEXT NOT NULL,
  detail_json TEXT
);

CREATE TABLE pairing_codes (
  code        TEXT PRIMARY KEY,
  issuer_device_id TEXT NOT NULL REFERENCES devices(id),
  expires_at  TEXT NOT NULL,
  used_at     TEXT,
  created_at  TEXT NOT NULL
);
